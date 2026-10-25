package ingress

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ServiceMeta provides workload metadata needed for ingress upstream resolution.
type ServiceMeta struct {
	ID        string
	Name      string
	Type      string
	InPod     bool
	PodID     string
	ProjectID string
}

// ServiceResolver queries service metadata by service ID or name.
type ServiceResolver func(ctx context.Context, idOrName string) (*ServiceMeta, error)

// Reconciler synchronizes database domain routes into Caddy reverse proxy.
type Reconciler struct {
	adminURL string
	// configuredAdminURL is the admin endpoint as provided by configuration. At
	// runtime adminURL may switch to a fallback endpoint; bind decisions in the
	// generated Caddyfile must stay based on the configured value.
	configuredAdminURL string
	caddyfilePath      string
	logFilePath        string
	controlPort        string
	httpPort           int
	httpsPort          int
	httpClient         *http.Client
	mu                 sync.RWMutex
	hostDomainMap      map[string]Domain
	serviceResolver    ServiceResolver
}

// AccessLogItem represents a structured Caddy JSON access log.
type AccessLogItem struct {
	ID          string  `json:"id"`
	Timestamp   string  `json:"timestamp"`
	TimeAgo     string  `json:"timeAgo"`
	ClientIP    string  `json:"clientIp"`
	Method      string  `json:"method"`
	Host        string  `json:"host"`
	URI         string  `json:"uri"`
	Status      int     `json:"status"`
	DurationMs  float64 `json:"durationMs"`
	BytesSent   string  `json:"bytesSent"`
	Upstream    string  `json:"upstream"`
	ServiceID   string  `json:"serviceId,omitempty"`
	ProjectID   string  `json:"projectId,omitempty"`
	ServiceName string  `json:"serviceName,omitempty"`
}

// NewReconciler creates a Caddy synchronizer.
func NewReconciler(adminURL, caddyfilePath string) *Reconciler {
	if adminURL == "" {
		adminURL = "http://127.0.0.1:2019"
	}
	if caddyfilePath == "" {
		if _, err := os.Stat("./caddy/Caddyfile"); err == nil {
			caddyfilePath = "./caddy/Caddyfile"
		} else if _, err := os.Stat("../caddy/Caddyfile"); err == nil {
			caddyfilePath = "../caddy/Caddyfile"
		} else {
			home, err := os.UserHomeDir()
			if err == nil {
				caddyfilePath = filepath.Join(home, ".config", "gopod", "caddy", "Caddyfile")
			} else {
				caddyfilePath = "Caddyfile"
			}
		}
	}

	logPath := os.Getenv("CADDY_LOG_PATH")
	if logPath == "" {
		if _, err := os.Stat("/var/log/caddy/access.log"); err == nil {
			logPath = "/var/log/caddy/access.log"
		} else if _, err := os.Stat("./caddy/logs/access.log"); err == nil {
			logPath = "./caddy/logs/access.log"
		} else if _, err := os.Stat("../caddy/logs/access.log"); err == nil {
			logPath = "../caddy/logs/access.log"
		} else if _, err := os.Stat("./caddy/logs"); err == nil {
			logPath = "./caddy/logs/access.log"
		} else if _, err := os.Stat("../caddy/logs"); err == nil {
			logPath = "../caddy/logs/access.log"
		} else {
			home, err := os.UserHomeDir()
			if err == nil {
				logPath = filepath.Join(home, ".local", "share", "gopod", "logs", "access.log")
			} else {
				logPath = "/var/log/caddy/access.log"
			}
		}
	}
	_ = os.MkdirAll(filepath.Dir(logPath), 0755)

	ctrlPort := os.Getenv("PORT")
	if ctrlPort == "" {
		ctrlPort = "8080"
	}

	return &Reconciler{
		adminURL:           adminURL,
		configuredAdminURL: adminURL,
		caddyfilePath:      caddyfilePath,
		logFilePath:        logPath,
		controlPort:        ctrlPort,
		httpClient:         &http.Client{Timeout: 5 * time.Second},
	}
}

// SetControlPort sets the GOPOD dashboard/control plane port for reverse proxying.
func (r *Reconciler) SetControlPort(port string) {
	if port != "" {
		r.controlPort = port
	}
}

// SetServiceResolver sets the service metadata lookup function for intelligent Pod DNS routing.
func (r *Reconciler) SetServiceResolver(resolver ServiceResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serviceResolver = resolver
}

