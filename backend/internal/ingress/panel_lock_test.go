package ingress

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/db"
)

func TestGenerateCaddyfilePanelDomainLock(t *testing.T) {
	r := NewReconciler("unix:///tmp/caddy-admin.sock", filepath.Join(t.TempDir(), "Caddyfile"))
	r.SetControlPort("8085")

	// Default unlocked: :80 reverse proxy active
	def := r.GenerateCaddyfile(nil)
	if !strings.Contains(def, ":80 {") || !strings.Contains(def, "reverse_proxy") {
		t.Fatalf("expected default :80 proxy in Caddyfile:\n%s", def)
	}

	// Configured domain with LockCatchall=true: domain served cleanly and :80 returns 404
	r.SetPanelSettings(PanelSettings{
		Domain:       "panel.homelab.my.id",
		TLS:          true,
		LockCatchall: true,
	})
	locked := r.GenerateCaddyfile(nil)
	if !strings.Contains(locked, "panel.homelab.my.id {") {
		t.Fatalf("expected panel domain site block in locked Caddyfile:\n%s", locked)
	}
	if !strings.Contains(locked, `respond "Not Found" 404`) {
		t.Fatalf("expected :80 404 responder when lockCatchall=true:\n%s", locked)
	}
}

func TestPanelSettingsValidationAndPersistence(t *testing.T) {
	// Locking without a domain must be rejected
	bad := PanelSettings{Domain: "", LockCatchall: true}
	if err := ValidatePanelSettings(&bad); err == nil {
		t.Fatal("expected error when locking catchall without domain")
	}

	// Wildcards and caddy metacharacters must be rejected
	inj := PanelSettings{Domain: "panel.example.com {\n respond \"pwned\"\n}", TLS: true}
	if err := ValidatePanelSettings(&inj); err == nil {
		t.Fatal("expected injection hostname to fail validation")
	}

	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	rec := NewReconciler("", filepath.Join(t.TempDir(), "Caddyfile"))
	svc := NewService(NewSQLiteRepository(database.DB), rec)
	svc.SetSettingsDB(database.DB)

	ctx := context.Background()
	saved, err := svc.SavePanelSettings(ctx, PanelSettings{
		Domain:       "https://Panel.Home.Local/",
		TLS:          true,
		LockCatchall: true,
	})
	if err != nil {
		t.Fatalf("SavePanelSettings failed: %v", err)
	}
	if saved.Domain != "panel.home.local" {
		t.Fatalf("expected normalized domain 'panel.home.local', got %q", saved.Domain)
	}

	loaded, err := svc.LoadPanelSettings(ctx)
	if err != nil {
		t.Fatalf("LoadPanelSettings failed: %v", err)
	}
	if loaded != saved {
		t.Fatalf("loaded = %+v, want %+v", loaded, saved)
	}
}

