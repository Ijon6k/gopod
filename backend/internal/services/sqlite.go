package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// SQLiteRepository implements services.Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository for services.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

const serviceSelectCols = "id, project_id, name, type, status, source, branch, image, port, COALESCE(host_port, 0), cpu_limit, memory_limit, restart_policy, description, quadlet_config, compose_yaml, k8s_yaml, runtime_target, in_pod, webhook_token, git_repo, git_branch, dockerfile_path, ssh_key_id, COALESCE(source_type, 'git'), COALESCE(build_type, 'dockerfile'), COALESCE(build_path, '/'), COALESCE(publish_dir, 'dist'), COALESCE(pod_id, ''), COALESCE(registry_id, ''), COALESCE(auto_deploy, 1), COALESCE(watch_paths, ''), COALESCE(command, ''), COALESCE(entrypoint, ''), env_vars, COALESCE(advanced, '{}'), created_at"

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
		var envJSON, advJSON string
		if err := rows.Scan(
			&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image,
			&s.Port, &s.HostPort, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description,
			&s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.InPod, &s.WebhookToken,
			&s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &s.SourceType, &s.BuildType,
			&s.BuildPath, &s.PublishDir, &s.PodID, &s.RegistryID, &s.AutoDeploy, &s.WatchPaths,
			&s.Command, &s.Entrypoint,
			&envJSON, &advJSON, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(envJSON), &s.EnvVars)
		if advJSON != "" && advJSON != "{}" {
			_ = json.Unmarshal([]byte(advJSON), &s.Advanced)
		}
		list = append(list, s)
	}
	if list == nil {
		list = []Service{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*Service, error) {
	var s Service
	var envJSON, advJSON string
	err := r.db.QueryRowContext(ctx,
		"SELECT "+serviceSelectCols+" FROM services WHERE id = ?", id).
		Scan(
			&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image,
			&s.Port, &s.HostPort, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description,
			&s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.InPod, &s.WebhookToken,
			&s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &s.SourceType, &s.BuildType,
			&s.BuildPath, &s.PublishDir, &s.PodID, &s.RegistryID, &s.AutoDeploy, &s.WatchPaths,
			&s.Command, &s.Entrypoint,
			&envJSON, &advJSON, &s.CreatedAt,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(envJSON), &s.EnvVars)
	if advJSON != "" && advJSON != "{}" {
		_ = json.Unmarshal([]byte(advJSON), &s.Advanced)
	}
	return &s, nil
}

func (r *SQLiteRepository) Create(ctx context.Context, s Service) error {
	envJSON, _ := json.Marshal(s.EnvVars)
	advJSON := []byte("{}")
	if s.Advanced != nil {
		if b, err := json.Marshal(s.Advanced); err == nil {
			advJSON = b
		}
	}
	if s.SourceType == "" {
		if s.Type == "compose" || strings.TrimSpace(s.ComposeYaml) != "" {
			s.SourceType = "compose"
		} else if s.Type == "kubernetes" || strings.TrimSpace(s.K8sYaml) != "" {
			s.SourceType = "kubernetes"
		} else if s.Type == "pod" {
			s.SourceType = "pod"
		} else if s.Type == "image" {
			s.SourceType = "image"
		} else if s.Type == "quadlet" {
			s.SourceType = "quadlet"
		} else {
			s.SourceType = "git"
		}
	}
	if s.BuildType == "" {
		s.BuildType = "dockerfile"
	}
	if s.BuildPath == "" {
		s.BuildPath = "/"
	}
	if s.PublishDir == "" {
		s.PublishDir = "dist"
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO services (
			id, project_id, name, type, status, source, branch, image, port, host_port,
			cpu_limit, memory_limit, restart_policy, description, quadlet_config,
			compose_yaml, k8s_yaml, runtime_target, in_pod, webhook_token, git_repo,
			git_branch, dockerfile_path, ssh_key_id, source_type, build_type, build_path,
			publish_dir, pod_id, registry_id, auto_deploy, watch_paths, command, entrypoint, env_vars, advanced, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image, s.Port, s.HostPort,
		s.CPULimit, s.MemoryLimit, s.RestartPolicy, s.Description, s.QuadletConfig,
		s.ComposeYaml, s.K8sYaml, s.RuntimeTarget, s.InPod, s.WebhookToken, s.GitRepo,
		s.GitBranch, s.DockerfilePath, s.SSHKeyID, s.SourceType, s.BuildType, s.BuildPath,
		s.PublishDir, s.PodID, s.RegistryID, s.AutoDeploy, s.WatchPaths, s.Command, s.Entrypoint, string(envJSON), string(advJSON), s.CreatedAt,
	)
	return err
}

func (r *SQLiteRepository) Update(ctx context.Context, s Service) error {
	envJSON, _ := json.Marshal(s.EnvVars)
	advJSON := []byte("{}")
	if s.Advanced != nil {
		if b, err := json.Marshal(s.Advanced); err == nil {
			advJSON = b
		}
	}
	if s.SourceType == "" {
		if s.Type == "compose" || strings.TrimSpace(s.ComposeYaml) != "" {
			s.SourceType = "compose"
		} else if s.Type == "kubernetes" || strings.TrimSpace(s.K8sYaml) != "" {
			s.SourceType = "kubernetes"
		} else if s.Type == "pod" {
			s.SourceType = "pod"
		} else if s.Type == "image" {
			s.SourceType = "image"
		} else if s.Type == "quadlet" {
			s.SourceType = "quadlet"
		} else {
			s.SourceType = "git"
		}
	}
	if s.BuildType == "" {
		s.BuildType = "dockerfile"
	}
	if s.BuildPath == "" {
		s.BuildPath = "/"
	}
	if s.PublishDir == "" {
		s.PublishDir = "dist"
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE services SET
			name = ?, type = ?, status = ?, source = ?, branch = ?, image = ?,
			port = ?, host_port = ?, cpu_limit = ?, memory_limit = ?, restart_policy = ?,
			description = ?, quadlet_config = ?, compose_yaml = ?, k8s_yaml = ?,
			runtime_target = ?, in_pod = ?, webhook_token = COALESCE(NULLIF(webhook_token, ''), ?),
			git_repo = ?, git_branch = ?, dockerfile_path = ?, ssh_key_id = ?,
			source_type = ?, build_type = ?, build_path = ?, publish_dir = ?,
			pod_id = ?, registry_id = ?, auto_deploy = ?, watch_paths = ?, command = ?, entrypoint = ?,
			env_vars = ?, advanced = ?
		WHERE id = ?`,
		s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image,
		s.Port, s.HostPort, s.CPULimit, s.MemoryLimit, s.RestartPolicy,
		s.Description, s.QuadletConfig, s.ComposeYaml, s.K8sYaml,
		s.RuntimeTarget, s.InPod, s.WebhookToken, s.GitRepo, s.GitBranch,
		s.DockerfilePath, s.SSHKeyID, s.SourceType, s.BuildType, s.BuildPath,
		s.PublishDir, s.PodID, s.RegistryID, s.AutoDeploy, s.WatchPaths, s.Command, s.Entrypoint,
		string(envJSON), string(advJSON), s.ID,
	)
	return err
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM services WHERE id = ?", id)
	return err
}

func (r *SQLiteRepository) GetByWebhookToken(ctx context.Context, token string) (*Service, error) {
	var s Service
	var envJSON, advJSON string
	err := r.db.QueryRowContext(ctx,
		"SELECT "+serviceSelectCols+" FROM services WHERE webhook_token = ?", token).
		Scan(
			&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image,
			&s.Port, &s.HostPort, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description,
			&s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.InPod, &s.WebhookToken,
			&s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &s.SourceType, &s.BuildType,
			&s.BuildPath, &s.PublishDir, &s.PodID, &s.RegistryID, &s.AutoDeploy, &s.WatchPaths,
			&s.Command, &s.Entrypoint,
			&envJSON, &advJSON, &s.CreatedAt,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(envJSON), &s.EnvVars)
	if advJSON != "" && advJSON != "{}" {
		_ = json.Unmarshal([]byte(advJSON), &s.Advanced)
	}
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
