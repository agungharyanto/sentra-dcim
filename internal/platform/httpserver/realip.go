package httpserver

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// realIP rewrites r.RemoteAddr from X-Real-IP, but only when the direct peer is a trusted proxy.
// Caddy overwrites X-Real-IP with the real client IP, so a client cannot spoof it.
func realIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if peer, ok := peerAddr(r.RemoteAddr); ok && isTrusted(peer, trusted) {
				if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
					if ip, err := netip.ParseAddr(xr); err == nil {
						r.RemoteAddr = net.JoinHostPort(ip.Unmap().String(), "0")
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func peerAddr(remote string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func isTrusted(ip netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
