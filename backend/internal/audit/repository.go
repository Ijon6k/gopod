package audit

import "context"

// Repository defines data access contract for audit logs.
type Repository interface {
	List(ctx context.Context, category string, limit int) ([]AuditLog, error)
	Create(ctx context.Context, log AuditLog) error
}
