package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service coordinates audit logging business logic.
type Service struct {
	repo Repository
}

// NewService creates a new audit Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Record records a new audit event.
func (s *Service) Record(ctx context.Context, action, actor, target, category, ip, status string) error {
	if actor == "" {
		actor = "system"
	}
	if category == "" {
		category = "runtime"
	}
	if status == "" {
		status = "success"
	}

	log := AuditLog{
		ID:        fmt.Sprintf("aud-%s", uuid.New().String()[:8]),
		Action:    action,
		Actor:     actor,
		Target:    target,
		Category:  category,
		IP:        ip,
		Status:    status,
		CreatedAt: time.Now().UTC(),
	}

	return s.repo.Create(ctx, log)
}

// List returns audit logs filtered by category and limited to limit.
func (s *Service) List(ctx context.Context, category string, limit int) ([]AuditLog, error) {
	return s.repo.List(ctx, category, limit)
}
