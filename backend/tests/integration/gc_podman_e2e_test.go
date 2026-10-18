package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopod/internal/podman"
)

// This suite exercises the real cleanup engine against the real (rootless)
// podman socket. Only genuinely safe operations are driven:
//
//   - GET  /api/system/gc/preview          (read-only inventory)
//   - GET  /api/system/gc/preview          (orphan volume surfaced from a real volume)
//   - POST /api/system/gc {scope:"units"}  (deletes ONLY stale quadlet files in a temp dir)
//   - POST /api/system/gc {scope:"bogus"}  (must be rejected)
//   - podman.Client.CreateVolume           (label round-trip on a throwaway volume)
//
// Destructive image/container prunes are deliberately NOT run here: they would
// touch the developer's real storage. Their decision logic is covered by unit
// tests with fixtures.
//
// Opt-in:
//
//	GOPOD_PODMAN_E2E=1 go test ./tests/integration/ -run TestGCEnginePodmanE2E -v

func TestGCEnginePodmanE2E(t *testing.T) {
	if !podmanE2EEnabled() {
		t.Skip("set GOPOD_PODMAN_E2E=1 with a reachable rootless podman socket to run")
	}

	// Keep the nightly scheduler inert for the duration of the test.
	t.Setenv("GOPOD_GC_DISABLE", "1")

	// Quadlet scanning is redirected to a scratch directory so the developer's
	// real ~/.config/containers/systemd is never read or written.
	quadletDir := t.TempDir()
	t.Setenv("QUADLET_DIR", quadletDir)

	dbPath := filepath.Join(t.TempDir(), "gopod-gc.db")
	handler := newIntegrationServer(t, dbPath, "http://127.0.0.1:9")
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client := loginAdmin(t, ts.URL)

	// A real project + service so the DB knows exactly one service ID.
	code, data := client.post(t, "/api/projects", []byte(`{"name":"proj-gc","description":"GC engine test"}`))
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("create project: %d %s", code, trunc(data))
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &project); err != nil || project.ID == "" {
		t.Fatalf("project payload: %v / %s", err, trunc(data))
	}
	svcBody, _ := json.Marshal(map[string]interface{}{
		"projectId": project.ID,
		"name":      "gc-known",
		"type":      "image",
		"image":     "docker.io/library/nginx:alpine",
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

	// ── Preview (read-only) ──────────────────────────────────────────────────
	code, data = client.get(t, "/api/system/gc/preview")
	if code != http.StatusOK {
		t.Fatalf("gc preview: %d %s", code, trunc(data))
	}
	var preview struct {
		Busy              bool     `json:"busy"`
		DanglingImages    int      `json:"danglingImages"`
		StoppedContainers int      `json:"stoppedContainers"`
		StaleUnitFiles    []string `json:"staleUnitFiles"`
		OrphanVolumes     []string `json:"orphanVolumes"`
		ReclaimableBytes  int64    `json:"reclaimableBytes"`
	}
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatalf("preview parse: %v / %s", err, trunc(data))
	}
	if preview.Busy {
		t.Errorf("preview should report busy=false when no deployment is running")
	}
	if preview.DanglingImages < 0 || preview.ReclaimableBytes < 0 {
		t.Errorf("preview must return well-formed counters: %+v", preview)
	}
	t.Logf("✓ preview: dangling=%d stopped=%d reclaimable=%d staleUnits=%v",
		preview.DanglingImages, preview.StoppedContainers, preview.ReclaimableBytes, preview.StaleUnitFiles)

	// ── Real labeled volume: create → label round-trip → orphan preview ──────
	volName := fmt.Sprintf("gopod-gc-it-%d", time.Now().UnixNano())
	pclient := podman.NewClient("")
	if pclient == nil {
		t.Fatal("podman client unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := pclient.CreateVolume(ctx, volName, map[string]string{
		"io.gopod.service": "svc-definitely-absent",
		"io.gopod.project": "proj-gc",
	}); err != nil {
		t.Fatalf("CreateVolume: %v", err)
	}
	t.Cleanup(func() { _ = pclient.DeleteVolume(context.Background(), volName, true) })

	// CreateVolume is idempotent: a second call on an existing volume must not error.
	if err := pclient.CreateVolume(ctx, volName, map[string]string{"io.gopod.service": "svc-definitely-absent"}); err != nil {
		t.Errorf("CreateVolume must be idempotent on an existing volume, got %v", err)
	}

	vols, err := pclient.GetVolumes(ctx)
	if err != nil {
		t.Fatalf("GetVolumes: %v", err)
	}
	labeled := false
	for _, v := range vols {
		if v.Name == volName {
			labeled = v.Labels["io.gopod.service"] == "svc-definitely-absent" &&
				v.Labels["io.gopod.project"] == "proj-gc"
		}
	}
	if !labeled {
		t.Errorf("created volume %q must carry the ownership labels", volName)
	}

	code, data = client.get(t, "/api/system/gc/preview")
	if code != http.StatusOK {
		t.Fatalf("gc preview #2: %d %s", code, trunc(data))
	}
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatalf("preview parse #2: %v", err)
	}
	foundOrphan := false
	for _, n := range preview.OrphanVolumes {
		if n == volName {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Errorf("preview must list the orphaned volume %q; got %v", volName, preview.OrphanVolumes)
	}

	// ── Units scope: stale file removed, live file kept ──────────────────────
	staleFile := filepath.Join(quadletDir, "gc-stale.container")
	if err := os.WriteFile(staleFile, []byte("[Container]\nImage=nginx\nLabel=io.gopod.service=svc-stale-gone\n"), 0o644); err != nil {
		t.Fatalf("write stale unit: %v", err)
	}
	keepFile := filepath.Join(quadletDir, "gc-keep.container")
	if err := os.WriteFile(keepFile, []byte(fmt.Sprintf("[Container]\nImage=nginx\nLabel=io.gopod.service=%s\n", svc.ID)), 0o644); err != nil {
		t.Fatalf("write kept unit: %v", err)
	}

	code, data = client.post(t, "/api/system/gc", []byte(`{"scope":"units"}`))
	if code != http.StatusOK {
		t.Fatalf("gc units run: %d %s", code, trunc(data))
	}
	var runResult struct {
		Scope            string   `json:"scope"`
		Skipped          bool     `json:"skipped"`
		UnitFilesRemoved []string `json:"unitFilesRemoved"`
	}
	if err := json.Unmarshal(data, &runResult); err != nil {
		t.Fatalf("gc run parse: %v / %s", err, trunc(data))
	}
	if runResult.Skipped {
		t.Fatalf("units run must not be skipped when idle")
	}
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("stale unit file must be deleted by the units scope")
	}
	if _, err := os.Stat(keepFile); err != nil {
		t.Errorf("unit file for a live service must survive: %v", err)
	}
	if len(runResult.UnitFilesRemoved) != 1 || !strings.Contains(runResult.UnitFilesRemoved[0], "gc-stale.container") {
		t.Errorf("removed units = %v, want [gc-stale.container]", runResult.UnitFilesRemoved)
	}

	// ── Invalid scope rejected ───────────────────────────────────────────────
	code, data = client.post(t, "/api/system/gc", []byte(`{"scope":"bogus"}`))
	if code != http.StatusBadRequest {
		t.Errorf("invalid scope expected 400, got %d: %s", code, trunc(data))
	}

	t.Logf("✓ cleanup engine verified: labels round-trip, orphan preview, stale-unit purge, scope validation")
}
