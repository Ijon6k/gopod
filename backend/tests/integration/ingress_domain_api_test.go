package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gopod/internal/server"
)

// This suite exercises the ingress domain HTTP API end to end against the full
// in-process backend (no podman required):
//
//   - malicious upstream targets / ports / hostnames are rejected with 400 and
//     never reach the generated Caddyfile;
//   - basic-auth passwords are write-only (no API response ever contains them)
//     and stored as bcrypt hashes which the generated Caddyfile uses;
//   - updating a domain without re-sending the password keeps the stored hash.
//
// The reconciler writes to a temp HOME, never the developer's real
// ~/.config/gopod/caddy/Caddyfile.

// newDomainAPITestServer boots the full backend like the E2E harness does but
// keeps the reconciler's temp HOME so tests can inspect the generated
// Caddyfile, and disables the nightly GC scheduler.
func newDomainAPITestServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("GOPOD_GC_DISABLE", "1")
	t.Setenv("CADDY_ADMIN_URL", "")
	t.Setenv("CADDY_ADMIN_HOST", "")
	t.Setenv("HOME", home)

	handler := server.NewServer(server.Config{
		Port:         "0",
		PodmanSocket: "", // podman client selects its default rootless socket; unused here
		DBPath:       filepath.Join(t.TempDir(), "gopod.db"),
		CaddyAdmin:   "http://127.0.0.1:9", // unreachable: never contacts a live Caddy
		DistFS:       fstest.MapFS{},
	})

	return handler, filepath.Join(home, ".config", "gopod", "caddy", "Caddyfile")
}

var bcryptHashRe = regexp.MustCompile(`\$2[abxy]\$[0-9]{2}\$[./A-Za-z0-9]{53}`)

