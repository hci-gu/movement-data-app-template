package study

import (
	"app/internal/kontoapi"
	"app/internal/testidentity"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

const SessionLifetime = 2 * time.Hour

type Config struct {
	Simulation     *testidentity.Run
	Environment    string
	AppID          string
	IOSAppID       string
	ReturnURL      string
	PublicURL      string
	KontoAPI       kontoapi.Config
	ActiveKey      string
	EncryptionKeys map[string][]byte
	TrustedProxies []netip.Prefix
}

func LoadConfig() (Config, error) {
	c := Config{
		Environment:    env("KONTOAPI_ENVIRONMENT", "disabled"),
		AppID:          os.Getenv("BANKID_APP_IDENTIFIER"),
		IOSAppID:       os.Getenv("BANKID_IOS_APP_ID"),
		ReturnURL:      os.Getenv("BANKID_RETURN_URL"),
		PublicURL:      strings.TrimRight(os.Getenv("BANKID_PUBLIC_URL"), "/"),
		KontoAPI:       kontoapi.Config{BaseURL: os.Getenv("KONTOAPI_URL"), TokenURL: os.Getenv("KONTOAPI_TOKEN_URL"), ClientID: os.Getenv("KONTOAPI_CLIENT_ID"), ClientSecret: os.Getenv("KONTOAPI_CLIENT_SECRET"), APIKey: os.Getenv("KONTOAPI_API_KEY")},
		ActiveKey:      os.Getenv("STUDY_ACTIVE_ENCRYPTION_KEY"),
		EncryptionKeys: map[string][]byte{},
	}
	if c.Environment != "disabled" && c.Environment != "test" && c.Environment != "production" {
		return c, errors.New("KONTOAPI_ENVIRONMENT must be disabled, test or production")
	}
	var keys map[string]string
	var simulationErr error
	c.Simulation, simulationErr = testidentity.Load(os.Getenv("STUDY_SIMULATION_ID"), os.Getenv("STUDY_DEPLOYMENT"))
	if simulationErr != nil {
		return c, simulationErr
	}
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
		if _, err := kontoapi.NewClient(c.KontoAPI); err != nil {
			return c, err
		}
		if err := c.ValidateSecrets(); err != nil {
			return c, err
		}
		if c.ReturnURL != "" {
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
