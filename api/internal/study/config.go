package study

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

const SessionLifetime = 2 * time.Hour

type Config struct {
	Environment               string
	AppID                     string
	IOSAppID                  string
	ReturnURL                 string
	PublicURL                 string
	CertFile, KeyFile, CAFile string
	ActiveKey                 string
	EncryptionKeys            map[string][]byte
	TrustedProxies            []netip.Prefix
}

func LoadConfig() (Config, error) {
	c := Config{
		Environment: env("BANKID_ENVIRONMENT", "disabled"),
		AppID:       os.Getenv("BANKID_APP_IDENTIFIER"),
		IOSAppID:    os.Getenv("BANKID_IOS_APP_ID"),
		ReturnURL:   os.Getenv("BANKID_RETURN_URL"),
		PublicURL:   strings.TrimRight(os.Getenv("BANKID_PUBLIC_URL"), "/"),
		CertFile:    os.Getenv("BANKID_CERT_FILE"), KeyFile: os.Getenv("BANKID_KEY_FILE"), CAFile: os.Getenv("BANKID_CA_FILE"),
		ActiveKey:      os.Getenv("STUDY_ACTIVE_ENCRYPTION_KEY"),
		EncryptionKeys: map[string][]byte{},
	}
	if c.Environment != "disabled" && c.Environment != "test" && c.Environment != "production" {
		return c, errors.New("BANKID_ENVIRONMENT must be disabled, test or production")
	}
	var keys map[string]string
	if file := os.Getenv("STUDY_SECRETS_FILE"); file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return c, fmt.Errorf("read study secrets file: %w", err)
		}
		var secretFile struct {
			ActiveKey string            `json:"activeKey"`
			Keys      map[string]string `json:"encryptionKeys"`
		}
		if err := json.Unmarshal(raw, &secretFile); err != nil {
			return c, errors.New("invalid study secrets JSON")
		}
		keys = secretFile.Keys
		if c.ActiveKey == "" {
			c.ActiveKey = secretFile.ActiveKey
		}
	}
	if raw := os.Getenv("STUDY_ENCRYPTION_KEYS"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &keys); err != nil {
			return c, errors.New("STUDY_ENCRYPTION_KEYS must be a JSON object of key IDs and Base64 keys")
		}
	}
	for id, value := range keys {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 || id == "" || strings.Contains(id, ":") {
			return c, errors.New("each encryption key needs a nonempty ID without ':' and 32 Base64-encoded bytes")
		}
		c.EncryptionKeys[id] = key
	}
	for _, raw := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return c, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS: %w", err)
		}
		c.TrustedProxies = append(c.TrustedProxies, prefix)
	}
	if c.Environment != "disabled" {
		if c.CertFile == "" || c.KeyFile == "" || c.CAFile == "" || c.AppID == "" {
			return c, errors.New("configure BANKID_CERT_FILE, BANKID_KEY_FILE, BANKID_CA_FILE and BANKID_APP_IDENTIFIER")
		}
		if err := c.ValidateSecrets(); err != nil {
			return c, err
		}
		u, err := url.Parse(c.ReturnURL)
		if err != nil || u.Fragment != "" || u.RawQuery != "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "researchsteps") || u.Host == "" {
			return c, errors.New("BANKID_RETURN_URL must be an HTTPS universal link or researchsteps://bankid/return, without query or fragment")
		}
		if u.Scheme == "researchsteps" && (u.Host != "bankid" || u.Path != "/return") {
			return c, errors.New("custom return URL must be researchsteps://bankid/return")
		}
		if c.Environment == "production" && u.Scheme != "https" {
			return c, errors.New("production requires an HTTPS universal return link")
		}
		if u.Scheme == "https" && (u.Path != "/bankid/return" || c.IOSAppID == "" || !strings.HasSuffix(c.IOSAppID, "."+c.AppID)) {
			return c, errors.New("HTTPS returns require /bankid/return and BANKID_IOS_APP_ID as Apple app ID prefix plus '.' plus BANKID_APP_IDENTIFIER")
		}
		if len(c.ReturnURL)+60 > 512 {
			return c, errors.New("BANKID_RETURN_URL is too long")
		}
		if c.PublicURL == "" && u.Scheme == "https" {
			c.PublicURL = "https://" + u.Host
		}
		if c.PublicURL != "" {
			public, err := url.Parse(c.PublicURL)
			if err != nil || public.Scheme != "https" || public.Host == "" || public.Path != "" || public.RawQuery != "" || public.Fragment != "" || public.User != nil {
				return c, errors.New("BANKID_PUBLIC_URL must be an HTTPS origin")
			}
			if len(c.PublicURL)+len("/guardian//return?nonce=")+7+43 > 512 {
				return c, errors.New("BANKID_PUBLIC_URL is too long for a guardian return URL")
			}
		}
	}
	return c, nil
}

func (c Config) ValidateSecrets() error {
	if len(c.EncryptionKeys[c.ActiveKey]) != 32 {
		return errors.New("configure STUDY_SECRETS_FILE or STUDY_ACTIVE_ENCRYPTION_KEY and STUDY_ENCRYPTION_KEYS")
	}
	return nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func randomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func hash(value string) string { s := sha256.Sum256([]byte(value)); return hex.EncodeToString(s[:]) }

// Associated data prevents ciphertext from being moved to another record/purpose.
func (c Config) Seal(purpose string, value any) (string, error) {
	block, err := aes.NewCipher(c.EncryptionKeys[c.ActiveKey])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, plain, []byte(purpose))
	return c.ActiveKey + ":" + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (c Config) Open(purpose, value string, target any) error {
	id, encoded, ok := strings.Cut(value, ":")
	if !ok {
		return errors.New("invalid encrypted value")
	}
	block, err := aes.NewCipher(c.EncryptionKeys[id])
	if err != nil {
		return errors.New("encrypted value requires an unavailable key")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < aead.NonceSize() {
		return errors.New("invalid encrypted value")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte(purpose))
	if err != nil {
		return errors.New("evidence authentication failed")
	}
	return json.Unmarshal(plain, target)
}

func (c Config) trusted(addr netip.Addr) bool {
	for _, p := range c.TrustedProxies {
		if p.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

// Walk right-to-left through X-Forwarded-For, only across explicitly trusted
// proxies. Never use a client-submitted IP or PocketBase's independent proxy settings.
func (c Config) endUserIP(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", errors.New("invalid peer address")
	}
	if !c.trusted(addr) {
		return addr.Unmap().String(), nil
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		if !c.trusted(addr) {
			break
		}
		addr, err = netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil {
			return "", errors.New("trusted proxy must supply a valid X-Forwarded-For chain")
		}
	}
	return addr.Unmap().String(), nil
}
