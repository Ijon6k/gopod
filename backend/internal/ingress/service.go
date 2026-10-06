package ingress

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
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

// GetTrafficRequests fetches recent access logs from Caddy.
func (s *Service) GetTrafficRequests(limit int) []AccessLogItem {
	if s.reconciler == nil {
		return []AccessLogItem{}
	}
	return s.reconciler.ReadAccessLogs(limit)
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
