package runtime

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"

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

// handleContainerExec provides interactive WebSocket terminal sessions with authentication.
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
		cmdName = "/bin/sh"
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	cmd := exec.CommandContext(ctx, "podman", "exec", "-i", id, cmdName)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[ERROR] Failed to attach stdin: "+err.Error()+"\x1b[0m\r\n"))
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[ERROR] Failed to attach stdout: "+err.Error()+"\x1b[0m\r\n"))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[ERROR] Failed to attach stderr: "+err.Error()+"\x1b[0m\r\n"))
		return
	}

	if err := cmd.Start(); err != nil {
		cmd = exec.CommandContext(ctx, "podman", "exec", "-i", id, "sh")
		stdin, _ = cmd.StdinPipe()
		stdout, _ = cmd.StdoutPipe()
		stderr, _ = cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[ERROR] Cannot start shell in container: "+err.Error()+"\x1b[0m\r\n"))
			return
		}
	}

	_ = ws.WriteMessage(websocket.TextMessage, []byte("\x1b[32m🚀 Connected to container "+id+" ("+cmdName+")\x1b[0m\r\n"))

	var wg sync.WaitGroup
	wg.Add(2)

	reader := io.MultiReader(stdout, stderr)
	go func() {
		defer wg.Done()
		buf := make([]byte, 2048)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				if writeErr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer stdin.Close()
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				break
			}
			if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
				if _, err := stdin.Write(data); err != nil {
					break
				}
			}
		}
	}()

	wg.Wait()
	_ = cmd.Wait()
	_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[33m[Session closed]\x1b[0m\r\n"))
}
