package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRuntimeActionsPodmanE2E drives the Runtime page's action endpoints
// (start/stop/restart/delete/logs for containers and start/stop/delete for
// pods) against the REAL rootless podman clockwork via the full in-process
// server, then verifies the image/volume/network delete endpoints' refusal
// semantics. A foreign unlabeled decoy must survive every runtime action.
//
// Opt-in (drives the host podman instance):
//
//	GOPOD_PODMAN_E2E=1 go test ./tests/integration/ -run TestRuntimeActionsPodmanE2E -v
func TestRuntimeActionsPodmanE2E(t *testing.T) {
	if !podmanE2EEnabled() {
		t.Skip("set GOPOD_PODMAN_E2E=1 with a reachable rootless podman socket to run")
	}

	dbPath := filepath.Join(t.TempDir(), "gopod.db")
	handler := newIntegrationServer(t, dbPath, "http://127.0.0.1:9")
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client := loginAdmin(t, ts.URL)

	// ── Project + scratch service; deploy once so a labeled container exists ──
	code, data := client.post(t, "/api/projects", []byte(`{"name":"proj-e2e-runt","description":"Runtime actions test project"}`))
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
		"name":          "e2e-runt-web",
		"type":          "image",
		"image":         "docker.io/library/nginx:alpine",
		"port":          80,
		"restartPolicy": "no",
	})
	code, data = client.post(t, "/api/services", svcBody)
	if code != http.StatusCreated {
		t.Fatalf("create service: %d %s", code, trunc(data))
	}
	var svc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &svc); err != nil || strings.TrimSpace(svc.ID) == "" {
		t.Fatalf("service payload incomplete: %v / %s", err, trunc(data))
	}
	t.Cleanup(func() {
		req, err := http.NewRequest(http.MethodDelete,
			ts.URL+"/api/services/"+svc.ID+"?delete_volumes=true", nil)
		if err == nil {
			if resp, err := client.http.Do(req); err == nil {
				resp.Body.Close()
			}
		}
	})

	code, data = client.post(t, "/api/services/"+svc.ID+"/deploy", []byte(`{"trigger":"manual"}`))
	if code != http.StatusAccepted {
		t.Fatalf("deploy expected 202, got %d: %s", code, trunc(data))
	}
	deps := waitAllDeploymentsTerminal(t, client, svc.ID)
	if len(deps) == 0 || deps[0].Status != "success" {
		t.Fatalf("deploy status = %v (want success). Logs: %s", deps, newestFailedLogs(t, client, deps))
	}

	// A foreign unlabeled decoy must survive every runtime action below.
	decoy := "e2e-runt-decoy"
	runDecoy(t, decoy, "docker.io/library/alpine:latest")

	// Find the service's labeled container via the runtime listing API.
	code, data = client.get(t, "/api/containers")
	if code != http.StatusOK {
		t.Fatalf("GET /api/containers: %d %s", code, trunc(data))
	}
	var listed []map[string]any
	if err := json.Unmarshal(data, &listed); err != nil {
		t.Fatalf("containers JSON: %v / %s", err, trunc(data))
	}
	var targetID string
	var targetName string
	for _, row := range listed {
		labels, _ := row["labels"].(map[string]any)
		if labels != nil && labels["io.gopod.service"] == svc.ID {
			if id, ok := row["id"].(string); ok && id != "" {
				targetID = id
				if names, ok := row["names"].([]any); ok && len(names) > 0 {
					if n, ok := names[0].(string); ok {
						targetName = strings.TrimPrefix(n, "/")
					}
				}
			}
			break
		}
	}
	if targetID == "" {
		t.Fatal("no labeled owned container found in /api/containers after a successful deploy")
	}

	// ── Auth middleware fight: a fresh client has no session; destructive and
	// read routes must refuse it with 401.
	anon := newE2EClient(ts.URL, 30*time.Second)
	if r := anon.do(http.MethodGet, "/api/containers", nil); r.Err == nil && r.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/containers without a session expected 401, got %d", r.Code)
	}
	if r := anon.do(http.MethodPost, "/api/containers/"+targetID+"/stop", []byte(`{}`)); r.Err == nil && r.Code != http.StatusUnauthorized {
		t.Errorf("POST /stop without a session expected 401, got %d", r.Code)
	}

	// ── Stop via the runtime endpoint: only the owned container halts ────────
	code, data = client.post(t, "/api/containers/"+targetID+"/stop", []byte(`{}`))
	if code != http.StatusOK {
		t.Fatalf("runtime stop: %d %s", code, trunc(data))
	}
	if containerRunning(t, targetName) {
		t.Errorf("owned container %q still running after runtime stop", targetName)
	}
	mustRunning(t, decoy)

	// ── Start via the runtime endpoint ───────────────────────────────────────
	code, data = client.post(t, "/api/containers/"+targetID+"/start", []byte(`{}`))
	if code != http.StatusOK {
		t.Fatalf("runtime start: %d %s", code, trunc(data))
	}
	if !containerRunning(t, targetName) {
		t.Errorf("owned container %q should run after runtime start", targetName)
	}
	mustRunning(t, decoy)

	// ── Restart via the runtime endpoint ─────────────────────────────────────
	code, data = client.post(t, "/api/containers/"+targetID+"/restart", []byte(`{}`))
	if code != http.StatusOK {
		t.Fatalf("runtime restart: %d %s", code, trunc(data))
	}
	if !containerRunning(t, targetName) {
		t.Errorf("owned container %q should run after runtime restart", targetName)
	}
	mustRunning(t, decoy)

	// ── Logs via the runtime endpoint: a tail fetch returns the string field ─
	code, data = client.get(t, "/api/containers/"+targetID+"/logs?tail=50")
	if code != http.StatusOK {
		t.Fatalf("runtime logs: %d %s", code, trunc(data))
	}
	var logsResp struct {
		Logs string `json:"logs"`
	}
	if err := json.Unmarshal(data, &logsResp); err != nil {
		t.Fatalf("logs JSON: %v / %s", err, trunc(data))
	}

	// ── Delete via the runtime endpoint; the row leaves the owned set ────────
	code, data = client.request(t, http.MethodDelete, "/api/containers/"+targetID+"?force=true", nil)
	if code != http.StatusOK {
		t.Fatalf("runtime delete: %d %s", code, trunc(data))
	}
	if left := ownedContainerNames(t, svc.ID); len(left) != 0 {
		t.Errorf("owned container set must be empty after runtime delete, left: %v", left)
	}
	if containerExists(t, targetName) {
		t.Errorf("owned container %q must be gone after runtime delete", targetName)
	}
	mustRunning(t, decoy)

	// ── Idempotent tease: deleting a stale id must not 500 ────────────────────
	code, data = client.request(t, http.MethodDelete, "/api/containers/"+targetID+"?force=true", nil)
	if code != http.StatusOK {
		t.Errorf("stale runtime delete expected idempotent 200, got %d: %s", code, trunc(data))
	}
	mustRunning(t, decoy)

	// ── Pods: create → start → stop → delete through the runtime endpoints ───
	podBody, _ := json.Marshal(map[string]interface{}{
		"name":    "e2e-runt-pod",
		"ports":   []string{},
		"network": "gopod-net",
	})
	code, data = client.post(t, "/api/pods", podBody)
	if code != http.StatusCreated {
		t.Fatalf("create pod: %d %s", code, trunc(data))
	}
	var pod struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &pod); err != nil || pod.ID == "" {
		t.Fatalf("pod payload incomplete: %v / %s", err, trunc(data))
	}
	t.Cleanup(func() {
		_, _ = podmanCLI(t, 20*time.Second, "pod", "rm", "-f", "e2e-runt-pod")
	})

	code, data = client.post(t, "/api/pods/"+pod.ID+"/start", []byte(`{}`))
	if code != http.StatusOK {
		t.Fatalf("runtime pod start: %d %s", code, trunc(data))
	}
	out, err := podmanCLI(t, 10*time.Second, "pod", "inspect", "--format", "{{.State}}", "e2e-runt-pod")
	if err != nil || strings.TrimSpace(out) != "Running" {
		t.Errorf("pod should be Running after runtime start, got %q (err=%v)", strings.TrimSpace(out), err)
	}

	code, data = client.post(t, "/api/pods/"+pod.ID+"/stop", []byte(`{}`))
	if code != http.StatusOK {
		t.Fatalf("runtime pod stop: %d %s", code, trunc(data))
	}
	out, err = podmanCLI(t, 10*time.Second, "pod", "inspect", "--format", "{{.State}}", "e2e-runt-pod")
	if err != nil || strings.TrimSpace(out) == "Running" {
		t.Errorf("pod should not be Running after runtime stop, got %q (err=%v)", strings.TrimSpace(out), err)
	}

	code, data = client.request(t, http.MethodDelete, "/api/pods/"+pod.ID, nil)
	if code != http.StatusOK {
		t.Fatalf("runtime pod delete: %d %s", code, trunc(data))
	}
	if _, existsErr := podmanCLI(t, 10*time.Second, "pod", "exists", "e2e-runt-pod"); existsErr == nil {
		t.Errorf("pod should be gone after runtime delete")
	}
	mustRunning(t, decoy)

	// ── Managed network refusal: 409, no mutation, detailed reason ────────────
	code, data = client.request(t, http.MethodDelete, "/api/networks/gopod-net", nil)
	if code != http.StatusConflict {
		t.Errorf("delete gopod-net expected 409 Conflict, got %d: %s", code, trunc(data))
	}
	if !strings.Contains(strings.ToLower(string(data)), "gopod-net") {
		t.Errorf("refusal body should name the managed network: %s", trunc(data))
	}
	if netOut, netErr := podmanCLI(t, 10*time.Second, "network", "inspect", "gopod-net"); netErr != nil {
		t.Errorf("gopod-net must survive the refused delete (err=%v, out=%s)", netErr, trunc([]byte(netOut)))
	}

	// ── Unknown network delete is idempotent (stale-list tolerance) ───────────
	code, data = client.request(t, http.MethodDelete, "/api/networks/e2e-runt-no-such-net", nil)
	if code != http.StatusOK {
		t.Errorf("delete of an absent network expected idempotent 200, got %d: %s", code, trunc(data))
	}

	// ── Unused volume delete: the only fully honest success path here ─────────
	if _, volErr := podmanCLI(t, 20*time.Second, "volume", "create", "e2e-runt-vol"); volErr != nil {
		t.Fatalf("cannot create scratch volume: %v", volErr)
	}
	t.Cleanup(func() { _, _ = podmanCLI(t, 20*time.Second, "volume", "rm", "-f", "e2e-runt-vol") })
	code, data = client.request(t, http.MethodDelete, "/api/volumes/e2e-runt-vol", nil)
	if code != http.StatusOK {
		t.Fatalf("delete unused volume: %d %s", code, trunc(data))
	}
	if _, inspectErr := podmanCLI(t, 10*time.Second, "volume", "inspect", "e2e-runt-vol"); inspectErr == nil {
		t.Errorf("scratch volume must be gone after runtime delete")
	}

	// ── Image in-use refusal: the decoy holds alpine; the endpoint must pass
	// the engine's refusal through without touching the image ──────────────────
	alpineID, err := podmanCLI(t, 10*time.Second, "images", "-q", "docker.io/library/alpine:latest")
	if err != nil || strings.TrimSpace(alpineID) == "" {
		t.Logf("skip image refusal check: alpine image unavailable (%s)", trunc([]byte(alpineID)))
	} else {
		imageRef := strings.Fields(strings.TrimSpace(alpineID))[0]
		code, data = client.request(t, http.MethodDelete, "/api/images/"+imageRef, nil)
		if code == http.StatusOK {
			t.Errorf("image in use by a running decoy must be refused, got 200 OK (%s)", trunc(data))
		}
		if !containerExists(t, decoy) || !containerRunning(t, decoy) {
			t.Errorf("in-use image refusal must not disturb the decoy container")
		}
	}

	// Stop the decoy so the host is not left with sleepers (t.Cleanup removes it).
	_, _ = podmanCLI(t, 20*time.Second, "kill", decoy)

	t.Logf("✓ runtime container actions + pod actions + resource refusals verified; decoy %s untouched", decoy)
}
