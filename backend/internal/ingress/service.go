package ingress

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gopod/internal/activity"
)

// Service encapsulates ingress routing and telemetry business logic.
type Service struct {
	repo       Repository
	reconciler *Reconciler
}

// NewService creates a new ingress domain service.
func NewService(repo Repository, reconciler *Reconciler) *Service {
	return &Service{
		repo:       repo,
		reconciler: reconciler,
	}
}

// ListDomains retrieves all domains, optionally filtered by projectID.
func (s *Service) ListDomains(ctx context.Context, projectID string) ([]Domain, error) {
	if s.repo == nil {
		return []Domain{}, nil
	}
	domains, err := s.repo.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for i := range domains {
		cleanHost := strings.ToLower(strings.TrimSpace(domains[i].Hostname))
		if strings.HasSuffix(cleanHost, ".localhost") || strings.HasSuffix(cleanHost, ".local") || strings.Contains(cleanHost, "127.0.0.1") || cleanHost == "localhost" {
			if domains[i].DNSStatus == "pending" || domains[i].DNSStatus == "" {
				domains[i].DNSStatus = "valid"
				_ = s.repo.Update(ctx, domains[i])
			}
		}
		domains[i].computeBasicAuthState()
	}
	return domains, nil
}

// GetDomain retrieves a single domain rule by ID.
func (s *Service) GetDomain(ctx context.Context, id string) (*Domain, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	d, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if d != nil {
		d.computeBasicAuthState()
	}
	return d, nil
}

// RevalidateDomain re-checks DNS for a domain rule and updates the database.
func (s *Service) RevalidateDomain(ctx context.Context, id string) (*Domain, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	d, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, errors.New("domain not found")
	}

	cleanHost := strings.ToLower(strings.TrimSpace(d.Hostname))
	cleanHost = strings.TrimPrefix(cleanHost, "https://")
	cleanHost = strings.TrimPrefix(cleanHost, "http://")
	if idx := strings.Index(cleanHost, ":"); idx != -1 {
		cleanHost = cleanHost[:idx]
	}

	isLoopback := strings.HasSuffix(cleanHost, ".localhost") || strings.HasSuffix(cleanHost, ".local") || strings.Contains(cleanHost, "127.0.0.1") || cleanHost == "localhost"

	if isLoopback {
		d.DNSStatus = "valid"
	} else {
		// Real DNS resolution check using net.Resolver
		lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		var resolver net.Resolver
		ips, err := resolver.LookupIP(lookupCtx, "ip", cleanHost)
		if err == nil && len(ips) > 0 {
			d.DNSStatus = "valid"
		} else {
			d.DNSStatus = "pending"
		}
	}

	if err := s.repo.Update(ctx, *d); err != nil {
		return nil, err
	}

	d.computeBasicAuthState()
	return d, nil
}

// cleanHostname strips the scheme and the port from a user-supplied hostname.
func cleanHostname(raw string) string {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "https://")
	clean = strings.TrimPrefix(clean, "http://")
	if idx := strings.Index(clean, ":"); idx != -1 {
		clean = clean[:idx]
	}
	return clean
}

// validateRouteFields validates every stored value that is interpolated into
// a generated Caddyfile route: the upstream identity, the port, the optional
// path matcher and the optional basic-auth username.
func validateRouteFields(d *Domain) error {
	if strings.TrimSpace(d.TargetContainer) != "" {
		if err := ValidateTargetContainer(d.TargetContainer); err != nil {
			return err
		}
	}
	if strings.TrimSpace(d.ServiceID) != "" {
		if err := ValidateTargetContainer(d.ServiceID); err != nil {
			return err
		}
	}
	if err := ValidateContainerPort(d.ContainerPort); err != nil {
		return err
	}
	clean := strings.TrimSpace(d.PathPrefix)
	if clean == "" || clean == "/" {
		clean = "/"
	}
	if err := ValidatePathPrefix(clean); err != nil {
		return err
	}
	d.PathPrefix = clean
	return ValidateBasicAuthUser(d.BasicAuthUser)
}

// hashBasicAuthPass returns the bcrypt hash of a plaintext password for storage
// at rest. Values that are already bcrypt hashes ($2a/$2b/$2y) pass through
// unchanged; an empty password stays empty.
func hashBasicAuthPass(pass string) (string, error) {
	clean := strings.TrimSpace(pass)
	if clean == "" || strings.HasPrefix(clean, "$2") {
		return clean, nil
	}
	h, err := bcrypt.GenerateFromPassword([]byte(clean), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("%w: password cannot be hashed: %v", ErrValidation, err)
	}
	return string(h), nil
}

