package httputil

import (
	"net/url"
	"os"
	"strings"
)

// IsOriginAllowed decides whether a browser Origin may receive CORS headers
// (and whether a WebSocket upgrade from that origin is acceptable).
//
// Rules, in order:
//  1. Same-origin: the Origin host[:port] equals the request host.
//  2. Explicit allowlist from ALLOWED_ORIGIN (comma separated, full origins
//     like "https://app.example.com" or bare "app.example.com:8443").
//
// Loopback addresses are not special-cased: an allowed origin must be
// same-host or listed in ALLOWED_ORIGIN. Dev setups list their origin
// explicitly (compose.dev.yml does for the Vite dev server).
func IsOriginAllowed(origin, host string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if host != "" && strings.EqualFold(u.Host, host) {
		return true
	}
	for _, entry := range strings.Split(os.Getenv("ALLOWED_ORIGIN"), ",") {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if trimmed == origin {
			return true
		}
		if strings.Contains(trimmed, "://") {
			if au, err := url.Parse(trimmed); err == nil && au.Host != "" && strings.EqualFold(au.Host, u.Host) {
				return true
			}
			continue
		}
		// Bare "host[:port]" entry — url.Parse would misread it as a scheme.
		if strings.EqualFold(trimmed, u.Host) {
			return true
		}
	}
	return false
}
