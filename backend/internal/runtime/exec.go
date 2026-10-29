package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"gopod/pkg/httputil"
)

var upgrader = websocket.Upgrader{
	// Same predicate as the HTTP CORS middleware: same-host or an explicit
	// ALLOWED_ORIGIN entry. Loopback is not special-cased — any local page
	// could otherwise open an authenticated terminal session.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser client (curl, tests)
		}
		return httputil.IsOriginAllowed(origin, r.Host)
	},
}

// execWsLimits bound terminal WebSocket frames and sessions. Without a read
// limit a client can push arbitrarily large frames straight into the input
// pump; without deadlines a half-open connection keeps the pty child and both
// pump goroutines alive forever (the client is often a laptop that suspends).
const (
	execReadLimit  = 1 << 20 // 1 MiB per frame
	execWriteWait  = 10 * time.Second
	execPongWait   = 60 * time.Second
	execPingPeriod = 25 * time.Second // must stay below execPongWait
)

// wsWriter serializes writes to the terminal WebSocket. gorilla/websocket
// permits only one concurrent writer, and both the PTY reader goroutine and
// the session teardown write to the socket.
type wsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsWriter) message(msgType int, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(execWriteWait))
	return w.conn.WriteMessage(msgType, payload)
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

// handleContainerTerminalTicket generates a short-lived single-use ticket for WebSocket terminal exec.
func (h *Handler) handleContainerTerminalTicket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing container ID")
		return
	}
	if strings.HasPrefix(id, "-") || strings.ContainsAny(id, " \t\r\n/\\`\"'$;&|<>") {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid container identifier")
		return
	}
	if h.ticketLimiter != nil && !h.ticketLimiter.allow(r) {
		httputil.WriteError(w, http.StatusTooManyRequests, "Too many terminal ticket requests — slow down and try again shortly")
		return
	}
	if h.ticketManager == nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Ticket manager unavailable")
		return
	}
	ticket, err := h.ticketManager.IssueTicket(id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to issue terminal ticket")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"ticket": ticket})
}

