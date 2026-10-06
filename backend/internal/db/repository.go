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

const serviceSelectCols = "id, project_id, name, type, status, source, branch, image, port, cpu_limit, memory_limit, restart_policy, description, quadlet_config, compose_yaml, k8s_yaml, runtime_target, webhook_token, git_repo, git_branch, dockerfile_path, ssh_key_id, env_vars, created_at"

func (r *Repository) ListServices(ctx context.Context, projectID string) ([]Service, error) {
	var query string
	var args []interface{}
	if projectID != "" {
		query = "SELECT " + serviceSelectCols + " FROM services WHERE project_id = ? ORDER BY created_at ASC"
		args = append(args, projectID)
	} else {
		query = "SELECT " + serviceSelectCols + " FROM services ORDER BY created_at ASC"
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
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image, &s.Port, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description, &s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.WebhookToken, &s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &envJSON, &s.CreatedAt); err != nil {
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
	err := r.db.QueryRowContext(ctx, "SELECT "+serviceSelectCols+" FROM services WHERE id = ?", id).
		Scan(&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image, &s.Port, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description, &s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.WebhookToken, &s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &envJSON, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(envJSON), &s.EnvVars)
	return &s, nil
}

func (r *Repository) GetServiceByWebhookToken(ctx context.Context, token string) (*Service, error) {
	var s Service
	var envJSON string
	err := r.db.QueryRowContext(ctx, "SELECT "+serviceSelectCols+" FROM services WHERE webhook_token = ?", token).
		Scan(&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image, &s.Port, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description, &s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.WebhookToken, &s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &envJSON, &s.CreatedAt)
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
		`INSERT INTO services (id, project_id, name, type, status, source, branch, image, port, cpu_limit, memory_limit, restart_policy, description, quadlet_config, compose_yaml, k8s_yaml, runtime_target, webhook_token, git_repo, git_branch, dockerfile_path, ssh_key_id, env_vars)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image, s.Port, s.CPULimit, s.MemoryLimit, s.RestartPolicy, s.Description, s.QuadletConfig, s.ComposeYaml, s.K8sYaml, s.RuntimeTarget, s.WebhookToken, s.GitRepo, s.GitBranch, s.DockerfilePath, s.SSHKeyID, string(envBytes),
	)
	return err
}

func (r *Repository) UpdateService(ctx context.Context, s Service) error {
	envBytes, _ := json.Marshal(s.EnvVars)
	_, err := r.db.ExecContext(ctx,
		`UPDATE services SET name = ?, type = ?, status = ?, source = ?, branch = ?, image = ?, port = ?, cpu_limit = ?, memory_limit = ?, restart_policy = ?, description = ?, quadlet_config = ?, compose_yaml = ?, k8s_yaml = ?, runtime_target = ?, git_repo = ?, git_branch = ?, dockerfile_path = ?, ssh_key_id = ?, env_vars = ?
		 WHERE id = ?`,
		s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image, s.Port, s.CPULimit, s.MemoryLimit, s.RestartPolicy, s.Description, s.QuadletConfig, s.ComposeYaml, s.K8sYaml, s.RuntimeTarget, s.GitRepo, s.GitBranch, s.DockerfilePath, s.SSHKeyID, string(envBytes), s.ID,
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

// ── SSH Keys ──

func (r *Repository) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, public_key, private_key, fingerprint, type, created_at FROM ssh_keys ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []SSHKey
	for rows.Next() {
		var k SSHKey
		if err := rows.Scan(&k.ID, &k.Name, &k.PublicKey, &k.PrivateKey, &k.Fingerprint, &k.Type, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *Repository) GetSSHKey(ctx context.Context, id string) (*SSHKey, error) {
	var k SSHKey
	err := r.db.QueryRowContext(ctx, "SELECT id, name, public_key, private_key, fingerprint, type, created_at FROM ssh_keys WHERE id = ?", id).
		Scan(&k.ID, &k.Name, &k.PublicKey, &k.PrivateKey, &k.Fingerprint, &k.Type, &k.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *Repository) CreateSSHKey(ctx context.Context, k SSHKey) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO ssh_keys (id, name, public_key, private_key, fingerprint, type) VALUES (?, ?, ?, ?, ?, ?)",
		k.ID, k.Name, k.PublicKey, k.PrivateKey, k.Fingerprint, k.Type,
	)
	return err
}

func (r *Repository) DeleteSSHKey(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM ssh_keys WHERE id = ?", id)
	return err
}

// ── Container Registries ──

func (r *Repository) ListRegistries(ctx context.Context) ([]ContainerRegistry, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, url, username, token, created_at FROM container_registries ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var regs []ContainerRegistry
	for rows.Next() {
		var reg ContainerRegistry
		if err := rows.Scan(&reg.ID, &reg.Name, &reg.URL, &reg.Username, &reg.Token, &reg.CreatedAt); err != nil {
			return nil, err
		}
		regs = append(regs, reg)
	}
	return regs, rows.Err()
}

func (r *Repository) CreateRegistry(ctx context.Context, reg ContainerRegistry) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO container_registries (id, name, url, username, token) VALUES (?, ?, ?, ?, ?)",
		reg.ID, reg.Name, reg.URL, reg.Username, reg.Token,
	)
	return err
}

func (r *Repository) DeleteRegistry(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM container_registries WHERE id = ?", id)
	return err
}

// ── Secrets ──

func (r *Repository) ListSecrets(ctx context.Context) ([]Secret, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, value, driver, created_at FROM secrets ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.Name, &s.Value, &s.Driver, &s.CreatedAt); err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

func (r *Repository) CreateSecret(ctx context.Context, s Secret) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO secrets (id, name, value, driver) VALUES (?, ?, ?, ?)",
		s.ID, s.Name, s.Value, s.Driver,
	)
	return err
}

func (r *Repository) DeleteSecret(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM secrets WHERE id = ?", id)
	return err
}

// ── Users & Sessions ──

func (r *Repository) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, "SELECT id, email, password_hash, name, role, created_at FROM users WHERE email = ?", email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, "SELECT id, email, password_hash, name, role, created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) CreateUser(ctx context.Context, u User) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO users (id, email, password_hash, name, role) VALUES (?, ?, ?, ?, ?)",
		u.ID, u.Email, u.PasswordHash, u.Name, u.Role,
	)
	return err
}

func (r *Repository) CreateSession(ctx context.Context, s Session) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		s.Token, s.UserID, s.ExpiresAt,
	)
	return err
}

func (r *Repository) GetSession(ctx context.Context, token string) (*Session, *User, error) {
	var s Session
	var u User
	err := r.db.QueryRowContext(ctx, `
		SELECT s.token, s.user_id, s.expires_at, s.created_at,
		       u.id, u.email, u.name, u.role, u.created_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP
	`, token).Scan(
		&s.Token, &s.UserID, &s.ExpiresAt, &s.CreatedAt,
		&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return &s, &u, nil
}

func (r *Repository) DeleteSession(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	return err
}

