package podman

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"testing"
)

func muxFrame(streamType byte, payload string) []byte {
	var hdr [8]byte
	hdr[0] = streamType
	binary.BigEndian.PutUint32(hdr[4:8], uint32(len(payload)))
	var buf bytes.Buffer
	buf.Write(hdr[:])
	buf.WriteString(payload)
	return buf.Bytes()
}

func TestGetContainerLogsDemuxesMultiplexedFrames(t *testing.T) {
	var raw bytes.Buffer
	line1 := "2026-10-28 08:00:12.039 UTC [25] LOG:  checkpoint starting: time\n"
	line2 := "2026-10-28 11:38:02.199 UTC [30] ERROR:  canceling statement due to user request\r\n"
	raw.Write(muxFrame(1, line1))
	raw.Write(muxFrame(2, line2))

	state := &fakeDaemonState{
		libpodVer:  fakePodman5,
		logsStatus: 200,
		logsBody:   raw.String(),
	}
	client, log := newFakeSocketClient(t, state)

	got, err := client.GetContainerLogsWithOptions(context.Background(), "pg1", 50, true)
	if err != nil {
		t.Fatalf("GetContainerLogsWithOptions failed: %v", err)
	}
	want := line1 + "2026-10-28 11:38:02.199 UTC [30] ERROR:  canceling statement due to user request\n"
	if got != want {
		t.Fatalf("demuxed mismatch:\n got: %q\nwant: %q", got, want)
	}
	if !log.hasQuery("timestamps=true") {
		t.Fatalf("expected timestamps=true query parameter in daemon call: %v", log.queries)
	}
}

func TestDemuxContainerLogsEdgeCases(t *testing.T) {
	if got := demuxContainerLogs(nil); got != "" {
		t.Fatalf("empty input = %q, want empty", got)
	}
	// Plain unframed TTY stream stays untouched
	plain := "plain stdout line\r\nsecond line\r\n"
	if got := demuxContainerLogs([]byte(plain)); got != "plain stdout line\nsecond line\n" {
		t.Fatalf("plain passthrough mismatch: %q", got)
	}
	// Truncated payload frame after a valid header falls back cleanly without panicking or dropping bytes
	var partial bytes.Buffer
	partial.Write(muxFrame(1, "first\n"))
	partial.Write([]byte{2, 0, 0, 0, 0, 0, 0, 99})
	partial.WriteString("tail")
	if got := demuxContainerLogs(partial.Bytes()); got != "first\ntail" {
		t.Fatalf("partial frame fallback mismatch: %q", got)
	}
}

// Delete must match pods/volumes semantics: 404 means the object is already
// gone, so teardown races (preflight cleanup vs pod cascade) stay idempotent.
func TestDeleteContainerTolerates404(t *testing.T) {
	state := &fakeDaemonState{libpodVer: fakePodman5, delStatus: 404}
	client, _ := newFakeSocketClient(t, state)

	if err := client.DeleteContainer(context.Background(), "c1", true); err != nil {
		t.Fatalf("delete of an already-gone container must succeed: %v", err)
	}

	state.mu.Lock()
	state.delStatus = 500
	state.mu.Unlock()
	if err := client.DeleteContainer(context.Background(), "c1", true); err == nil {
		t.Fatalf("delete failure must be surfaced")
	} else if !strings.Contains(err.Error(), "delete-body") {
		t.Fatalf("error must carry the daemon body, got: %v", err)
	}
}

// Logs must never stream a non-200 body to the UI as "logs"; a vanished
// container maps to the typed ErrNotFound.
func TestGetContainerLogsChecksStatus(t *testing.T) {
	state := &fakeDaemonState{libpodVer: fakePodman5, logsStatus: 404, logsBody: "no such container"}
	client, _ := newFakeSocketClient(t, state)

	if _, err := client.GetContainerLogs(context.Background(), "gone", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("404 logs must map to ErrNotFound, got: %v", err)
	}

	state.mu.Lock()
	state.logsStatus = 200
	state.logsBody = "line one\nline two\n"
	state.mu.Unlock()
	logs, err := client.GetContainerLogs(context.Background(), "c1", 10)
	if err != nil {
		t.Fatalf("logs for a live container: %v", err)
	}
	if logs != "line one\nline two\n" {
		t.Fatalf("logs body mismatch: %q", logs)
	}

	state.mu.Lock()
	state.logsStatus = 500
	state.logsBody = "boom"
	state.mu.Unlock()
	if _, err := client.GetContainerLogs(context.Background(), "c1", 10); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("a failed logs request must be an error, got: %v", err)
	}
}

