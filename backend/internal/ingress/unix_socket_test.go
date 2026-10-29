package ingress

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminSocketHelpers(t *testing.T) {
	cases := []struct {
		in       string
		isUDS    bool
		wantPath string
		wantReq  string
	}{
		{"unix:///run/gopod-caddy/admin.sock", true, "/run/gopod-caddy/admin.sock", "http://unix"},
		{"unix:/run/gopod-caddy/admin.sock", true, "/run/gopod-caddy/admin.sock", "http://unix"},
		{"unix//run/gopod-caddy/admin.sock", true, "/run/gopod-caddy/admin.sock", "http://unix"},
		{"unix:///tmp/test.sock", true, "/tmp/test.sock", "http://unix"},
		{"unix:", true, "", "http://unix"},
		{"http://127.0.0.1:2019", false, "", "http://127.0.0.1:2019"},
		{"http://caddy:2019", false, "", "http://caddy:2019"},
		{"https://caddy:2019", false, "", "https://caddy:2019"},
	}

	for _, tc := range cases {
		if got := isAdminUDS(tc.in); got != tc.isUDS {
			t.Errorf("isAdminUDS(%q) = %v; want %v", tc.in, got, tc.isUDS)
		}
		if tc.isUDS {
			if got := adminSocketPath(tc.in); got != tc.wantPath {
				t.Errorf("adminSocketPath(%q) = %q; want %q", tc.in, got, tc.wantPath)
			}
		}
		if got := adminRequestURL(tc.in); got != tc.wantReq {
			t.Errorf("adminRequestURL(%q) = %q; want %q", tc.in, got, tc.wantReq)
		}
	}
}

func TestReconcilerUnixSocketCaddyfile(t *testing.T) {
	t.Setenv("CADDY_ADMIN_HOST", "")
	t.Setenv("CADDY_ADMIN_URL", "")
	t.Setenv("GOPOD_BIND_HOST", "")

	sockURL := "unix:///run/gopod-caddy/admin.sock"
	r := NewReconciler(sockURL, "")

	if out := r.GenerateCaddyfile(nil); !strings.Contains(out, "admin unix//run/gopod-caddy/admin.sock") {
		t.Errorf("expected Caddyfile to contain 'admin unix//run/gopod-caddy/admin.sock', got:\n%s", out)
	}

	// In Pod mode (GOPOD_BIND_HOST=127.0.0.1), dashboard reverse_proxy must point to loopback
	t.Setenv("GOPOD_BIND_HOST", "127.0.0.1")
	outPod := r.GenerateCaddyfile(nil)
	if !strings.Contains(outPod, "reverse_proxy 127.0.0.1:") {
		t.Errorf("expected pod mode (GOPOD_BIND_HOST=127.0.0.1) to use 127.0.0.1 upstream, got:\n%s", outPod)
	}

	// In container/standalone mode without GOPOD_BIND_HOST, dashboard reverse_proxy points to gopod:port
	t.Setenv("GOPOD_BIND_HOST", "")
	t.Setenv("CADDY_ADMIN_URL", "unix:///run/gopod-caddy/admin.sock")
	outStandalone := r.GenerateCaddyfile(nil)
	if !strings.Contains(outStandalone, "reverse_proxy gopod:") {
		t.Errorf("expected container mode to use gopod:port upstream, got:\n%s", outStandalone)
	}
}

func TestAdminHTTPClientDialsUnixSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "admin.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create unix socket listener: %v", err)
	}
	defer listener.Close()

	received := make(chan string, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			received <- string(body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}),
	}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Shutdown(context.Background()) }()

	client := newAdminHTTPClient("unix://" + sockPath)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://unix/load", strings.NewReader("dummy-caddyfile-content"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do failed to dial unix socket: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	select {
	case body := <-received:
		if body != "dummy-caddyfile-content" {
			t.Errorf("expected server to receive 'dummy-caddyfile-content', got %q", body)
		}
	default:
		t.Fatal("expected request body was not received by server")
	}
}

func TestReconcileLiveUnixSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "admin.sock")
	caddyfilePath := filepath.Join(dir, "Caddyfile")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create unix socket listener: %v", err)
	}
	defer listener.Close()

	loadCount := 0
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/load" && r.Method == http.MethodPost {
				loadCount++
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}),
	}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Shutdown(context.Background()) }()

	r := NewReconciler("unix://"+sockPath, caddyfilePath)
	err = r.Reconcile(context.Background(), []Domain{
		{
			ID:              "dom-1",
			ProjectID:       "proj-1",
			ServiceID:       "svc-1",
			TargetContainer: "web-svc",
			Hostname:        "app.example.com",
			ContainerPort:   8080,
			PathPrefix:      "/",
		},
	})
	if err != nil {
		t.Fatalf("Reconcile via unix socket failed: %v", err)
	}

	if loadCount != 1 {
		t.Fatalf("expected /load to be called once via unix socket, got %d", loadCount)
	}

	// Verify the Caddyfile was persisted to disk with admin unix//...
	out, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("expected Caddyfile at %s: %v", caddyfilePath, err)
	}
	if !strings.Contains(string(out), "admin unix/"+sockPath) {
		t.Errorf("expected Caddyfile to contain 'admin unix/%s', got:\n%s", sockPath, string(out))
	}
}
