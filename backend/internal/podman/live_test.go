package podman

import (
	"context"
	"os"
	"testing"
)

// TestLiveDaemonHandshake probes the real rootless podman socket when
// GOPOD_PODMAN_LIVE=1. It is strictly read-only (ping handshake, pods list,
// system info) and skips in normal runs.
func TestLiveDaemonHandshake(t *testing.T) {
	if os.Getenv("GOPOD_PODMAN_LIVE") != "1" {
		t.Skip("set GOPOD_PODMAN_LIVE=1 to probe the real podman socket (read-only)")
	}
	client := NewClient("")
	if client.httpClient == nil {
		t.Skip("no podman socket discovered on this host")
	}
	ctx := context.Background()

	pods, err := client.GetPods(ctx)
	if err != nil {
		t.Fatalf("live GetPods after handshake: %v", err)
	}
	client.versionMu.Lock()
	prefix := client.negotiated
	client.versionMu.Unlock()

	info, err := client.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("live GetSystemInfo: %v", err)
	}
	t.Logf("live daemon: negotiated prefix=%s podman=%s pods=%d", prefix, info.PodmanVersion, len(pods))
}
