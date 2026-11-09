package kontoapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	c, err := NewClient(Config{BaseURL: server.URL, TokenURL: server.URL + "/token", ClientID: "id", ClientSecret: "secret", APIKey: "key"})
	if err != nil {
		t.Fatal(err)
	}
	c.http = server.Client()
	c.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}
func TestTokenCachingExpiryAndConcurrentCollection(t *testing.T) {
	var tokens atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_secret") != "secret" || r.Header.Get("x-api-key") != "" {
				t.Error("invalid token request")
			}
			n := tokens.Add(1)
			fmt.Fprintf(w, `{"access_token":"token%d","token_type":"bearer","expires_in":3600}`, n)
			return
		}
		if r.Header.Get("x-api-key") != "key" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer token") {
			t.Error("missing service credentials")
		}
		if r.Method != "POST" || r.URL.Path != "/api/Bankid/GetAuthStatus" || r.URL.Query().Get("getName") != "true" {
			t.Error("invalid collect request")
		}
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		if in["orderRef"] != "order" {
			t.Error("wrong order")
		}
		fmt.Fprint(w, "200001012384:Test:Person")
	})
	now := time.Now()
	c.now = func() time.Time { return now }
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := c.Collect(context.Background(), "order")
			if err != nil || r.PersonalNumber != "200001012384" {
				t.Errorf("result: %v %v", r, err)
			}
		}()
	}
	wg.Wait()
	if tokens.Load() != 1 {
		t.Fatal("parallel token refresh", tokens.Load())
	}
	now = now.Add(3590 * time.Second)
	if _, err := c.Collect(context.Background(), "order"); err != nil {
		t.Fatal(err)
	}
	if tokens.Load() != 2 {
		t.Fatal("token expiry treated incorrectly")
	}
}
func TestStartNeverRetriesAndCollectionRefreshesOnce(t *testing.T) {
	var starts, collects, tokens int
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			tokens++
			fmt.Fprint(w, `{"access_token":"token","token_type":"Bearer","expires_in":3600}`)
			return
		}
		if strings.Contains(r.URL.Path, "GetQrAndMobileAuthRef") {
			starts++
			w.WriteHeader(401)
			return
		}
		collects++
		if collects == 1 {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `""`)
	})
	if _, err := c.Start(context.Background(), "192.0.2.1"); err == nil {
		t.Fatal("unauthorized start accepted")
	}
	if starts != 1 || tokens != 1 {
		t.Fatal("start retried")
	}
	r, err := c.Collect(context.Background(), "order")
	if err != nil || r.Status != "pending" || collects != 2 || tokens != 3 {
		t.Fatalf("retry: %+v %v %d %d", r, err, collects, tokens)
	}
}
func TestOrderRetainsQRFieldsAndOnlyPNGIsAccepted(t *testing.T) {
	invalid := false
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			fmt.Fprint(w, `{"access_token":"token","token_type":"Bearer","expires_in":3600}`)
		case "/api/Bankid/GetQrAndMobileAuthRef/192.0.2.1":
			fmt.Fprint(w, `{"OrderRef":"order","AutoStartToken":"launch","StartToken":"qr-token","StartSecret":"qr-secret","StartTime":"2026-11-03T10:00:00Z"}`)
		case "/api/Bankid/GetQrImage":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["StartSecret"] != "qr-secret" || body["StartTime"] != "2026-11-03T10:00:00Z" {
				t.Error("QR fields lost")
			}
			if invalid {
				fmt.Fprint(w, `"https://example.invalid/image"`)
				return
			}
			fmt.Fprint(w, `"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII="`)
		default:
			t.Error("unexpected path", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	order, err := c.Start(context.Background(), "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if order.OrderRef != "order" || order.AutoStartToken != "launch" {
		t.Fatal("order mapping")
	}
	if _, err = c.QRImage(context.Background(), order); err != nil {
		t.Fatal(err)
	}
	invalid = true
	if _, err = c.QRImage(context.Background(), order); err == nil {
		t.Fatal("untrusted QR URL accepted")
	}
}
func TestMalformedResponsesNeverAuthenticate(t *testing.T) {
	for _, value := range []string{`null`, `error`, `200001012384`, `200001012384:Name:Surname:extra`, `abcdefghijkl:Name:Surname`} {
		t.Run(value, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					fmt.Fprint(w, `{"access_token":"token","token_type":"Bearer","expires_in":3600}`)
				} else {
					fmt.Fprint(w, value)
				}
			})
			if _, err := c.Collect(context.Background(), "order"); err == nil {
				t.Fatal("malformed identity accepted")
			}
		})
	}
}
