package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/auth"
	"gopod/internal/db"
	"gopod/internal/podman"
)

func TestValidateTargetTag(t *testing.T) {
	valid := []string{"latest", "dev", "v0.1.0", "0.2.3", "v1.2.3-rc.1"}
	for _, tag := range valid {
		got, err := ValidateTargetTag(tag)
		if err != nil || got != tag {
			t.Fatalf("expected valid tag %q, got %q (err=%v)", tag, got, err)
		}
	}

	invalid := []string{"", " ", "-rf", "--pull", "evil/repo:latest", "v1.0.0;rm -rf /", "ghcr.io/x/y:1", "../v1.0.0", strings.Repeat("a", 65)}
	for _, tag := range invalid {
		if _, err := ValidateTargetTag(tag); err == nil {
			t.Fatalf("expected invalid tag %q to be rejected", tag)
		}
	}
}

func TestSSRFRedirectBlocked(t *testing.T) {
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("evil"))
	}))
	defer attacker.Close()

	redirector := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attacker.URL+"/payload", http.StatusFound)
	}))
	defer redirector.Close()

	client := newSafeUpdateHTTPClient(false)
	// Bypass TLS self-signed verification on localhost redirector while keep CheckRedirect strict.
	tr := redirector.Client().Transport
	client.Transport = tr

	if _, err := client.Get(redirector.URL + "/releases"); err == nil {
		t.Fatal("expected redirect to non-allowlisted HTTP server to be blocked")
	}
}

func TestCheckForUpdatesAndDigestDetection(t *testing.T) {
	remoteDigest := "sha256:" + strings.Repeat("a", 64)
	ghMux := http.NewServeMux()
	ghMux.HandleFunc("/repos/ijon6k/gopod/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://github.com/ijon6k/gopod/releases/tag/v0.2.0","body":"Bug fixes","published_at":"2026-11-26T00:00:00Z"}`))
	})
	ghMux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"tok-test"}`))
	})
	ghMux.HandleFunc("/v2/ijon6k/gopod/manifests/v0.2.0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", remoteDigest)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(ghMux)
	defer srv.Close()

	origVersion := Version
	Version = "v0.1.0"
	defer func() { Version = origVersion }()

	svc := NewService(nil, t.TempDir(), nil, nil)
	svc.allowLocalHTTP = true
	svc.httpClient = newSafeUpdateHTTPClient(true)
	svc.githubAPIBase = srv.URL
	svc.ghcrBaseURL = srv.URL

	res, err := svc.CheckForUpdates(context.Background(), "latest", true)
	if err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	if !res.UpdateAvailable || res.LatestTag != "v0.2.0" || res.RemoteDigest != remoteDigest {
		t.Fatalf("unexpected update check result: %+v", res)
	}
}

func TestExecuteNativeBinaryUpdateVerifiesSHA256AndRejectsTampering(t *testing.T) {
	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "gopod")
	originalPayload := []byte("old-binary-v1")
	if err := os.WriteFile(exePath, originalPayload, 0755); err != nil {
		t.Fatalf("write original exe: %v", err)
	}

	goodBinary := []byte("new-binary-v2-verified")
	sum := sha256.Sum256(goodBinary)
	goodHex := hex.EncodeToString(sum[:])

	badBinary := []byte("tampered-evil-payload")
	serveBad := true

	dlMux := http.NewServeMux()
	dlMux.HandleFunc("/ijon6k/gopod/releases/download/v0.2.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  gopod-linux-amd64\n%s  gopod-linux-arm64\n", goodHex, goodHex)
	})
	dlMux.HandleFunc("/ijon6k/gopod/releases/download/v0.2.0/gopod-linux-amd64", func(w http.ResponseWriter, r *http.Request) {
		if serveBad {
			_, _ = w.Write(badBinary)
			return
		}
		_, _ = w.Write(goodBinary)
	})
	dlMux.HandleFunc("/ijon6k/gopod/releases/download/v0.2.0/gopod-linux-arm64", func(w http.ResponseWriter, r *http.Request) {
		if serveBad {
			_, _ = w.Write(badBinary)
			return
		}
		_, _ = w.Write(goodBinary)
	})
	dlSrv := httptest.NewServer(dlMux)
	defer dlSrv.Close()

	svc := NewService(nil, tmpDir, nil, nil)
	svc.allowLocalHTTP = true
	svc.httpClient = newSafeUpdateHTTPClient(true)
	svc.githubDownloadURL = dlSrv.URL

	self := SelfInspection{
		Mode:           ModeNative,
		CanAutoUpdate:  true,
		ExecutablePath: exePath,
	}

	// 1. Tampered payload MUST fail checksum check and preserve original binary
	if err := svc.executeNativeBinaryUpdate(context.Background(), self, "v0.2.0", "2026-11-26T00:00:00Z"); err == nil {
		t.Fatal("expected checksum verification failure for tampered binary")
	}
	got, _ := os.ReadFile(exePath)
	if string(got) != string(originalPayload) {
		t.Fatalf("original executable was modified on failed checksum: got %q", string(got))
	}

	// 2. Valid payload MUST verify checksum, keep .bak, and replace executable atomically
	serveBad = false
	if err := svc.executeNativeBinaryUpdate(context.Background(), self, "v0.2.0", "2026-11-26T00:00:00Z"); err != nil {
		t.Fatalf("expected clean native update, got err: %v", err)
	}
	got, _ = os.ReadFile(exePath)
	if string(got) != string(goodBinary) {
		t.Fatalf("executable not replaced cleanly: got %q", string(got))
	}
	bak, _ := os.ReadFile(exePath + ".bak")
	if string(bak) != string(originalPayload) {
		t.Fatalf("backup binary mismatch: got %q", string(bak))
	}
}

