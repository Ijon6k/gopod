package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	port := flag.String("port", defaultPort, "HTTP server port")
	socket := flag.String("socket", defaultSocket, "Podman unix socket path (optional)")
	flag.Parse()

	// Prepare embedded dist filesystem
	distFS, err := backend.DistFS()
	if err != nil {
		log.Printf("[WARN] Failed to load embedded frontend: %v", err)
	}

	// Initialize Podman client to test connectivity
	pClient := podman.NewClient(*socket)
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

	srvHandler := server.NewServer(server.Config{
		Port:         *port,
		PodmanSocket: *socket,
		DistFS:       distFS,
	})

	srv := &http.Server{
		Addr:         ":" + *port,
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
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("GoPod server cleanly stopped.")
}
