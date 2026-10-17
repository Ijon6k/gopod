package services

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"gopod/internal/podman"
)

// TestLiveSmoke_LabelOnlyMatching exercises the REAL rootless podman socket
// and the REAL StopService/TeardownService code paths of this working tree.
//
// Scenario: a labeled gopod container with two UNLABELED foreign containers
// whose names collide with the service's identity (the exact class of bug
// this suite guards against):
//   - target  : labeled gopod container "gopod-smoke-web"   (service A, id svc-smoke-web)
//   - decoy 1 : UNLABELED container "web"                   (foreign)
//   - decoy 2 : UNLABELED container "web-history"           (foreign)
//
// Name-based matching considered the container named `web` to BE service
// A (name equality) and stopped/removed it. Label-only matching must leave
// both decoys untouched at every stage.
//
// Opt-in:  GOPOD_LIVE_SMOKE=1 go test ./internal/services/ -run TestLiveSmoke -v
// Cleanup: every object this test creates is force-removed by exact name in
// t.Cleanup, regardless of ownership, because the harness owns them.

const (
	liveSmokeTarget = "gopod-smoke-web"
	liveSmokeSvcID  = "svc-smoke-web"
	liveSmokeProj   = "proj-smoke"
)

func liveSmokeContainerState(t *testing.T, name string) (string, bool) {
	t.Helper()
	cli := podman.NewClient("")
	containers, err := cli.GetContainers(context.Background())
	if err != nil {
		t.Fatalf("GetContainers failed: %v", err)
	}
	for _, c := range containers {
		for _, n := range c.Names {
			if strings.TrimPrefix(n, "/") == name {
				return strings.ToLower(c.State), true
			}
		}
	}
	return "", false
}

func liveSmokeRun(t *testing.T, name string, labels []string, args ...string) {
	t.Helper()
	fullArgs := append([]string{"run", "-d", "--name", name}, labels...)
	fullArgs = append(fullArgs, "docker.io/library/alpine:latest")
	fullArgs = append(fullArgs, args...)
	out, err := exec.Command("podman", fullArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("podman %v failed: %v\n%s", fullArgs, err, out)
	}
}

func TestLiveSmoke_LabelOnlyMatching(t *testing.T) {
	if os.Getenv("GOPOD_LIVE_SMOKE") != "1" {
		t.Skip("live smoke requires GOPOD_LIVE_SMOKE=1 (touches the real podman)")
	}

	cli := podman.NewClient("")
	if cli.SocketPath() == "" {
		t.Skip("no podman REST socket detected")
	}

	svc := Service{ID: liveSmokeSvcID, ProjectID: liveSmokeProj, Name: "web", Type: "container", Image: "docker.io/library/alpine:latest"}
	decoys := []string{"web", "web-history"}

	// Deterministic cleanup: force-remove harness-created objects by exact name.
	t.Cleanup(func() {
		names := append([]string{liveSmokeTarget}, decoys...)
		for _, n := range names {
			_, _ = exec.Command("podman", "rm", "-f", n).CombinedOutput()
		}
	})

	// Guard: if any harness name is already occupied by an existing container,
	// skip — the harness must never force-remove somebody else's object.
	for _, n := range append([]string{liveSmokeTarget}, decoys...) {
		if _, exists := liveSmokeContainerState(t, n); exists {
			t.Skipf("harness name %q already exists; refusing to run to avoid touching foreign objects", n)
		}
	}

	// Setup: image pull best-effort (cached on dev hosts), target with gopod
	// labels, decoys unlabeled.
	if out, err := exec.Command("podman", "pull", "docker.io/library/alpine:latest").CombinedOutput(); err != nil {
		t.Skipf("could not pull alpine image: %v\n%s", err, out)
	}
	liveSmokeRun(t, liveSmokeTarget, []string{"--label", "io.gopod.service=" + liveSmokeSvcID, "--label", "io.gopod.project=" + liveSmokeProj}, "sleep", "600")
	for _, d := range decoys {
		liveSmokeRun(t, d, nil, "sleep", "600")
	}

	for _, n := range append([]string{liveSmokeTarget}, decoys...) {
		if st, ok := liveSmokeContainerState(t, n); !ok || st != "running" {
			t.Fatalf("setup: %s must be running, got %q (exists=%v)", n, st, ok)
		}
	}

	// Phase 1: StopService must stop ONLY the labeled target.
	d := NewDeployer(cli, nil, nil, t.TempDir())
	ctx := context.Background()
	if err := d.StopService(ctx, svc); err != nil {
		t.Fatalf("StopService error: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if st, ok := liveSmokeContainerState(t, liveSmokeTarget); ok && st == "running" {
		t.Fatalf("labeled target %s must be stopped by StopService", liveSmokeTarget)
	}
	for _, dName := range decoys {
		if st, ok := liveSmokeContainerState(t, dName); !ok || st != "running" {
			t.Fatalf("FOREIGN container %q must remain untouched after StopService (state=%q exists=%v)", dName, st, ok)
		}
	}

	// Phase 2: TeardownService must remove ONLY the labeled target.
	if err := d.TeardownService(ctx, svc, false); err != nil {
		t.Fatalf("TeardownService error: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if _, ok := liveSmokeContainerState(t, liveSmokeTarget); ok {
		t.Fatalf("labeled target %s must be gone after TeardownService", liveSmokeTarget)
	}
	for _, dName := range decoys {
		if _, ok := liveSmokeContainerState(t, dName); !ok {
			t.Fatalf("FOREIGN container %q must survive TeardownService", dName)
		}
	}
}