func TestEvaluatePodmanCompatibilityAndHandlerAuth(t *testing.T) {
	oldCompat := EvaluatePodmanCompatibility(&podman.SystemInfo{PodmanVersion: "4.9.3", OS: "Ubuntu 24.04", Rootless: true})
	if oldCompat.Compatible || !strings.Contains(oldCompat.UpgradeHint, "apt-get") {
		t.Fatalf("expected Podman 4.9.3 incompatible with apt hint, got %+v", oldCompat)
	}
	newCompat := EvaluatePodmanCompatibility(&podman.SystemInfo{PodmanVersion: "5.8.9", OS: "Ultramarine Linux 44", Rootless: true})
	if !newCompat.Compatible || !strings.Contains(newCompat.UpgradeHint, "dnf") {
		t.Fatalf("expected Podman 5.8.9 compatible with dnf hint, got %+v", newCompat)
	}

	// Handler RBAC + CSRF Origin verification
	database, err := db.Open(filepath.Join(t.TempDir(), "updater-auth.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	authRepo := auth.NewSQLiteRepository(database.DB)
	authSvc := auth.NewService(authRepo)
	_, adminTok, _ := authSvc.SetupAdmin(ctx, "Admin", "admin@gopod.local", "adminpass123")
	ac := auth.NewAccessChecker(database.DB)
	_, _ = ac.CreateManagedUser(ctx, auth.CreateUserRequest{
		Name: "Member", Email: "member@gopod.local", Password: "memberpass123", Role: "member",
	})
	_, memberTok, _ := authSvc.Login(ctx, "member@gopod.local", "memberpass123")

	mw := auth.NewMiddleware(authSvc)
	mw.SetAccessChecker(ac)

	svc := NewService(nil, t.TempDir(), nil, nil)
	h := NewHandler(svc, mw)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Member denied on GET /api/system/updates & POST /api/system/update
	reqMember := httptest.NewRequest(http.MethodGet, "/api/system/updates", nil)
	reqMember.Header.Set("Authorization", "Bearer "+memberTok)
	rrM := httptest.NewRecorder()
	mux.ServeHTTP(rrM, reqMember)
	if rrM.Code != http.StatusForbidden {
		t.Fatalf("member on /api/system/updates got %d, want 403", rrM.Code)
	}

	// Admin with cross-site Origin rejected with 403
	reqCSRF := httptest.NewRequest(http.MethodPost, "/api/system/update", strings.NewReader(`{"tag":"latest"}`))
	reqCSRF.Host = "panel.homelab.local"
	reqCSRF.Header.Set("Origin", "https://evil.example.com")
	reqCSRF.Header.Set("Authorization", "Bearer "+adminTok)
	rrCSRF := httptest.NewRecorder()
	mux.ServeHTTP(rrCSRF, reqCSRF)
	if rrCSRF.Code != http.StatusForbidden {
		t.Fatalf("untrusted Origin on POST /api/system/update got %d, want 403", rrCSRF.Code)
	}
}

