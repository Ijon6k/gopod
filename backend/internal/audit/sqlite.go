package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SQLiteRepository implements audit.Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository for audit logs.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// List returns audit logs optionally filtered by category.
func (r *SQLiteRepository) List(ctx context.Context, category string, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if category != "" && category != "all" {
		query := "SELECT id, action, actor, target, category, ip, status, created_at FROM audit_logs WHERE category = ? ORDER BY created_at DESC LIMIT ?"
		rows, err = r.db.QueryContext(ctx, query, category, limit)
	} else {
		query := "SELECT id, action, actor, target, category, ip, status, created_at FROM audit_logs ORDER BY created_at DESC LIMIT ?"
		rows, err = r.db.QueryContext(ctx, query, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var createdAtRaw interface{}
		if err := rows.Scan(&log.ID, &log.Action, &log.Actor, &log.Target, &log.Category, &log.IP, &log.Status, &createdAtRaw); err != nil {
			return nil, err
		}

		if t, ok := createdAtRaw.(time.Time); ok {
			log.CreatedAt = t
		} else if s, ok := createdAtRaw.(string); ok {
			if parsed, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
				log.CreatedAt = parsed
			} else if parsed, err := time.Parse(time.RFC3339, s); err == nil {
				log.CreatedAt = parsed
			}
		}

		log.TimeAgo = formatTimeAgo(log.CreatedAt)
		logs = append(logs, log)
	}

	if logs == nil {
		logs = []AuditLog{}
	}
	return logs, rows.Err()
}

// Create inserts a new audit log.
func (r *SQLiteRepository) Create(ctx context.Context, log AuditLog) error {
	now := time.Now().UTC()
	if log.CreatedAt.IsZero() {
		log.CreatedAt = now
	}
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO audit_logs (id, action, actor, target, category, ip, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		log.ID, log.Action, log.Actor, log.Target, log.Category, log.IP, log.Status, log.CreatedAt,
	)
	return err
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "Recently"
	}
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "Just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}