// An accepted pods/create response with an empty Id must resolve the pod from
// the store listing — no CLI fork that would only log a confusing conflict.
func TestCreatePodResolvesEmptyIDWithoutCLI(t *testing.T) {
	fullID := strings.Repeat("a", 64)
	state := &fakeDaemonState{
		libpodVer: fakePodman5,
		createdID: `{"Id": ""}`,
		podsJSON:  testPodJSON(fullID, "web-pod"),
	}
	client, log := newFakeSocketClient(t, state)
	marker := fakeCLIBin(t)

	id, err := client.CreatePod(context.Background(), CreatePodOptions{Name: "web-pod"})
	if err != nil {
		t.Fatalf("CreatePod with empty response Id must resolve via the store listing: %v", err)
	}
	// The store listing truncates IDs to 12 characters (CLI parity), so the
	// resolved id is the truncated form of the listed pod.
	if id != fullID[:12] {
		t.Fatalf("CreatePod id = %q; want the listed store id %q", id, fullID[:12])
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the podman CLI must not be invoked for an empty-Id create (marker: %v)", err)
	}
	if !log.contains("/libpod/pods/create") || !log.contains("/libpod/pods/json") {
		t.Fatalf("expected create + list requests, got: %v", log)
	}
}

// When the daemon accepted the create but the store listing cannot vouch for
// the new pod (fault or unexpected content), CreatePod must report an explicit
// error instead of falling back to the CLI, which would only log a confusing
// conflict.
func TestCreatePodExplicitErrorsWithFaultyListing(t *testing.T) {
	state := &fakeDaemonState{libpodVer: fakePodman5, createdID: `{"Id": ""}`, podsJSON: "[]", failListings: true}
	client, _ := newFakeSocketClient(t, state)
	marker := fakeCLIBin(t)

	_, err := client.CreatePod(context.Background(), CreatePodOptions{Name: "web-pod"})
	if err == nil {
		t.Fatalf("create with a failed listing must error explicitly")
	} else if !strings.Contains(err.Error(), "pods listing failed") {
		t.Fatalf("error must name the failed listing, got: %v", err)
	}
	if _, serr := os.Stat(marker); !os.IsNotExist(serr) {
		t.Fatalf("the podman CLI must not be invoked (marker: %v)", serr)
	}

	// A listing answer that omits the new pod is equally explicit.
	state.mu.Lock()
	state.failListings = false
	state.podsJSON = testPodJSON(strings.Repeat("b", 64), "other-pod")
	state.mu.Unlock()
	_, err = client.CreatePod(context.Background(), CreatePodOptions{Name: "web-pod"})
	if err == nil || !strings.Contains(err.Error(), "missing from the store listing") {
		t.Fatalf("expected an explicit missing-from-listing error, got: %v", err)
	}
	if _, serr := os.Stat(marker); !os.IsNotExist(serr) {
		t.Fatalf("the podman CLI must not be invoked (marker: %v)", serr)
	}
}

func TestInspectPodMatchesNameFullIDAndPrefix(t *testing.T) {
	fullID := strings.Repeat("f", 64)
	state := &fakeDaemonState{libpodVer: fakePodman5, podsJSON: testPodJSON(fullID, "web-pod")}
	client, _ := newFakeSocketClient(t, state)

	ctx := context.Background()
	for _, probe := range []string{"web-pod", fullID, fullID[:12]} {
		pod, err := client.InspectPod(ctx, probe)
		if err != nil {
			t.Fatalf("InspectPod(%q): %v", probe, err)
		}
		if pod.Name != "web-pod" {
			t.Fatalf("InspectPod(%q) = %+v; want the listed pod", probe, pod)
		}
	}
	if _, err := client.InspectPod(ctx, fullID[:5]); err == nil {
		t.Fatalf("InspectPod with a <12-char prefix must not match")
	}
	if _, err := client.InspectPod(ctx, "no-such-pod"); err == nil {
		t.Fatalf("InspectPod with an unknown name must fail")
	}
}
