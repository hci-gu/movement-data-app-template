// Package kontoapi authenticates people through GU's KontoAPI. Credentials and
// QR secrets stay on the backend; this service does not sign consent documents.
package kontoapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Config struct{ BaseURL, TokenURL, ClientID, ClientSecret, APIKey string }
type Order struct {
	OrderRef       string          `json:"orderRef"`
	AutoStartToken string          `json:"autoStartToken"`
	Raw            json.RawMessage `json:"-"`
}
type Result struct {
	Status         string `json:"status"`
	PersonalNumber string `json:"personalNumber,omitempty"`
	Raw            string `json:"response"`
}
type Provider interface {
	Start(context.Context, string) (Order, error)
	Collect(context.Context, string) (Result, error)
	QRImage(context.Context, Order) (string, error)
}
type APIError struct{ Status int }

func (e *APIError) Error() string { return fmt.Sprintf("KontoAPI HTTP %d", e.Status) }

type Client struct {
	cfg     Config
	http    *http.Client
	mu      sync.Mutex
	token   string
	expires time.Time
	now     func() time.Time
}

func NewClient(cfg Config) (*Client, error) {
	for _, value := range []string{cfg.BaseURL, cfg.TokenURL} {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.New("KontoAPI and OAuth URLs must be HTTPS URLs without credentials, query or fragment")
		}
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.APIKey == "" {
		return nil, errors.New("configure KontoAPI client ID, client secret and API key")
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{cfg: cfg, now: time.Now, http: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func readBody(r *http.Response) ([]byte, error) {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 2<<20+1))
	if err != nil || len(raw) > 2<<20 {
		return nil, errors.New("invalid KontoAPI response size or transport")
	}
	return raw, nil
}
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Before(c.expires) {
		return c.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {c.cfg.ClientID}, "client_secret": {c.cfg.ClientSecret}}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("invalid OAuth request")
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	started := c.now()
	response, err := c.http.Do(r)
	if err != nil {
		return "", errors.New("OAuth service could not be reached")
	}
	raw, err := readBody(response)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("OAuth HTTP %d", response.StatusCode)
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if json.Unmarshal(raw, &result) != nil || result.AccessToken == "" || !strings.EqualFold(result.TokenType, "Bearer") || result.ExpiresIn <= 0 || result.ExpiresIn > 86400*30 {
		return "", errors.New("invalid OAuth token response")
	}
	margin := time.Duration(result.ExpiresIn) * time.Second / 10
	if margin > 30*time.Second {
		margin = 30 * time.Second
	}
	c.token = result.AccessToken
	c.expires = started.Add(time.Duration(result.ExpiresIn)*time.Second - margin)
	return c.token, nil
}
func (c *Client) call(ctx context.Context, method, path string, body []byte, retry bool) ([]byte, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("invalid KontoAPI request")
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("x-api-key", c.cfg.APIKey)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Cache-Control", "no-store")
	response, err := c.http.Do(r)
	if err != nil {
		return nil, errors.New("KontoAPI could not be reached")
	}
	raw, err := readBody(response)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == 401 {
		c.mu.Lock()
		if c.token == token {
			c.token = ""
		}
		c.mu.Unlock()
		if retry {
			return c.call(ctx, method, path, body, false)
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{Status: response.StatusCode}
	}
	return raw, nil
}
func (c *Client) Start(ctx context.Context, ip string) (Order, error) {
	if net.ParseIP(ip) == nil {
		return Order{}, errors.New("invalid end-user IP")
	}
	// This GET creates an order. Never retry it automatically.
	raw, err := c.call(ctx, "GET", "/api/Bankid/GetQrAndMobileAuthRef/"+url.PathEscape(ip), nil, false)
	if err != nil {
		return Order{}, err
	}
	var order Order
	if json.Unmarshal(raw, &order) != nil || order.OrderRef == "" || order.AutoStartToken == "" {
		return Order{}, errors.New("invalid KontoAPI order response")
	}
	order.Raw = append(json.RawMessage(nil), raw...)
	return order, nil
}
func responseText(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, "\"") {
		var decoded string
		if json.Unmarshal(raw, &decoded) == nil {
			return decoded
		}
	}
	return text
}
func (c *Client) Collect(ctx context.Context, ref string) (Result, error) {
	body, _ := json.Marshal(map[string]string{"orderRef": ref})
	raw, err := c.call(ctx, "POST", "/api/Bankid/GetAuthStatus?getName=true", body, true)
	if err != nil {
		return Result{}, err
	}
	value := responseText(raw)
	if value == "" {
		return Result{Status: "pending"}, nil
	}
	parts := strings.Split(value, ":")
	if len(parts) != 3 || len(parts[0]) != 12 {
		return Result{}, errors.New("unrecognized KontoAPI authentication response")
	}
	for _, digit := range parts[0] {
		if digit < '0' || digit > '9' {
			return Result{}, errors.New("invalid KontoAPI identity")
		}
	}
	return Result{Status: "complete", PersonalNumber: parts[0], Raw: value}, nil
}
func (c *Client) QRImage(ctx context.Context, order Order) (string, error) {
	raw, err := c.call(ctx, "POST", "/api/Bankid/GetQrImage", order.Raw, true)
	if err != nil {
		return "", err
	}
	value := responseText(raw)
	encoded, ok := strings.CutPrefix(value, "data:image/png;base64,")
	if !ok {
		return "", errors.New("KontoAPI QR response must be a PNG data URI")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("invalid QR image encoding")
	}
	info, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || info.Width < 1 || info.Height < 1 || info.Width > 1024 || info.Height > 1024 {
		return "", errors.New("invalid QR image")
	}
	return value, nil
}
