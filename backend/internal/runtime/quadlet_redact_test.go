package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetQuadletContentMasksSecretEnvValues pins the API contract: quadlet
// responses mask env values GOPOD stores encrypted, while the unit file on
// disk keeps the real value because systemd needs it.
func TestGetQuadletContentMasksSecretEnvValues(t *testing.T) {
	quadDir := filepath.Join(t.TempDir(), "quadlets")
	if err := os.MkdirAll(quadDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("QUADLET_DIR", quadDir)
	t.Setenv("HOME", t.TempDir())

	content := strings.Join([]string{
		"[Unit]",
		"Description=demo",
		"[Container]",
		"Image=alpine:latest",
		`Environment="DATABASE_URL=postgres://user:pw@db/app" "PUBLIC_FLAG=1"`,
		`Environment="API_SECRET=s3cr3t"`,
		"Environment=PLAIN=visible",
		"",
	}, "\n")
	path := filepath.Join(quadDir, "redact-demo.container")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write unit: %v", err)
	}

	svc := NewService(nil)
	svc.SetEnvSecretKeyProvider(func() map[string]bool {
		return map[string]bool{"DATABASE_URL": true, "API_SECRET": true}
	})

	got, err := svc.GetQuadletContent("redact-demo.container")
	if err != nil {
		t.Fatalf("GetQuadletContent: %v", err)
	}
	for _, leak := range []string{"postgres://user:pw@db/app", "s3cr3t"} {
		if strings.Contains(got, leak) {
			t.Fatalf("secret %q leaked through the quadlet content API: %s", leak, got)
		}
	}
	if !strings.Contains(got, "DATABASE_URL="+envValueMask) || !strings.Contains(got, "API_SECRET="+envValueMask) {
		t.Fatalf("expected masked secret values, got: %s", got)
	}
	if !strings.Contains(got, `"PUBLIC_FLAG=1"`) {
		t.Fatalf("non-secret neighbor on the same line must stay intact: %s", got)
	}
	if !strings.Contains(got, "PLAIN=visible") {
		t.Fatalf("plain assignment must stay intact: %s", got)
	}

	// systemd still needs the real values on disk.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read unit: %v", err)
	}
	if !strings.Contains(string(raw), "postgres://user:pw@db/app") {
		t.Fatalf("on-disk unit must keep the real value for systemd")
	}

	// The list endpoint masks as well.
	units, err := svc.ListQuadlets(context.Background())
	if err != nil {
		t.Fatalf("ListQuadlets: %v", err)
	}
	found := false
	for _, u := range units {
		if u.Name != "redact-demo.container" {
			continue
		}
		found = true
		if strings.Contains(u.Content, "s3cr3t") {
			t.Fatalf("list endpoint leaked a secret value")
		}
		if !strings.Contains(u.Content, "API_SECRET="+envValueMask) {
			t.Fatalf("list endpoint must mask secret values")
		}
	}
	if !found {
		t.Fatalf("expected the demo unit in the listing")
	}
}

// TestMaskEnvironmentValuesEdgeCases covers quoting, several assignments per
// line, indentation and the no-secret-keys fast path.
func TestMaskEnvironmentValuesEdgeCases(t *testing.T) {
	keys := map[string]bool{"SECRET": true}
	cases := []struct{ in, want string }{
		{`Environment="SECRET=abc def"`, `Environment="SECRET=` + envValueMask + `"`},
		{`Environment=SECRET=abc`, `Environment=SECRET=` + envValueMask},
		{`Environment=OTHER=1 SECRET=2`, `Environment=OTHER=1 SECRET=` + envValueMask},
		{`    Environment="SECRET=x"`, `    Environment="SECRET=` + envValueMask + `"`},
		{`Environment="SECRET=a\"b"`, `Environment="SECRET=` + envValueMask + `"`},
		{`EnvironmentFile=/etc/app.env`, `EnvironmentFile=/etc/app.env`},
		{`Environment="OTHER=a b"`, `Environment="OTHER=a b"`},
	}
	for _, c := range cases {
		if got := maskEnvironmentValues(c.in, keys); got != c.want {
			t.Errorf("maskEnvironmentValues(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	if got := maskEnvironmentValues(`Environment="SECRET=x"`, nil); got != `Environment="SECRET=x"` {
		t.Errorf("nil key set must leave content untouched, got %q", got)
	}
}
