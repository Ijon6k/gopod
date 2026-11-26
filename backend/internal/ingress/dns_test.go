package ingress

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestCheckDomainDNS(t *testing.T) {
	ctx := context.Background()

	t.Run("local loopback domain", func(t *testing.T) {
		res, err := checkDomainDNSWithHooks(ctx, "kanban.localhost", "203.0.113.10", true, dnsLookupHooks{
			lookupIP: func(context.Context, string) ([]net.IP, error) {
				t.Fatal("local domain should not trigger external DNS lookup")
				return nil, nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "local" || !res.Valid || res.BehindCloudflare {
			t.Fatalf("unexpected local result: %+v", res)
		}
	})

	t.Run("direct match with public server IP", func(t *testing.T) {
		res, err := checkDomainDNSWithHooks(ctx, "app.example.com", "203.0.113.10", true, dnsLookupHooks{
			lookupIP: func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP("203.0.113.10")}, nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "valid" || !res.Valid || res.ResolvedIP != "203.0.113.10" {
			t.Fatalf("unexpected valid result: %+v", res)
		}
	})

	t.Run("behind cloudflare proxy IP", func(t *testing.T) {
		res, err := checkDomainDNSWithHooks(ctx, "app.example.com", "203.0.113.10", true, dnsLookupHooks{
			lookupIP: func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP("104.21.16.42"), net.ParseIP("172.67.180.5")}, nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "cloudflare" || !res.Valid || !res.BehindCloudflare {
			t.Fatalf("expected cloudflare status, got: %+v", res)
		}
	})

	t.Run("mismatch against server public IP", func(t *testing.T) {
		res, err := checkDomainDNSWithHooks(ctx, "app.example.com", "203.0.113.10", true, dnsLookupHooks{
			lookupIP: func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP("198.51.100.25")}, nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "mismatch" || res.Valid || res.BehindCloudflare {
			t.Fatalf("expected mismatch result, got: %+v", res)
		}
	})

	t.Run("private container fallback does not count as verified public match", func(t *testing.T) {
		res, err := checkDomainDNSWithHooks(ctx, "app.example.com", "10.89.12.5", false, dnsLookupHooks{
			lookupIP: func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP("10.89.12.5")}, nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "mismatch" || res.Valid {
			t.Fatalf("expected private container IP not to mark valid, got: %+v", res)
		}
	})

	t.Run("pending when DNS lookup fails", func(t *testing.T) {
		res, err := checkDomainDNSWithHooks(ctx, "new.example.com", "203.0.113.10", true, dnsLookupHooks{
			lookupIP: func(context.Context, string) ([]net.IP, error) {
				return nil, errors.New("no such host")
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "pending" || res.Valid {
			t.Fatalf("expected pending result, got: %+v", res)
		}
	})
}

