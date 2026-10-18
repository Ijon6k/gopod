package httputil

import "testing"

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		host    string
		allowed string
		want    bool
	}{
		{name: "same host and port", origin: "http://gopod.local:8085", host: "gopod.local:8085", allowed: "", want: true},
		{name: "host match is case insensitive", origin: "http://GoPod.LOCAL:8085", host: "gopod.local:8085", allowed: "", want: true},
		{name: "different host", origin: "http://evil.example.com", host: "gopod.local:8085", allowed: "", want: false},
		{name: "different port is a different origin", origin: "http://localhost:5173", host: "localhost:8085", allowed: "", want: false},
		{name: "localhost is not special-cased", origin: "http://localhost:9999", host: "gopod.local:8085", allowed: "", want: false},
		{name: "explicit allowlist full origin", origin: "http://localhost:5173", host: "gopod.local:8085", allowed: "http://localhost:5173", want: true},
		{name: "explicit allowlist bare host", origin: "https://app.example.com:8443", host: "gopod.local:8085", allowed: "app.example.com:8443", want: true},
		{name: "allowlist with unrelated entries", origin: "http://localhost:5173", host: "gopod.local:8085", allowed: "https://other.example.com, http://localhost:5173", want: true},
		{name: "allowlist miss", origin: "http://localhost:5174", host: "gopod.local:8085", allowed: "http://localhost:5173", want: false},
		{name: "empty origin rejected", origin: "", host: "gopod.local:8085", allowed: "http://localhost:5173", want: false},
		{name: "garbage origin rejected", origin: "not a url", host: "gopod.local:8085", allowed: "http://localhost:5173", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ALLOWED_ORIGIN", tc.allowed)
			if got := IsOriginAllowed(tc.origin, tc.host); got != tc.want {
				t.Fatalf("IsOriginAllowed(%q, %q) = %v, want %v", tc.origin, tc.host, got, tc.want)
			}
		})
	}
}
