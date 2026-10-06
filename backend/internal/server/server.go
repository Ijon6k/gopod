package server

import (
	"database/sql"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"gopod/internal/audit"
	"gopod/internal/auth"
	"gopod/internal/credentials"
	"gopod/internal/db"
	"gopod/internal/ingress"
	"gopod/internal/podman"
	"gopod/internal/projects"
	"gopod/internal/runtime"
	"gopod/internal/services"
	"gopod/internal/storage"
)

// Config holds server configuration.
type Config struct {
	Port         string
	PodmanSocket string
	DBPath       string
	CaddyAdmin   string
	DistFS       fs.FS
}

// NewServer configures http.Handler with domain API routes and SPA fallback.
func NewServer(cfg Config) http.Handler {
	podmanClient := podman.NewClient(cfg.PodmanSocket)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Printf("[ERROR] Failed to open SQLite database: %v", err)
	}

	var sqlDB *sql.DB
	if database != nil {
		sqlDB = database.DB
	}

	// ── Domain Layer Initialization ──
	authRepo := auth.NewSQLiteRepository(sqlDB)
	authService := auth.NewService(authRepo)
	authMiddleware := auth.NewMiddleware(authService)
	authHandler := auth.NewHandler(authService)

	projectRepo := projects.NewSQLiteRepository(sqlDB)
	projectService := projects.NewService(projectRepo)
	projectHandler := projects.NewHandler(projectService, authMiddleware)

	credentialsRepo := credentials.NewSQLiteRepository(sqlDB)
	credentialsService := credentials.NewService(credentialsRepo)
	credentialsHandler := credentials.NewHandler(credentialsService, authMiddleware)

	serviceRepo := services.NewSQLiteRepository(sqlDB)
	deployer := services.NewDeployer(podmanClient, credentialsService)
	workloadService := services.NewWorkloadService(serviceRepo, deployer)
	serviceHandler := services.NewHandler(workloadService, authMiddleware)

	runtimeService := runtime.NewService(podmanClient)
	runtimeHandler := runtime.NewHandler(runtimeService, authMiddleware)

	caddyReconciler := ingress.NewReconciler(cfg.CaddyAdmin, "")
	ingressRepo := ingress.NewSQLiteRepository(sqlDB)
	ingressService := ingress.NewService(ingressRepo, caddyReconciler)
	ingressHandler := ingress.NewHandler(ingressService, authMiddleware)

	storageRepo := storage.NewSQLiteRepository(sqlDB)
	storageService := storage.NewService(storageRepo)
	storageHandler := storage.NewHandler(storageService, authMiddleware)

	auditRepo := audit.NewSQLiteRepository(sqlDB)
	auditService := audit.NewService(auditRepo)
	auditHandler := audit.NewHandler(auditService, authMiddleware)

	authService.SetAuditRecorder(auditService)
	workloadService.SetAuditRecorder(auditService)

	mux := http.NewServeMux()

	// ── Register Domain Routes ──
	authHandler.RegisterRoutes(mux)
	projectHandler.RegisterRoutes(mux)
	serviceHandler.RegisterRoutes(mux)
	runtimeHandler.RegisterRoutes(mux)
	ingressHandler.RegisterRoutes(mux)
	credentialsHandler.RegisterRoutes(mux)
	storageHandler.RegisterRoutes(mux)
	auditHandler.RegisterRoutes(mux)

	// ── SPA & Static Files Handler ──
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

			// SPA Fallback: serve index.html for any client-side route
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

		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

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
