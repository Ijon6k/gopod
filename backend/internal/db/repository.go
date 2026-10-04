package db

import (
	"context"
	"database/sql"
	"encoding/json"
)

// Repository handles CRUD database operations.
type Repository struct {
	db *DB
}

// NewRepository creates a new database repository.
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// ── Projects ──

func (r *Repository) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, description, created_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *Repository) GetProject(ctx context.Context, id string) (*Project, error) {
	var p Project
	err := r.db.QueryRowContext(ctx, "SELECT id, name, description, created_at FROM projects WHERE id = ?", id).
		Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) CreateProject(ctx context.Context, p Project) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO projects (id, name, description) VALUES (?, ?, ?)",
		p.ID, p.Name, p.Description,
	)
	return err
}

func (r *Repository) DeleteProject(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	return err
}

// ── Services ──

func (r *Repository) ListServices(ctx context.Context, projectID string) ([]Service, error) {
	var query string
	var args []interface{}
	if projectID != "" {
		query = "SELECT id, project_id, name, type, status, source, branch, image, port, cpu_limit, memory_limit, restart_policy, env_vars, created_at FROM services WHERE project_id = ? ORDER BY created_at ASC"
		args = append(args, projectID)
	} else {
		query = "SELECT id, project_id, name, type, status, source, branch, image, port, cpu_limit, memory_limit, restart_policy, env_vars, created_at FROM services ORDER BY created_at ASC"
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		var envJSON string
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image, &s.Port, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &envJSON, &s.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(envJSON), &s.EnvVars)
		services = append(services, s)
	}
	return services, rows.Err()
}

func (r *Repository) GetService(ctx context.Context, id string) (*Service, error) {
	var s Service
	var envJSON string
	err := r.db.QueryRowContext(ctx, "SELECT id, project_id, name, type, status, source, branch, image, port, cpu_limit, memory_limit, restart_policy, env_vars, created_at FROM services WHERE id = ?", id).
		Scan(&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image, &s.Port, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &envJSON, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(envJSON), &s.EnvVars)
	return &s, nil
}

func (r *Repository) CreateService(ctx context.Context, s Service) error {
	envBytes, _ := json.Marshal(s.EnvVars)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO services (id, project_id, name, type, status, source, branch, image, port, cpu_limit, memory_limit, restart_policy, env_vars)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image, s.Port, s.CPULimit, s.MemoryLimit, s.RestartPolicy, string(envBytes),
	)
	return err
}

func (r *Repository) UpdateServiceStatus(ctx context.Context, id string, status string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE services SET status = ? WHERE id = ?", status, id)
	return err
}

func (r *Repository) DeleteService(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM services WHERE id = ?", id)
	return err
}

// ── Domains ──

func (r *Repository) ListDomains(ctx context.Context, projectID string) ([]Domain, error) {
	var query string
	var args []interface{}
	if projectID != "" {
		query = "SELECT id, project_id, service_id, hostname, container_port, tls, https_redirect, path_prefix, strip_path_prefix, websocket, cors, hsts, basic_auth, basic_auth_user, dns_status, created_at FROM domains WHERE project_id = ? ORDER BY created_at DESC"
		args = append(args, projectID)
	} else {
		query = "SELECT id, project_id, service_id, hostname, container_port, tls, https_redirect, path_prefix, strip_path_prefix, websocket, cors, hsts, basic_auth, basic_auth_user, dns_status, created_at FROM domains ORDER BY created_at DESC"
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(
			&d.ID, &d.ProjectID, &d.ServiceID, &d.Hostname, &d.ContainerPort,
			&d.TLS, &d.HTTPSRedirect, &d.PathPrefix, &d.StripPathPrefix,
			&d.WebSocket, &d.CORS, &d.HSTS, &d.BasicAuth, &d.BasicAuthUser,
			&d.DNSStatus, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

func (r *Repository) CreateDomain(ctx context.Context, d Domain) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO domains (id, project_id, service_id, hostname, container_port, tls, https_redirect, path_prefix, strip_path_prefix, websocket, cors, hsts, basic_auth, basic_auth_user, dns_status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ProjectID, d.ServiceID, d.Hostname, d.ContainerPort, d.TLS, d.HTTPSRedirect, d.PathPrefix, d.StripPathPrefix, d.WebSocket, d.CORS, d.HSTS, d.BasicAuth, d.BasicAuthUser, d.DNSStatus,
	)
	return err
}

func (r *Repository) UpdateDomain(ctx context.Context, d Domain) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE domains SET container_port = ?, tls = ?, https_redirect = ?, path_prefix = ?, strip_path_prefix = ?, websocket = ?, cors = ?, hsts = ?, basic_auth = ?, basic_auth_user = ?, dns_status = ?
		 WHERE id = ?`,
		d.ContainerPort, d.TLS, d.HTTPSRedirect, d.PathPrefix, d.StripPathPrefix, d.WebSocket, d.CORS, d.HSTS, d.BasicAuth, d.BasicAuthUser, d.DNSStatus, d.ID,
	)
	return err
}

func (r *Repository) DeleteDomain(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM domains WHERE id = ?", id)
	return err
}

// ── Volume Snapshots & Schedules ──

func (r *Repository) ListVolumeSnapshots(ctx context.Context, projectID string) ([]VolumeSnapshot, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression, created_at FROM volume_snapshots WHERE project_id = ? ORDER BY created_at DESC",
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []VolumeSnapshot
	for rows.Next() {
		var s VolumeSnapshot
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ServiceID, &s.VolumeName, &s.Filename, &s.Size, &s.SizeBytes, &s.Status, &s.Compression, &s.CreatedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, rows.Err()
}

func (r *Repository) CreateVolumeSnapshot(ctx context.Context, s VolumeSnapshot) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO volume_snapshots (id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.ServiceID, s.VolumeName, s.Filename, s.Size, s.SizeBytes, s.Status, s.Compression,
	)
	return err
}

func (r *Repository) DeleteVolumeSnapshot(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM volume_snapshots WHERE id = ?", id)
	return err
}

func (r *Repository) ListVolumeSchedules(ctx context.Context, projectID string) ([]VolumeSchedule, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, project_id, volume_name, cron, label, retention_count, enabled, last_run, created_at FROM volume_schedules WHERE project_id = ? ORDER BY created_at ASC",
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []VolumeSchedule
	for rows.Next() {
		var s VolumeSchedule
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.VolumeName, &s.Cron, &s.Label, &s.RetentionCount, &s.Enabled, &s.LastRun, &s.CreatedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, rows.Err()
}

func (r *Repository) ToggleVolumeSchedule(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE volume_schedules SET enabled = NOT enabled WHERE id = ?", id)
	return err
}
