package ingress

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// ErrValidation marks user-input validation failures so HTTP handlers can map
// them to 400 responses instead of 500.
var ErrValidation = errors.New("validation failed")

// targetNamePattern accepts DNS-safe upstream identifiers (container or service
// names): alphanumeric first character, then alphanumerics, dots, dashes and
// underscores, 64 characters max.
var targetNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

// ValidateTargetContainer checks a user-supplied upstream identifier before it
// is interpolated into the generated Caddyfile. It is used for both explicit
// targets and the service-id fallback.
func ValidateTargetContainer(s string) error {
	clean := strings.TrimSpace(s)
	if clean == "" {
		return fmt.Errorf("%w: target container is required", ErrValidation)
	}
	if !targetNamePattern.MatchString(clean) {
		return fmt.Errorf("%w: target container %q contains invalid characters", ErrValidation, clean)
	}
	return nil
}

// ValidateHostname checks a cleaned domain hostname (scheme and port already
// stripped). Only hostname-safe characters are accepted: letters, digits, dots,
// dashes, underscores and wildcard labels.
func ValidateHostname(host string) error {
	clean := strings.TrimSpace(host)
	if clean == "" {
		return fmt.Errorf("%w: hostname is required", ErrValidation)
	}
	if len(clean) > 253 {
		return fmt.Errorf("%w: hostname exceeds 253 characters", ErrValidation)
	}
	for _, r := range clean {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_', r == '*':
		default:
			return fmt.Errorf("%w: hostname contains invalid character %q", ErrValidation, string(r))
		}
	}
	return nil
}

// ValidateContainerPort checks the upstream port range for fmt.Sprintf
// interpolation into the Caddyfile.
func ValidateContainerPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%w: container port %d is out of range (1-65535)", ErrValidation, port)
	}
	return nil
}

// caddyfileMetachars are characters that must never reach a generated Caddyfile
// unquoted: whitespace/newlines/braces end tokens and blocks, while quotes,
// backticks and hashes can retokenize or comment out the rest of a line.
const caddyfileMetachars = " \t\r\n{};\"`$#\\()"

// parseAdminBindOverride parses the CADDY_ADMIN_HOST environment override for
// the generated `admin` bind address. Accepted forms: "localhost", any specific
// IP address (IPv4 or IPv6, optionally followed by ":port", default 2019).
// Wildcard addresses (0.0.0.0 / ::) and unix sockets are rejected: the override
// only narrows the bind, never widens it to every interface.
func parseAdminBindOverride(v string) (string, bool) {
	clean := strings.TrimSpace(v)
	if clean == "" {
		return "", false
	}

	host := clean
	port := "2019"

	if strings.HasPrefix(clean, "[") {
		if h, p, err := net.SplitHostPort(clean); err == nil {
			host, port = h, p
		} else if strings.HasSuffix(clean, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(clean, "["), "]")
		}
	} else if h, p, err := net.SplitHostPort(clean); err == nil {
		host, port = h, p
	} else if net.ParseIP(clean) != nil {
		// Bare IP without port (IPv6 literals contain colons).
		host = clean
	} else if strings.Count(clean, ":") == 1 {
		parts := strings.SplitN(clean, ":", 2)
		host, port = parts[0], parts[1]
	}

	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || ip.IsUnspecified() {
			return "", false
		}
	}

	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return "", false
	}

	if strings.Contains(host, ":") { // IPv6 literal
		host = "[" + host + "]"
	}
	return fmt.Sprintf("%s:%d", host, p), true
}
