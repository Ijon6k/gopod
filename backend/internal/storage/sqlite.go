package storage

import (
	"context"
	"database/sql"
)

// SQLiteRepository implements storage.Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository for storage backups.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// ── Snapshots ──

func (r *SQLiteRepository) ListSnapshots(ctx context.Context, projectID string) ([]VolumeSnapshot, error) {
	var query string
	var args []interface{}
	if projectID != "" {
		query = "SELECT id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression, created_at FROM volume_snapshots WHERE project_id = ? ORDER BY created_at DESC"
		args = append(args, projectID)
	} else {
		query = "SELECT id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression, created_at FROM volume_snapshots ORDER BY created_at DESC"
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []VolumeSnapshot
	for rows.Next() {
		var s VolumeSnapshot
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ServiceID, &s.VolumeName, &s.Filename, &s.Size, &s.SizeBytes, &s.Status, &s.Compression, &s.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if list == nil {
		list = []VolumeSnapshot{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) GetSnapshot(ctx context.Context, id string) (*VolumeSnapshot, error) {
	var s VolumeSnapshot
	err := r.db.QueryRowContext(ctx,
		"SELECT id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression, created_at FROM volume_snapshots WHERE id = ?", id).
		Scan(&s.ID, &s.ProjectID, &s.ServiceID, &s.VolumeName, &s.Filename, &s.Size, &s.SizeBytes, &s.Status, &s.Compression, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SQLiteRepository) CreateSnapshot(ctx context.Context, s VolumeSnapshot) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO volume_snapshots (id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.ServiceID, s.VolumeName, s.Filename, s.Size, s.SizeBytes, s.Status, s.Compression,
	)
	return err
}

func (r *SQLiteRepository) DeleteSnapshot(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM volume_snapshots WHERE id = ?", id)
	return err
}

// ── Schedules ──

func (r *SQLiteRepository) ListSchedules(ctx context.Context, projectID string) ([]VolumeSchedule, error) {
	var query string
	var args []interface{}
	if projectID != "" {
		query = "SELECT id, project_id, volume_name, cron, label, retention_count, enabled, last_run, created_at FROM volume_schedules WHERE project_id = ? ORDER BY created_at DESC"
		args = append(args, projectID)
	} else {
		query = "SELECT id, project_id, volume_name, cron, label, retention_count, enabled, last_run, created_at FROM volume_schedules ORDER BY created_at DESC"
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []VolumeSchedule
	for rows.Next() {
		var s VolumeSchedule
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.VolumeName, &s.Cron, &s.Label, &s.RetentionCount, &s.Enabled, &s.LastRun, &s.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if list == nil {
		list = []VolumeSchedule{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) GetSchedule(ctx context.Context, id string) (*VolumeSchedule, error) {
	var s VolumeSchedule
	err := r.db.QueryRowContext(ctx,
		"SELECT id, project_id, volume_name, cron, label, retention_count, enabled, last_run, created_at FROM volume_schedules WHERE id = ?", id).
		Scan(&s.ID, &s.ProjectID, &s.VolumeName, &s.Cron, &s.Label, &s.RetentionCount, &s.Enabled, &s.LastRun, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SQLiteRepository) CreateSchedule(ctx context.Context, s VolumeSchedule) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO volume_schedules (id, project_id, volume_name, cron, label, retention_count, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.VolumeName, s.Cron, s.Label, s.RetentionCount, s.Enabled,
	)
	return err
}

func (r *SQLiteRepository) DeleteSchedule(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM volume_schedules WHERE id = ?", id)
	return err
}
