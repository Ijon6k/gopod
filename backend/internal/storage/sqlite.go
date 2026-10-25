package storage

import (
	"context"
	"database/sql"

	"gopod/pkg/crypto"
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
	const snapCols = "id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression, COALESCE(remote_provider,''), COALESCE(remote_destination_id,''), COALESCE(remote_key,''), COALESCE(checksum,''), COALESCE(remote_status,''), COALESCE(remote_error,''), created_at"
	var query string
	var args []interface{}
	if projectID != "" {
		query = "SELECT " + snapCols + " FROM volume_snapshots WHERE project_id = ? ORDER BY created_at DESC"
		args = append(args, projectID)
	} else {
		query = "SELECT " + snapCols + " FROM volume_snapshots ORDER BY created_at DESC"
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []VolumeSnapshot
	for rows.Next() {
		var s VolumeSnapshot
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ServiceID, &s.VolumeName, &s.Filename, &s.Size, &s.SizeBytes, &s.Status, &s.Compression, &s.RemoteProvider, &s.RemoteDestinationID, &s.RemoteKey, &s.Checksum, &s.RemoteStatus, &s.RemoteError, &s.CreatedAt); err != nil {
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
		"SELECT id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression, COALESCE(remote_provider,''), COALESCE(remote_destination_id,''), COALESCE(remote_key,''), COALESCE(checksum,''), COALESCE(remote_status,''), COALESCE(remote_error,''), created_at FROM volume_snapshots WHERE id = ?", id).
		Scan(&s.ID, &s.ProjectID, &s.ServiceID, &s.VolumeName, &s.Filename, &s.Size, &s.SizeBytes, &s.Status, &s.Compression, &s.RemoteProvider, &s.RemoteDestinationID, &s.RemoteKey, &s.Checksum, &s.RemoteStatus, &s.RemoteError, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateSnapshotRemote records the outcome of a remote-mirror push. The local
// row is never mutated beyond remote metadata: verify-before-mutate happens
// in the service layer.
func (r *SQLiteRepository) UpdateSnapshotRemote(ctx context.Context, id string, result SnapshotRemoteState) error {
	_, err := r.db.ExecContext(ctx, `UPDATE volume_snapshots
		SET remote_provider = ?, remote_destination_id = ?, remote_key = ?, checksum = ?, remote_status = ?, remote_error = ?
		WHERE id = ?`,
		result.Provider, result.DestinationID, result.RemoteKey, result.Checksum, result.Status, result.ErrText, id,
	)
	return err
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
		query = "SELECT id, project_id, COALESCE(service_id,''), volume_name, cron, label, retention_count, enabled, last_run, COALESCE(destination_id,''), created_at FROM volume_schedules WHERE project_id = ? ORDER BY created_at DESC"
		args = append(args, projectID)
	} else {
		query = "SELECT id, project_id, COALESCE(service_id,''), volume_name, cron, label, retention_count, enabled, last_run, COALESCE(destination_id,''), created_at FROM volume_schedules ORDER BY created_at DESC"
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []VolumeSchedule
	for rows.Next() {
		var s VolumeSchedule
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ServiceID, &s.VolumeName, &s.Cron, &s.Label, &s.RetentionCount, &s.Enabled, &s.LastRun, &s.DestinationID, &s.CreatedAt); err != nil {
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
		"SELECT id, project_id, COALESCE(service_id,''), volume_name, cron, label, retention_count, enabled, last_run, COALESCE(destination_id,''), created_at FROM volume_schedules WHERE id = ?", id).
		Scan(&s.ID, &s.ProjectID, &s.ServiceID, &s.VolumeName, &s.Cron, &s.Label, &s.RetentionCount, &s.Enabled, &s.LastRun, &s.DestinationID, &s.CreatedAt)
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
		`INSERT INTO volume_schedules (id, project_id, service_id, volume_name, cron, label, retention_count, enabled, destination_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, ''))`,
		s.ID, s.ProjectID, nullableString(s.ServiceID), s.VolumeName, s.Cron, s.Label, s.RetentionCount, s.Enabled, nullableString(s.DestinationID),
	)
	return err
}

func (r *SQLiteRepository) UpdateSchedule(ctx context.Context, s VolumeSchedule) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE volume_schedules SET
			enabled = ?, cron = ?, label = ?, retention_count = ?, destination_id = COALESCE(?, destination_id), service_id = COALESCE(?, service_id)
		 WHERE id = ?`,
		s.Enabled, s.Cron, s.Label, s.RetentionCount, nullableString(s.DestinationID), nullableString(s.ServiceID), s.ID,
	)
	return err
}

func (r *SQLiteRepository) DeleteSchedule(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM volume_schedules WHERE id = ?", id)
	return err
}

// ── Destinations (S3-compatible mirrors) ──

func (r *SQLiteRepository) ListDestinations(ctx context.Context) ([]BackupDestination, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, endpoint, COALESCE(region,''), COALESCE(access_key,''), secret_enc, bucket, COALESCE(prefix,''), path_style, created_at FROM backup_destinations ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []BackupDestination
	for rows.Next() {
		var d BackupDestination
		var secretEnc string
		var pathStyle int
		if err := rows.Scan(&d.ID, &d.Name, &d.Endpoint, &d.Region, &d.AccessKey, &secretEnc, &d.Bucket, &d.Prefix, &pathStyle, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.PathStyle = pathStyle != 0
		if plain, err := crypto.Decrypt(secretEnc); err == nil {
			d.SecretKey = plain
		}
		list = append(list, d)
	}
	if list == nil {
		list = []BackupDestination{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) GetDestination(ctx context.Context, id string) (*BackupDestination, error) {
	var d BackupDestination
	var secretEnc string
	var pathStyle int
	err := r.db.QueryRowContext(ctx, `SELECT id, name, endpoint, COALESCE(region,''), COALESCE(access_key,''), secret_enc, bucket, COALESCE(prefix,''), path_style, created_at FROM backup_destinations WHERE id = ?`, id).
		Scan(&d.ID, &d.Name, &d.Endpoint, &d.Region, &d.AccessKey, &secretEnc, &d.Bucket, &d.Prefix, &pathStyle, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.PathStyle = pathStyle != 0
	if plain, err := crypto.Decrypt(secretEnc); err == nil {
		d.SecretKey = plain
	}
	return &d, nil
}

// CreateDestination persists a destination row. secretEnc is stored as the
// already-encrypted ciphertext — the service layer owns the crypto boundary
// so no plaintext secret ever reaches the data layer.
func (r *SQLiteRepository) CreateDestination(ctx context.Context, d BackupDestination, secretEnc string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO backup_destinations
		(id, name, endpoint, region, access_key, secret_enc, bucket, prefix, path_style)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.Endpoint, d.Region, d.AccessKey, secretEnc, d.Bucket, d.Prefix, boolToInt(d.PathStyle),
	)
	return err
}

func (r *SQLiteRepository) DeleteDestination(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM backup_destinations WHERE id = ?", id)
	return err
}

// nullableString turns an empty identifier into SQL NULL-safe empty text.
func nullableString(v string) *string {
	if v == "" {
		p := ""
		return &p
	}
	return &v
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