func sanitizePodSlug(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.TrimPrefix(lower, "pod_")
	var b strings.Builder
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' || r == '.' || r == '/' || r == ':' {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func canonicalPodHost(raw string) string {
	slug := sanitizePodSlug(raw)
	if slug == "" {
		slug = "pod-service"
	}
	return fmt.Sprintf("pod_%s", slug)
}

// SetIngressPorts manually sets the public/dev HTTP and HTTPS ports.
func (r *Reconciler) SetIngressPorts(httpPort, httpsPort int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if httpPort > 0 {
		r.httpPort = httpPort
	}
	if httpsPort > 0 {
		r.httpsPort = httpsPort
	}
}

// GetIngressPorts returns the active HTTP and HTTPS host ports.
func (r *Reconciler) GetIngressPorts() (int, int) {
	r.mu.RLock()
	hp, hsp := r.httpPort, r.httpsPort
	r.mu.RUnlock()
	if hp > 0 && hsp > 0 {
		return hp, hsp
	}
	return detectCaddyPorts()
}

func detectCaddyPorts() (int, int) {
	httpPort := 80
	httpsPort := 443
	if env := os.Getenv("HTTP_PORT"); env != "" {
		if p, err := strconv.Atoi(env); err == nil && p > 0 {
			httpPort = p
		}
	}
	if env := os.Getenv("HTTPS_PORT"); env != "" {
		if p, err := strconv.Atoi(env); err == nil && p > 0 {
			httpsPort = p
		}
	}

	// In dev or rootless, inspect active caddy container ports via Podman CLI
	cmd := exec.Command("podman", "ps", "--filter", "name=caddy", "--format", "{{.Ports}}")
	if out, err := cmd.Output(); err == nil && len(out) > 0 {
		portsStr := string(out)
		for _, part := range strings.Split(portsStr, ",") {
			part = strings.TrimSpace(part)
			if strings.Contains(part, "->80/tcp") {
				if idx := strings.Index(part, "->"); idx != -1 {
					hostPart := part[:idx]
					if colon := strings.LastIndex(hostPart, ":"); colon != -1 {
						if p, err := strconv.Atoi(hostPart[colon+1:]); err == nil && p > 0 {
							httpPort = p
						}
					}
				}
			} else if strings.Contains(part, "->443/tcp") {
				if idx := strings.Index(part, "->"); idx != -1 {
					hostPart := part[:idx]
					if colon := strings.LastIndex(hostPart, ":"); colon != -1 {
						if p, err := strconv.Atoi(hostPart[colon+1:]); err == nil && p > 0 {
							httpsPort = p
						}
					}
				}
			}
		}
	}
	return httpPort, httpsPort
}

