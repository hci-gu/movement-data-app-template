// Package bankid implements the Swedish BankID RP API. It never runs in the app.
package bankid

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const TestURL = "https://appapi2.test.bankid.com/rp/v6.0"
const ProductionURL = "https://appapi2.bankid.com/rp/v6.0"

type Request struct {
	EndUserIP          string       `json:"endUserIp"`
	UserVisibleData    string       `json:"userVisibleData,omitempty"`
	UserNonVisibleData string       `json:"userNonVisibleData,omitempty"`
	ReturnURL          string       `json:"returnUrl,omitempty"`
	ReturnRisk         bool         `json:"returnRisk"`
	App                *AppInfo     `json:"app,omitempty"`
	Requirement        *Requirement `json:"requirement,omitempty"`
}

type AppInfo struct {
	AppIdentifier string `json:"appIdentifier"`
}
type Requirement struct {
	PersonalNumber string `json:"personalNumber,omitempty"`
}
type Order struct {
	OrderRef       string `json:"orderRef"`
	AutoStartToken string `json:"autoStartToken"`
	QRStartToken   string `json:"qrStartToken"`
	QRStartSecret  string `json:"qrStartSecret"`
}
type User struct {
	PersonalNumber string `json:"personalNumber"`
	Name           string `json:"name"`
	GivenName      string `json:"givenName"`
	Surname        string `json:"surname"`
}
type Completion struct {
	User         User   `json:"user"`
	Signature    string `json:"signature"`
	OCSPResponse string `json:"ocspResponse"`
	Risk         string `json:"risk"`
}
type Result struct {
	OrderRef       string      `json:"orderRef"`
	Status         string      `json:"status"`
	HintCode       string      `json:"hintCode"`
	CompletionData *Completion `json:"completionData,omitempty"`
	// Preserve unknown fields and all evidence, not just the fields used by this service.
	Raw json.RawMessage `json:"-"`
}

type Provider interface {
	Start(context.Context, string, Request) (Order, error)
	Collect(context.Context, string) (Result, error)
	Cancel(context.Context, string) error
}

// APIError contains a machine-readable code only. Never log provider details,
// which can contain submitted personal information.
type APIError struct {
	Status int
	Code   string
}

func (e *APIError) Error() string { return fmt.Sprintf("BankID HTTP %d (%s)", e.Status, e.Code) }

type Client struct {
	http               *http.Client
	baseURL            string
	Fingerprint        string
	CertificateExpires time.Time
}

func NewClient(environment, certFile, keyFile, caFile string) (*Client, error) {
	base := TestURL
	if environment == "production" {
		base = ProductionURL
	} else if environment != "test" {
		return nil, errors.New("invalid BankID environment")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load BankID client certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}
	if time.Now().Before(leaf.NotBefore) || !time.Now().Before(leaf.NotAfter) {
		return nil, errors.New("BankID client certificate is not currently valid")
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("load BankID server CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("BankID server CA file has no certificates")
	}
	fingerprint := sha256.Sum256(cert.Certificate[0])
	return &Client{
		baseURL:            base,
		Fingerprint:        hex.EncodeToString(fingerprint[:]),
		CertificateExpires: leaf.NotAfter,
		http: &http.Client{
			Timeout:       12 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{
				TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, RootCAs: roots},
				TLSHandshakeTimeout:   8 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				IdleConnTimeout:       60 * time.Second,
			},
		},
	}, nil
}

func (c *Client) post(ctx context.Context, method string, body any) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("BankID connection failed; result may be unknown")
	}
	defer res.Body.Close()
	const limit = 2 << 20
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || len(data) > limit {
		return nil, errors.New("could not read bounded BankID response")
	}
	if res.StatusCode != http.StatusOK {
		var payload struct {
			Code string `json:"errorCode"`
		}
		_ = json.Unmarshal(data, &payload)
		return nil, &APIError{Status: res.StatusCode, Code: payload.Code}
	}
	return data, nil
}

func (c *Client) Start(ctx context.Context, purpose string, req Request) (Order, error) {
	var order Order
	if purpose != "auth" && purpose != "sign" {
		return order, errors.New("unsupported BankID purpose")
	}
	if req.EndUserIP == "" || (purpose == "sign" && req.UserVisibleData == "") || len(req.UserVisibleData) > 40000 || len(req.UserNonVisibleData) > 200000 || len(req.ReturnURL) > 512 {
		return order, errors.New("invalid BankID request")
	}
	raw, err := c.post(ctx, purpose, req)
	if err != nil {
		return order, err
	}
	if err = json.Unmarshal(raw, &order); err != nil {
		return order, errors.New("invalid BankID order response")
	}
	if order.OrderRef == "" || order.AutoStartToken == "" || order.QRStartToken == "" || order.QRStartSecret == "" {
		return Order{}, errors.New("incomplete BankID order response")
	}
	return order, nil
}

func (c *Client) Collect(ctx context.Context, ref string) (Result, error) {
	var result Result
	raw, err := c.post(ctx, "collect", map[string]string{"orderRef": ref})
	if err != nil {
		return result, err
	}
	// print raw response for debugging
	fmt.Println(string(raw))
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, errors.New("invalid BankID collect response")
	}
	if result.OrderRef != ref {
		return result, errors.New("BankID order reference mismatch")
	}
	result.Raw = append([]byte(nil), raw...)
	return result, nil
}

func (c *Client) Cancel(ctx context.Context, ref string) error {
	_, err := c.post(ctx, "cancel", map[string]string{"orderRef": ref})
	return err
}

// QRPayload must be called for the current frame, never to pre-generate future frames.
func QRPayload(order Order, receivedAt, now time.Time) string {
	seconds := int64(now.Sub(receivedAt).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	t := strconv.FormatInt(seconds, 10)
	mac := hmac.New(sha256.New, []byte(order.QRStartSecret))
	mac.Write([]byte(t))
	return "bankid." + order.QRStartToken + "." + t + "." + hex.EncodeToString(mac.Sum(nil))
}
