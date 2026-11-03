package study

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestLoopbackDevelopmentAndStrictProxyChains(t *testing.T) {
	c := Config{Environment: "test", TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}}
	r := httptest.NewRequest("GET", "/api/consent/current", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	if ip, err := c.endUserIP(r); err != nil || ip != "127.0.0.1" {
		t.Fatal(ip, err)
	}
	r.Header.Set("X-Forwarded-For", "192.0.2.1")
	if ip, err := c.endUserIP(r); err != nil || ip != "192.0.2.1" {
		t.Fatal(ip, err)
	}
	r.Header.Set("X-Forwarded-For", "malformed")
	if _, err := c.endUserIP(r); err == nil {
		t.Fatal("accepted malformed chain")
	}
	r.Header.Del("X-Forwarded-For")
	c.Environment = "production"
	if _, err := c.endUserIP(r); err == nil {
		t.Fatal("production accepted a missing proxy chain")
	}
	c.Environment = "test"
	c.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	r.RemoteAddr = "192.0.2.2:54321"
	if _, err := c.endUserIP(r); err == nil {
		t.Fatal("nonloopback proxy accepted a missing chain")
	}
}
