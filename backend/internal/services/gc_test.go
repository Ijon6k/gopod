package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/db"
	"gopod/internal/podman"
)

// fakeGCRuntime is a deterministic stand-in for the podman client so the
// cleanup engine's decision logic can be verified without touching real
// storage.
type fakeGCRuntime struct {
	containers []podman.ContainerItem
	volumes    []podman.VolumeItem
	df         []podman.SystemDiskUsage
	dangling   *podman.DanglingImageStat

	imagePrunes    []bool
	containerPrune int
	buildPrunes    int
	deletedVolumes []string
}

func (f *fakeGCRuntime) GetContainers(context.Context) ([]podman.ContainerItem, error) {
	return f.containers, nil
}
func (f *fakeGCRuntime) GetVolumes(context.Context) ([]podman.VolumeItem, error) {
	return f.volumes, nil
}
func (f *fakeGCRuntime) GetDiskUsage(context.Context) ([]podman.SystemDiskUsage, error) {
	return f.df, nil
}
func (f *fakeGCRuntime) GetDanglingImages(context.Context) (*podman.DanglingImageStat, error) {
	return f.dangling, nil
}
func (f *fakeGCRuntime) PruneImages(_ context.Context, all bool) error {
	f.imagePrunes = append(f.imagePrunes, all)
	return nil
}
func (f *fakeGCRuntime) PruneContainers(context.Context) error {
	f.containerPrune++
	return nil
}
func (f *fakeGCRuntime) PruneBuilds(context.Context) error {
	f.buildPrunes++
	return nil
}
func (f *fakeGCRuntime) DeleteVolume(_ context.Context, name string, _ bool) error {
	f.deletedVolumes = append(f.deletedVolumes, name)
	return nil
}

// newGCService builds a WorkloadService backed by a temp SQLite DB containing
// the given service IDs, with the fake runtime injected.
func newGCService(t *testing.T, fake *fakeGCRuntime, serviceIDs ...string) *WorkloadService {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "gc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-gc', 'GC Project')"); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	repo := NewSQLiteRepository(database.DB)
	for _, id := range serviceIDs {
		if err := repo.Create(context.Background(), Service{ID: id, ProjectID: "proj-gc", Name: id, Type: "image"}); err != nil {
			t.Fatalf("create service %s: %v", id, err)
		}
	}
	ws := NewWorkloadService(repo, nil, nil)
	ws.gcRuntimeOverride = fake
	return ws
}

func TestGCPreviewComputesCountsFromFixtures(t *testing.T) {
	fake := &fakeGCRuntime{
		dangling: &podman.DanglingImageStat{Count: 3, Bytes: 300 * 1024 * 1024},
		containers: []podman.ContainerItem{
			{ID: "a", State: "running"},
			{ID: "b", State: "exited", Labels: map[string]string{"io.gopod.service": "svc-gone"}},
			{ID: "c", State: "exited", Labels: map[string]string{"io.gopod.service": "svc-gone"}},
			// Host-wide stopped container without gopod labels: never counted,
			// because a filtered prune must never touch it.
			{ID: "d", State: "exited"},
		},
		volumes: []podman.VolumeItem{
			{Name: "known-vol", Labels: map[string]string{"io.gopod.service": "svc-known"}},
			{Name: "orphan-vol", Labels: map[string]string{"io.gopod.service": "svc-gone"}},
			{Name: "unlabeled-vol"},
		},
		df: []podman.SystemDiskUsage{
			{Type: "Images", RawReclaimable: 1000},
			{Type: "Containers", RawReclaimable: 20},
			{Type: "Local Volumes", RawReclaimable: 5000},
		},
	}
	ws := newGCService(t, fake, "svc-known")

	preview, err := ws.GCPreview(context.Background())
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Busy {
		t.Errorf("preview must not be busy")
	}
	if preview.DanglingImages != 3 {
		t.Errorf("dangling images = %d, want 3", preview.DanglingImages)
	}
	if preview.StoppedContainers != 2 {
		t.Errorf("stopped containers = %d, want 2", preview.StoppedContainers)
	}
	if len(preview.OrphanVolumes) != 1 || preview.OrphanVolumes[0] != "orphan-vol" {
		t.Errorf("orphan volumes = %v, want [orphan-vol]", preview.OrphanVolumes)
	}
	// Volume reclaimable bytes must never be counted (volumes are manual-only).
	if preview.WeightedReclaimable != 1020 {
		t.Errorf("weighted reclaimable = %d, want 1020 (images+containers only)", preview.WeightedReclaimable)
	}
}

