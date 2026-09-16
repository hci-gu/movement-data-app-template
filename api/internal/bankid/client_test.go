package bankid

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQRMatchesOfficialVectors(t *testing.T) {
	order := Order{QRStartToken: "67df3917-fa0d-44e5-b327-edcc928297f8", QRStartSecret: "d28db9a7-4cde-429e-a983-359be676944c"}
	for i, want := range []string{"dc69358e712458a66a7525beef148ae8526b1c71610eff2c16cdffb4cdac9bf8", "949d559bf23403952a94d103e67743126381eda00f0b3cbddbf7c96b1adcbce2", "a9e5ec59cb4eee4ef4117150abc58fad7a85439a6a96ccbecc3668b41795b3f3"} {
		got := QRPayload(order, time.Unix(0, 0), time.Unix(int64(i), 999))
		if !strings.HasSuffix(got, "."+want) {
			t.Fatalf("frame %d: %s", i, got)
		}
	}
}

func TestClientProtocolAndEvidencePreservation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect HTTP contract")
		}
		if r.URL.Path == "/sign" {
			var req Request
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.UserVisibleData != "Y29uc2VudA==" {
				t.Error("signed bytes changed")
			}
			_, _ = w.Write([]byte(`{"orderRef":"ref","autoStartToken":"auto","qrStartToken":"qr","qrStartSecret":"secret"}`))
			return
		}
		_, _ = w.Write([]byte(`{"orderRef":"ref","status":"complete","completionData":{"extraFutureEvidence":{"kept":true}}}`))
	}))
	defer server.Close()
	c := &Client{http: server.Client(), baseURL: server.URL}
	order, err := c.Start(context.Background(), "sign", Request{EndUserIP: "192.0.2.1", UserVisibleData: "Y29uc2VudA=="})
	if err != nil || order.OrderRef != "ref" {
		t.Fatal(order, err)
	}
	result, err := c.Collect(context.Background(), "ref")
	if err != nil || !strings.Contains(string(result.Raw), "extraFutureEvidence") {
		t.Fatal("lost completion fields", err)
	}
	if _, err := c.Collect(context.Background(), "different"); err == nil {
		t.Fatal("accepted mismatched order reference")
	}
}

func TestClientDoesNotRetryAndRedactsErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errorCode":"invalidParameters","details":"sensitive-personal-number"}`))
	}))
	defer server.Close()
	c := &Client{http: server.Client(), baseURL: server.URL}
	_, err := c.Start(context.Background(), "auth", Request{EndUserIP: "192.0.2.1"})
	if err == nil || calls != 1 || strings.Contains(err.Error(), "sensitive") {
		t.Fatal("unsafe error/retry handling", err, calls)
	}
}

func TestMutualTLSRequiresClientCertificateAndTrustedServer(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	now := time.Now()
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, _ = x509.ParseCertificate(rootDER)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	dir := t.TempDir()
	certFile, keyFile, caFile := filepath.Join(dir, "client.pem"), filepath.Join(dir, "key.pem"), filepath.Join(dir, "ca.pem")
	for path, block := range map[string]*pem.Block{certFile: {Type: "CERTIFICATE", Bytes: leafDER}, keyFile: {Type: "EC PRIVATE KEY", Bytes: keyDER}, caFile: {Type: "CERTIFICATE", Bytes: rootDER}} {
		if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pair, _ := tls.LoadX509KeyPair(certFile, keyFile)
	roots := x509.NewCertPool()
	roots.AddCert(root)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.VerifiedChains) == 0 {
			t.Error("client certificate not verified")
		}
		_, _ = w.Write([]byte(`{"orderRef":"ref","status":"pending"}`))
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	c, err := NewClient("test", certFile, keyFile, caFile)
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = server.URL // Test-local endpoint; runtime environment URLs are fixed.
	if _, err := c.Collect(context.Background(), "ref"); err != nil {
		t.Fatal("valid mutual TLS failed", err)
	}
	transport := c.http.Transport.(*http.Transport)
	transport.CloseIdleConnections()
	transport.TLSClientConfig.Certificates = nil
	if _, err := c.Collect(context.Background(), "ref"); err == nil {
		t.Fatal("missing client certificate accepted")
	}
	transport.CloseIdleConnections()
	transport.TLSClientConfig.Certificates = []tls.Certificate{pair}
	transport.TLSClientConfig.RootCAs = x509.NewCertPool()
	if _, err := c.Collect(context.Background(), "ref"); err == nil {
		t.Fatal("untrusted server accepted")
	}
}