// Reconcile generates the Caddyfile and hot-reloads Caddy via Admin API.
func (r *Reconciler) Reconcile(ctx context.Context, domains []Domain) error {
	r.mu.Lock()
	r.hostDomainMap = make(map[string]Domain, len(domains))
	for _, d := range domains {
		cleanHost := strings.ToLower(strings.TrimSpace(d.Hostname))
		if cleanHost != "" {
			r.hostDomainMap[cleanHost] = d
		}
	}
	r.mu.Unlock()

	content := r.GenerateCaddyfile(domains)

	// Verbose per-route mapping log: hostname -> upstream host:port + flags.
	validDomains := 0
	skippedDomains := 0
	for _, d := range domains {
		route, err := r.buildDomainRoute(d)
		if err != nil {
			skippedDomains++
			log.Printf("[WARN] Skipping invalid domain rule %q: %v", d.Hostname, err)
			continue
		}
		validDomains++
		targetUpstream := fmt.Sprintf("%s:%d", route.Upstream, d.ContainerPort)
		path := d.PathPrefix
		if path == "" {
			path = "/"
		}
		log.Printf("[ingress] route %s -> %s (tls=%v basicAuth=%v path=%s)", route.Hostname, targetUpstream, d.TLS, d.BasicAuth, path)
	}
	if skippedDomains > 0 {
		emitEngineEvent(ctx, "warn", "system", "", "", "engine", "ingress.domains_skipped",
			fmt.Sprintf("Ingress skipped %d invalid domain rule(s)", skippedDomains),
			map[string]any{"skipped": skippedDomains, "domains": len(domains)})
	}

	// 1. Hot-reload via the Caddy Admin API (/load with Caddyfile adapter),
	// with endpoint automatic fallback. An actively rejected config (HTTP 400+)
	// must never be persisted to disk: a broken Caddyfile would make Caddy
	// refuse to start at next boot. When no admin endpoint answers at all the
	// file is still written, so the config reaches Caddy on its next restart.
	endpoints := []string{r.adminURL}
	if r.adminURL == "http://caddy:2019" {
		endpoints = append(endpoints, "http://127.0.0.1:2019", "http://localhost:2019")
	} else if strings.Contains(r.adminURL, "127.0.0.1") || strings.Contains(r.adminURL, "localhost") {
		endpoints = append(endpoints, "http://caddy:2019")
	}

	var lastErr error
	rejected := false
	reloaded := false
	unreachable := 0
	for _, endpoint := range endpoints {
		// A dead endpoint must not stall the whole fallback chain (or the
		// persistence below) for the httpClient's full default timeout: bound
		// every single /load attempt independently.
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, 2*time.Second)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, endpoint+"/load", bytes.NewBufferString(content))
		if err != nil {
			cancelAttempt()
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "text/caddyfile")

		resp, err := r.httpClient.Do(req)
		cancelAttempt()
		if err != nil {
			lastErr = err
			unreachable++
			continue
		}
		// Close explicitly per iteration instead of deferring inside the loop
		// (defer would hold every response body until the loop exits).
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("caddy API reload returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			rejected = true
			continue
		}

		r.adminURL = endpoint
		log.Printf("✅ Caddy ingress updated with %d domain rules via %s", validDomains, endpoint)
		reloaded = true
		rejected = false
		break
	}

	if rejected {
		log.Printf("[WARN] Caddy rejected the new configuration, keeping the previous on-disk Caddyfile: %v; in-memory config is unchanged", lastErr)
	} else {
		// 2. Persist to disk so the config survives restarts. The file holds
		// bcrypt password hashes — owner-only permissions (chmod re-tightens
		// files that predate the 0600 rule).
		dir := filepath.Dir(r.caddyfilePath)
		_ = os.MkdirAll(dir, 0700)
		if err := os.WriteFile(r.caddyfilePath, []byte(content), 0600); err != nil {
			log.Printf("[WARN] Failed to write Caddyfile to disk (%s): %v", r.caddyfilePath, err)
		} else {
			_ = os.Chmod(r.caddyfilePath, 0600)
		}
	}

	if !reloaded && lastErr != nil {
		if rejected {
			log.Printf("[INFO] Caddy Admin API not reachable on remaining endpoints after a rejected config; the previous Caddyfile at %s stays in charge.", r.caddyfilePath)
		} else {
			log.Printf("[INFO] Caddy Admin API not reachable on any endpoint (%d tried: %v). Caddyfile written to %s, applied at next restart.", unreachable, lastErr, r.caddyfilePath)
		}
	}
	emitIngressReconcileEvent(ctx, validDomains, reloaded)
	return nil
}

// domainRoute is the cleaned, render-ready form of a stored domain rule.
type domainRoute struct {
	Hostname string // scheme/port stripped
	Upstream string // validated DNS host of the upstream, without port
	Path     string // validated path matcher ("/" = whole host)
}