func TestRunGCSkipsWhenDeploymentInFlight(t *testing.T) {
	fake := &fakeGCRuntime{dangling: &podman.DanglingImageStat{Count: 1}}
	ws := newGCService(t, fake, "svc-a")
	// A held deploy lock means a deployment is in flight.
	ws.deployLocks.Store("svc-a", "dep-1")

	result, err := ws.RunGC(context.Background(), GCScopeAll, false, nil)
	if err != nil {
		t.Fatalf("RunGC: %v", err)
	}
	if !result.Skipped {
		t.Fatalf("expected the run to be skipped while a deployment is in flight")
	}
	if len(fake.imagePrunes) != 0 || fake.containerPrune != 0 || fake.buildPrunes != 0 {
		t.Errorf("busy run must not prune anything: images=%v containers=%d builds=%d",
			fake.imagePrunes, fake.containerPrune, fake.buildPrunes)
	}
}

func TestRunGCImagesScopePrunesDanglingOnly(t *testing.T) {
	fake := &fakeGCRuntime{dangling: &podman.DanglingImageStat{Count: 4}}
	ws := newGCService(t, fake, "svc-a")

	result, err := ws.RunGC(context.Background(), GCScopeImages, false, nil)
	if err != nil {
		t.Fatalf("RunGC: %v", err)
	}
	if len(fake.imagePrunes) != 1 || fake.imagePrunes[0] != false {
		t.Errorf("images scope must prune dangling only (all=false), got %v", fake.imagePrunes)
	}
	if fake.containerPrune != 0 || fake.buildPrunes != 0 {
		t.Errorf("images scope must not touch containers/builds")
	}
	if result.DanglingImages != 4 {
		t.Errorf("reported dangling images = %d, want 4", result.DanglingImages)
	}
}

func TestRunGCAllCoversEveryScopeButVolumes(t *testing.T) {
	fake := &fakeGCRuntime{
		dangling: &podman.DanglingImageStat{Count: 2},
		volumes:  []podman.VolumeItem{{Name: "v", Labels: map[string]string{"io.gopod.service": "svc-gone"}}},
	}
	ws := newGCService(t, fake, "svc-a")

	_, err := ws.RunGC(context.Background(), GCScopeAll, false, nil)
	if err != nil {
		t.Fatalf("RunGC: %v", err)
	}
	if len(fake.imagePrunes) != 1 || fake.containerPrune != 1 || fake.buildPrunes != 1 {
		t.Errorf("all scope must run images+containers+builds: images=%v containers=%d builds=%d",
			fake.imagePrunes, fake.containerPrune, fake.buildPrunes)
	}
	if len(fake.deletedVolumes) != 0 {
		t.Errorf("volumes must never be deleted without an explicit request, got %v", fake.deletedVolumes)
	}
}

func TestRunGCIncludeVolumesDeletesOnlyRequestedOrphans(t *testing.T) {
	fake := &fakeGCRuntime{
		volumes: []podman.VolumeItem{
			{Name: "orphan", Labels: map[string]string{"io.gopod.service": "svc-gone"}},
			{Name: "known", Labels: map[string]string{"io.gopod.service": "svc-a"}},
			{Name: "unlabeled"},
		},
	}
	ws := newGCService(t, fake, "svc-a")

	result, err := ws.RunGC(context.Background(), GCScopeAll, true, []string{"orphan", "known", "unlabeled"})
	if err != nil {
		t.Fatalf("RunGC: %v", err)
	}
	if len(fake.deletedVolumes) != 1 || fake.deletedVolumes[0] != "orphan" {
		t.Errorf("only the requested, gopod-labeled orphan volume may be deleted, got %v", fake.deletedVolumes)
	}
	if len(result.VolumesRemoved) != 1 || result.VolumesRemoved[0] != "orphan" {
		t.Errorf("result volumes removed = %v, want [orphan]", result.VolumesRemoved)
	}
}

