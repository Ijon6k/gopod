package podman

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

const fakePodman5 = "5.0.0"

func TestSplitMajorMinor(t *testing.T) {
	tests := []struct {
		in        string
		wantMajor int
		wantMinor int
		wantOK    bool
	}{
		{"5.8.9", 5, 8, true},
		{"4.9.2", 4, 9, true},
		{"1.44", 1, 44, true},
		{" 5.8 ", 5, 8, true},
		{"5", 0, 0, false},
		{"", 0, 0, false},
		{"abc", 0, 0, false},
		{"0.9.1", 0, 0, false},
	}
	for _, tt := range tests {
		major, minor, ok := splitMajorMinor(tt.in)
		if ok != tt.wantOK || (ok && (major != tt.wantMajor || minor != tt.wantMinor)) {
			t.Errorf("splitMajorMinor(%q) = (%d, %d, %v); want (%d, %d, %v)", tt.in, major, minor, ok, tt.wantMajor, tt.wantMinor, tt.wantOK)
		}
	}
}

// The compat "Api-Version" header (1.x) must be ignored; the libpod path
// prefix comes from "Libpod-Api-Version". Regression guard: a 5.8.9 daemon
// must receive /v5.8.0/libpod requests, never /v5.0.0 or /v1.44.0.
func TestHandshakeUsesLibpodAPIVersionHeader(t *testing.T) {
	state := &fakeDaemonState{libpodVer: "5.8.9", podsJSON: "[]"}
	client, log := newFakeSocketClient(t, state)

	pods, err := client.GetPods(context.Background())
	if err != nil {
		t.Fatalf("GetPods after handshake: %v", err)
	}
	if len(pods) != 0 {
		t.Fatalf("expected empty pod list, got %d", len(pods))
	}
	client.versionMu.Lock()
	got := client.negotiated
	client.versionMu.Unlock()
	if got != "v5.8.0" {
		t.Fatalf("negotiated prefix = %q; want v5.8.0", got)
	}
	if !log.contains("/v5.8.0/libpod/pods/json") {
		t.Fatalf("pods list must target the negotiated prefix, requests: %v", log)
	}
	if log.contains("v1.44") || log.contains("v5.0.0/libpod/pods") {
		t.Fatalf("compat version or pinned default leaked into libpod paths: %v", log)
	}
}

// When the unversioned /_ping is unavailable the client probes the pinned
// versioned ping path and still recovers the real daemon version.
func TestHandshakeFallbackToVersionedPing(t *testing.T) {
	state := &fakeDaemonState{libpodVer: "4.9.2", podsJSON: "[]"}
	client, _ := newFakeSocketClient(t, state)

	// Make the unversioned ping 404 by serving it from a wrapper: the shared
	// state serves both ping paths, so intercept /_ping only.
	client.httpClient.Transport = &pingBlockTransport{next: client.httpClient.Transport}

	pods, err := client.GetPods(context.Background())
	if err != nil {
		t.Fatalf("GetPods via versioned ping fallback: %v", err)
	}
	if len(pods) != 0 {
		t.Fatalf("expected empty pod list, got %d", len(pods))
	}
	client.versionMu.Lock()
	got := client.negotiated
	client.versionMu.Unlock()
	if got != "v4.9.0" {
		t.Fatalf("negotiated prefix = %q; want v4.9.0", got)
	}
}

// pingBlockTransport 404s /_ping while delegating everything else, to exercise
// the versioned-ping fallback path deterministically.
type pingBlockTransport struct{ next http.RoundTripper }

func (p *pingBlockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/_ping" {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     "404 Not Found",
			Body:       http.NoBody,
			Header:     http.Header{},
			Request:    req,
		}, nil
	}
	return p.next.RoundTrip(req)
}

// A daemon that answers no ping at all must leave the client functional on
// the pinned version, and the failed handshake must not re-probe on every
// call (cooldown).
func TestHandshakeFailureFallsBackToPinnedAndCoolsDown(t *testing.T) {
	state := &fakeDaemonState{libpodVer: "", podsJSON: "[]"}
	client, log := newFakeSocketClient(t, state)

	for i := 0; i < 3; i++ {
		if _, err := client.GetPods(context.Background()); err != nil {
			t.Fatalf("GetPods #%d with dead handshake must still work on pinned version: %v", i, err)
		}
	}
	if got := log.pingCount(); got != 2 {
		t.Fatalf("expected exactly 2 ping probes (unversioned + versioned) across 3 calls, got %d", got)
	}
	client.versionMu.Lock()
	failSet := !client.lastFailed.IsZero()
	client.versionMu.Unlock()
	if !failSet {
		t.Fatalf("a failed handshake must record the failure time for the cooldown")
	}
}

