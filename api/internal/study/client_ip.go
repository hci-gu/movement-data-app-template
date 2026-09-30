package study

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

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
