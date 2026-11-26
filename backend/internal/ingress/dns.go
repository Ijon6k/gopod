package ingress

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"
)

// DNSCheckResult describes the live DNS verification outcome for a domain rule.
type DNSCheckResult struct {
	Hostname         string   `json:"hostname"`
	Status           string   `json:"status"` // "local" | "valid" | "cloudflare" | "mismatch" | "pending"
	Valid            bool     `json:"valid"`
	BehindCloudflare bool     `json:"behindCloudflare"`
	ResolvedIPs      []string `json:"resolvedIps"`
	ResolvedIP       string   `json:"resolvedIp,omitempty"`
	ServerIP         string   `json:"serverIp,omitempty"`
	ServerIPPublic   bool     `json:"serverIpPublic"`
	CNAME            string   `json:"cname,omitempty"`
	Message          string   `json:"message"`
}

var cloudflareCIDRs = parseCIDRList([]string{
	// Cloudflare IPv4 published ranges (https://www.cloudflare.com/ips-v4/)
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
	// Cloudflare IPv6 published ranges (https://www.cloudflare.com/ips-v6/)
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
})

func parseCIDRList(raw []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(raw))
	for _, s := range raw {
		_, netBlock, err := net.ParseCIDR(s)
		if err == nil && netBlock != nil {
			out = append(out, netBlock)
		}
	}
	return out
}

// IsCloudflareIP reports whether ip belongs to a published Cloudflare edge range.
func IsCloudflareIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, block := range cloudflareCIDRs {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// IsLocalLoopbackHostname reports whether the cleaned hostname targets local development loopback.
func IsLocalLoopbackHostname(cleanHost string) bool {
	h := strings.ToLower(strings.TrimSpace(cleanHost))
	return h == "localhost" ||
		strings.HasSuffix(h, ".localhost") ||
		strings.HasSuffix(h, ".local") ||
		h == "127.0.0.1" ||
		h == "::1"
}

type dnsLookupHooks struct {
	lookupIP    func(ctx context.Context, host string) ([]net.IP, error)
	lookupCNAME func(ctx context.Context, host string) (string, error)
}

func defaultDNSLookupHooks() dnsLookupHooks {
	var resolver net.Resolver
	return dnsLookupHooks{
		lookupIP: func(ctx context.Context, host string) ([]net.IP, error) {
			return resolver.LookupIP(ctx, "ip", host)
		},
		lookupCNAME: func(ctx context.Context, host string) (string, error) {
			return resolver.LookupCNAME(ctx, host)
		},
	}
}

// CheckDomainDNS resolves rawHost and evaluates whether DNS points to this server,
// sits behind Cloudflare's reverse proxy, uses local loopback, or mismatches/is pending.
func CheckDomainDNS(ctx context.Context, rawHost, expectedServerIP string, serverIPPublic bool) (*DNSCheckResult, error) {
	return checkDomainDNSWithHooks(ctx, rawHost, expectedServerIP, serverIPPublic, defaultDNSLookupHooks())
}

func checkDomainDNSWithHooks(
	ctx context.Context,
	rawHost, expectedServerIP string,
	serverIPPublic bool,
	hooks dnsLookupHooks,
) (*DNSCheckResult, error) {
	cleanHost := cleanHostname(rawHost)
	if err := ValidateHostname(cleanHost); err != nil {
		return nil, err
	}

	res := &DNSCheckResult{
		Hostname:       cleanHost,
		ResolvedIPs:    []string{},
		ServerIP:       strings.TrimSpace(expectedServerIP),
		ServerIPPublic: serverIPPublic,
	}

	if IsLocalLoopbackHostname(cleanHost) {
		res.Status = "local"
		res.Valid = true
		res.ResolvedIPs = []string{"127.0.0.1"}
		res.ResolvedIP = "127.0.0.1"
		res.Message = "Local loopback hostname routes directly to Caddy on this host."
		return res, nil
	}

	// Handle bare public/private IPv4 literal if someone enters an IP host directly.
	if literalIP := net.ParseIP(cleanHost); literalIP != nil {
		res.ResolvedIP = literalIP.String()
		res.ResolvedIPs = []string{res.ResolvedIP}
		res.BehindCloudflare = IsCloudflareIP(literalIP)
		return finalizeDNSCheck(res, expectedServerIP, serverIPPublic), nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	ips, err := hooks.lookupIP(lookupCtx, cleanHost)
	if hooks.lookupCNAME != nil {
		if cname, cnameErr := hooks.lookupCNAME(lookupCtx, cleanHost); cnameErr == nil {
			trimmed := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(cname)), ".")
			if trimmed != "" && trimmed != cleanHost {
				res.CNAME = trimmed
			}
		}
	}

	if err != nil || len(ips) == 0 {
		res.Status = "pending"
		res.Valid = false
		if serverIPPublic && res.ServerIP != "" {
			res.Message = fmt.Sprintf("No A or AAAA record resolves yet. Point your domain to %s.", res.ServerIP)
		} else {
			res.Message = "No DNS record resolves for this domain yet."
		}
		return res, nil
	}

	seen := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		s := ip.String()
		if _, dup := seen[s]; !dup {
			seen[s] = struct{}{}
			res.ResolvedIPs = append(res.ResolvedIPs, s)
		}
		if IsCloudflareIP(ip) {
			res.BehindCloudflare = true
		}
	}
	if !res.BehindCloudflare && res.CNAME != "" &&
		(strings.HasSuffix(res.CNAME, ".cloudflare.net") || strings.HasSuffix(res.CNAME, ".cdn.cloudflare.net")) {
		res.BehindCloudflare = true
	}
	if len(res.ResolvedIPs) > 0 {
		res.ResolvedIP = res.ResolvedIPs[0]
	}

	return finalizeDNSCheck(res, expectedServerIP, serverIPPublic), nil
}

func finalizeDNSCheck(res *DNSCheckResult, expectedServerIP string, serverIPPublic bool) *DNSCheckResult {
	if res.BehindCloudflare {
		res.Status = "cloudflare"
		res.Valid = true
		res.Message = "Proxied through Cloudflare. Keep SSL/TLS mode on Full or Full (strict) in Cloudflare."
		return res
	}

	expected := strings.TrimSpace(expectedServerIP)
	matchedExpected := expected != "" && slices.Contains(res.ResolvedIPs, expected)
	if matchedExpected && serverIPPublic {
		res.Status = "valid"
		res.Valid = true
		res.Message = fmt.Sprintf("DNS resolves to your server IP (%s).", expected)
		return res
	}

	// Non-public IP resolved (e.g. Podman container IP or LAN fallback) or another host IP.
	res.Status = "mismatch"
	res.Valid = false
	first := res.ResolvedIP
	if first == "" && len(res.ResolvedIPs) > 0 {
		first = res.ResolvedIPs[0]
	}
	if serverIPPublic && expected != "" {
		res.Message = fmt.Sprintf("Resolves to %s, but your server IP is %s.", first, expected)
	} else {
		res.Message = fmt.Sprintf("Resolves to %s. Verify it points to your public server IP.", first)
	}
	return res
}