// buildDomainRoute cleans and validates a stored domain rule for Caddyfile
// generation. It fails for entries that must not be interpolated into the file
// (invalid hostname, target or port) as defense-in-depth behind the API write
// boundaries, so a stored row can never inject configuration.
func (r *Reconciler) buildDomainRoute(d Domain) (domainRoute, error) {
	host := cleanHostname(d.Hostname)
	if host == "" {
		return domainRoute{}, errors.New("empty hostname")
	}
	if strings.ContainsAny(host, caddyfileMetachars) {
		return domainRoute{}, fmt.Errorf("hostname %q contains invalid characters", host)
	}

	if err := ValidateContainerPort(d.ContainerPort); err != nil {
		return domainRoute{}, err
	}

	// Render-ready field validation (defense in depth behind the API write
	// boundaries): a stored row with a Caddyfile-metacharacter path prefix or
	// basic-auth user is skipped entirely rather than escaping the sanitizer.
	cleanPath := strings.TrimSpace(d.PathPrefix)
	if cleanPath == "" {
		cleanPath = "/"
	}
	if err := ValidatePathPrefix(cleanPath); err != nil {
		return domainRoute{}, err
	}
	if err := ValidateBasicAuthUser(d.BasicAuthUser); err != nil {
		return domainRoute{}, err
	}

	upstreamHost := "127.0.0.1"
	if target := strings.TrimSpace(d.TargetContainer); target != "" {
		if err := ValidateTargetContainer(target); err != nil {
			return domainRoute{}, err
		}
		upstreamHost = target
	} else if svcID := strings.TrimSpace(d.ServiceID); svcID != "" {
		if err := ValidateTargetContainer(svcID); err != nil {
			return domainRoute{}, err
		}
		upstreamHost = svcID
	}

	// Intelligent Podman Pod DNS resolution:
	// In Podman Netavark DNS, all containers in a Pod share the network namespace,
	// and ONLY the Pod name (e.g. 'pod_cluster-demo') is registered in DNS.
	// Dialing individual member container names yields 'bad address' (502 Bad Gateway).
	if r.serviceResolver != nil {
		var targetSvc *ServiceMeta
		if d.ServiceID != "" {
			if s, err := r.serviceResolver(context.Background(), d.ServiceID); err == nil && s != nil {
				targetSvc = s
			}
		}
		if targetSvc == nil && d.TargetContainer != "" {
			if s, err := r.serviceResolver(context.Background(), d.TargetContainer); err == nil && s != nil {
				targetSvc = s
			}
		}

		if targetSvc != nil {
			if targetSvc.Type == "pod" {
				podHost := canonicalPodHost(targetSvc.Name)
				if strings.TrimSpace(targetSvc.PodID) != "" {
					podHost = canonicalPodHost(targetSvc.PodID)
				} else if strings.TrimSpace(targetSvc.ID) != "" && podHost == "pod_pod-service" {
					podHost = canonicalPodHost(targetSvc.ID)
				}
				upstreamHost = podHost
			} else if targetSvc.InPod || targetSvc.PodID != "" {
				podHost := canonicalPodHost(targetSvc.Name)
				if strings.TrimSpace(targetSvc.PodID) != "" {
					podHost = canonicalPodHost(targetSvc.PodID)
				}
				upstreamHost = podHost
			}
		}
	}

	return domainRoute{Hostname: host, Upstream: upstreamHost, Path: cleanPath}, nil
}

// adminListenAddress derives the admin API bind address for the generated
// Caddyfile.
//
// Host-mode installs (the default) run GOPOD and Caddy on the same machine, so
// the admin API only needs the loopback interface. Containerized deployments
// reach Caddy by its podman-network name (e.g. CADDY_ADMIN_URL is
// http://caddy:2019) and therefore require a non-loopback bind inside the
// container. The CADDY_ADMIN_HOST override may narrow the bind to localhost or
// a specific IP address, never a wildcard.
func (r *Reconciler) adminListenAddress() string {
	address := "localhost:2019"
	if !isLoopbackAdminURL(r.configuredAdminURL) {
		address = "0.0.0.0:2019"
	}
	if override := strings.TrimSpace(os.Getenv("CADDY_ADMIN_HOST")); override != "" {
		if parsed, ok := parseAdminBindOverride(override); ok {
			address = parsed
		} else {
			log.Printf("[WARN] Ignoring invalid CADDY_ADMIN_HOST %q: only localhost or a specific IP address is allowed", override)
		}
	}
	return address
}

// isLoopbackAdminURL reports whether the configured admin endpoint lives on the
// same loopback interface as GOPOD (host-mode install or a shared pod network
// namespace).
func isLoopbackAdminURL(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" || strings.HasPrefix(value, "unix:") || strings.HasPrefix(value, "unix/") {
		return true
	}
	host := value
	if u, err := url.Parse(value); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// GenerateCaddyfile constructs declarative Caddyfile configuration from domains while preserving the control plane.
