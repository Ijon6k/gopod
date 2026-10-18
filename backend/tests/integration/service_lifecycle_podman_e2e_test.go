package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"gopod/internal/server"
)

// This suite exercises the REAL service lifecycle HTTP API of the REAL working
// tree against the REAL (rootless) podman socket — deploy, status polling,
// stop/start, redeploy, token-authenticated webhook deploy, delete, and the
// deploy-in-progress conflict response — with unlabeled foreign containers
// present whose names collide with every name a label-free matcher would
// target:
//
//   - "<svc.Name>"                  (name equality)
//   - "<svc.ProjectID>-<svc.Name>"  (name prefix)
//
// The matching rules under test are strictly label-based; the decoys must
// survive every lifecycle operation untouched.
//
// Opt-in (drives the host podman instance):
//
//	GOPOD_PODMAN_E2E=1 go test ./tests/integration/ -run TestServiceLifecyclePodmanE2E -v

const (
	e2eProjectName = "proj-e2e"
	e2eSvcName     = "e2e-web"

	e2eDeployTimeout = 180 * time.Second
	e2ePollInterval  = 300 * time.Millisecond
)

// newIntegrationServer boots the full in-process backend (auth, projects,
// services, ingress reconciler, storage scheduler) the same way the real
// binary does, with a SQLite database in a temp directory.
func newIntegrationServer(t *testing.T, dbPath, caddyAdmin string) http.Handler {
	t.Helper()
	// The ingress reconciler resolves its Caddyfile path via os.UserHomeDir()
	// during construction; swap HOME just for this call so its writes never
	// touch the developer's real ~/.config/gopod/caddy/Caddyfile.
	realHome := os.Getenv("HOME")
	t.Setenv("HOME", t.TempDir())
	handler, err := server.NewServer(server.Config{
		Port:         "0",
		PodmanSocket: "", // podman client selects its default rootless socket
		DBPath:       dbPath,
		CaddyAdmin:   caddyAdmin, // unreachable: never contacts a live Caddy
		DistFS:       fstest.MapFS{},
	})
	if err != nil {
		t.Fatalf("failed to create integration server: %v", err)
	}
	if realHome != "" {
		_ = os.Setenv("HOME", realHome)
	}
	return handler
}