func TestFindStaleUnitFilesOnlyMatchesGopodLabeledGoneServices(t *testing.T) {
	quadletDir := t.TempDir()
	t.Setenv("QUADLET_DIR", quadletDir)

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(quadletDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("known.container", "[Container]\nImage=nginx\nLabel=io.gopod.service=svc-known\n")
	write("gone.pod", "[Pod]\nPodName=pod_gone\nLabel=io.gopod.service=svc-gone\n")
	write("foreign.pod", "[Pod]\nPodName=pod_foreign\n")
	write("unrelated.txt", "io.gopod.service=svc-gone\n")

	ws := NewWorkloadService(nil, nil, nil)
	stale := ws.findStaleUnitFiles(map[string]bool{"svc-known": true})

	if len(stale) != 1 || filepath.Base(stale[0]) != "gone.pod" {
		t.Fatalf("stale unit files = %v, want [gone.pod]", stale)
	}
}

func TestRunGCUnitsRemovesStaleFilesAndReloads(t *testing.T) {
	quadletDir := t.TempDir()
	t.Setenv("QUADLET_DIR", quadletDir)
	stalePath := filepath.Join(quadletDir, "gone.container")
	if err := os.WriteFile(stalePath, []byte("[Container]\nImage=nginx\nLabel=io.gopod.service=svc-gone\n"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	keepPath := filepath.Join(quadletDir, "keep.container")
	if err := os.WriteFile(keepPath, []byte("[Container]\nImage=nginx\nLabel=io.gopod.service=svc-known\n"), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}

	fake := &fakeGCRuntime{}
	ws := newGCService(t, fake, "svc-known")

	result, err := ws.RunGC(context.Background(), GCScopeUnits, false, nil)
	if err != nil {
		t.Fatalf("RunGC: %v", err)
	}
	if len(result.UnitFilesRemoved) != 1 || filepath.Base(result.UnitFilesRemoved[0]) != "gone.container" {
		t.Fatalf("removed unit files = %v, want [gone.container]", result.UnitFilesRemoved)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Errorf("stale unit file must be deleted")
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Errorf("non-stale unit file must survive: %v", err)
	}
	// Unit scope alone must not prune images/containers/builds.
	if len(fake.imagePrunes) != 0 || fake.containerPrune != 0 || fake.buildPrunes != 0 {
		t.Errorf("units scope must not touch engine artifacts")
	}
}

func TestParseGCScope(t *testing.T) {
	cases := map[string]GCScope{
		"":            GCScopeAll,
		"all":         GCScopeAll,
		"IMAGES":      GCScopeImages,
		" containers": GCScopeContainers,
		"builds":      GCScopeBuilds,
		"units":       GCScopeUnits,
	}
	for in, want := range cases {
		got, err := ParseGCScope(in)
		if err != nil || got != want {
			t.Errorf("ParseGCScope(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseGCScope("bogus"); err == nil {
		t.Errorf("expected error for an invalid scope")
	}
}

func TestGCScheduleOverride(t *testing.T) {
	t.Setenv("GOPOD_GC_AT", "01:07")
	h, m := gcSchedule()
	if h != 1 || m != 7 {
		t.Errorf("gcSchedule = %02d:%02d, want 01:07", h, m)
	}
	t.Setenv("GOPOD_GC_AT", "not-a-time")
	h, m = gcSchedule()
	if h != 23 || m != 50 {
		t.Errorf("invalid override must fall back to 23:50, got %02d:%02d", h, m)
	}
}

func TestIsNamedVolumeSource(t *testing.T) {
	named := []string{"data", "gopod-web_data", "app.db"}
	for _, s := range named {
		if !isNamedVolumeSource(s) {
			t.Errorf("%q should be a named volume", s)
		}
	}
	binds := []string{"", "/var/lib/data", "./rel", "../up", "~/home", "/srv/x", "a/b"}
	for _, s := range binds {
		if isNamedVolumeSource(s) {
			t.Errorf("%q should be treated as a host bind, not a named volume", s)
		}
	}
	if got := volumeSourceOf("data:/var/lib/data:Z"); got != "data" {
		t.Errorf("volumeSourceOf = %q, want data", got)
	}
}

func TestBuildPodQuadletBundleCarriesOwnershipLabels(t *testing.T) {
	s := Service{ID: "proj-web-abc123", Name: "web", ProjectID: "proj"}
	bundle := buildPodQuadletBundle(s, "pod_proj-web-abc123", []string{"18401:80"}, nil)

	podChunk := bundle
	if idx := strings.Index(bundle, "\n---"); idx >= 0 {
		podChunk = bundle[:idx]
	}
	if !strings.Contains(podChunk, "Label=io.gopod.service=proj-web-abc123\n") {
		t.Errorf(".pod unit must carry the service ownership label:\n%s", podChunk)
	}
	if !strings.Contains(podChunk, "Label=io.gopod.project=proj\n") {
		t.Errorf(".pod unit must carry the project ownership label:\n%s", podChunk)
	}
}

func TestGenerateDefaultQuadletContentPodCarriesOwnershipLabels(t *testing.T) {
	d := NewDeployer(nil, nil, nil, t.TempDir())
	s := Service{
		ID:          "svc-multi-1",
		ProjectID:   "proj-multi",
		Name:        "stack",
		ComposeYaml: "services:\n  web:\n    image: nginx:alpine\n  db:\n    image: postgres:16\n",
	}
	content := d.generateDefaultQuadletContent(context.Background(), s, "stack")

	podChunk := content
	if idx := strings.Index(content, "\n---"); idx >= 0 {
		podChunk = content[:idx]
	}
	if !strings.Contains(podChunk, "Label=io.gopod.service=svc-multi-1\n") {
		t.Errorf("generated .pod unit must carry the service ownership label:\n%s", podChunk)
	}
	if !strings.Contains(podChunk, "Label=io.gopod.project=proj-multi\n") {
		t.Errorf("generated .pod unit must carry the project ownership label:\n%s", podChunk)
	}
}