func (r *Reconciler) GenerateCaddyfile(domains []Domain) string {
	var b strings.Builder

	adminListen := r.adminListenAddress()

	// Caddy's internal log file path inside container / daemon
	caddyLogFile := "/var/log/caddy/access.log"
	if envLog := os.Getenv("CADDY_CONTAINER_LOG_PATH"); envLog != "" {
		caddyLogFile = envLog
	}

	ctrlUpstream := fmt.Sprintf("127.0.0.1:%s", r.controlPort)
	if strings.Contains(r.adminURL, "caddy:") || os.Getenv("CADDY_ADMIN_URL") != "" {
		ctrlUpstream = fmt.Sprintf("gopod:%s", r.controlPort)
	}

	fmt.Fprintf(&b, `# Auto-generated by GOPOD — Do not edit manually
{
    admin %s
}

# Default Control Plane Dashboard Proxy
:80 {
    log {
        output file %s {
            roll_size 20MiB
            roll_keep 7
        }
        format json
    }
    reverse_proxy %s
}

`, adminListen, caddyLogFile, ctrlUpstream)

	for _, d := range domains {
		route, err := r.buildDomainRoute(d)
		if err != nil {
			log.Printf("[WARN] Skipping invalid domain rule %q (not written to Caddyfile): %v", d.Hostname, err)
			continue
		}
		cleanHost := route.Hostname

		isLocal := strings.HasSuffix(cleanHost, ".localhost") || strings.HasSuffix(cleanHost, ".local") || strings.Contains(cleanHost, "127.0.0.1")

		siteAddr := cleanHost
		if isLocal {
			if d.TLS {
				// Allow both HTTP (port 80/8080) and HTTPS (port 443/8443) on localhost development
				siteAddr = fmt.Sprintf("http://%s, https://%s", cleanHost, cleanHost)
			} else {
				siteAddr = fmt.Sprintf("http://%s", cleanHost)
			}
		} else if !d.TLS {
			siteAddr = "http://" + cleanHost
		}

		fmt.Fprintf(&b, "%s {\n", siteAddr)
		fmt.Fprintf(&b, "    log {\n        output file %s {\n            roll_size 20MiB\n            roll_keep 7\n        }\n        format json\n    }\n", caddyLogFile)

		b.WriteString("    encode zstd gzip\n")

		if d.CORS {
			b.WriteString("    header Access-Control-Allow-Origin *\n")
			b.WriteString("    header Access-Control-Allow-Methods \"GET, POST, PUT, DELETE, OPTIONS\"\n")
		}
		if d.HSTS && d.TLS && !isLocal {
			b.WriteString("    header Strict-Transport-Security \"max-age=31536000; includeSubDomains; preload\"\n")
		}
		if d.TLS && isLocal {
			b.WriteString("    tls internal\n")
		}
		if d.BasicAuth && d.BasicAuthUser != "" {
			// The route-level validation guarantees a metachar-free token;
			// TrimSpace only normalizes incidental surrounding whitespace.
			cleanUser := strings.TrimSpace(d.BasicAuthUser)

			passHash := strings.TrimSpace(d.BasicAuthPass)
			if passHash != "" && !strings.HasPrefix(passHash, "$2") {
				// security: never write the plaintext password when hashing
				// fails (e.g. bcrypt's 72-byte length cap on legacy rows) —
				// but publishing the route unprotected would be worse. The
				// whole site block is skipped below (fail closed).
				if h, err := bcrypt.GenerateFromPassword([]byte(passHash), bcrypt.DefaultCost); err == nil {
					passHash = string(h)
				} else {
					log.Printf("[WARN] Skipping domain %q: basicauth password cannot be hashed (%v) and rendering it would publish the route without protection", cleanHost, err)
					continue
				}
			}

			if cleanUser != "" && passHash != "" {
				b.WriteString("    basicauth {\n")
				b.WriteString(fmt.Sprintf("        %s %s\n", cleanUser, passHash))
				b.WriteString("    }\n")
			}
		}

		upstreamHost := route.Upstream

		targetUpstream := fmt.Sprintf("%s:%d", upstreamHost, d.ContainerPort)

		if path := route.Path; path != "/" {
			stripOrHandle := "handle"
			if d.StripPathPrefix {
				stripOrHandle = "handle_path"
			}
			b.WriteString(fmt.Sprintf("    %s %s* {\n", stripOrHandle, path))
			b.WriteString(fmt.Sprintf("        reverse_proxy %s\n", targetUpstream))
			b.WriteString("    }\n")
		} else {
			b.WriteString(fmt.Sprintf("    reverse_proxy %s\n", targetUpstream))
		}

		b.WriteString("}\n\n")
	}

	return b.String()
}

