package podman

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Per-request budgets: lifecycle calls (stop/restart) legitimately block up
// to the daemon's default 10s grace, and bulk prunes can run for minutes, so
// a single client-wide timeout cannot serve both. Vars so tests can shrink
// them.
var (
	readTimeout  = 10 * time.Second
	writeTimeout = 60 * time.Second
	pruneTimeout = 120 * time.Second
	pingTimeout  = 5 * time.Second
	// handshakeCooldown pauses re-probes after a failed handshake so a dead
	// socket cannot turn every request into a ping storm.
	handshakeCooldown = time.Minute
)

// pinnedLibpodVersion is the libpod path used when a handshake cannot report
// a daemon version. Daemons at libpod >= 5.0 accept it; older daemons reject
// the path and their calls fall back to the CLI.
const pinnedLibpodVersion = "v5.0.0"

// OwnershipLabel is the podman label gopod stamps on every container, pod,
// image and volume it creates (io.gopod.service=<serviceID>). Deletion and
// prune paths must only touch objects carrying this label — host-wide
// unlabeled objects can belong to tools other than gopod.
const OwnershipLabel = "io.gopod.service"

// ownershipFilterJSON is the libpod prune request body filter restricting a
// prune to gopod-owned objects, e.g. /libpod/containers/prune?filters=...
var ownershipFilterJSON = fmt.Sprintf(`{"label":["%s"]}`, OwnershipLabel)

// pruneFiltersURL appends the ownership label filter to a libpod prune URL.
func pruneFiltersURL(base string) string {
	return base + "?filters=" + url.QueryEscape(ownershipFilterJSON)
}

// Client handles communication with Podman via Unix socket or CLI.
type Client struct {
	socketPath string
	httpClient *http.Client

	// libpod API version handshake state. negotiated is cached until cleared
	// by a transport error; lastFailed arms the cooldown after a failed probe
	// round and is cleared by the next successful probe.
	versionMu  sync.Mutex
	negotiated string
	lastFailed time.Time
}

// NewClient creates a new Podman client discovering available sockets.
func NewClient(customSocket string) *Client {
	socket := detectSocket(customSocket)
	var httpClient *http.Client

	if socket != "" {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
			DisableKeepAlives: false,
		}
		httpClient = &http.Client{
			Transport: transport,
		}
	}

	return &Client{
		socketPath: socket,
		httpClient: httpClient,
	}
}

// do executes req with a per-request timeout budget. Genuine transport
// failures clear the negotiated libpod version so a restarted or replaced
// daemon is re-handshaked on the next call; caller cancellation and budget
// expiry say nothing about the daemon and leave the cache alone. The returned
// cancel releases the request context; callers register it with defer right
// next to the Body.Close defer — the LIFO ordering releases it after the body
// has been consumed, so streamed bodies are never truncated.
func (c *Client) do(ctx context.Context, budget time.Duration, req *http.Request) (*http.Response, func(), error) {
	reqCtx, cancel := context.WithTimeout(ctx, budget)
	resp, err := c.httpClient.Do(req.WithContext(reqCtx))
	if err != nil {
		cancel()
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			c.invalidateVersion()
		}
		return nil, nil, err
	}
	return resp, cancel, nil
}

// libpodVersion returns the versioned libpod path prefix for API calls, for
// example "v5.8.0". The daemon handshake reports its API version from the
// "Libpod-Api-Version" response header (the "Api-Version" header is the
// docker-compat version and must not be used for libpod paths). A successful
// probe result is cached until invalidated; a failed probe is retried only
// after handshakeCooldown. Probes run outside the client lock on a detached
// context with their own short budget, so a slow or cancelled caller can
// never stall or poison the cache for concurrent callers.
func (c *Client) libpodVersion() string {
	c.versionMu.Lock()
	prefix := c.negotiated
	coolingDown := prefix == "" && !c.lastFailed.IsZero() && time.Since(c.lastFailed) < handshakeCooldown
	c.versionMu.Unlock()
	if prefix != "" {
		return prefix
	}
	if coolingDown {
		return pinnedLibpodVersion
	}

	probed, ver, ok := c.probeVersion()
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	if ok {
		// A successful probe round proves the daemon answers: forget any
		// failure cooldown left by a slower racing loser. Concurrent probes
		// converge on the first stored result; only the winner logs.
		c.lastFailed = time.Time{}
		if c.negotiated == "" {
			c.negotiated = probed
			if probed != pinnedLibpodVersion {
				log.Printf("[podman] libpod API %s detected; requests target /%s/libpod (pinned default: %s)", ver, probed, pinnedLibpodVersion)
			}
		}
		return c.negotiated
	}
	c.lastFailed = time.Now()
	return pinnedLibpodVersion
}

