package services

import (
	"context"
	"database/sql"
	"encoding/json"
)

// SQLiteRepository implements services.Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository for services.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

const serviceSelectCols = "id, project_id, name, type, status, source, branch, image, port, cpu_limit, memory_limit, restart_policy, description, quadlet_config, compose_yaml, k8s_yaml, runtime_target, webhook_token, git_repo, git_branch, dockerfile_path, ssh_key_id, env_vars, created_at"

func (r *SQLiteRepository) List(ctx context.Context, projectID string) ([]Service, error) {
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

	var list []Service
	for rows.Next() {
		var s Service
		var envJSON string
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image, &s.Port, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description, &s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.WebhookToken, &s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &envJSON, &s.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(envJSON), &s.EnvVars)
		list = append(list, s)
	}
	if list == nil {
		list = []Service{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*Service, error) {
	var s Service
	var envJSON string
	err := r.db.QueryRowContext(ctx,
		"SELECT "+serviceSelectCols+" FROM services WHERE id = ?", id).
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

func (r *SQLiteRepository) Create(ctx context.Context, s Service) error {
	envJSON, _ := json.Marshal(s.EnvVars)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO services (
			id, project_id, name, type, status, source, branch, image, port,
			cpu_limit, memory_limit, restart_policy, description, quadlet_config,
			compose_yaml, k8s_yaml, runtime_target, webhook_token, git_repo,
			git_branch, dockerfile_path, ssh_key_id, env_vars, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image, s.Port,
		s.CPULimit, s.MemoryLimit, s.RestartPolicy, s.Description, s.QuadletConfig,
		s.ComposeYaml, s.K8sYaml, s.RuntimeTarget, s.WebhookToken, s.GitRepo,
		s.GitBranch, s.DockerfilePath, s.SSHKeyID, string(envJSON), s.CreatedAt,
	)
	return err
}

func (r *SQLiteRepository) Update(ctx context.Context, s Service) error {
	envJSON, _ := json.Marshal(s.EnvVars)
	_, err := r.db.ExecContext(ctx,
		`UPDATE services SET
			name = ?, type = ?, status = ?, source = ?, branch = ?, image = ?,
			port = ?, cpu_limit = ?, memory_limit = ?, restart_policy = ?,
			description = ?, quadlet_config = ?, compose_yaml = ?, k8s_yaml = ?,
			runtime_target = ?, webhook_token = COALESCE(NULLIF(webhook_token, ''), ?),
			git_repo = ?, git_branch = ?, dockerfile_path = ?, ssh_key_id = ?,
			env_vars = ?
		WHERE id = ?`,
		s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image,
		s.Port, s.CPULimit, s.MemoryLimit, s.RestartPolicy,
		s.Description, s.QuadletConfig, s.ComposeYaml, s.K8sYaml,
		s.RuntimeTarget, s.WebhookToken, s.GitRepo, s.GitBranch,
		s.DockerfilePath, s.SSHKeyID, string(envJSON), s.ID,
	)
	return err
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM services WHERE id = ?", id)
	return err
}

func (r *SQLiteRepository) GetByWebhookToken(ctx context.Context, token string) (*Service, error) {
	var s Service
	var envJSON string
	err := r.db.QueryRowContext(ctx,
		"SELECT "+serviceSelectCols+" FROM services WHERE webhook_token = ?", token).
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

func (r *SQLiteRepository) CreateDeployment(ctx context.Context, d Deployment) error {
	if d.Trigger == "" {
		d.Trigger = "manual"
	}
	if d.Number <= 0 {
		d.Number = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO deployments (id, project_id, service_id, number, trigger, image, version, commit_hash, commit_message, status, duration, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ProjectID, d.ServiceID, d.Number, d.Trigger, d.Image, d.Version, d.CommitHash, d.CommitMessage, d.Status, d.Duration, d.StartedAt, d.FinishedAt,
	)
	return err
}

func (r *SQLiteRepository) UpdateDeployment(ctx context.Context, d Deployment) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE deployments SET
			version = ?, commit_hash = ?, commit_message = ?, status = ?,
			duration = ?, finished_at = ?, image = ?
		 WHERE id = ?`,
		d.Version, d.CommitHash, d.CommitMessage, d.Status,
		d.Duration, d.FinishedAt, d.Image, d.ID,
	)
	return err
}

func (r *SQLiteRepository) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	var d Deployment
	err := r.db.QueryRowContext(ctx,
		"SELECT id, project_id, service_id, COALESCE(number, 1), COALESCE(trigger, 'manual'), COALESCE(image, ''), version, commit_hash, commit_message, status, duration, started_at, finished_at FROM deployments WHERE id = ?",
		id,
	).Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.Number, &d.Trigger, &d.Image, &d.Version, &d.CommitHash, &d.CommitMessage, &d.Status, &d.Duration, &d.StartedAt, &d.FinishedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *SQLiteRepository) ListDeployments(ctx context.Context, serviceID string) ([]Deployment, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, project_id, service_id, COALESCE(number, 1), COALESCE(trigger, 'manual'), COALESCE(image, ''), version, commit_hash, commit_message, status, duration, started_at, finished_at FROM deployments WHERE service_id = ? ORDER BY started_at DESC LIMIT 50",
		serviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.Number, &d.Trigger, &d.Image, &d.Version, &d.CommitHash, &d.CommitMessage, &d.Status, &d.Duration, &d.StartedAt, &d.FinishedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	if list == nil {
		list = []Deployment{}
	}
	return list, rows.Err()
}
