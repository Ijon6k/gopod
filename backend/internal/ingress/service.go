package ingress

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
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
	return s.repo.List(ctx, projectID)
}

// GetDomain retrieves a single domain rule by ID.
func (s *Service) GetDomain(ctx context.Context, id string) (*Domain, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	return s.repo.Get(ctx, id)
}

// CreateDomain validates and stores a new domain rule, then triggers Caddy reload.
func (s *Service) CreateDomain(ctx context.Context, d Domain) (*Domain, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if d.Hostname == "" {
		return nil, errors.New("hostname is required")
	}

	if d.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		d.ID = fmt.Sprintf("dom-%s", hex.EncodeToString(b))
	}
	d.CreatedAt = time.Now()
	if d.DNSStatus == "" {
		d.DNSStatus = "active"
	}

	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}

	s.asyncReconcile(ctx)
	return &d, nil
}

// UpdateDomain updates an existing domain rule and refreshes Caddy.
func (s *Service) UpdateDomain(ctx context.Context, d Domain) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	if err := s.repo.Update(ctx, d); err != nil {
		return err
	}

	s.asyncReconcile(ctx)
	return nil
}

// DeleteDomain deletes a domain rule and refreshes Caddy.
func (s *Service) DeleteDomain(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

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
