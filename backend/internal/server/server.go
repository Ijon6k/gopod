package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"gopod/internal/activity"
	"gopod/internal/audit"
	"gopod/internal/auth"
	"gopod/internal/credentials"
	"gopod/internal/db"
	"gopod/internal/ingress"
	"gopod/internal/notifications"
	"gopod/internal/podman"
	"gopod/internal/projects"
	"gopod/internal/runtime"
	"gopod/internal/services"
	"gopod/internal/storage"
	"gopod/internal/updater"
	"gopod/pkg/crypto"
	"gopod/pkg/httputil"
)

// Config holds server configuration.
type Config struct {
	Port         string
	PodmanSocket string
	DBPath       string
	CaddyAdmin   string
	DistFS       fs.FS
}

type activityRecorderFunc func(ctx context.Context, actor, action, resourceType, resourceID, projectID, serviceID, status, message, ip string)

func (f activityRecorderFunc) Record(ctx context.Context, actor, action, resourceType, resourceID, projectID, serviceID, status, message, ip string) {
	f(ctx, actor, action, resourceType, resourceID, projectID, serviceID, status, message, ip)
}

// NewServer configures http.Handler with domain API routes and SPA fallback.
// It returns an error when a hard prerequisite (the encryption master key)
// cannot be established, so the caller can refuse to boot instead of running
// with secrets that would silently become unreadable after a restart.
func NewServer(cfg Config) (http.Handler, error) {
	podmanClient := podman.NewClient(cfg.PodmanSocket)
	if podmanClient != nil {
		if err := podmanClient.EnsureNetwork(context.Background(), "gopod-net"); err != nil {
			log.Printf("[WARN] Failed to ensure gopod-net network: %v", err)
		}
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		// Fail fast: a server built around a nil *sql.DB would panic on the
		// first request and answer every client with a dropped connection
		// instead of a comprehensible error. main() turns the returned error
		// into a clean startup failure ("Refusing to start").
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	sqlDB := database.DB

	// ── Domain Layer Initialization ──
	authRepo := auth.NewSQLiteRepository(sqlDB)
	authService := auth.NewService(authRepo)
	authMiddleware := auth.NewMiddleware(authService)
	accessChecker := auth.NewAccessChecker(sqlDB)
	authMiddleware.SetAccessChecker(accessChecker)
	authHandler := auth.NewHandler(authService)
	notifService := notifications.NewService(sqlDB)
	notifHandler := notifications.NewHandler(notifService, authMiddleware)

	// Rewrite plaintext session tokens at rest to hashed form (idempotent:
	// rows already carrying the hash prefix are skipped).
	if sqlDB != nil {
		if n, err := authRepo.MigrateSessionTokens(context.Background()); err != nil {
			log.Printf("[WARN] session token migration failed: %v", err)
		} else if n > 0 {
			log.Printf("[auth] Migrated %d stored session token(s) to hashed form", n)
		}
		if n, err := authRepo.MigrateSessionIDs(context.Background()); err != nil {
			log.Printf("[WARN] session id migration failed: %v", err)
		} else if n > 0 {
			log.Printf("[auth] Assigned surrogate IDs to %d legacy session(s)", n)
		}
	}

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
		return nil, fmt.Errorf("initialize AES master key: %w", err)
	}
	// Start the hourly session prune only after the hard startup checks passed:
	// a failed init must not leave a background goroutine holding the DB open.
	go authService.StartSessionCleanup(context.Background())
	keyOnDisk := crypto.KeyFromEnv()
	if !keyOnDisk {
		if _, statErr := os.Stat(crypto.KeyFilePath(dataDir)); statErr == nil {
			keyOnDisk = true
		}
	}
	logMgr := services.NewLogManager(dataDir)

	serviceRepo := services.NewSQLiteRepository(sqlDB)
	deployer := services.NewDeployer(podmanClient, credentialsService, logMgr, dataDir)
	deployer.SetRegistryProvider(credentialsService)
	deployer.SetGitTokenProvider(credentialsService)
	deployer.SetServiceRepo(serviceRepo)
	workloadService := services.NewWorkloadService(serviceRepo, deployer, logMgr)
	_ = workloadService.ReconcileOrphanedDeployments(context.Background())
	workloadService.StartGCScheduler(context.Background())
	serviceHandler := services.NewHandler(workloadService, authMiddleware)

	runtimeService := runtime.NewService(podmanClient)
	runtimeService.SetMetricsStore(runtime.NewMetricsStore(sqlDB))
	runtimeService.SetServiceScopeResolver(func(ctx context.Context, serviceID string) (string, bool) {
		svc, err := serviceRepo.Get(ctx, serviceID)
		if err != nil || svc == nil {
			return "", false
		}
		return svc.ProjectID, true
	})
	go runtimeService.StartMetricsSampler(context.Background())
	runtimeHandler := runtime.NewHandler(runtimeService, authMiddleware)
	// Quadlet API responses mask env values GOPOD stores encrypted; the unit
	// files keep the real value because systemd needs it.
	runtimeService.SetEnvSecretKeyProvider(func() map[string]bool {
		list, err := serviceRepo.List(context.Background(), "")
		if err != nil {
			return nil
		}
		keys := map[string]bool{}
		for _, svc := range list {
			for _, v := range svc.EnvVars {
				if v.Secret && strings.TrimSpace(v.Key) != "" {
					keys[v.Key] = true
				}
			}
		}
		return keys
	})

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
	ingressService.SetSettingsDB(sqlDB)
	ingressService.SetServerIPProvider(func() (string, bool) {
		info := podman.DetectHostIPInfo()
		return info.IP, info.IsPublic
	})
	_, _ = ingressService.LoadPanelSettings(context.Background())
	ingressHandler := ingress.NewHandler(ingressService, authMiddleware)

	// Reconcile Caddy on startup with retry backoff so a concurrent Caddy boot
	// has time to create its admin unix socket before we give up.
	go func() {
		backoffs := []time.Duration{0, 1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
		for i, delay := range backoffs {
			if delay > 0 {
				time.Sleep(delay)
			}
			bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			domains, err := ingressRepo.List(bgCtx, "")
			if err == nil {
				if i == 0 {
					// Rewrite plaintext basic-auth passwords at rest to bcrypt hashes.
					ingressService.MigrateBasicAuthHashes(bgCtx, domains)
				}
				recErr := caddyReconciler.Reconcile(bgCtx, domains)
				cancel()
				if recErr == nil {
					return
				}
			} else {
				cancel()
			}
		}
	}()

	storageRepo := storage.NewSQLiteRepository(sqlDB)
	storageService := storage.NewService(storageRepo)
	storageService.SetSystemSource(sqlDB, dataDir)
	storageService.SetServiceEnvResolver(func(ctx context.Context, serviceID string) (map[string]string, error) {
		svc, err := serviceRepo.Get(ctx, serviceID)
		if err != nil || svc == nil {
			return nil, err
		}
		envMap := make(map[string]string, len(svc.EnvVars))
		for _, ev := range svc.EnvVars {
			envMap[ev.Key] = ev.Value
		}
		return envMap, nil
	})
	storageService.StartScheduler(context.Background())
	storageHandler := storage.NewHandler(storageService, authMiddleware)

	auditRepo := audit.NewSQLiteRepository(sqlDB)
	auditService := audit.NewService(auditRepo)
	auditHandler := audit.NewHandler(auditService, authMiddleware)

	activity.Init(sqlDB)
	if !keyOnDisk {
		activity.EmitEngineEvent(context.Background(), "warn", "system", "", "", "engine", "system.encryption_key_missing",
			"No persisted encryption key found: new secrets could not be decrypted after a restart.", nil)
	}
	activityStore := activity.NewStore(sqlDB)
	activityHandler := activity.NewHandler(activityStore, authMiddleware)

	authService.SetAuditRecorder(auditService)
	authService.SetActivityRecorder(activityRecorderFunc(activity.RecordActivity))
	workloadService.SetAuditRecorder(auditService)
	workloadService.SetNotifier(notifService)
	storageService.SetAuditRecorder(auditService)
	storageService.SetNotifier(notifService)

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
	auth.RegisterAdminRoutes(mux, authMiddleware, accessChecker)
	projectHandler.RegisterRoutes(mux)
	serviceHandler.RegisterRoutes(mux)
	runtimeHandler.RegisterRoutes(mux)
	ingressHandler.RegisterRoutes(mux)
	credentialsHandler.RegisterRoutes(mux)
	storageHandler.RegisterRoutes(mux)
	auditHandler.RegisterRoutes(mux)
	activityHandler.RegisterRoutes(mux)
	notifHandler.RegisterRoutes(mux)
	updaterService := updater.NewService(podmanClient, dataDir, auditService, notifService)
	updaterHandler := updater.NewHandler(updaterService, authMiddleware)
	updaterHandler.RegisterRoutes(mux)

	// ── License & Build Metadata (AGPL-3.0 §13) ──
	// Public route: any user interacting with the service over the network can
	// discover under which license, from which repository, and from which VCS
	// revision the running build was produced.
	mux.HandleFunc("GET /api/about", aboutHandler)

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

	return withMiddleware(mux), nil
}

// aboutHandler reports the license and provenance metadata of the running
// build. AGPL-3.0 section 13 requires operators who let users interact with a
// modified version over a network to offer the Corresponding Source of that
// version; this endpoint gives every remote user the license identifier, the
// upstream repository, and the VCS revision the binary was built from, so any
// modified build can be traced to the source tree it shipped from.
func aboutHandler(w http.ResponseWriter, r *http.Request) {
	revision := ""
	vcsModified := false
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				revision = s.Value
			case "vcs.modified":
				vcsModified = s.Value == "true"
			}
		}
	}
	source := "https://github.com/ijon6k/gopod"
	if revision != "" {
		source += "/tree/" + revision
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name":           "GoPod",
		"version":        updater.Version,
		"license":        "AGPL-3.0-only",
		"source_url":     source,
		"build_revision": revision,
		"vcs_modified":   vcsModified,
	})
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		origin := r.Header.Get("Origin")
		if origin != "" && httputil.IsOriginAllowed(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Add("Vary", "Origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'self'; base-uri 'self'; object-src 'none';")

		isTrusted := auth.IsTrustedProxy(r.RemoteAddr)
		isSecure := r.TLS != nil ||
			(isTrusted && (strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") ||
				strings.EqualFold(r.Header.Get("X-Forwarded-Scheme"), "https") ||
				strings.EqualFold(r.Header.Get("X-Forwarded-Ssl"), "on")))
		if isSecure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		// Handler panics must answer the client (500) instead of dropping the
		// connection silently, and must be loud in the log with a stack trace.
		// http.ErrAbortHandler is repanicked: it is the stdlib's own contract
		// for aborting a hijacked/streamed response.
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				log.Printf("[PANIC] %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				httputil.WriteError(wrapped, http.StatusInternalServerError, "Internal server error")
			}
		}()
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
