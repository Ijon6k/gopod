package server

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopod/internal/activity"
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
	"gopod/pkg/crypto"
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
	if podmanClient != nil {
		if err := podmanClient.EnsureNetwork(context.Background(), "gopod-net"); err != nil {
			log.Printf("[WARN] Failed to ensure gopod-net network: %v", err)
		}
	}

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

	dataDir := filepath.Dir(cfg.DBPath)
	if dataDir == "." || dataDir == "" {
		dataDir = "./data"
	}
	if err := crypto.InitMasterKey(dataDir); err != nil {
		log.Printf("[ERROR] Failed to initialize AES master key: %v", err)
	}
	logMgr := services.NewLogManager(dataDir)

	serviceRepo := services.NewSQLiteRepository(sqlDB)
	deployer := services.NewDeployer(podmanClient, credentialsService, logMgr, dataDir)
	deployer.SetRegistryProvider(credentialsService)
	workloadService := services.NewWorkloadService(serviceRepo, deployer, logMgr)
	_ = workloadService.ReconcileOrphanedDeployments(context.Background())
	serviceHandler := services.NewHandler(workloadService, authMiddleware)

	runtimeService := runtime.NewService(podmanClient)
	runtimeHandler := runtime.NewHandler(runtimeService, authMiddleware)

	caddyReconciler := ingress.NewReconciler(cfg.CaddyAdmin, "")
	if cfg.Port != "" {
		caddyReconciler.SetControlPort(cfg.Port)
	}
	caddyReconciler.SetServiceResolver(func(ctx context.Context, idOrName string) (*ingress.ServiceMeta, error) {
		svc, err := serviceRepo.Get(ctx, idOrName)
		if err == nil && svc != nil {
			return &ingress.ServiceMeta{
				ID:        svc.ID,
				Name:      svc.Name,
				Type:      svc.Type,
				InPod:     svc.InPod,
				PodID:     svc.PodID,
				ProjectID: svc.ProjectID,
			}, nil
		}
		svcs, listErr := serviceRepo.List(ctx, "")
		if listErr == nil {
			for _, s := range svcs {
				if s.Name == idOrName || s.ID == idOrName {
					return &ingress.ServiceMeta{
						ID:        s.ID,
						Name:      s.Name,
						Type:      s.Type,
						InPod:     s.InPod,
						PodID:     s.PodID,
						ProjectID: s.ProjectID,
					}, nil
				}
			}
		}
		return nil, fmt.Errorf("service not found: %s", idOrName)
	})
	ingressRepo := ingress.NewSQLiteRepository(sqlDB)
	ingressService := ingress.NewService(ingressRepo, caddyReconciler)
	ingressHandler := ingress.NewHandler(ingressService, authMiddleware)

	// Reconcile Caddy on startup to establish reverse proxy and access logging
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		domains, err := ingressRepo.List(bgCtx, "")
		if err == nil {
			_ = caddyReconciler.Reconcile(bgCtx, domains)
		}
	}()

	storageRepo := storage.NewSQLiteRepository(sqlDB)
	storageService := storage.NewService(storageRepo)
	storageService.StartScheduler(context.Background())
	storageHandler := storage.NewHandler(storageService, authMiddleware)

	auditRepo := audit.NewSQLiteRepository(sqlDB)
	auditService := audit.NewService(auditRepo)
	auditHandler := audit.NewHandler(auditService, authMiddleware)

	activity.Init(sqlDB)
	activityStore := activity.NewStore(sqlDB)
	activityHandler := activity.NewHandler(activityStore, authMiddleware)

	authService.SetAuditRecorder(auditService)
	workloadService.SetAuditRecorder(auditService)

	projectService.SetServiceChecker(func(ctx context.Context, projectID string) (int, error) {
		svcs, err := serviceRepo.List(ctx, projectID)
		if err != nil {
			return 0, err
		}
		return len(svcs), nil
	})

	projectService.SetProviders(
		func(ctx context.Context, projectID string) ([]services.Service, error) {
			return serviceRepo.List(ctx, projectID)
		},
		func(ctx context.Context, projectID string) ([]projects.ProjectDomain, error) {
			domains, err := ingressRepo.List(ctx, projectID)
			if err != nil {
				return nil, err
			}
			res := make([]projects.ProjectDomain, len(domains))
			for i, d := range domains {
				res[i] = projects.ProjectDomain{
					ID:        d.ID,
					Hostname:  d.Hostname,
					ServiceID: d.ServiceID,
					TLS:       d.TLS,
					Status:    d.DNSStatus,
				}
			}
			return res, nil
		},
	)

	workloadService.SetIngressCleaner(func(ctx context.Context, serviceID string) {
		doms, err := ingressRepo.List(ctx, "")
		if err == nil {
			for _, d := range doms {
				if d.ServiceID == serviceID {
					_ = ingressRepo.Delete(ctx, d.ID)
				}
			}
		}
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			remaining, _ := ingressRepo.List(bgCtx, "")
			_ = caddyReconciler.Reconcile(bgCtx, remaining)
		}()
	})

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
	activityHandler.RegisterRoutes(mux)

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
					if cleanPath == "index.html" {
						w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Set("Pragma", "no-cache")
						w.Header().Set("Expires", "0")
					} else if strings.HasPrefix(cleanPath, "_app/immutable/") {
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					}
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
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
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

func isOriginAllowed(origin, host string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == host {
		return true
	}
	hostname := u.Hostname()
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}
	if allowed := os.Getenv("ALLOWED_ORIGIN"); allowed != "" {
		for _, a := range strings.Split(allowed, ",") {
			trimmed := strings.TrimSpace(a)
			if trimmed == origin || trimmed == u.Host {
				return true
			}
		}
	}
	return false
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		origin := r.Header.Get("Origin")
		if origin != "" && isOriginAllowed(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

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

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
}

