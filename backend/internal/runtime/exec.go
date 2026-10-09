package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host || strings.HasPrefix(u.Host, "localhost:") || strings.HasPrefix(u.Host, "127.0.0.1:") || u.Host == "localhost"
	},
}

type resizeMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

var allowedShells = map[string]bool{
	"sh":        true,
	"bash":      true,
	"zsh":       true,
	"ash":       true,
	"/bin/sh":   true,
	"/bin/bash": true,
	"/bin/zsh":  true,
	"/bin/ash":  true,
}

// handleContainerExec provides interactive WebSocket terminal sessions with pseudo-terminal (PTY) emulation.
func (h *Handler) handleContainerExec(w http.ResponseWriter, r *http.Request) {
	// Guard WebSocket endpoint with authentication check
	var authenticated bool
	h.middleware.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		authenticated = true
	})(w, r)
	if !authenticated {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing container ID", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[EXEC] WebSocket upgrade error: %v", err)
		return
	}
	defer ws.Close()

	cmdName := r.URL.Query().Get("cmd")
	if cmdName == "" {
		cmdName = r.URL.Query().Get("activeWay")
	}
	if cmdName == "" || !allowedShells[cmdName] {
		cmdName = "sh"
	}

	cols := 80
	rows := 24
	if c, err := strconv.Atoi(r.URL.Query().Get("cols")); err == nil && c > 0 && c <= 1000 {
		cols = c
	}
	if rw, err := strconv.Atoi(r.URL.Query().Get("rows")); err == nil && rw > 0 && rw <= 1000 {
		rows = rw
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	targetID := id
	var status string

	// 1. Check direct container inspect (--type container prevents inspecting images)
	checkCmd := exec.CommandContext(ctx, "podman", "inspect", "--type", "container", "--format", "{{.State.Status}}", targetID)
	if out, err := checkCmd.CombinedOutput(); err == nil {
		status = strings.TrimSpace(string(out))
	}

	// 2. If not found or not running, try finding via container filters
	if status != "running" {
		filters := []string{
			fmt.Sprintf("name=%s", id),
			fmt.Sprintf("label=com.docker.compose.service=%s", id),
			fmt.Sprintf("label=io.podman.compose.service=%s", id),
			fmt.Sprintf("label=com.docker.compose.project=%s", id),
			fmt.Sprintf("label=io.podman.compose.project=%s", id),
		}

		for _, filter := range filters {
			findCmd := exec.CommandContext(ctx, "podman", "ps", "-a", "--filter", filter, "--format", "{{.Names}}\t{{.State}}")
			if findOut, findErr := findCmd.CombinedOutput(); findErr == nil && len(findOut) > 0 {
				lines := strings.Split(strings.TrimSpace(string(findOut)), "\n")
				for _, line := range lines {
					parts := strings.Split(line, "\t")
					if len(parts) > 0 && parts[0] != "" {
						targetID = parts[0]
						if len(parts) > 1 {
							status = parts[1]
						}
						if status == "running" {
							break
						}
					}
				}
				if status == "running" {
					break
				}
			}
		}
	}

	if status == "" {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[33m⚠️ Container '%s' was not found.\x1b[0m\r\n\x1b[90mPlease deploy this service first before opening an interactive terminal session.\x1b[0m\r\n", id)))
		return
	}

	if status != "running" {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[33m⚠️ Container '%s' is currently %s.\x1b[0m\r\n\x1b[90mPlease start or deploy this service to open an interactive terminal session.\x1b[0m\r\n", targetID, strings.ToUpper(status))))
		return
	}

	// 2. Spawn pseudo-terminal via creack/pty with podman exec -it
	cmd := exec.CommandContext(ctx, "podman", "exec", "-it", "-w", "/", targetID, cmdName)
	if h.service != nil {
		h.service.SetupCmdEnv(cmd)
	}
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31m[ERROR] Failed to start pseudo-terminal: %v\x1b[0m\r\n", err)))
		return
	}

	var once sync.Once
	closeAll := func() {
		once.Do(func() {
			_ = ptmx.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = ws.Close()
		})
	}
	defer closeAll()

	// Read PTY -> Write WebSocket
	go func() {
		defer closeAll()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if writeErr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				// EOF or EIO when pty terminates
				return
			}
		}
	}()

	// Read WebSocket -> Write PTY / Resize
	go func() {
		defer closeAll()
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if (msgType == websocket.TextMessage || msgType == websocket.BinaryMessage) && len(data) > 0 {
				if data[0] == '{' {
					var rm resizeMessage
					if jsonErr := json.Unmarshal(data, &rm); jsonErr == nil && rm.Type == "resize" {
						if rm.Cols > 0 && rm.Rows > 0 {
							_ = pty.Setsize(ptmx, &pty.Winsize{
								Rows: uint16(rm.Rows),
								Cols: uint16(rm.Cols),
							})
						}
						continue
					}
				}
				if _, writeErr := ptmx.Write(data); writeErr != nil {
					return
				}
			}
		}
	}()

	err = cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	if exitCode == 127 {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[33m[Container closed: '%s' not found in container]\x1b[0m\r\n\x1b[90m(If this image is a minimal scratch/distroless container, it does not include an interactive shell)\x1b[0m\r\n", cmdName)))
	} else {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[33m[Session closed with exit code %d]\x1b[0m\r\n", exitCode)))
	}
}