// A transport failure (daemon restart) must clear the cached version so the
// next call re-handshakes and picks up a replaced daemon of a different
// version instead of silently targeting stale paths.
func TestTransportErrorRetriggersHandshake(t *testing.T) {
	state := &fakeDaemonState{libpodVer: "5.8.9", podsJSON: "[]"}
	client, log := newFakeSocketClient(t, state)

	if _, err := client.GetPods(context.Background()); err != nil {
		t.Fatalf("first GetPods: %v", err)
	}
	client.versionMu.Lock()
	first := client.negotiated
	client.versionMu.Unlock()
	if first != "v5.8.0" {
		t.Fatalf("first negotiation = %q; want v5.8.0", first)
	}

	state.mu.Lock()
	state.breakConn = true
	state.mu.Unlock()
	if _, err := client.GetPods(context.Background()); err == nil {
		t.Fatalf("GetPods against a dropped daemon connection must fail")
	}

	// Daemon came back as an older version.
	state.mu.Lock()
	state.breakConn = false
	state.libpodVer = "4.9.2"
	state.mu.Unlock()
	pods, err := client.GetPods(context.Background())
	if err != nil {
		t.Fatalf("GetPods after daemon replacement: %v", err)
	}
	if len(pods) != 0 {
		t.Fatalf("expected empty pod list, got %d", len(pods))
	}
	client.versionMu.Lock()
	second := client.negotiated
	client.versionMu.Unlock()
	if second != "v4.9.0" {
		t.Fatalf("re-negotiated prefix = %q; want v4.9.0", second)
	}
	if !log.contains("/v4.9.0/libpod/pods/json") {
		t.Fatalf("post-restart traffic must target the new version, requests: %v", log)
	}
}

// The client must not carry a global timeout: lifecycle calls legitimately
// exceed it, and budgets are enforced per request instead.
func TestNoGlobalClientTimeout(t *testing.T) {
	state := &fakeDaemonState{libpodVer: fakePodman5, podsJSON: "[]"}
	client, _ := newFakeSocketClient(t, state)
	if client.httpClient.Timeout != 0 {
		t.Fatalf("global http.Client.Timeout = %v; it must stay unset so per-request budgets rule", client.httpClient.Timeout)
	}
}

func TestReadBudgetAppliesToGetRequests(t *testing.T) {
	state := &fakeDaemonState{libpodVer: "5.8.9", podsJSON: "[]"}
	client, _ := newFakeSocketClient(t, state)

	oldRead := readTimeout
	defer func() { readTimeout = oldRead }()

	readTimeout = 80 * time.Millisecond
	client.httpClient.Transport = &sleepTransport{next: client.httpClient.Transport, d: 400 * time.Millisecond}
	if _, err := client.GetPods(context.Background()); err == nil {
		t.Fatalf("read past the per-request budget must fail")
	} else if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected a deadline error, got: %v", err)
	}
	// A caller-side budget expiry says nothing about the daemon; the
	// negotiated version must survive it without an extra probe round.
	client.versionMu.Lock()
	survived := client.negotiated
	client.versionMu.Unlock()
	if survived != "v5.8.0" {
		t.Fatalf("a budget expiry must not clear the negotiated version (got %q)", survived)
	}

	readTimeout = 5 * time.Second
	if _, err := client.GetPods(context.Background()); err != nil {
		t.Fatalf("read within the per-request budget must succeed: %v", err)
	}
}

func TestWriteBudgetAppliesToLifecycleRequests(t *testing.T) {
	state := &fakeDaemonState{libpodVer: fakePodman5, delStatus: 204}
	client, _ := newFakeSocketClient(t, state)

	oldWrite := writeTimeout
	defer func() { writeTimeout = oldWrite }()

	writeTimeout = 80 * time.Millisecond
	client.httpClient.Transport = &sleepTransport{next: client.httpClient.Transport, d: 400 * time.Millisecond}
	if err := client.DeleteContainer(context.Background(), "c1", false); err == nil {
		t.Fatalf("lifecycle call past the per-request budget must fail")
	}

	writeTimeout = 5 * time.Second
	if err := client.DeleteContainer(context.Background(), "c1", false); err != nil {
		t.Fatalf("lifecycle call within the per-request budget must succeed: %v", err)
	}
}

func TestPruneBudgetAppliesToPruneRequests(t *testing.T) {
	state := &fakeDaemonState{libpodVer: fakePodman5}
	client, _ := newFakeSocketClient(t, state)

	oldPrune := pruneTimeout
	defer func() { pruneTimeout = oldPrune }()

	pruneTimeout = 80 * time.Millisecond
	client.httpClient.Transport = &sleepTransport{next: client.httpClient.Transport, d: 400 * time.Millisecond}
	if err := client.PruneContainers(context.Background()); err == nil {
		t.Fatalf("prune past the per-request budget must fail")
	} else if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected a deadline error, got: %v", err)
	}
}

// sleepTransport delays every round trip; used to simulate slow daemons.
type sleepTransport struct {
	next http.RoundTripper
	d    time.Duration
}

func (s *sleepTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	time.Sleep(s.d)
	return s.next.RoundTrip(req)
}