// MigrateBasicAuthHashes rewrites plaintext basic-auth passwords as bcrypt
// hashes stored at rest. Safe to call on every startup: rows that are
// already hashed (or empty) are left untouched. The passed slice is updated in
// place so a following reconcile renders fresh state.
func (s *Service) MigrateBasicAuthHashes(ctx context.Context, domains []Domain) {
	if s.repo == nil {
		return
	}
	for i := range domains {
		pass := strings.TrimSpace(domains[i].BasicAuthPass)
		if pass == "" || strings.HasPrefix(pass, "$2") {
			continue
		}
		hashed, err := hashBasicAuthPass(pass)
		if err != nil {
			log.Printf("[WARN] Cannot migrate basic-auth password for domain %q: %v", domains[i].Hostname, err)
			continue
		}
		domains[i].BasicAuthPass = hashed
		domains[i].computeBasicAuthState()
		if err := s.repo.Update(ctx, domains[i]); err != nil {
			log.Printf("[WARN] Failed to persist migrated basic-auth password for domain %q: %v", domains[i].Hostname, err)
			continue
		}
		log.Printf("[ingress] Migrated plaintext basic-auth password for domain %q to bcrypt", domains[i].Hostname)
	}
}

// CreateDomain validates and stores a new domain rule, then triggers Caddy reload.
func (s *Service) CreateDomain(ctx context.Context, d Domain) (*Domain, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}

	cleanHost := cleanHostname(d.Hostname)
	if err := ValidateHostname(cleanHost); err != nil {
		return nil, err
	}
	d.Hostname = cleanHost

	if err := validateRouteFields(&d); err != nil {
		return nil, err
	}

	pass, err := hashBasicAuthPass(d.BasicAuthPass)
	if err != nil {
		return nil, err
	}
	d.BasicAuthPass = pass
	d.computeBasicAuthState()

	isLoopback := strings.HasSuffix(cleanHost, ".localhost") || strings.HasSuffix(cleanHost, ".local") || strings.Contains(cleanHost, "127.0.0.1") || cleanHost == "localhost"

	if d.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		d.ID = fmt.Sprintf("dom-%s", hex.EncodeToString(b))
	}
	d.CreatedAt = time.Now()
	if isLoopback {
		d.DNSStatus = "valid"
	} else if d.DNSStatus == "" {
		d.DNSStatus = "pending"
	}

	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}

	activity.RecordActivity(ctx, "operator", "domain.create", "domain", d.ID, d.ProjectID, d.ServiceID, "success",
		fmt.Sprintf("Domain '%s' created -> service '%s' port %d", d.Hostname, d.ServiceID, d.ContainerPort), "")
	s.asyncReconcile(ctx)
	return &d, nil
}

// UpdateDomain validates and updates an existing domain rule, then refreshes
// Caddy. A missing/empty basic-auth password keeps the stored hash
// (empty = keep); a new password is hashed before storage.
func (s *Service) UpdateDomain(ctx context.Context, d Domain) (*Domain, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}

	cleanHost := cleanHostname(d.Hostname)
	if err := ValidateHostname(cleanHost); err != nil {
		return nil, err
	}
	d.Hostname = cleanHost

	if err := validateRouteFields(&d); err != nil {
		return nil, err
	}

	existing, err := s.repo.Get(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(d.BasicAuthPass) == "" {
		d.BasicAuthPass = ""
		if existing != nil {
			d.BasicAuthPass = existing.BasicAuthPass
		}
	} else {
		pass, err := hashBasicAuthPass(d.BasicAuthPass)
		if err != nil {
			return nil, err
		}
		d.BasicAuthPass = pass
	}
	d.computeBasicAuthState()

	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}

	s.asyncReconcile(ctx)
	return &d, nil
}

// DeleteDomain deletes a domain rule and refreshes Caddy.
func (s *Service) DeleteDomain(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	existing, _ := s.repo.Get(ctx, id)
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	hostname, projectID, serviceID := id, "", ""
	if existing != nil {
		hostname, projectID, serviceID = existing.Hostname, existing.ProjectID, existing.ServiceID
	}
	activity.RecordActivity(ctx, "operator", "domain.delete", "domain", id, projectID, serviceID, "success",
		fmt.Sprintf("Domain '%s' deleted", hostname), "")

	s.asyncReconcile(ctx)
	return nil
}

// TrafficFilter specifies query criteria for Caddy access logs.
type TrafficFilter struct {
	Limit            int
	ProjectID        string
	ServiceID        string
	StatusGroup      string // "2xx", "3xx", "4xx", "5xx"
	TimeRange        string // "1h", "24h", "7d"
	Search           string
	ExcludeDashboard bool // Exclude GOPOD control plane and internal telemetry polling requests
}

// TrafficMetrics summarizes Caddy reverse proxy telemetry.
type TrafficMetrics struct {
	TotalRequests int     `json:"totalRequests"`
	SuccessRate   float64 `json:"successRate"`
	Error4xx      int     `json:"error4xx"`
	Error5xx      int     `json:"error5xx"`
	AvgLatency    float64 `json:"avgLatency"`
}