func podmanE2EEnabled() bool {
	if os.Getenv("GOPOD_PODMAN_E2E") != "1" {
		return false
	}
	if _, err := exec.LookPath("podman"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "podman", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func trunc(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 500 {
		return s[:500] + "…"
	}
	return s
}

// ————— authenticated API client —————

type e2eClient struct {
	base string
	http *http.Client
}

func newE2EClient(base string, timeout time.Duration) *e2eClient {
	jar, _ := cookiejar.New(nil)
	return &e2eClient{base: base, http: &http.Client{Jar: jar, Timeout: timeout}}
}

type e2eResp struct {
	Code int
	Body []byte
	Err  error
}

// do issues one request and never calls testing.T — safe inside goroutines.
func (c *e2eClient) do(method, path string, body []byte) e2eResp {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return e2eResp{Err: err}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return e2eResp{Err: err}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return e2eResp{Code: resp.StatusCode, Body: data}
}

func (c *e2eClient) post(t *testing.T, path string, body []byte) (int, []byte) {
	t.Helper()
	r := c.do(http.MethodPost, path, body)
	if r.Err != nil {
		t.Fatalf("POST %s: %v", path, r.Err)
	}
	return r.Code, r.Body
}

func (c *e2eClient) get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	r := c.do(http.MethodGet, path, nil)
	if r.Err != nil {
		t.Fatalf("GET %s: %v", path, r.Err)
	}
	return r.Code, r.Body
}

func (c *e2eClient) request(t *testing.T, method, path string, body []byte) (int, []byte) {
	t.Helper()
	r := c.do(method, path, body)
	if r.Err != nil {
		t.Fatalf("%s %s: %v", method, path, r.Err)
	}
	return r.Code, r.Body
}

// loginAdmin performs the first-run setup (creates admin + session cookie).
func loginAdmin(t *testing.T, baseURL string) *e2eClient {
	t.Helper()
	c := newE2EClient(baseURL, 120*time.Second)
	body, _ := json.Marshal(map[string]string{
		"email":    "e2e-admin@gopod.test",
		"password": "sup3r-secret-password",
		"name":     "E2E Admin",
	})
	code, data := c.post(t, "/api/auth/setup", body)
	if code != http.StatusCreated {
		t.Fatalf("expected setup 201, got %d: %s", code, trunc(data))
	}
	return c
}

// ————— podman CLI helpers (assertions run through the host CLI) —————

func podmanCLI(t *testing.T, timeout time.Duration, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "podman", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ownedContainerNames lists containers owned by the service via the ownership
// label — the same strict rule the deployer itself is required to follow.
func ownedContainerNames(t *testing.T, svcID string) []string {
	t.Helper()
	out, _ := podmanCLI(t, 15*time.Second,
		"ps", "-a",
		"--filter", "label=io.gopod.service="+svcID,
		"--format", "{{.Names}}")
	var names []string
	for _, n := range strings.Fields(strings.TrimSpace(out)) {
		names = append(names, strings.TrimPrefix(n, "/"))
	}
	return names
}

func containerRunning(t *testing.T, name string) bool {
	t.Helper()
	out, err := podmanCLI(t, 10*time.Second, "inspect", "--format", "{{.State.Running}}", name)
	if err != nil || strings.TrimSpace(out) == "" {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

func containerExists(t *testing.T, name string) bool {
	t.Helper()
	out, err := podmanCLI(t, 10*time.Second, "inspect", "--format", "ok", name)
	return err == nil && strings.TrimSpace(out) == "ok"
}

func mustRunning(t *testing.T, name string) {
	t.Helper()
	if !containerExists(t, name) {
		t.Errorf("decoy container %q must survive, but it was REMOVED", name)
		return
	}
	if !containerRunning(t, name) {
		t.Errorf("decoy container %q must still be RUNNING, but it is not", name)
	}
}

func runDecoy(t *testing.T, name, image string) {
	t.Helper()
	if containerExists(t, name) {
		_, _ = podmanCLI(t, 10*time.Second, "rm", "-f", name)
	}
	// Intentionally unlabeled: ownership checks must ignore unlabeled containers.
	out, err := podmanCLI(t, 40*time.Second,
		"run", "-d",
		"--entrypoint", "/bin/sh",
		"--name", name, image, "-c", "sleep 600")
	if err != nil {
		t.Fatalf("cannot create decoy %q: %v / %s", name, err, trunc([]byte(out)))
	}
	t.Cleanup(func() {
		_, _ = podmanCLI(t, 20*time.Second, "rm", "-f", name)
	})
}

// ————— deployment record polling —————

type deploymentEntry struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Trigger string `json:"trigger"`
}

func deployTerminal(status string) bool {
	return status != "building" && status != "deploying" && status != "running"
}

// waitAllDeploymentsTerminal polls until every visible deployment for the
// service is in a terminal state, then returns the full list.
func waitAllDeploymentsTerminal(t *testing.T, c *e2eClient, svcID string) []deploymentEntry {
	t.Helper()
	deadline := time.Now().Add(e2eDeployTimeout)
	var last []deploymentEntry
	for time.Now().Before(deadline) {
		code, data := c.get(t, "/api/services/"+svcID+"/deployments")
		if code != http.StatusOK {
			t.Fatalf("GET deployments: %d %s", code, trunc(data))
		}
		var deps []deploymentEntry
		if err := json.Unmarshal(data, &deps); err != nil {
			t.Fatalf("deployment JSON parse: %v / %s", err, trunc(data))
		}
		if len(deps) > 0 {
			last = deps
			allTerminal := true
			for _, d := range deps {
				if !deployTerminal(d.Status) {
					allTerminal = false
					break
				}
			}
			if allTerminal {
				return deps
			}
		}
		time.Sleep(e2ePollInterval)
	}
	t.Fatalf("deployments did not reach an all-terminal state in time; last=%+v", last)
	return nil
}

// listDeployments reads the raw deployment list once.
func listDeployments(t *testing.T, c *e2eClient, svcID string) []deploymentEntry {
	t.Helper()
	code, data := c.get(t, "/api/services/"+svcID+"/deployments")
	if code != http.StatusOK {
		t.Fatalf("GET deployments: %d %s", code, trunc(data))
	}
	var deps []deploymentEntry
	if err := json.Unmarshal(data, &deps); err != nil {
		t.Fatalf("deployment JSON parse: %v / %s", err, trunc(data))
	}
	return deps
}

// assertExactlyOneOwnedRunning polls until exactly one labeled container owns
// the service AND it runs, retrying across the replace window (old container
// removed, replacement created).
func assertExactlyOneOwnedRunning(t *testing.T, svcID string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		owned := ownedContainerNames(t, svcID)
		if len(owned) == 1 && containerRunning(t, owned[0]) {
			return
		}
		time.Sleep(e2ePollInterval)
	}
	t.Fatalf("expected exactly 1 owned running container for %s within deadline; owned=%v",
		svcID, ownedContainerNames(t, svcID))
}

func newestFailedLogs(t *testing.T, c *e2eClient, deps []deploymentEntry) string {
	t.Helper()
	for _, d := range deps {
		if d.Status != "success" {
			code, data := c.get(t, "/api/deployments/"+d.ID+"/logs")
			if code == http.StatusOK && len(data) > 0 && strings.TrimSpace(string(data)) != "[]" {
				return trunc(data)
			}
		}
	}
	return "(no deployment logs retrieved)"
}

// ————— the actual end-to-end scenario —————

func TestServiceLifecyclePodmanE2E(t *testing.T) {
	if !podmanE2EEnabled() {
		t.Skip("set GOPOD_PODMAN_E2E=1 with a reachable rootless podman socket to run")
	}

	// Caddy Admin pointed at an unreachable loopback port: the background
	// reconcile writes its config inside the temporary HOME and leaves the
	// developer's running Caddy completely alone.
	dbPath := filepath.Join(t.TempDir(), "gopod.db")
	handler := newIntegrationServer(t, dbPath, "http://127.0.0.1:9")
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client := loginAdmin(t, ts.URL)

	// ── Project + service bootstrap ──────────────────────────────────────────
	code, data := client.post(t, "/api/projects", []byte(`{"name":"proj-e2e","description":"E2E lifecycle test project"}`))
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("create project: %d %s", code, trunc(data))
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &project); err != nil || project.ID == "" {
		t.Fatalf("project payload incomplete: %v / %s", err, trunc(data))
	}

	// A free host port keeps the published port collision-free.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no free port: %v", err)
	}
	hostPort := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	svcBody, _ := json.Marshal(map[string]interface{}{
		"projectId":     project.ID,
		"name":          e2eSvcName,
		"type":          "image",
		"image":         "docker.io/library/nginx:alpine",
		"port":          80,
		"hostPort":      hostPort,
		"restartPolicy": "no",
		"envVars": []map[string]interface{}{
			{"key": "NGINX_HOST", "value": "e2e.local", "secret": false},
		},
	})
	code, data = client.post(t, "/api/services", svcBody)
	if code != http.StatusCreated {
		t.Fatalf("create service: %d %s", code, trunc(data))
	}
	var svc struct {
		ID           string `json:"id"`
		WebhookToken string `json:"webhookToken"`
	}
	if err := json.Unmarshal(data, &svc); err != nil || strings.TrimSpace(svc.ID) == "" {
		t.Fatalf("service payload incomplete: %v / %s", err, trunc(data))
	}
	if !strings.HasPrefix(svc.WebhookToken, "wh_") {
		t.Errorf("service should have a webhook token assigned at creation, got %q", svc.WebhookToken)
	}
	// Best-effort cleanup: delete the service (which tears down its containers)
	// even when an assertion fails midway through the suite.
	t.Cleanup(func() {
		req, err := http.NewRequest(http.MethodDelete,
			ts.URL+"/api/services/"+svc.ID+"?delete_volumes=true", nil)
		if err == nil {
			if resp, err := client.http.Do(req); err == nil {
				resp.Body.Close()
			}
		}
	})

	// ── Decoys (names a label-free matcher would target) ────────────────────
	decoyOne := e2eSvcName                    // collides on exact name
	decoyTwo := project.ID + "-" + e2eSvcName // collides on name prefix
	runDecoy(t, decoyOne, "docker.io/library/alpine:latest")
	runDecoy(t, decoyTwo, "docker.io/library/alpine:latest")

	// ── Deploy #1 + 409 conflict hammer ─────────────────────────────────────
	deployBody := []byte(`{"trigger":"manual"}`)
	code, data = client.post(t, "/api/services/"+svc.ID+"/deploy", deployBody)
	if code != http.StatusAccepted {
		t.Fatalf("deploy#1 expected 202, got %d: %s", code, trunc(data))
	}

	// The per-service deploy lock is held from StartDeploy until
	// executeDeployment finishes, so requests fired immediately after the 202
	// response must conflict with 409 instead of interleaving.
	var once sync.Once
	saw409 := false
	hammerStop := make(chan struct{})
	go func() {
		for counter := 0; counter < 12; counter++ {
			select {
			case <-hammerStop:
				return
			default:
			}
			r := client.do(http.MethodPost, "/api/services/"+svc.ID+"/deploy", deployBody)
			if r.Err != nil {
				counter++
				continue
			}
			switch r.Code {
			case http.StatusConflict:
				saw409 = true
				once.Do(func() { close(hammerStop) })
				return
			case http.StatusAccepted:
				// Defensive: a hammer hit landing after deploy#1 finished may
				// start an extra deployment. waitAllDeploymentsTerminal below
				// settles every visible deployment before asserts resume.
			}
			time.Sleep(25 * time.Millisecond)
		}
	}()

	deps := waitAllDeploymentsTerminal(t, client, svc.ID)
	once.Do(func() { close(hammerStop) })
	if len(deps) == 0 {
		t.Fatal("no deployment records visible")
	}
	if deps[0].Status != "success" {
		t.Fatalf("deploy#1 status = %s (want success). Logs: %s", deps[0].Status, newestFailedLogs(t, client, deps))
	}
	if !saw409 {
		t.Errorf("expected a concurrent POST /deploy during deploy#1 to be rejected with 409 Conflict")
	} else {
		t.Logf("✓ concurrent deploy during deploy#1 was rejected with 409 Conflict")
	}

	// Exactly one owned container must exist (running) after deploy #1.
	owned := ownedContainerNames(t, svc.ID)
	if len(owned) != 1 {
		t.Errorf("after deploy#1 expected exactly 1 owned container, got %v", owned)
	}
	for _, n := range owned {
		if !containerRunning(t, n) {
			t.Errorf("owned container %q should be running after deploy#1", n)
		}
	}
	mustRunning(t, decoyOne)
	mustRunning(t, decoyTwo)

	// ── Stop: must touch ONLY the owned container ────────────────────────────
	code, data = client.post(t, "/api/services/"+svc.ID+"/stop", []byte(`{}`))
	if code != http.StatusOK {
		t.Fatalf("stop: %d %s", code, trunc(data))
	}
	owned = ownedContainerNames(t, svc.ID)
	for _, n := range owned {
		if containerRunning(t, n) {
			t.Errorf("owned container %q still running after stop", n)
		}
	}
	mustRunning(t, decoyOne)
	mustRunning(t, decoyTwo)

	// ── Start: owned container serving again, decoys untouched ──────────────
	code, data = client.post(t, "/api/services/"+svc.ID+"/start", []byte(`{}`))
	if code != http.StatusOK {
		t.Fatalf("start: %d %s", code, trunc(data))
	}
	owned = ownedContainerNames(t, svc.ID)
	if len(owned) != 1 {
		t.Errorf("after start expected exactly 1 owned container, got %v", owned)
	}
	for _, n := range owned {
		if !containerRunning(t, n) {
			t.Errorf("owned container %q should be running after start", n)
		}
	}
	mustRunning(t, decoyOne)
	mustRunning(t, decoyTwo)

	// ── Deploy #2 (redeploy): predecessor retired, exactly one survivor ─────
	code, data = client.post(t, "/api/services/"+svc.ID+"/deploy", deployBody)
	if code != http.StatusAccepted {
		t.Fatalf("deploy#2 expected 202, got %d: %s", code, trunc(data))
	}
	deps = waitAllDeploymentsTerminal(t, client, svc.ID)
	if newest := deps[0]; newest.Status != "success" {
		t.Fatalf("deploy#2 status = %s (want success). Logs: %s", newest.Status, newestFailedLogs(t, client, deps))
	}
	owned = ownedContainerNames(t, svc.ID)
	if len(owned) != 1 {
		t.Errorf("after redeploy expected exactly 1 owned container, got %v", owned)
	}
	for _, n := range owned {
		if !containerRunning(t, n) {
			t.Errorf("owned container %q should be running after redeploy", n)
		}
	}
	mustRunning(t, decoyOne)
	mustRunning(t, decoyTwo)

	// ── Webhook deploy (token-authenticated, no session) ────────────────────
	// The 202 response returns before the deployment record is materialized;
	// wait explicitly for a terminal trigger=webhook deployment instead of
	// racing the list.
	whResp := client.do(http.MethodPost, "/api/deploy/webhook/"+svc.WebhookToken, []byte(`{"ref":"refs/heads/main"}`))
	if whResp.Err != nil {
		t.Fatalf("webhook call failed: %v", whResp.Err)
	}
	if whResp.Code != http.StatusAccepted {
		t.Fatalf("webhook deploy expected 202, got %d: %s", whResp.Code, trunc(whResp.Body))
	}
	mustRunning(t, decoyOne)
	mustRunning(t, decoyTwo)

	var whDep *deploymentEntry
	whDeadline := time.Now().Add(e2eDeployTimeout)
	for whDep == nil && time.Now().Before(whDeadline) {
		deps := listDeployments(t, client, svc.ID)
		for i := range deps {
			if deps[i].Trigger == "webhook" {
				d := deps[i]
				whDep = &d
				break
			}
		}
		if whDep == nil || !deployTerminal(whDep.Status) {
			whDep = nil
			time.Sleep(e2ePollInterval)
		}
	}
	if whDep == nil {
		t.Fatalf("no trigger=webhook deployment appeared in time; last list: %+v",
			listDeployments(t, client, svc.ID))
	}
	if whDep.Status != "success" {
		t.Fatalf("deploy#3 (webhook) status = %s (want success). Logs: %s",
			whDep.Status, newestFailedLogs(t, client, listDeployments(t, client, svc.ID)))
	}
	assertExactlyOneOwnedRunning(t, svc.ID)
	mustRunning(t, decoyOne)
	mustRunning(t, decoyTwo)

	// ── Webhook with a bogus token must NOT deploy ───────────────────────────
	code, data = client.request(t, http.MethodPost, "/api/deploy/webhook/not-a-real-token", []byte(`{}`))
	if code != http.StatusUnauthorized {
		t.Errorf("webhook with bogus token expected 401, got %d: %s", code, trunc(data))
	}

	// ── Delete (teardown): owned containers gone, decoys SURVIVE ─────────────
	code, data = client.request(t, http.MethodDelete, "/api/services/"+svc.ID+"?delete_volumes=true", nil)
	if code != http.StatusOK {
		t.Fatalf("delete service: %d %s", code, trunc(data))
	}
	if left := ownedContainerNames(t, svc.ID); len(left) != 0 {
		t.Errorf("teardown must remove every owned container, left: %v", left)
	}
	mustRunning(t, decoyOne)
	mustRunning(t, decoyTwo)

	// Stop the decoys so the host is not left with sleepers (t.Cleanup will
	// still force-remove their records afterwards).
	_, _ = podmanCLI(t, 20*time.Second, "kill", decoyOne)
	_, _ = podmanCLI(t, 20*time.Second, "kill", decoyTwo)

	t.Logf("✓ full lifecycle + label-only guarantee verified for svc=%s decoys=[%s %s]",
		svc.ID, decoyOne, decoyTwo)
}