// handleContainerExec provides interactive WebSocket terminal sessions with pseudo-terminal (PTY) emulation.
func (h *Handler) handleContainerExec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing container ID", http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(id, "-") || strings.ContainsAny(id, " \t\r\n/\\`\"'$;&|<>") {
		http.Error(w, "Invalid container identifier", http.StatusBadRequest)
		return
	}

	// Guard WebSocket endpoint: check one-time ticket first, then fallback to session auth
	var authenticated bool
	if ticket := r.URL.Query().Get("ticket"); ticket != "" && h.ticketManager != nil {
		if h.ticketManager.ValidateAndConsume(ticket, id) {
			authenticated = true
		}
	}
	if !authenticated {
		// WebSocket clients cannot set an Authorization header during the
		// upgrade handshake, so the session may arrive in the query string; the
		// one-time ticket above remains the preferred path.
		h.middleware.RequireAuthAllowQuery(func(w http.ResponseWriter, r *http.Request) {
			authenticated = true
		})(w, r)
	}
	if !authenticated {
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[EXEC] WebSocket upgrade error: %v", err)
		return
	}
	defer ws.Close()
	writer := &wsWriter{conn: ws}

	// Session-lifetime guards: the pong handler (browsers answer protocol
	// pings automatically and pong on their own) resets the read deadline, so
	// an interactive session with a responsive client stays open
	// indefinitely. A client that stops ponging hits the read deadline, the
	// input pump exits, and closeAll() reaps the pty child and its
	// goroutines.
	ws.SetReadLimit(execReadLimit)
	_ = ws.SetReadDeadline(time.Now().Add(execPongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(execPongWait))
	})

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
	checkCmd := exec.CommandContext(ctx, "podman", "inspect", "--type", "container", "--format", "{{.State.Status}}", "--", targetID)
	if out, err := checkCmd.CombinedOutput(); err == nil {
		status = strings.TrimSpace(string(out))
	}

	// 2. If not found or not running, try finding via container filters.
	// Strict ownership order: ownership label first (exact label
	// equality semantics), then per-container compose SERVICE labels. Name
	// substring (`name=web` also matches `webhook`), compose-PROJECT labels
	// (they match every member) and `pod=` filters are NOT used — they attach
	// the terminal to a cousin container. An exact-name pass runs last.
	if status != "running" {
		filters := []string{
			fmt.Sprintf("label=io.gopod.service=%s", id),
			fmt.Sprintf("label=com.docker.compose.service=%s", id),
			fmt.Sprintf("label=io.podman.compose.service=%s", id),
		}

		for _, filter := range filters {
			findCmd := exec.CommandContext(ctx, "podman", "ps", "-a", "--filter", filter, "--format", "{{.Names}}\t{{.State}}")
			if findOut, findErr := findCmd.CombinedOutput(); findErr == nil && len(findOut) > 0 {
				lines := strings.Split(strings.TrimSpace(string(findOut)), "\n")
				for _, line := range lines {
					parts := strings.Split(line, "\t")
					if len(parts) > 0 && parts[0] != "" {
						cName := parts[0]
						if strings.HasSuffix(cName, "-infra") {
							continue
						}
						targetID = cName
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

		// Exact-name fallback: only a container whose name equals the id is a
		// match — never partial/substring names.
		if status != "running" {
			listCmd := exec.CommandContext(ctx, "podman", "ps", "-a", "--format", "{{.Names}}\t{{.State}}")
			if listOut, listErr := listCmd.CombinedOutput(); listErr == nil && len(listOut) > 0 {
				lines := strings.Split(strings.TrimSpace(string(listOut)), "\n")
				for _, line := range lines {
					parts := strings.Split(line, "\t")
					if len(parts) > 0 && parts[0] != "" {
						if strings.TrimPrefix(parts[0], "/") != id {
							continue
						}
						targetID = id
						if len(parts) > 1 {
							status = parts[1]
						}
						break
					}
				}
			}
		}
	}

	if status == "" {
		_ = writer.message(websocket.TextMessage, fmt.Appendf(nil, "\r\n\x1b[33m⚠️ Container '%s' was not found.\x1b[0m\r\n\x1b[90mPlease deploy this service first before opening an interactive terminal session.\x1b[0m\r\n", id))
		return
	}

	if status != "running" {
		_ = writer.message(websocket.TextMessage, fmt.Appendf(nil, "\r\n\x1b[33m⚠️ Container '%s' is currently %s.\x1b[0m\r\n\x1b[90mPlease start or deploy this service to open an interactive terminal session.\x1b[0m\r\n", targetID, strings.ToUpper(status)))
		return
	}

	// 2. Spawn pseudo-terminal via creack/pty with podman exec -it
	cmd := exec.CommandContext(ctx, "podman", "exec", "-it", "-w", "/", "--", targetID, cmdName)
	if h.service != nil {
		h.service.SetupCmdEnv(cmd)
	}
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		_ = writer.message(websocket.TextMessage, fmt.Appendf(nil, "\r\n\x1b[31m[ERROR] Failed to start pseudo-terminal: %v\x1b[0m\r\n", err))
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

	// Keepalive: ping the client every execPingPeriod; its automatic pong
	// resets the read deadline (see the pong handler above). All writes go
	// through wsWriter so the ping never races the PTY pump.
	go func() {
		ticker := time.NewTicker(execPingPeriod)
		defer ticker.Stop()
		for range ticker.C {
			if err := writer.message(websocket.PingMessage, nil); err != nil {
				closeAll()
				return
			}
		}
	}()

	// Read PTY -> Write WebSocket
	go func() {
		defer closeAll()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				outStr := string(buf[:n])
				if strings.Contains(outStr, "executable file not found in $PATH") || strings.Contains(outStr, "exec: \"sh\": not found") {
					_ = writer.message(websocket.TextMessage, []byte("\r\n\x1b[33m⚠️ Container image is built FROM scratch and contains no shell environment (/bin/sh).\x1b[0m\r\n\x1b[90m💡 Scratch containers (like traefik/whoami) only contain the compiled binary without a shell.\x1b[0m\r\n\x1b[90m💡 To test connectivity inside the pod network, use an adjacent container with a shell (e.g. redis:alpine or alpine).\x1b[0m\r\n"))
				}
				if writeErr := writer.message(websocket.BinaryMessage, buf[:n]); writeErr != nil {
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
		_ = writer.message(websocket.TextMessage, fmt.Appendf(nil, "\r\n\x1b[33m[Container closed: '%s' not found in container]\x1b[0m\r\n\x1b[90m(If this image is a minimal scratch/distroless container, it does not include an interactive shell)\x1b[0m\r\n", cmdName))
	} else {
		_ = writer.message(websocket.TextMessage, fmt.Appendf(nil, "\r\n\x1b[33m[Session closed with exit code %d]\x1b[0m\r\n", exitCode))
	}
}