func waitForCaddyfile(t *testing.T, path string, cond func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			last = string(b)
			if cond(last) {
				return last
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("condition not met on %s within 10s; last content:\n%s", path, last)
	return last
}

func extractBasicAuthHash(t *testing.T, caddyfile string) string {
	t.Helper()
	m := bcryptHashRe.FindString(caddyfile)
	if m == "" {
		t.Fatalf("no bcrypt hash found in Caddyfile:\n%s", caddyfile)
	}
	return m
}

func TestIngressDomainSecurityAPI(t *testing.T) {
	handler, caddyfilePath := newDomainAPITestServer(t)

	srv := httptest.NewServer(handler)
	defer srv.Close()
	c := loginAdmin(t, srv.URL)

	// ——— 0. Bootstrap a project + service (domain rows carry FK constraints) ———

	code, data := c.post(t, "/api/projects", []byte(`{"name":"ingress-api","description":"Ingress domain API test project"}`))
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("create project: %d %s", code, trunc(data))
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &project); err != nil || project.ID == "" {
		t.Fatalf("project payload incomplete: %v / %s", err, trunc(data))
	}

	svcBody, _ := json.Marshal(map[string]interface{}{
		"projectId":     project.ID,
		"name":          "ingress-api-web",
		"type":          "image",
		"image":         "docker.io/library/nginx:alpine",
		"port":          8080,
		"restartPolicy": "no",
	})
	code, data = c.post(t, "/api/services", svcBody)
	if code != http.StatusCreated {
		t.Fatalf("create service: %d %s", code, trunc(data))
	}
	var svc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &svc); err != nil || svc.ID == "" {
		t.Fatalf("service payload incomplete: %v / %s", err, trunc(data))
	}

	// ——— 1. Malicious / invalid payloads are rejected with 400 ———

	poisoned := map[string]string{
		"target injection":   `{"hostname":"evil-target.localhost","serviceId":"ok-svc","containerPort":80,"targetContainer":"foo}\nroute /pwn { * }"}`,
		"service id poison":  `{"hostname":"evil-svc.localhost","serviceId":"x; tls off","containerPort":80}`,
		"out-of-range port":  `{"hostname":"evil-port.localhost","serviceId":"ok-svc","containerPort":-1}`,
		"hostname injection": `{"hostname":"evil{host.localhost","serviceId":"ok-svc","containerPort":80}`,
	}
	for name, body := range poisoned {
		code, data := c.post(t, "/api/domains", []byte(body))
		if code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", name, code, trunc(data))
		}
	}

	// ——— 2. Valid create with basic auth — password is write-only ———

	createBody, _ := json.Marshal(map[string]any{
		"hostname": "secure.localhost", "projectId": project.ID, "serviceId": svc.ID,
		"containerPort": 8080, "tls": true, "pathPrefix": "/",
		"basicAuth": true, "basicAuthUser": "admin", "basicAuthPass": "s3cret",
	})
	code, data = c.post(t, "/api/domains", createBody)
	if code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", code, trunc(data))
	}
	if strings.Contains(string(data), "s3cret") || strings.Contains(string(data), "basicAuthPass") {
		t.Fatalf("create response must not leak the password: %s", trunc(data))
	}
	if !strings.Contains(string(data), `"hasBasicAuth":true`) {
		t.Fatalf("create response must expose hasBasicAuth=true: %s", trunc(data))
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		t.Fatalf("cannot parse created id (%v): %s", err, trunc(data))
	}

	// ——— 3. Generated Caddyfile carries the bcrypt hash, never plaintext ———

	caddyfile := waitForCaddyfile(t, caddyfilePath, func(s string) bool {
		return strings.Contains(s, "secure.localhost") && strings.Contains(s, "basicauth")
	})
	if strings.Contains(caddyfile, "s3cret") {
		t.Fatalf("plaintext password must never reach the Caddyfile:\n%s", caddyfile)
	}
	if !strings.Contains(caddyfile, "admin localhost:2019") {
		t.Fatalf("host mode must bind the admin API to localhost:\n%s", caddyfile)
	}
	hashAfterCreate := extractBasicAuthHash(t, caddyfile)
	if err := bcrypt.CompareHashAndPassword([]byte(hashAfterCreate), []byte("s3cret")); err != nil {
		t.Fatalf("generated hash does not match the configured password: %v", err)
	}

	// ——— 4. UI-style update without password keeps the stored hash ———

	updateBody, _ := json.Marshal(map[string]any{
		"hostname": "secure.localhost", "projectId": project.ID, "serviceId": svc.ID,
		"containerPort": 8080, "tls": true, "pathPrefix": "/api",
		"basicAuth": true, "basicAuthUser": "admin",
	})
	code, data = c.request(t, http.MethodPut, "/api/domains/"+created.ID, updateBody)
	if code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", code, trunc(data))
	}
	if strings.Contains(string(data), "basicAuthPass") || !strings.Contains(string(data), `"hasBasicAuth":true`) {
		t.Fatalf("update response must keep hasBasicAuth and omit the password: %s", trunc(data))
	}
	caddyfile = waitForCaddyfile(t, caddyfilePath, func(s string) bool {
		return strings.Contains(s, "handle /api*")
	})
	if got := extractBasicAuthHash(t, caddyfile); got != hashAfterCreate {
		t.Fatalf("empty password update must keep the stored hash:\nbefore %s\nafter  %s", hashAfterCreate, got)
	}

	// ——— 5. Poisoned update is rejected and the file stays clean ———

	poisonedUpdate, _ := json.Marshal(map[string]any{
		"hostname": "secure.localhost", "projectId": project.ID, "serviceId": svc.ID,
		"containerPort": 8080, "tls": true, "pathPrefix": "/api",
		"targetContainer": "foo}\nroute /pwn { * }",
	})
	code, data = c.request(t, http.MethodPut, "/api/domains/"+created.ID, poisonedUpdate)
	if code != http.StatusBadRequest {
		t.Fatalf("poisoned update: expected 400, got %d: %s", code, trunc(data))
	}
	final, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("read caddyfile: %v", err)
	}
	if strings.Contains(string(final), "route /pwn") {
		t.Fatalf("injected block reached the Caddyfile:\n%s", final)
	}

	// ——— 6. List responses never contain the password field ———

	code, data = c.get(t, "/api/domains")
	if code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", code, trunc(data))
	}
	if strings.Contains(string(data), "basicAuthPass") || strings.Contains(string(data), "s3cret") {
		t.Fatalf("list response must not leak passwords: %s", trunc(data))
	}
	if !strings.Contains(string(data), `"hasBasicAuth":true`) {
		t.Fatalf("list response must expose hasBasicAuth: %s", trunc(data))
	}
}
