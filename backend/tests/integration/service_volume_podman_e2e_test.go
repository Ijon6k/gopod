package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This suite proves the ownership-labeling of explicitly named volumes end to
// end: a service that mounts a named volume must have that volume labeled at
// creation time (so it can be matched later), and teardown with
// delete_volumes=true must remove it via the label match.
//
// Opt-in:
//
//	GOPOD_PODMAN_E2E=1 go test ./tests/integration/ -run TestServiceVolumeLabelsPodmanE2E -v

func TestServiceVolumeLabelsPodmanE2E(t *testing.T) {
	if !podmanE2EEnabled() {
		t.Skip("set GOPOD_PODMAN_E2E=1 with a reachable rootless podman socket to run")
	}

	t.Setenv("GOPOD_GC_DISABLE", "1")

	dbPath := filepath.Join(t.TempDir(), "gopod-vol.db")
	handler := newIntegrationServer(t, dbPath, "http://127.0.0.1:9")
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client := loginAdmin(t, ts.URL)

	code, data := client.post(t, "/api/projects", []byte(`{"name":"proj-vol","description":"volume label test"}`))
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("create project: %d %s", code, trunc(data))
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &project); err != nil || project.ID == "" {
		t.Fatalf("project payload: %v / %s", err, trunc(data))
	}

	volName := fmt.Sprintf("gopod-e2e-vol-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = podmanCLI(t, 20*time.Second, "volume", "rm", "-f", volName)
	})

	svcBody, _ := json.Marshal(map[string]interface{}{
		"projectId":     project.ID,
		"name":          "vol-web",
		"type":          "image",
		"image":         "docker.io/library/nginx:alpine",
		"restartPolicy": "no",
		"advanced": map[string]interface{}{
			"storage": map[string]interface{}{
				"volumes": []map[string]string{
					{"source": volName, "target": "/data"},
				},
			},
		},
	})
	code, data = client.post(t, "/api/services", svcBody)
	if code != http.StatusCreated {
		t.Fatalf("create service: %d %s", code, trunc(data))
	}
	var svc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &svc); err != nil || svc.ID == "" {
		t.Fatalf("service payload: %v / %s", err, trunc(data))
	}
	t.Cleanup(func() {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/services/"+svc.ID+"?delete_volumes=true", nil)
		if err == nil {
			if resp, err := client.http.Do(req); err == nil {
				resp.Body.Close()
			}
		}
	})

	// ── Deploy ───────────────────────────────────────────────────────────────
	code, data = client.post(t, "/api/services/"+svc.ID+"/deploy", []byte(`{"trigger":"manual"}`))
	if code != http.StatusAccepted {
		t.Fatalf("deploy expected 202, got %d: %s", code, trunc(data))
	}
	deps := waitAllDeploymentsTerminal(t, client, svc.ID)
	if len(deps) == 0 || deps[0].Status != "success" {
		t.Fatalf("deploy status = %v (want success). Logs: %s", deps, newestFailedLogs(t, client, deps))
	}

	// ── The named volume must exist AND carry ownership labels ───────────────
	deadline := time.Now().Add(20 * time.Second)
	var labels map[string]string
	for time.Now().Before(deadline) {
		out, err := podmanCLI(t, 10*time.Second, "volume", "inspect", volName, "--format", "{{json .Labels}}")
		if err == nil && strings.TrimSpace(out) != "" {
			_ = json.Unmarshal([]byte(strings.TrimSpace(out)), &labels)
			if labels["io.gopod.service"] == svc.ID {
				break
			}
		}
		time.Sleep(e2ePollInterval)
	}
	if labels["io.gopod.service"] != svc.ID {
		t.Errorf("named volume %q must be labeled io.gopod.service=%s, got %v", volName, svc.ID, labels)
	}
	if labels["io.gopod.project"] != project.ID {
		t.Errorf("named volume %q must be labeled io.gopod.project=%s, got %v", volName, project.ID, labels)
	}

	// ── Teardown with delete_volumes must remove it via the label match ──────
	code, data = client.request(t, http.MethodDelete, "/api/services/"+svc.ID+"?delete_volumes=true", nil)
	if code != http.StatusOK {
		t.Fatalf("delete service: %d %s", code, trunc(data))
	}
	goneDeadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(goneDeadline) {
		out, err := podmanCLI(t, 10*time.Second, "volume", "inspect", volName, "--format", "{{.Name}}")
		if err != nil || strings.TrimSpace(out) == "" {
			t.Logf("✓ named volume %q labeled at creation and removed at teardown", volName)
			return
		}
		time.Sleep(e2ePollInterval)
	}
	t.Errorf("named volume %q should have been removed by teardown (delete_volumes=true)", volName)
}
