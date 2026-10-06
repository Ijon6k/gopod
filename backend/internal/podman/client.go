package podman

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Client handles communication with Podman via Unix socket or CLI.
type Client struct {
	socketPath string
	httpClient *http.Client
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
			Timeout:   8 * time.Second,
		}
	}

	return &Client{
		socketPath: socket,
		httpClient: httpClient,
	}
}

// SocketPath returns active socket path or empty string if CLI only.
func (c *Client) SocketPath() string {
	return c.socketPath
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
