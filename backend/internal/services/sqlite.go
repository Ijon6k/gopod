package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gopod/pkg/crypto"
)

// SQLiteRepository implements services.Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository for services.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

const serviceSelectCols = "id, project_id, name, type, status, source, branch, image, port, COALESCE(host_port, 0), COALESCE(host_ip, ''), cpu_limit, memory_limit, restart_policy, description, quadlet_config, compose_yaml, k8s_yaml, runtime_target, in_pod, webhook_token, git_repo, git_branch, dockerfile_path, ssh_key_id, COALESCE(git_credential_id, ''), COALESCE(build_args, '[]'), COALESCE(webhook_provider, ''), COALESCE(webhook_secret, ''), COALESCE(source_type, 'git'), COALESCE(build_type, 'dockerfile'), COALESCE(build_path, '/'), COALESCE(publish_dir, 'dist'), COALESCE(pod_id, ''), COALESCE(registry_id, ''), COALESCE(auto_deploy, 1), COALESCE(watch_paths, ''), COALESCE(command, ''), COALESCE(entrypoint, ''), env_vars, COALESCE(advanced, '{}'), created_at"

// hydrateService decodes the encrypted and JSON-backed service columns.
func hydrateService(s *Service, envJSON, advJSON, buildArgsJSON, webhookSecretEnc string) {
	if dec, err := crypto.Decrypt(envJSON); err == nil {
		envJSON = dec
	} else {
		// Keep the row intact, flag it, and never present ciphertext as a value.
		log.Printf("[WARN] failed to decrypt env vars for service %s: %v", s.ID, err)
		s.EnvDecryptionError = true
		envJSON = ""
	}
	if envJSON != "" {
		_ = json.Unmarshal([]byte(envJSON), &s.EnvVars)
	}
	if advJSON != "" && advJSON != "{}" {
		_ = json.Unmarshal([]byte(advJSON), &s.Advanced)
	}
	if buildArgsJSON != "" && buildArgsJSON != "[]" {
		_ = json.Unmarshal([]byte(buildArgsJSON), &s.BuildArgs)
	}
	if webhookSecretEnc != "" {
		if dec, err := crypto.Decrypt(webhookSecretEnc); err == nil {
			s.WebhookSecret = dec
		} else {
			log.Printf("[WARN] failed to decrypt webhook secret for service %s: %v", s.ID, err)
			s.WebhookSecretError = true
		}
	}
}

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
		var envJSON, advJSON, buildArgsJSON, webhookSecretEnc string
		if err := rows.Scan(
			&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image,
			&s.Port, &s.HostPort, &s.HostIP, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description,
			&s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.InPod, &s.WebhookToken,
			&s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &s.GitCredentialID, &buildArgsJSON,
			&s.WebhookProvider, &webhookSecretEnc, &s.SourceType, &s.BuildType,
			&s.BuildPath, &s.PublishDir, &s.PodID, &s.RegistryID, &s.AutoDeploy, &s.WatchPaths,
			&s.Command, &s.Entrypoint,
			&envJSON, &advJSON, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		hydrateService(&s, envJSON, advJSON, buildArgsJSON, webhookSecretEnc)
		list = append(list, s)
	}
	if list == nil {
		list = []Service{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*Service, error) {
	var s Service
	var envJSON, advJSON, buildArgsJSON, webhookSecretEnc string
	err := r.db.QueryRowContext(ctx,
		"SELECT "+serviceSelectCols+" FROM services WHERE id = ?", id).
		Scan(
			&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image,
			&s.Port, &s.HostPort, &s.HostIP, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description,
			&s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.InPod, &s.WebhookToken,
			&s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &s.GitCredentialID, &buildArgsJSON,
			&s.WebhookProvider, &webhookSecretEnc, &s.SourceType, &s.BuildType,
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
	hydrateService(&s, envJSON, advJSON, buildArgsJSON, webhookSecretEnc)
	return &s, nil
}

func (r *SQLiteRepository) Create(ctx context.Context, s Service) error {
	// Env blob encryption failure must abort the write: falling back to
	// plaintext would silently store secrets unencrypted.
	rawEnv, _ := json.Marshal(s.EnvVars)
	envJSON := string(rawEnv)
	encEnv, err := crypto.Encrypt(envJSON)
	if err != nil {
		return fmt.Errorf("encrypt env vars: %w", err)
	}
	envJSON = encEnv
	advJSON := []byte("{}")
	if s.Advanced != nil {
		if b, err := json.Marshal(s.Advanced); err == nil {
			advJSON = b
		}
	}
	if s.SourceType == "" {
		if s.Type == "pod" {
			s.SourceType = "pod"
		} else if s.Type == "compose" || (s.Type != "quadlet" && s.Type != "pod" && strings.TrimSpace(s.ComposeYaml) != "") {
			s.SourceType = "compose"
		} else if s.Type == "kubernetes" || s.Type == "k8s" || (s.Type != "compose" && s.Type != "quadlet" && s.Type != "pod" && s.Type != "image" && strings.TrimSpace(s.K8sYaml) != "") {
			s.SourceType = "kubernetes"
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
	buildArgsJSON := marshalServiceBuildArgs(s.BuildArgs)
	encWebhookSecret, err := crypto.Encrypt(s.WebhookSecret)
	if err != nil {
		return fmt.Errorf("encrypt webhook secret: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO services (
			id, project_id, name, type, status, source, branch, image, port, host_port, host_ip,
			cpu_limit, memory_limit, restart_policy, description, quadlet_config,
			compose_yaml, k8s_yaml, runtime_target, in_pod, webhook_token, git_repo,
			git_branch, dockerfile_path, ssh_key_id, git_credential_id, build_args,
			webhook_provider, webhook_secret, source_type, build_type, build_path,
			publish_dir, pod_id, registry_id, auto_deploy, watch_paths, command, entrypoint, env_vars, advanced, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image, s.Port, s.HostPort, s.HostIP,
		s.CPULimit, s.MemoryLimit, s.RestartPolicy, s.Description, s.QuadletConfig,
		s.ComposeYaml, s.K8sYaml, s.RuntimeTarget, s.InPod, s.WebhookToken, s.GitRepo,
		s.GitBranch, s.DockerfilePath, s.SSHKeyID, s.GitCredentialID, buildArgsJSON,
		s.WebhookProvider, encWebhookSecret, s.SourceType, s.BuildType, s.BuildPath,
		s.PublishDir, s.PodID, s.RegistryID, s.AutoDeploy, s.WatchPaths, s.Command, s.Entrypoint, string(envJSON), string(advJSON), s.CreatedAt,
	)
	return err
}

func (r *SQLiteRepository) Update(ctx context.Context, s Service) error {
	// Env blob encryption failure must abort the write: falling back to
	// plaintext would silently store secrets unencrypted. When the caller
	// carries a service whose stored env blob could not be decrypted, preserve
	// the ciphertext: status-only updates must not replace the only copy with
	// an empty list.
	rawEnv, _ := json.Marshal(s.EnvVars)
	envJSON := string(rawEnv)
	encEnv, err := crypto.Encrypt(envJSON)
	if err != nil {
		return fmt.Errorf("encrypt env vars: %w", err)
	}
	envJSON = encEnv
	advJSON := []byte("{}")
	if s.Advanced != nil {
		if b, err := json.Marshal(s.Advanced); err == nil {
			advJSON = b
		}
	}
	if s.SourceType == "" {
		if s.Type == "pod" {
			s.SourceType = "pod"
		} else if s.Type == "compose" || (s.Type != "quadlet" && s.Type != "pod" && strings.TrimSpace(s.ComposeYaml) != "") {
			s.SourceType = "compose"
		} else if s.Type == "kubernetes" || s.Type == "k8s" || (s.Type != "compose" && s.Type != "quadlet" && s.Type != "pod" && s.Type != "image" && strings.TrimSpace(s.K8sYaml) != "") {
			s.SourceType = "kubernetes"
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
	buildArgsJSON := marshalServiceBuildArgs(s.BuildArgs)
	encWebhookSecret, err := crypto.Encrypt(s.WebhookSecret)
	if err != nil {
		return fmt.Errorf("encrypt webhook secret: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE services SET
			name = ?, type = ?, status = ?, source = ?, branch = ?, image = ?,
			port = ?, host_port = ?, host_ip = ?, cpu_limit = ?, memory_limit = ?, restart_policy = ?,
			description = ?, quadlet_config = ?, compose_yaml = ?, k8s_yaml = ?,
			runtime_target = ?, in_pod = ?, webhook_token = COALESCE(NULLIF(webhook_token, ''), ?),
			git_repo = ?, git_branch = ?, dockerfile_path = ?, ssh_key_id = ?,
			git_credential_id = ?, build_args = ?, webhook_provider = ?,
			webhook_secret = COALESCE(NULLIF(webhook_secret, ''), ?),
			source_type = ?, build_type = ?, build_path = ?, publish_dir = ?,
			pod_id = ?, registry_id = ?, auto_deploy = ?, watch_paths = ?, command = ?, entrypoint = ?,
			env_vars = CASE WHEN ? THEN env_vars ELSE ? END, advanced = ?
		WHERE id = ?`,
		s.Name, s.Type, s.Status, s.Source, s.Branch, s.Image,
		s.Port, s.HostPort, s.HostIP, s.CPULimit, s.MemoryLimit, s.RestartPolicy,
		s.Description, s.QuadletConfig, s.ComposeYaml, s.K8sYaml,
		s.RuntimeTarget, s.InPod, s.WebhookToken, s.GitRepo, s.GitBranch,
		s.DockerfilePath, s.SSHKeyID, s.GitCredentialID, buildArgsJSON, s.WebhookProvider,
		encWebhookSecret, s.SourceType, s.BuildType, s.BuildPath,
		s.PublishDir, s.PodID, s.RegistryID, s.AutoDeploy, s.WatchPaths, s.Command, s.Entrypoint,
		s.EnvDecryptionError, string(envJSON), string(advJSON), s.ID,
	)
	return err
}

func marshalServiceBuildArgs(args []string) string {
	if len(args) == 0 {
		return "[]"
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM services WHERE id = ?", id)
	return err
}

// SetWebhookToken replaces the webhook token of a service. Update cannot do
// this (its SQL keeps an existing token via COALESCE), so rotation has an
// explicit setter.
func (r *SQLiteRepository) SetWebhookToken(ctx context.Context, id, token string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE services SET webhook_token = ? WHERE id = ?", token, id)
	return err
}

// SetWebhookSecret replaces the provider signature secret, encrypted at rest.
func (r *SQLiteRepository) SetWebhookSecret(ctx context.Context, id, secret string) error {
	enc, err := crypto.Encrypt(secret)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, "UPDATE services SET webhook_secret = ? WHERE id = ?", enc, id)
	return err
}

func (r *SQLiteRepository) GetByWebhookToken(ctx context.Context, token string) (*Service, error) {
	var s Service
	var envJSON, advJSON, buildArgsJSON, webhookSecretEnc string
	err := r.db.QueryRowContext(ctx,
		"SELECT "+serviceSelectCols+" FROM services WHERE webhook_token = ?", token).
		Scan(
			&s.ID, &s.ProjectID, &s.Name, &s.Type, &s.Status, &s.Source, &s.Branch, &s.Image,
			&s.Port, &s.HostPort, &s.HostIP, &s.CPULimit, &s.MemoryLimit, &s.RestartPolicy, &s.Description,
			&s.QuadletConfig, &s.ComposeYaml, &s.K8sYaml, &s.RuntimeTarget, &s.InPod, &s.WebhookToken,
			&s.GitRepo, &s.GitBranch, &s.DockerfilePath, &s.SSHKeyID, &s.GitCredentialID, &buildArgsJSON,
			&s.WebhookProvider, &webhookSecretEnc, &s.SourceType, &s.BuildType,
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
	hydrateService(&s, envJSON, advJSON, buildArgsJSON, webhookSecretEnc)
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
		`INSERT INTO deployments (id, project_id, service_id, number, trigger, image, version, commit_hash, commit_message, status, duration, config_snapshot, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ProjectID, d.ServiceID, d.Number, d.Trigger, d.Image, d.Version, d.CommitHash, d.CommitMessage, d.Status, d.Duration, d.ConfigSnapshot, d.StartedAt, d.FinishedAt,
	)
	return err
}

func (r *SQLiteRepository) UpdateDeployment(ctx context.Context, d Deployment) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE deployments SET
			version = ?, commit_hash = ?, commit_message = ?, status = ?,
			duration = ?, config_snapshot = ?, finished_at = ?, image = ?
		 WHERE id = ?`,
		d.Version, d.CommitHash, d.CommitMessage, d.Status,
		d.Duration, d.ConfigSnapshot, d.FinishedAt, d.Image, d.ID,
	)
	return err
}

func (r *SQLiteRepository) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	var d Deployment
	err := r.db.QueryRowContext(ctx,
		"SELECT id, project_id, service_id, COALESCE(number, 1), COALESCE(trigger, 'manual'), COALESCE(image, ''), version, commit_hash, commit_message, status, duration, COALESCE(config_snapshot, ''), started_at, finished_at FROM deployments WHERE id = ?",
		id,
	).Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.Number, &d.Trigger, &d.Image, &d.Version, &d.CommitHash, &d.CommitMessage, &d.Status, &d.Duration, &d.ConfigSnapshot, &d.StartedAt, &d.FinishedAt)
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
		"SELECT id, project_id, service_id, COALESCE(number, 1), COALESCE(trigger, 'manual'), COALESCE(image, ''), version, commit_hash, commit_message, status, duration, COALESCE(config_snapshot, ''), started_at, finished_at FROM deployments WHERE service_id = ? ORDER BY started_at DESC LIMIT 50",
		serviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.Number, &d.Trigger, &d.Image, &d.Version, &d.CommitHash, &d.CommitMessage, &d.Status, &d.Duration, &d.ConfigSnapshot, &d.StartedAt, &d.FinishedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	if list == nil {
		list = []Deployment{}
	}
	return list, rows.Err()
}

// NextDeploymentNumber returns the highest deployment number of a service
// plus one. It is the only correct source for sequence logic: the UI-facing
// ListDeployments stays capped at 50 rows, which would freeze numbering at 51.
func (r *SQLiteRepository) NextDeploymentNumber(ctx context.Context, serviceID string) (int, error) {
	var maxNum sql.NullInt64
	if err := r.db.QueryRowContext(ctx,
		"SELECT MAX(COALESCE(number, 0)) FROM deployments WHERE service_id = ?",
		serviceID,
	).Scan(&maxNum); err != nil {
		return 0, err
	}
	if !maxNum.Valid || maxNum.Int64 < 1 {
		return 1, nil
	}
	return int(maxNum.Int64) + 1, nil
}

// ListRunningDeployments selects all non-terminal deployments of a service.
// The set is at most a handful of rows (one live deploy per service), so the
// query is unbounded by design.
func (r *SQLiteRepository) ListRunningDeployments(ctx context.Context, serviceID string) ([]Deployment, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, project_id, service_id, COALESCE(number, 1), COALESCE(trigger, 'manual'), COALESCE(image, ''), version, commit_hash, commit_message, status, duration, COALESCE(config_snapshot, ''), started_at, finished_at FROM deployments WHERE service_id = ? AND status IN ('building', 'deploying')",
		serviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.Number, &d.Trigger, &d.Image, &d.Version, &d.CommitHash, &d.CommitMessage, &d.Status, &d.Duration, &d.ConfigSnapshot, &d.StartedAt, &d.FinishedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	if list == nil {
		list = []Deployment{}
	}
	return list, rows.Err()
}
