package server

import (
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"gopod/internal/api"
	"gopod/internal/caddy"
	"gopod/internal/db"
	"gopod/internal/podman"
	"gopod/internal/runner"
)

// Config holds server configuration.
type Config struct {
	Port         string
	PodmanSocket string
	DBPath       string
	CaddyAdmin   string
	DistFS       fs.FS
}

// NewServer configures http.Handler with API routes and SPA fallback.
func NewServer(cfg Config) http.Handler {
	podmanClient := podman.NewClient(cfg.PodmanSocket)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Printf("[ERROR] Failed to open SQLite database: %v", err)
	}

	var repo *db.Repository
	if database != nil {
		repo = db.NewRepository(database)
	}

	caddyReconciler := caddy.NewReconciler(cfg.CaddyAdmin, "")
	deployer := runner.NewDeployer(podmanClient)

	apiHandler := api.NewHandler(podmanClient, repo, caddyReconciler, deployer)

	mux := http.NewServeMux()

	// Register API endpoints
	apiHandler.RegisterRoutes(mux)

	// SPA & Static Files Handler
	if cfg.DistFS != nil {
		fileServer := http.FileServer(http.FS(cfg.DistFS))

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Don't intercept API routes
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}

			// Clean path
			cleanPath := strings.TrimPrefix(r.URL.Path, "/")
			if cleanPath == "" {
				cleanPath = "index.html"
			}

			// Check if file exists in embedded filesystem
			f, err := cfg.DistFS.Open(cleanPath)
			if err == nil {
				defer f.Close()
				stat, err := f.Stat()
				if err == nil && !stat.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}

			// SPA Fallback: serve index.html for any route
			indexFile, err := cfg.DistFS.Open("index.html")
			if err != nil {
				http.Error(w, "SPA index.html not found in build", http.StatusInternalServerError)
				return
			}
			defer indexFile.Close()

			stat, _ := indexFile.Stat()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if stat != nil {
				http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
			} else {
				_, _ = io.Copy(w, indexFile)
			}
		})
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html><html><body><h1>GoPod Backend</h1><p>Frontend static files not embedded.</p></body></html>`))
		})
	}

	return withMiddleware(mux)
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		// Log API requests
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
