package ingress

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTargetContainerPayloads(t *testing.T) {
	rejected := []string{
		"foo}",
		"foo\nroute /pwn { * }",
		"foo; tls off",
		"$(whoami)",
		"`x`",
		`"y"`,
		"foo bar",
		"",
		"   ",
		".foo",
		"foo\\bar",
		"foo#comment",
		strings.Repeat("a", 65),
	}
	for _, in := range rejected {
		if err := ValidateTargetContainer(in); err == nil {
			t.Errorf("expected %q to be rejected", in)
		}
	}

	accepted := []string{
		"web-gui",
		"my.ctr_2",
		"web_server-1",
		"Self-Hosted_Web-App-abc123",
		"pod_cluster-demo",
	}
	for _, in := range accepted {
		if err := ValidateTargetContainer(in); err != nil {
			t.Errorf("expected %q to be accepted, got %v", in, err)
		}
	}
}

func TestValidateHostnamePayloads(t *testing.T) {
	rejected := []string{
		"",
		"   ",
		"foo bar",
		"foo{}",
		"foo\nbar",
		"foo;bar",
		"foo`bar",
		`foo"bar`,
		"$(x).com",
		"foo(bar).com",
		"foo\\bar.com",
		strings.Repeat("a", 254),
	}
	for _, in := range rejected {
		if err := ValidateHostname(in); err == nil {
			t.Errorf("expected %q to be rejected", in)
		}
	}

	accepted := []string{
		"app.example.com",
		"*.example.com",
		"foo.localhost",
		"127.0.0.1",
		"my_host.example.com",
		"xn--bcher-kva.example",
	}
	for _, in := range accepted {
		if err := ValidateHostname(in); err != nil {
			t.Errorf("expected %q to be accepted, got %v", in, err)
		}
	}
}

func TestValidateContainerPort(t *testing.T) {
	for _, in := range []int{0, -1, 65536} {
		if err := ValidateContainerPort(in); err == nil {
			t.Errorf("expected port %d to be rejected", in)
		}
	}
	for _, in := range []int{1, 80, 443, 8080, 65535} {
		if err := ValidateContainerPort(in); err != nil {
			t.Errorf("expected port %d to be accepted, got %v", in, err)
		}
	}
}

