package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopod/internal/services"
)

// This suite exercises the pod → Quadlet bundle end to end against the REAL
// (rootless) podman socket: a pod service plus two grouped children must
// persist the .pod unit AND one .container unit per child, every unit must
// pass the real `quadlet -dryrun`, and deleting each service must remove the
// units it owns (member units used to leak before the bundle was recorded on
// the service row).
//
// Opt-in (drives the host podman instance):
//
//	GOPOD_PODMAN_E2E=1 go test ./tests/integration/ -run TestPodPromotionQuadletE2E -v

func TestPodPromotionQuadletE2E(t *testing.T) {
	if !podmanE2EEnabled() {
		t.Skip("set GOPOD_PODMAN_E2E=1 with a reachable rootless podman socket to run")
	}

	// Quadlet units land in a scratch directory; the developer's real
	// ~/.config/containers/systemd is never read or written.
	quadletDir := t.TempDir()
	t.Setenv("QUADLET_DIR", quadletDir)

	dbPath := filepath.Join(t.TempDir(), "gopod-podpromo.db")
	handler := newIntegrationServer(t, dbPath, "http://127.0.0.1:9")
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client := loginAdmin(t, ts.URL)

	// ── Project + pod service + two attached children ───────────────────────
	code, data := client.post(t, "/api/projects", []byte(`{"name":"proj-podpromo","description":"Pod promotion bundle"}`))
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("create project: %d %s", code, trunc(data))
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &project); err != nil || project.ID == "" {
		t.Fatalf("project payload: %v / %s", err, trunc(data))
	}

	podBody, _ := json.Marshal(map[string]interface{}{
		"projectId": project.ID,
		"name":      "promo-stack",
		"type":      "pod",
		"port":      80,
		"hostPort":  18436,
	})
	code, data = client.post(t, "/api/services", podBody)
	if code != http.StatusCreated {
		t.Fatalf("create pod service: %d %s", code, trunc(data))
	}
	var podSvc struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &podSvc); err != nil || podSvc.ID == "" {
		t.Fatalf("pod service payload: %v / %s", err, trunc(data))
	}
	if podSvc.Type != "pod" {
		t.Fatalf("pod service type = %q (want pod)", podSvc.Type)
	}

	childIDs := make([]string, 0, 2)
	for _, child := range []struct{ name, image string }{
		{"promo-api", "docker.io/library/nginx:alpine"},
		{"promo-db", "docker.io/library/postgres:16-alpine"},
	} {
		body, _ := json.Marshal(map[string]interface{}{
			"projectId": project.ID,
			"name":      child.name,
			"type":      "image",
			"image":     child.image,
		})
		code, data = client.post(t, "/api/services", body)
		if code != http.StatusCreated {
			t.Fatalf("create child %s: %d %s", child.name, code, trunc(data))
		}
		var svc struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data, &svc); err != nil || svc.ID == "" {
			t.Fatalf("child payload: %v / %s", err, trunc(data))
		}
		childIDs = append(childIDs, svc.ID)
	}

	// ── Group both children onto the pod (no raw pod creation needed) ───────
	for _, childID := range childIDs {
		body, _ := json.Marshal(map[string]interface{}{
			"podName":   podSvc.ID,
			"createPod": false,
			"ports":     []string{},
		})
		code, data = client.post(t, "/api/services/"+childID+"/group-pod", body)
		if code != http.StatusOK && code != http.StatusCreated {
			t.Fatalf("group-pod %s: %d %s", childID, code, trunc(data))
		}
	}

	// ── Deploy the pod service ──────────────────────────────────────────────
	code, data = client.post(t, "/api/services/"+podSvc.ID+"/deploy", []byte(`{"trigger":"manual"}`))
	if code != http.StatusAccepted {
		t.Fatalf("pod deploy expected 202, got %d: %s", code, trunc(data))
	}
	deps := waitAllDeploymentsTerminal(t, client, podSvc.ID)
	if len(deps) == 0 {
		t.Fatal("no deployment records for the pod service")
	}
	if deps[0].Status != "success" {
		t.Fatalf("pod deploy status = %s (want success). Logs: %s", deps[0].Status, newestFailedLogs(t, client, deps))
	}

	// ── Assert the on-disk bundle: 1 .pod + 2 labeled .container ───────────
	deadline := time.Now().Add(20 * time.Second)
	var unitNames []string
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(quadletDir)
		if err != nil {
			t.Fatalf("read quadlet dir: %v", err)
		}
		unitNames = nil
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".pod") || strings.HasSuffix(e.Name(), ".container") {
				unitNames = append(unitNames, e.Name())
			}
		}
		if len(unitNames) == 3 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if len(unitNames) != 3 {
		t.Fatalf("expected pod + 2 member units on disk, got %v", unitNames)
	}

	files := make(map[string]string, len(unitNames))
	var podFile string
	for _, name := range unitNames {
		content, err := os.ReadFile(filepath.Join(quadletDir, name))
		if err != nil {
			t.Fatalf("read unit %s: %v", name, err)
		}
		files[name] = string(content)
		if strings.HasSuffix(name, ".pod") {
			podFile = string(content)
		}
	}
	if !strings.Contains(podFile, "PublishPort=18436:80") {
		t.Errorf(".pod must publish the pod service's port:\n%s", podFile)
	}
	for _, childID := range childIDs {
		found := false
		for _, content := range files {
			if strings.Contains(content, "Label=io.gopod.service="+childID) {
				found = true
				if !strings.Contains(content, "Pod=pod_"+podSvc.ID+".pod") {
					t.Errorf("member unit must reference Pod=pod_%s.pod:\n%s", podSvc.ID, content)
				}
				if strings.Contains(content, "PublishPort=") {
					t.Errorf("member unit must not publish ports:\n%s", content)
				}
			}
		}
		if !found {
			t.Errorf("no on-disk member unit carries Label=io.gopod.service=%s", childID)
		}
	}

	// Real `quadlet -dryrun`: an unsupported key aborts conversion, so this is
	// the authoritative check that every generated unit is installable.
	if valid, out, err := services.ValidateQuadletFiles(files); err != nil || !valid {
		t.Fatalf("on-disk bundle must pass quadlet dryrun: valid=%v err=%v out=%s", valid, err, out)
	}

	// ── Delete pod + children; every written unit must disappear ───────────
	for _, id := range append(childIDs, podSvc.ID) {
		code, data = client.request(t, http.MethodDelete, "/api/services/"+id, nil)
		if code != http.StatusOK {
			t.Fatalf("delete service %s: %d %s", id, code, trunc(data))
		}
	}
	for _, name := range unitNames {
		if _, err := os.Stat(filepath.Join(quadletDir, name)); err == nil {
			t.Errorf("unit %s survived service delete; member units must not leak", name)
		}
	}
}