// GetTrafficRequests fetches recent access logs from Caddy with optional filtering.
func (s *Service) GetTrafficRequests(filter TrafficFilter) []AccessLogItem {
	if s.reconciler == nil {
		return []AccessLogItem{}
	}

	raw := s.reconciler.ReadAccessLogs(500)
	filtered := s.filterLogs(raw, filter)

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// GetTrafficMetrics calculates ingress statistics.
func (s *Service) GetTrafficMetrics(filter TrafficFilter) TrafficMetrics {
	if s.reconciler == nil {
		return TrafficMetrics{SuccessRate: 100.0}
	}

	raw := s.reconciler.ReadAccessLogs(500)
	filtered := s.filterLogs(raw, filter)

	total := len(filtered)
	if total == 0 {
		return TrafficMetrics{SuccessRate: 100.0}
	}

	var s2xx3xx, s4xx, s5xx int
	var totalDuration float64
	for _, l := range filtered {
		if l.Status >= 200 && l.Status < 400 {
			s2xx3xx++
		} else if l.Status >= 400 && l.Status < 500 {
			s4xx++
		} else if l.Status >= 500 {
			s5xx++
		}
		totalDuration += l.DurationMs
	}

	rate := (float64(s2xx3xx) / float64(total)) * 100.0
	avgLat := totalDuration / float64(total)

	return TrafficMetrics{
		TotalRequests: total,
		SuccessRate:   rate,
		Error4xx:      s4xx,
		Error5xx:      s5xx,
		AvgLatency:    avgLat,
	}
}

func (s *Service) filterLogs(logs []AccessLogItem, filter TrafficFilter) []AccessLogItem {
	var result []AccessLogItem
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))

	for _, l := range logs {
		if filter.ExcludeDashboard {
			// Exclude GOPOD control plane and internal telemetry polling requests
			isDashboardHost := l.Host == "localhost" || l.Host == "127.0.0.1" || strings.HasPrefix(l.Host, "localhost:") || strings.HasPrefix(l.Host, "127.0.0.1:")
			isApiOrInternal := strings.HasPrefix(l.URI, "/api/") || l.URI == "/" || strings.HasPrefix(l.URI, "/_app/")
			if isDashboardHost && (l.ServiceID == "" || l.ServiceID == "gopod") && isApiOrInternal {
				continue
			}
		}

		if filter.ProjectID != "" && filter.ProjectID != "all" && l.ProjectID != filter.ProjectID {
			continue
		}
		if filter.ServiceID != "" && filter.ServiceID != "all" && l.ServiceID != filter.ServiceID {
			continue
		}

		if filter.StatusGroup != "" && filter.StatusGroup != "all" {
			switch filter.StatusGroup {
			case "2xx":
				if l.Status < 200 || l.Status >= 300 {
					continue
				}
			case "3xx":
				if l.Status < 300 || l.Status >= 400 {
					continue
				}
			case "4xx":
				if l.Status < 400 || l.Status >= 500 {
					continue
				}
			case "5xx":
				if l.Status < 500 {
					continue
				}
			}
		}

		if filter.TimeRange != "" && filter.TimeRange != "all" {
			t, err := time.Parse(time.RFC3339, l.Timestamp)
			if err == nil {
				age := time.Since(t)
				switch filter.TimeRange {
				case "1h":
					if age > time.Hour {
						continue
					}
				case "24h":
					if age > 24*time.Hour {
						continue
					}
				case "7d":
					if age > 7*24*time.Hour {
						continue
					}
				}
			}
		}

		if searchLower != "" {
			match := strings.Contains(strings.ToLower(l.URI), searchLower) ||
				strings.Contains(strings.ToLower(l.Host), searchLower) ||
				strings.Contains(strings.ToLower(l.ClientIP), searchLower) ||
				strings.Contains(strings.ToLower(l.Method), searchLower)
			if !match {
				continue
			}
		}

		result = append(result, l)
	}
	return result
}

// GetCaddyfile returns the raw Caddyfile.
func (s *Service) GetCaddyfile() string {
	if s.reconciler == nil {
		return ""
	}
	return s.reconciler.ReadCurrentCaddyfile()
}

// ReloadCaddy manually reconciles and reloads Caddy.
func (s *Service) ReloadCaddy(ctx context.Context) error {
	if s.reconciler == nil || s.repo == nil {
		return errors.New("reconciler or repository not initialized")
	}
	domains, err := s.repo.List(ctx, "")
	if err != nil {
		return err
	}
	return s.reconciler.Reconcile(ctx, domains)
}

func (s *Service) asyncReconcile(ctx context.Context) {
	if s.reconciler == nil || s.repo == nil {
		return
	}
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		domains, err := s.repo.List(bgCtx, "")
		if err != nil {
			log.Printf("[WARN] Failed to fetch domains for Caddy reconciliation: %v", err)
			return
		}
		if err := s.reconciler.Reconcile(bgCtx, domains); err != nil {
			log.Printf("[WARN] Ingress reconciliation failed: %v", err)
		}
	}()
}
