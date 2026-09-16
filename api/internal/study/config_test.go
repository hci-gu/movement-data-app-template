package study

import (
	"bytes"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestEncryptedValuesBoundToRecordAndOldKeys(t *testing.T) {
	s, _, _ := testService(t)
	sealed, err := s.Config.Seal("record:a", identity{PersonalNumber: "200001012384"})
	if err != nil {
		t.Fatal(err)
	}
	var value identity
	if err := s.Config.Open("record:b", sealed, &value); err == nil {
		t.Fatal("ciphertext can be substituted")
	}
	s.Config.EncryptionKeys["v2"] = bytes.Repeat([]byte{3}, 32)
	s.Config.ActiveKey = "v2"
	if err := s.Config.Open("record:a", sealed, &value); err != nil || value.PersonalNumber != "200001012384" {
		t.Fatal("rotation lost old evidence", err)
	}
}

func TestTrustedProxyChain(t *testing.T) {
	c := Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	for _, tc := range []struct{ peer, forwarded, want string }{{"192.0.2.1:1", "198.51.100.1", "192.0.2.1"}, {"10.0.0.1:1", "203.0.113.9, 198.51.100.1, 10.0.0.2", "198.51.100.1"}, {"[2001:db8::1]:1", "", "2001:db8::1"}} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.forwarded)
		got, err := c.endUserIP(r)
		if err != nil || got != tc.want {
			t.Fatal(got, err, tc.want)
		}
	}
}