// invalidateVersion forgets the cached libpod version so the next call
// re-probes the daemon. A separately armed failure cooldown keeps re-probing
// rate-limited while the daemon stays unreachable.
func (c *Client) invalidateVersion() {
	c.versionMu.Lock()
	c.negotiated = ""
	c.versionMu.Unlock()
}

// probeVersion asks /_ping for the daemon's libpod API version and falls back
// to the pinned versioned ping path when the unversioned probe is unavailable.
func (c *Client) probeVersion() (prefix, ver string, ok bool) {
	if v, p, found := c.ping("http://d/_ping"); found {
		return p, v, true
	}
	if v, p, found := c.ping("http://d/" + pinnedLibpodVersion + "/libpod/_ping"); found {
		return p, v, true
	}
	return "", "", false
}

func (c *Client) ping(url string) (ver, prefix string, found bool) {
	probeCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", false
	}
	ver = resp.Header.Get("Libpod-Api-Version")
	if ver == "" {
		return "", "", false
	}
	major, minor, ok := splitMajorMinor(ver)
	if !ok {
		return "", "", false
	}
	return ver, fmt.Sprintf("v%d.%d.0", major, minor), true
}

// splitMajorMinor extracts the major and minor components from a version
// string such as "5.8.9"; the patch segment is ignored.
func splitMajorMinor(ver string) (major, minor int, ok bool) {
	parts := strings.Split(strings.TrimSpace(ver), ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || major < 1 {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minor < 0 {
		return 0, 0, false
	}
	return major, minor, true
}

// SocketPath returns active socket path or empty string if CLI only.
func (c *Client) SocketPath() string {
	return c.socketPath
}

// SetupCmdEnv configures subprocess environment variables to use the active Podman socket.
func (c *Client) SetupCmdEnv(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	socket := c.socketPath
	if socket == "" {
		socket = os.Getenv("PODMAN_SOCKET")
	}
	if socket == "" {
		socket = "/run/podman/podman.sock"
	}
	socketClean := strings.TrimPrefix(socket, "unix://")
	socketURL := fmt.Sprintf("unix://%s", socketClean)

	existingEnv := os.Environ()
	var newEnv []string
	for _, e := range existingEnv {
		if !strings.HasPrefix(e, "CONTAINER_HOST=") &&
			!strings.HasPrefix(e, "DOCKER_HOST=") &&
			!strings.HasPrefix(e, "PODMAN_SOCKET=") {
			newEnv = append(newEnv, e)
		}
	}
	newEnv = append(newEnv,
		fmt.Sprintf("CONTAINER_HOST=%s", socketURL),
		fmt.Sprintf("DOCKER_HOST=%s", socketURL),
		fmt.Sprintf("PODMAN_SOCKET=%s", socketClean),
	)
	cmd.Env = newEnv
}

func detectSocket(custom string) string {
	if custom != "" {
		trimmed := strings.TrimPrefix(custom, "unix://")
		if _, err := os.Stat(trimmed); err == nil {
			return trimmed
		}
	}

	if envHost := os.Getenv("CONTAINER_HOST"); envHost != "" {
		trimmed := strings.TrimPrefix(envHost, "unix://")
		if _, err := os.Stat(trimmed); err == nil {
			return trimmed
		}
	}

	if envSock := os.Getenv("PODMAN_SOCKET"); envSock != "" {
		trimmed := strings.TrimPrefix(envSock, "unix://")
		if _, err := os.Stat(trimmed); err == nil {
			return trimmed
		}
	}

	uid := os.Getuid()
	candidates := []string{
		fmt.Sprintf("/run/user/%d/podman/podman.sock", uid),
		filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "podman/podman.sock"),
		"/run/podman/podman.sock",
		"/var/run/docker.sock",
	}

	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprint(v)
	}
	return ""
}

// getLabels extracts a podman Labels object (map[string]interface{} from JSON
// decode of the CLI --format json output) into a regular map.
func getLabels(m map[string]interface{}, key string) map[string]string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if labelMap, ok := v.(map[string]interface{}); ok {
		out := make(map[string]string, len(labelMap))
		for k, lv := range labelMap {
			if s, ok := lv.(string); ok {
				out[k] = s
			}
		}
		return out
	}
	return nil
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case string:
			f, _ := strconv.ParseFloat(n, 64)
			return f
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
