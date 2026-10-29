package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof" // developer pprof endpoints; only served when GOPOD_DEBUG=1
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopod"
	"gopod/internal/podman"
	"gopod/internal/server"
)

func main() {
	defaultPort := os.Getenv("PORT")
	if defaultPort == "" {
		defaultPort = "8080"
	}
	defaultSocket := os.Getenv("PODMAN_SOCKET")
	defaultDB := os.Getenv("GOPOD_DB_PATH")
	defaultCaddy := os.Getenv("CADDY_ADMIN_URL")
	if defaultCaddy == "" {
		defaultCaddy = "http://127.0.0.1:2019"
	}

	port := flag.String("port", defaultPort, "HTTP server port")
	socket := flag.String("socket", defaultSocket, "Podman unix socket path (optional)")
	dbPath := flag.String("db", defaultDB, "SQLite database file path (optional)")
	caddyAdmin := flag.String("caddy", defaultCaddy, "Caddy Admin API endpoint (optional)")
	flag.Parse()

	// Prepare embedded dist filesystem
	distFS, err := backend.DistFS()
	if err != nil {
		log.Printf("[WARN] Failed to load embedded frontend: %v", err)
	}

	// Initialize Podman client to test connectivity
	pClient := podman.NewClient(*socket)

	// Propagate CONTAINER_HOST and DOCKER_HOST for subprocesses (podman CLI, podman-compose)
	sockPath := *socket
	if sockPath == "" {
		sockPath = pClient.SocketPath()
	}
	if sockPath == "" {
		sockPath = os.Getenv("PODMAN_SOCKET")
	}
	if sockPath == "" {
		sockPath = "/run/podman/podman.sock"
	}
	sockClean := strings.TrimPrefix(sockPath, "unix://")
	sockURL := fmt.Sprintf("unix://%s", sockClean)
	_ = os.Setenv("PODMAN_SOCKET", sockClean)
	_ = os.Setenv("CONTAINER_HOST", sockURL)
	_ = os.Setenv("DOCKER_HOST", sockURL)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	fmt.Println("==================================================")
	fmt.Println("           GOPOD - Single Binary Fullstack        ")
	fmt.Println("==================================================")

	sysInfo, err := pClient.GetSystemInfo(ctx)
	if err != nil {
		fmt.Printf("⚠️  Podman Engine: Not directly connected (%v)\n", err)
		fmt.Println("   (Will fallback to CLI or mock if needed)")
	} else {
		fmt.Printf("✅ Podman Engine: Connected (v%s, Rootless: %t)\n", sysInfo.PodmanVersion, sysInfo.Rootless)
		fmt.Printf("💻 Host System  : %s (%s, %d vCPUs)\n", sysInfo.Hostname, sysInfo.OS, sysInfo.VCPU)
		fmt.Printf("📦 Containers   : %d running / %d total\n", sysInfo.RunningCount, sysInfo.TotalCount)
		if pClient.SocketPath() != "" {
			fmt.Printf("🔌 Socket Path  : %s\n", pClient.SocketPath())
		}
	}

	srvHandler, err := server.NewServer(server.Config{
		Port:         *port,
		PodmanSocket: *socket,
		DBPath:       *dbPath,
		CaddyAdmin:   *caddyAdmin,
		DistFS:       distFS,
	})
	if err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}

	// Optional developer diagnostics: opt-in Go pprof on a loopback-only
	// listener, disabled by default and never registered on the API mux.
	if os.Getenv("GOPOD_DEBUG") == "1" {
		go func() {
			addr := os.Getenv("GOPOD_DEBUG_ADDR")
			if addr == "" || (!strings.HasPrefix(addr, "127.0.0.1") && !strings.HasPrefix(addr, "localhost")) {
				addr = "127.0.0.1:6060"
			}
			log.Printf("[DEBUG] pprof available at http://%s/debug/pprof/", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				log.Printf("[DEBUG] pprof listener stopped: %v", err)
			}
		}()
	}

	// Control-plane bind address. Default binds every local interface; pod
	// layouts can narrow it to loopback so tenant containers on the shared
	// pod network cannot reach the control plane directly (only through
	// Caddy or from inside the same pod netns).
	bindHost := strings.TrimSpace(os.Getenv("GOPOD_BIND_HOST"))
	srv := &http.Server{
		Addr:         net.JoinHostPort(bindHost, *port),
		Handler:      srvHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Run in background goroutine
	go func() {
		fmt.Printf("\n🚀 GoPod server listening at: http://localhost:%s\n", *port)
		fmt.Printf("📊 Live Podman Stats API    : http://localhost:%s/api/stats\n", *port)
		fmt.Printf("🖥️  System Metrics API       : http://localhost:%s/api/system\n", *port)
		fmt.Printf("📡 Live SSE Stream          : http://localhost:%s/api/stats/stream\n", *port)
		fmt.Println("==================================================")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down GoPod server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		// SSE/WS connections (hijacked or with cleared write deadlines) cannot
		// be joined by Shutdown; say what actually happened instead of exiting
		// via Fatal (same exit code, honest message).
		log.Printf("Server shutdown incomplete (active streams cut): %v", err)
		os.Exit(1)
	}

	log.Println("GoPod server cleanly stopped.")
}
