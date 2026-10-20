package podman

import (
	"fmt"
	"strings"
)

// NormalizeMemorySwap validates the --memory-swap value the way podman accepts
// it: omitted ("" / "0" / "default") to keep the engine default, "-1" (or
// "unlimited") for unbounded swap, otherwise a byte size optionally carrying a
// b/kb/mb/gb (or k/m/g) unit suffix. podman itself rejects anything else, so
// the value is validated here to surface a clear error instead of a cryptic
// CLI failure at deploy time.
func NormalizeMemorySwap(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "0" || s == "default" {
		return "", nil
	}
	if strings.EqualFold(s, "unlimited") {
		return "-1", nil
	}
	if s == "-1" {
		return s, nil
	}

	body := s
	for _, suffix := range []string{"kb", "mb", "gb", "b", "k", "m", "g"} {
		if len(s) > len(suffix) && strings.HasSuffix(strings.ToLower(s), suffix) {
			body = strings.TrimSpace(s[:len(s)-len(suffix)])
			break
		}
	}
	if body == "" {
		return "", fmt.Errorf("invalid memory swap value %q: missing size", s)
	}
	for _, r := range body {
		if (r < '0' || r > '9') && r != '.' {
			return "", fmt.Errorf("invalid memory swap value %q: size must be numeric, optionally with b/kb/mb/gb suffix", s)
		}
	}
	// Canonicalize away interior whitespace so "512 KB" becomes "512KB".
	return strings.ReplaceAll(s, " ", ""), nil
}