func TestParseAdminBindOverride(t *testing.T) {
	valid := map[string]string{
		"localhost":      "localhost:2019",
		"127.0.0.1":      "127.0.0.1:2019",
		"127.0.0.1:2030": "127.0.0.1:2030",
		"localhost:2020": "localhost:2020",
		"::1":            "[::1]:2019",
		"[::1]:2020":     "[::1]:2020",
	}
	for in, want := range valid {
		got, ok := parseAdminBindOverride(in)
		if !ok || got != want {
			t.Errorf("parseAdminBindOverride(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}

	invalid := []string{
		"0.0.0.0",
		"0.0.0.0:2019",
		"::",
		"[::]",
		"evil.com",
		"evil.com:2019",
		"unix:///tmp/admin.sock",
		"localhost:0",
		"localhost:70000",
		"   ",
	}
	for _, in := range invalid {
		if got, ok := parseAdminBindOverride(in); ok {
			t.Errorf("parseAdminBindOverride(%q) unexpectedly accepted as %q", in, got)
		}
	}
}

func TestGenerateCaddyfileBlocksInjection(t *testing.T) {
	t.Setenv("CADDY_ADMIN_HOST", "")
	t.Setenv("CADDY_ADMIN_URL", "")

	r := NewReconciler("http://127.0.0.1:2019", "")
	domains := []Domain{
		{ID: "evil-target", Hostname: "evil-target.localhost", TargetContainer: "foo}\nroute /pwn { * }\n", ContainerPort: 80},
		{ID: "evil-port", Hostname: "evil-port.localhost", ServiceID: "ok-svc", ContainerPort: -1},
		{ID: "evil-svcid", Hostname: "evil-svcid.localhost", ServiceID: "x}", ContainerPort: 80},
		{ID: "evil-host", Hostname: "evil\"host.localhost", ServiceID: "ok-svc", ContainerPort: 80},
		{ID: "good", Hostname: "good.localhost", ServiceID: "web-gui", ContainerPort: 3000},
	}

	out := r.GenerateCaddyfile(domains)

	for _, leaked := range []string{"route /pwn", "foo}", "evil-target.localhost", "evil-port.localhost", "evil-svcid.localhost", `evil"host.localhost`} {
		if strings.Contains(out, leaked) {
			t.Errorf("generated Caddyfile leaked invalid domain content %q:\n%s", leaked, out)
		}
	}
	if !strings.Contains(out, "reverse_proxy web-gui:3000") {
		t.Errorf("expected the valid domain to still be rendered, got:\n%s", out)
	}
	if !strings.Contains(out, "admin localhost:2019") {
		t.Errorf("expected loopback admin bind in host mode, got:\n%s", out)
	}
}

func TestAdminListenAddressModes(t *testing.T) {
	t.Setenv("CADDY_ADMIN_HOST", "")
	t.Setenv("CADDY_ADMIN_URL", "")

	// Host mode (default): loopback-only admin bind.
	hostMode := NewReconciler("http://127.0.0.1:2019", "")
	if out := hostMode.GenerateCaddyfile(nil); !strings.Contains(out, "admin localhost:2019") {
		t.Errorf("host mode must bind admin to localhost, got:\n%s", out)
	}

	// Containerized endpoint (podman-network name): non-loopback bind so the
	// GOPOD container can reach the admin API by service DNS name.
	containerMode := NewReconciler("http://caddy:2019", "")
	if out := containerMode.GenerateCaddyfile(nil); !strings.Contains(out, "admin 0.0.0.0:2019") {
		t.Errorf("container mode must keep the network bind, got:\n%s", out)
	}

	// Explicit override narrows the bind.
	t.Setenv("CADDY_ADMIN_HOST", "127.0.0.1:2030")
	if out := containerMode.GenerateCaddyfile(nil); !strings.Contains(out, "admin 127.0.0.1:2030") {
		t.Errorf("CADDY_ADMIN_HOST override must apply, got:\n%s", out)
	}

	// Wildcard override is rejected and falls back to the derived default.
	t.Setenv("CADDY_ADMIN_HOST", "0.0.0.0")
	out := hostMode.GenerateCaddyfile(nil)
	if strings.Contains(out, "0.0.0.0") {
		t.Errorf("wildcard override must be ignored, got:\n%s", out)
	}
	if !strings.Contains(out, "admin localhost:2019") {
		t.Errorf("invalid override must fall back to the host-mode default, got:\n%s", out)
	}
}

func TestReconcileSkipsPoisonedDomainsOnDisk(t *testing.T) {
	t.Setenv("CADDY_ADMIN_HOST", "")
	t.Setenv("CADDY_ADMIN_URL", "")

	dir := t.TempDir()
	caddyfilePath := filepath.Join(dir, "Caddyfile")
	r := NewReconciler("http://127.0.0.1:9", caddyfilePath)

	domains := []Domain{
		{ID: "evil", Hostname: "evil.localhost", TargetContainer: "foo}\nroute /pwn { * }", ContainerPort: 80},
		{ID: "good", Hostname: "good.localhost", ServiceID: "web-gui", ContainerPort: 3000},
	}
	// Contract per the ingress audit: a reconcile whose generated Caddyfile
	// could NOT be picked up by a running Caddy (here: a discard-port admin
	// endpoint) is an error wrapping ErrCaddyNotApplied — "persisted for next
	// restart" is not a successful live load. The on-disk assertions below
	// are the actual target of this test.
	err := r.Reconcile(context.Background(), domains)
	if !errors.Is(err, ErrCaddyNotApplied) {
		t.Fatalf("Reconcile must report a not-applied reload (unreachable admin endpoint), got: %v", err)
	}

	data, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("read generated Caddyfile: %v", err)
	}
	if strings.Contains(string(data), "route /pwn") {
		t.Fatalf("injected block reached the Caddyfile on disk:\n%s", data)
	}
	if !strings.Contains(string(data), "reverse_proxy web-gui:3000") {
		t.Fatalf("valid domain missing from the Caddyfile on disk:\n%s", data)
	}
}

func TestGenerateCaddyfileHashesPlaintextBasicAuth(t *testing.T) {
	t.Setenv("CADDY_ADMIN_HOST", "")
	t.Setenv("CADDY_ADMIN_URL", "")

	r := NewReconciler("http://127.0.0.1:2019", "")
	out := r.GenerateCaddyfile([]Domain{
		{
			ID:            "legacy",
			Hostname:      "legacy.localhost",
			ServiceID:     "web-gui",
			ContainerPort: 80,
			BasicAuth:     true,
			BasicAuthUser: "admin",
			BasicAuthPass: "legacyplain",
		},
	})

	if strings.Contains(out, "legacyplain") {
		t.Errorf("plaintext password must never reach the generated Caddyfile:\n%s", out)
	}
	if !strings.Contains(out, "$2") || !strings.Contains(out, "basicauth") {
		t.Errorf("expected a bcrypt basicauth block, got:\n%s", out)
	}
}