// ReadAccessLogs reads parsed JSON logs from disk returning the most recent entries.
func (r *Reconciler) ReadAccessLogs(limit int) []AccessLogItem {
	if limit <= 0 {
		limit = 50
	}

	candidates := []string{
		r.logFilePath,
		"./caddy/logs/access.log",
		"../caddy/logs/access.log",
		"/var/log/caddy/access.log",
	}
	if home, errHome := os.UserHomeDir(); errHome == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "share", "gopod", "logs", "access.log"))
	}

	var file *os.File
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if f, err := os.Open(p); err == nil {
			file = f
			break
		}
	}

	if file == nil {
		return []AccessLogItem{}
	}
	defer file.Close()

	// Bound disk I/O: seek to end of file if larger than tail buffer
	if fi, err := file.Stat(); err == nil {
		const bytesPerLineEstimate = 1024
		tailBytes := int64(limit * bytesPerLineEstimate * 2)
		if tailBytes < 64*1024 {
			tailBytes = 64 * 1024
		}
		if fi.Size() > tailBytes {
			_, _ = file.Seek(fi.Size()-tailBytes, io.SeekStart)
			// Discard first partial line after seek
			br := bufio.NewReader(file)
			_, _ = br.ReadString('\n')
		}
	}

	// Maintain circular buffer of the most recent lines
	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) == 0 {
			continue
		}
		if len(lines) < limit {
			lines = append(lines, text)
		} else {
			lines = append(lines[1:], text)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[WARN] Error reading access logs: %v", err)
	}

	// Parse in reverse chronological order (newest first)
	logs := make([]AccessLogItem, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		req, _ := raw["request"].(map[string]interface{})
		if req == nil {
			continue
		}
		uri, _ := req["uri"].(string)
		if uri == "" {
			continue
		}

		host, _ := req["host"].(string)
		method, _ := req["method"].(string)
		remoteIP, _ := req["remote_ip"].(string)
		status, _ := raw["status"].(float64)
		duration, _ := raw["duration"].(float64)
		ts, _ := raw["ts"].(float64)

		sec := int64(ts)
		nsec := int64((ts - float64(sec)) * 1e9)
		logTime := time.Unix(sec, nsec)

		bytesSent := "—"
		if sizeVal, ok := raw["size"].(float64); ok && sizeVal > 0 {
			if sizeVal < 1024 {
				bytesSent = fmt.Sprintf("%d B", int64(sizeVal))
			} else if sizeVal < 1024*1024 {
				bytesSent = fmt.Sprintf("%.1f KB", sizeVal/1024.0)
			} else {
				bytesSent = fmt.Sprintf("%.1f MB", sizeVal/(1024.0*1024.0))
			}
		} else if bytesVal, ok := raw["bytes_read"].(float64); ok && bytesVal > 0 {
			bytesSent = fmt.Sprintf("%.1f KB", bytesVal/1024.0)
		}

		upstream := "—"
		if u, ok := raw["upstream"].(string); ok && u != "" {
			upstream = u
		} else if u, ok := raw["upstream_addr"].(string); ok && u != "" {
			upstream = u
		} else if u, ok := raw["dial"].(string); ok && u != "" {
			upstream = u
		} else if host != "" {
			upstream = host
		}

		r.mu.RLock()
		hdMap := r.hostDomainMap
		r.mu.RUnlock()

		cleanHostKey := strings.ToLower(host)
		if idx := strings.Index(cleanHostKey, ":"); idx != -1 {
			cleanHostKey = cleanHostKey[:idx]
		}

		var serviceID, projectID, serviceName string
		if d, ok := hdMap[cleanHostKey]; ok {
			serviceID = d.ServiceID
			projectID = d.ProjectID
			if d.TargetContainer != "" {
				serviceName = d.TargetContainer
			} else if d.ServiceID != "" {
				serviceName = d.ServiceID
			}
		}

		logs = append(logs, AccessLogItem{
			ID:          fmt.Sprintf("log-%d", len(logs)+1),
			Timestamp:   logTime.Format(time.RFC3339),
			TimeAgo:     timeAgo(logTime),
			ClientIP:    remoteIP,
			Method:      method,
			Host:        host,
			URI:         uri,
			Status:      int(status),
			DurationMs:  duration * 1000,
			BytesSent:   bytesSent,
			Upstream:    upstream,
			ServiceID:   serviceID,
			ProjectID:   projectID,
			ServiceName: serviceName,
		})
	}

	return logs
}

func timeAgo(t time.Time) string {
	diff := time.Since(t)
	if diff < time.Minute {
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
}

// ReadCurrentCaddyfile returns the active raw Caddyfile from disk.
func (r *Reconciler) ReadCurrentCaddyfile() string {
	candidates := []string{
		r.caddyfilePath,
		"./caddy/Caddyfile",
		"/etc/caddy/Caddyfile",
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			return string(data)
		}
	}
	return r.GenerateCaddyfile(nil)
}

// GetAdminURL returns the configured Caddy Admin API endpoint.
func (r *Reconciler) GetAdminURL() string {
	return r.adminURL
}
