package podman

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// requestLog records the wire requests a test issues so assertions can check
// which libpod version prefix and endpoints the client actually used.
type requestLog struct {
	mu   sync.Mutex
	got  []string
	ping int
}

func (l *requestLog) record(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.got = append(l.got, r.Method+" "+r.URL.Path)
	if r.URL.Path == "/_ping" || strings.HasSuffix(r.URL.Path, "/libpod/_ping") {
		l.ping++
	}
}

func (l *requestLog) contains(sub string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, req := range l.got {
		if strings.Contains(req, sub) {
			return true
		}
	}
	return false
}

func (l *requestLog) pingCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ping
}

// fakeDaemonState is the mutable behavior a test configures per scenario.
type fakeDaemonState struct {
	mu        sync.Mutex
	libpodVer string // value served as Libpod-Api-Version; "" → ping 404s
	breakConn bool   // panics inside data handlers → client-side transport error.
	// The panic never covers the ping paths: TestTransportErrorRetriggersHandshake
	// depends on the handshake succeeding while data paths fail, so the
	// re-negotiation assert stays observable across the cooldown logic.
	// net/http logs each recovered panic; stack-trace noise in test output is
	// expected and harmless.
	// failListings makes pods/json GETs fail, simulating a listing fault too
	// specific for the global breakConn switch.
	failListings bool
	podsJSON     string
	contJSON     string
	createdID    string
	logsStatus   int
	logsBody     string
	delStatus    int
}

func (s *fakeDaemonState) handler(log *requestLog) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		s.mu.Lock()
		libpodVer, breakConn := s.libpodVer, s.breakConn
		failListings := s.failListings
		podsJSON, contJSON, createdID := s.podsJSON, s.contJSON, s.createdID
		logsStatus, logsBody, delStatus := s.logsStatus, s.logsBody, s.delStatus
		s.mu.Unlock()

		if r.URL.Path == "/_ping" || strings.HasSuffix(r.URL.Path, "/libpod/_ping") {
			if libpodVer == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Libpod-Api-Version", libpodVer)
			w.WriteHeader(http.StatusOK)
			return
		}
		switch {
		case breakConn:
			// net/http recovers the panic and drops the connection, which
			// surfaces client-side as a transport error.
			panic("fake daemon connection dropped")
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/libpod/pods/json"):
			if failListings {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("listing unavailable"))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(podsJSON))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/libpod/containers/json"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(contJSON))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/libpod/pods/create"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(createdID))
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/libpod/containers/"):
			w.WriteHeader(delStatus)
			_, _ = w.Write([]byte("delete-body"))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/libpod/containers/") && strings.HasSuffix(r.URL.Path, "/logs"):
			w.WriteHeader(logsStatus)
			_, _ = w.Write([]byte(logsBody))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("fake daemon: unexpected " + r.Method + " " + r.URL.Path))
		}
	})
}

// newFakeSocketClient starts a libpod API stub on a real unix socket and
// returns a client wired to it. The client must first pass the version
// handshake, exactly as against the real daemon.
func newFakeSocketClient(t *testing.T, state *fakeDaemonState) (*Client, *requestLog) {
	t.Helper()
	log := &requestLog{}
	sock := filepath.Join(t.TempDir(), "podman.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen on unix socket: %v", err)
	}
	srv := &http.Server{Handler: state.handler(log)}
	serveDone := make(chan struct{})
	go func() {
		_ = srv.Serve(ln)
		close(serveDone)
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		_ = srv.Close()
		<-serveDone
	})
	return NewClient(sock), log
}

// fakeCLIBin replaces podman on PATH with a script that records every
// invocation into a marker file and fails, so any accidental CLI fallback is
// detected both by the marker and by the returned error.
func fakeCLIBin(t *testing.T) string {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "cli-marker")
	binDir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GOPOD_FAKE_CLI_MARKER\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "podman"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("GOPOD_FAKE_CLI_MARKER", marker)
	return marker
}

func testPodJSON(id, name string) string {
	return `[
		{
			"Id": "` + id + `",
			"Name": "` + name + `",
			"Status": "Running",
			"Created": "today",
			"InfraId": "b00b000b00b0",
			"Labels": {"io.gopod.service": "svc-1"},
			"Containers": [{"Id": "c001", "Names": ["/member"]}]
		}
	]`
}
