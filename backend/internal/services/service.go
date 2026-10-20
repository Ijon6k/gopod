package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"gopod/internal/activity"
	"gopod/internal/podman"
)

// AuditRecorder records workload actions.
type AuditRecorder interface {
	Record(ctx context.Context, action, actor, target, category, ip, status string) error
}

// IngressCleaner cleans up routing and reverse proxy rules for deleted services.
type IngressCleaner func(ctx context.Context, serviceID string)

// WorkloadService encapsulates workload management and deployment orchestration.
type WorkloadService struct {
	repo              Repository
	deployer          *Deployer
	logManager        *LogManager
	audit             AuditRecorder
	ingressCleaner    IngressCleaner
	activeCancels     sync.Map // deploymentID string -> context.CancelFunc
	activeServiceDeps sync.Map // serviceID string -> deploymentID string
	// deployLocks is an atomic per-service deploy mutex: at most one StartDeploy
	// or Deploy may run for a given service at a time. Key: serviceID,
	// value: deploymentID of the current owner.
	deployLocks   sync.Map
	webhookSem    chan struct{} // caps concurrent webhook-spawned deploys (lazy)
	webhookSemSet sync.Once
	// gcRuntimeOverride lets tests substitute a fake engine for the cleanup
	// engine; nil means "use the deployer's podman client".
	gcRuntimeOverride gcRuntime
}

// NewWorkloadService creates a new services domain service.
func NewWorkloadService(repo Repository, deployer *Deployer, logMgr *LogManager) *WorkloadService {
	if deployer != nil && repo != nil {
		deployer.SetServiceRepo(repo)
	}
	return &WorkloadService{
		repo:       repo,
		deployer:   deployer,
		logManager: logMgr,
	}
}

// SetAuditRecorder configures audit logging.
func (s *WorkloadService) SetAuditRecorder(ar AuditRecorder) {
	s.audit = ar
}

// SetIngressCleaner configures ingress domain teardown.
func (s *WorkloadService) SetIngressCleaner(cleaner IngressCleaner) {
	s.ingressCleaner = cleaner
}

// ListServices retrieves services, optionally filtered by projectID.
func (s *WorkloadService) ListServices(ctx context.Context, projectID string) ([]Service, error) {
	if s.repo == nil {
		return []Service{}, nil
	}
	list, err := s.repo.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if s.deployer != nil && len(list) > 0 {
		statusMap := s.deployer.BatchRuntimeStatus(ctx, list)
		for i := range list {
			svc := &list[i]
			if svc.Status == "deploying" || svc.Status == "building" {
				deps, err := s.repo.ListDeployments(ctx, svc.ID)
				hasActive := false
				if err == nil && len(deps) > 0 {
					for _, d := range deps {
						if d.Status == "building" || d.Status == "deploying" {
							hasActive = true
							break
						}
					}
				}
				if hasActive {
					continue
				}
			}
			if realSt, ok := statusMap[svc.ID]; ok && realSt != "" && realSt != svc.Status {
				svc.Status = realSt
				toUpdate := *svc
				go func(u Service) {
					bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = s.repo.Update(bgCtx, u)
				}(toUpdate)
			}
		}
	}
	return list, nil
}

// GetService retrieves a single service by ID.
func (s *WorkloadService) GetService(ctx context.Context, id string) (*Service, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	svc, err := s.repo.Get(ctx, id)
	if err != nil || svc == nil {
		return svc, err
	}
	if svc.WebhookToken == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		svc.WebhookToken = "wh_" + hex.EncodeToString(b)
		_ = s.repo.Update(ctx, *svc)
	}
	s.reconcileRuntimeStatus(ctx, svc)
	return svc, nil
}

// CreateService creates a new service, assigning generated ID and unguessable webhook token.
func (s *WorkloadService) CreateService(ctx context.Context, svc Service) (*Service, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if svc.Name == "" {
		return nil, errors.New("service name is required")
	}
	if svc.ProjectID == "" {
		return nil, errors.New("projectId is required")
	}

	if svc.ID == "" {
		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		randomSuffix := hex.EncodeToString(randBytes)
		cleanProj := strings.ToLower(strings.ReplaceAll(svc.ProjectID, " ", "-"))
		cleanName := strings.ToLower(strings.ReplaceAll(svc.Name, " ", "-"))
		svc.ID = fmt.Sprintf("%s-%s-%s", cleanProj, cleanName, randomSuffix)
	}
	if svc.WebhookToken == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		svc.WebhookToken = "wh_" + hex.EncodeToString(b)
	}

	if strings.TrimSpace(svc.ComposeYaml) != "" {
		var raw map[string]interface{}
		if err := yaml.Unmarshal([]byte(svc.ComposeYaml), &raw); err != nil {
			return nil, fmt.Errorf("invalid Compose YAML syntax: %w", err)
		}
	}

	if svc.Type == "quadlet" && strings.TrimSpace(svc.QuadletConfig) != "" {
		parsed := ParseQuadlet(svc.QuadletConfig)
		if parsed.Image != "" {
			svc.Image = parsed.Image
		}
		if parsed.Port > 0 {
			svc.Port = parsed.Port
		}
		if parsed.HostPort > 0 {
			svc.HostPort = parsed.HostPort
		}
	}

	svc.CreatedAt = time.Now()

	if err := s.repo.Create(ctx, svc); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.create", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.create", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Service '%s' created (type=%s)", svc.Name, svc.Type), "127.0.0.1")
	activity.EmitEngineEvent(ctx, "info", "service", svc.ID, svc.ProjectID, "operator", "service.create",
		fmt.Sprintf("Service '%s' created (type=%s)", svc.Name, svc.Type), map[string]any{"service": svc.Name, "type": svc.Type})
	return &svc, nil
}

// preserveRedactedEnvSecrets replaces masked/empty secret values with the
// stored value for the same key. New keys keep whatever the client sent.
//
// When the stored blob is known to be undecryptable (master key replaced) an
// update that would overwrite it with an empty list is refused outright:
// keeping the ciphertext at least leaves a chance of recovery, silently
// blanking the row does not.
func (s *WorkloadService) preserveRedactedEnvSecrets(ctx context.Context, svc *Service) error {
	// Entries with an empty key are editor noise (blank lines), not data: they
	// must not defeat the emptiness check below.
	effective := 0
	needsLookup := false
	for _, v := range svc.EnvVars {
		if strings.TrimSpace(v.Key) != "" {
			effective++
		}
		// The mask sentinel is unambiguous: it always means "keep the stored
		// value", whatever the client claims about the secret flag. An empty
		// value only means "keep" for entries flagged secret.
		if v.Value == EnvSecretMask || (v.Secret && v.Value == "") {
			needsLookup = true
		}
	}
	if effective == 0 {
		needsLookup = true
	}
	if !needsLookup {
		return nil
	}
	existing, err := s.repo.Get(ctx, svc.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	if existing.EnvDecryptionError {
		if effective == 0 {
			return errors.New("stored environment variables cannot be decrypted with the current master key; refusing to overwrite them")
		}
		// A "keep" sentinel (mask or empty secret) cannot be resolved against
		// an unreadable blob: persisting the literal mask would look like a
		// saved secret. Force the caller to re-enter the value instead.
		for _, v := range svc.EnvVars {
			if v.Value == EnvSecretMask || (v.Secret && v.Value == "") {
				return errors.New("stored environment variables cannot be decrypted with the current master key; masked or empty secret values cannot be preserved, re-enter them")
			}
		}
		return nil
	}
	stored := make(map[string]EnvVar, len(existing.EnvVars))
	for _, v := range existing.EnvVars {
		stored[v.Key] = v
	}
	for i := range svc.EnvVars {
		v := &svc.EnvVars[i]
		if v.Value != EnvSecretMask && !(v.Secret && v.Value == "") {
			continue
		}
		if prev, ok := stored[v.Key]; ok {
			v.Value = prev.Value
			// A lossy client that drops the secret flag must not downgrade a
			// stored secret to a plain var.
			if prev.Secret {
				v.Secret = true
			}
		}
	}
	return nil
}

// RevealEnvValues returns the decrypted value of the requested env keys.
// Only keys that actually exist on the service are returned; the caller is
// an authenticated operator.
func (s *WorkloadService) RevealEnvValues(ctx context.Context, id string, keys []string) (map[string]string, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	svc, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}
	if svc.EnvDecryptionError {
		return nil, errors.New("stored environment variables cannot be decrypted with the current master key")
	}
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	out := make(map[string]string)
	for _, v := range svc.EnvVars {
		if want[v.Key] {
			out[v.Key] = v.Value
		}
	}
	// Reading secrets is a security-relevant act: leave an audit trail (key
	// count only, never values).
	if len(out) > 0 {
		if s.audit != nil {
			_ = s.audit.Record(ctx, "service.env_reveal", "operator", svc.Name, "security", "127.0.0.1", "success")
		}
		activity.RecordActivity(ctx, "operator", "service.env_reveal", "service", svc.ID, svc.ProjectID, svc.ID, "success",
			fmt.Sprintf("Revealed %d environment value(s) for service '%s'", len(out), svc.Name), "127.0.0.1")
	}
	return out, nil
}

// WebhookTokenFor returns the current webhook token, generating and persisting
// one when the service never had it (parity with GetService's lazy backfill),
// for the explicit reveal endpoint.
func (s *WorkloadService) WebhookTokenFor(ctx context.Context, id string) (string, error) {
	if s.repo == nil {
		return "", errors.New("repository not initialized")
	}
	svc, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if svc == nil {
		return "", errors.New("service not found")
	}
	token := svc.WebhookToken
	if token == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		token = "wh_" + hex.EncodeToString(b)
		if err := s.repo.SetWebhookToken(ctx, id, token); err != nil {
			return "", err
		}
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.webhook_reveal", "operator", svc.Name, "security", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.webhook_reveal", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Webhook token revealed for service '%s'", svc.Name), "127.0.0.1")
	return token, nil
}

// RotateWebhookToken generates and persists a fresh webhook token, returning
// it once. Old tokens stop working immediately.
func (s *WorkloadService) RotateWebhookToken(ctx context.Context, id string) (string, error) {
	if s.repo == nil {
		return "", errors.New("repository not initialized")
	}
	svc, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if svc == nil {
		return "", errors.New("service not found")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := "wh_" + hex.EncodeToString(b)
	if err := s.repo.SetWebhookToken(ctx, id, token); err != nil {
		return "", err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.webhook_rotate", "operator", svc.Name, "security", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.webhook_rotate", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Webhook token rotated for service '%s'", svc.Name), "127.0.0.1")
	return token, nil
}

// UpdateService modifies service configuration and syncs declarative units.
func (s *WorkloadService) UpdateService(ctx context.Context, svc Service) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}

	// Secret env vars redacted on read (mask or empty) must keep their stored
	// value: the UI submits the whole env list on every save, so treating a
	// mask as a literal value would silently overwrite real secrets.
	if err := s.preserveRedactedEnvSecrets(ctx, &svc); err != nil {
		return err
	}

	if (svc.Type == "quadlet" || svc.RuntimeTarget == "quadlet") && strings.TrimSpace(svc.QuadletConfig) != "" {
		parsed := ParseQuadlet(svc.QuadletConfig)
		if parsed.Image != "" {
			svc.Image = parsed.Image
		}
		if parsed.Port > 0 {
			svc.Port = parsed.Port
		}
		if parsed.HostPort > 0 {
			svc.HostPort = parsed.HostPort
		}
	}

	if strings.TrimSpace(svc.ComposeYaml) != "" {
		var raw map[string]interface{}
		if err := yaml.Unmarshal([]byte(svc.ComposeYaml), &raw); err != nil {
			return fmt.Errorf("invalid Compose YAML syntax: %w", err)
		}
	}

	if !svc.InPod && svc.RuntimeTarget != "pod" && svc.Type != "pod" {
		svc.PodID = ""
	} else if svc.Type == "pod" {
		svc.PodID = PodNameForService(svc)
	} else if svc.InPod || svc.RuntimeTarget == "pod" {
		rawPod := strings.TrimSpace(svc.PodID)
		if rawPod == "" {
			rawPod = svc.Name
		}
		svc.PodID = PodNameFor(rawPod)
	}

	if err := s.repo.Update(ctx, svc); err != nil {
		return err
	}

	// Real Quadlet unit compilation to disk and systemd daemon-reload
	if (svc.Type == "quadlet" || svc.RuntimeTarget == "quadlet" || strings.TrimSpace(svc.QuadletConfig) != "") && s.deployer != nil {
		cleanName := QuadletUnitName(svc)
		content := s.deployer.prepareQuadletContent(ctx, svc, cleanName)
		files := SplitQuadletUnits(content, cleanName)

		quadletDir := filepath.Join(s.deployer.dataDir, "quadlets")
		_ = os.MkdirAll(quadletDir, 0755)

		systemdDir := os.Getenv("QUADLET_DIR")
		if systemdDir == "" {
			if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
				systemdDir = filepath.Join(homeDir, ".config", "containers", "systemd")
			}
		}
		if systemdDir != "" {
			_ = os.MkdirAll(systemdDir, 0755)
		}

		for fname, uContent := range files {
			// Unit files embed real env values (systemd needs them): keep them
			// owner-only, and fix the mode of files written by older builds.
			unitPath := filepath.Join(quadletDir, fname)
			_ = os.WriteFile(unitPath, []byte(uContent), 0600)
			_ = os.Chmod(unitPath, 0600)
			if systemdDir != "" {
				userPath := filepath.Join(systemdDir, fname)
				_ = os.WriteFile(userPath, []byte(uContent), 0600)
				_ = os.Chmod(userPath, 0600)
			}
		}

		if s.deployer != nil && s.deployer.hasUserManager() {
			_, _ = s.deployer.systemctlUser(ctx, "daemon-reload")
			if svc.Status == "running" {
				targetSvc := fmt.Sprintf("%s.service", cleanName)
				for fname := range files {
					if strings.HasSuffix(fname, ".pod") {
						targetSvc = fmt.Sprintf("%s-pod.service", strings.TrimSuffix(fname, ".pod"))
						break
					}
				}
				_, _ = s.deployer.systemctlUser(ctx, "restart", targetSvc)
			}
		}
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.update", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.update", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Service '%s' updated (type=%s)", svc.Name, svc.Type), "127.0.0.1")

	return nil
}

// DeleteService removes a service, stopping and tearing down all running workloads.
func (s *WorkloadService) DeleteService(ctx context.Context, id string, deleteVolumes bool) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}

	// 1. Abort any active in-flight deployment for this service
	if val, ok := s.activeServiceDeps.Load(id); ok {
		if depID, ok := val.(string); ok {
			_, _ = s.CancelDeployment(ctx, depID)
		}
	}

	svc, err := s.GetService(ctx, id)
	if err == nil && svc != nil {
		if s.deployer != nil {
			_ = s.deployer.TeardownService(ctx, *svc, deleteVolumes)
		}
		if s.ingressCleaner != nil {
			s.ingressCleaner(ctx, svc.ID)
		}
	}

	err = s.repo.Delete(ctx, id)
	if err == nil && s.audit != nil {
		_ = s.audit.Record(ctx, "service.delete", "operator", id, "runtime", "127.0.0.1", "success")
	}
	if err == nil {
		projectID, serviceName := "", id
		if svc != nil {
			projectID = svc.ProjectID
			serviceName = svc.Name
		}
		activity.RecordActivity(ctx, "operator", "service.delete", "service", id, projectID, id, "success",
			fmt.Sprintf("Service '%s' deleted (deleteVolumes=%v)", serviceName, deleteVolumes), "127.0.0.1")
		activity.EmitEngineEvent(ctx, "info", "service", id, projectID, "operator", "service.teardown",
			fmt.Sprintf("Service '%s' torn down (deleteVolumes=%v)", serviceName, deleteVolumes), map[string]any{"service": serviceName, "deleteVolumes": deleteVolumes})
	}
	return err
}

// StartService powers on the workload containers or units for a service.
func (s *WorkloadService) StartService(ctx context.Context, id string) (*Service, error) {
	svc, err := s.GetService(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	if s.deployer != nil {
		if err := s.deployer.StartService(ctx, *svc); err != nil {
			svc.Status = "failed"
			_ = s.repo.Update(ctx, *svc)
			activity.RecordActivity(ctx, "operator", "service.start", "service", svc.ID, svc.ProjectID, svc.ID, "failed",
				fmt.Sprintf("Service '%s' start failed: %v", svc.Name, err), "127.0.0.1")
			activity.EmitEngineEvent(ctx, "error", "service", svc.ID, svc.ProjectID, "operator", "lifecycle.start",
				fmt.Sprintf("Service '%s' start failed: %v", svc.Name, err), map[string]any{"service": svc.Name})
			return nil, err
		}
		svc.Status = s.deployer.GetRuntimeStatus(ctx, *svc)
	} else {
		svc.Status = "running"
	}

	if err := s.repo.Update(ctx, *svc); err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.start", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.start", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Service '%s' started (status=%s)", svc.Name, svc.Status), "127.0.0.1")
	activity.EmitEngineEvent(ctx, "info", "service", svc.ID, svc.ProjectID, "operator", "lifecycle.start",
		fmt.Sprintf("Service '%s' started (status=%s)", svc.Name, svc.Status), map[string]any{"service": svc.Name, "status": svc.Status})
	return svc, nil
}

// StopService halts the workload containers or units for a service.
func (s *WorkloadService) StopService(ctx context.Context, id string) (*Service, error) {
	svc, err := s.GetService(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	if s.deployer != nil {
		if err := s.deployer.StopService(ctx, *svc); err != nil {
			activity.RecordActivity(ctx, "operator", "service.stop", "service", svc.ID, svc.ProjectID, svc.ID, "failed",
				fmt.Sprintf("Service '%s' stop failed: %v", svc.Name, err), "127.0.0.1")
			activity.EmitEngineEvent(ctx, "error", "service", svc.ID, svc.ProjectID, "operator", "lifecycle.stop",
				fmt.Sprintf("Service '%s' stop failed: %v", svc.Name, err), map[string]any{"service": svc.Name})
			return nil, err
		}
		svc.Status = s.deployer.GetRuntimeStatus(ctx, *svc)
	} else {
		svc.Status = "stopped"
	}

	if err := s.repo.Update(ctx, *svc); err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.stop", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.stop", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Service '%s' stopped (status=%s)", svc.Name, svc.Status), "127.0.0.1")
	activity.EmitEngineEvent(ctx, "info", "service", svc.ID, svc.ProjectID, "operator", "lifecycle.stop",
		fmt.Sprintf("Service '%s' stopped (status=%s)", svc.Name, svc.Status), map[string]any{"service": svc.Name, "status": svc.Status})
	return svc, nil
}

// RestartService restarts the workload containers or units for a service.
func (s *WorkloadService) RestartService(ctx context.Context, id string) (*Service, error) {
	svc, err := s.GetService(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	if s.deployer != nil {
		if err := s.deployer.RestartService(ctx, *svc); err != nil {
			activity.RecordActivity(ctx, "operator", "service.restart", "service", svc.ID, svc.ProjectID, svc.ID, "failed",
				fmt.Sprintf("Service '%s' restart failed: %v", svc.Name, err), "127.0.0.1")
			activity.EmitEngineEvent(ctx, "error", "service", svc.ID, svc.ProjectID, "operator", "lifecycle.restart",
				fmt.Sprintf("Service '%s' restart failed: %v", svc.Name, err), map[string]any{"service": svc.Name})
			return nil, err
		}
		svc.Status = s.deployer.GetRuntimeStatus(ctx, *svc)
	} else {
		svc.Status = "running"
	}
	if err := s.repo.Update(ctx, *svc); err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.restart", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.restart", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Service '%s' restarted (status=%s)", svc.Name, svc.Status), "127.0.0.1")
	activity.EmitEngineEvent(ctx, "info", "service", svc.ID, svc.ProjectID, "operator", "lifecycle.restart",
		fmt.Sprintf("Service '%s' restarted (status=%s)", svc.Name, svc.Status), map[string]any{"service": svc.Name, "status": svc.Status})
	return svc, nil
}

// GetServiceJournal retrieves systemd journal logs for a service workload.
func (s *WorkloadService) GetServiceJournal(ctx context.Context, id string, lines int) (string, error) {
	svc, err := s.GetService(ctx, id)
	if err != nil {
		return "", err
	}
	if svc == nil {
		return "", errors.New("service not found")
	}
	if s.deployer == nil {
		return "", errors.New("deployer not initialized")
	}
	return s.deployer.GetServiceJournal(ctx, *svc, lines)
}

// ValidateQuadlet performs structural and official podman quadlet dryrun validation.
func (s *WorkloadService) ValidateQuadlet(ctx context.Context, content string, files map[string]string) (bool, string, error) {
	if len(files) == 0 {
		files = SplitQuadletUnits(content, "validation")
	}
	return ValidateQuadletFiles(files)
}

// PreviewQuadlet compiles the service's declarative configuration to unit files
// through the ONE shared generator and validates the result with the real
// `quadlet -dryrun`. Pod services render the pod plus one .container per
// attached child. The output is data only: nothing touches the systemd search
// path or the user manager.
func (s *WorkloadService) PreviewQuadlet(ctx context.Context, id string) (map[string]string, bool, string, error) {
	if s.deployer == nil {
		return nil, false, "", errors.New("deployer not initialized")
	}
	svc, err := s.GetService(ctx, id)
	if err != nil {
		return nil, false, "", err
	}
	if svc == nil {
		return nil, false, "", errors.New("service not found")
	}

	cleanName := QuadletUnitName(*svc)
	var content string
	if svc.Type == "pod" {
		content = buildPodQuadletBundle(
			*svc,
			PodNameForService(*svc),
			extractPodPorts(*svc),
			s.deployer.podMemberServices(ctx, *svc),
		)
	} else {
		content = s.deployer.prepareQuadletContent(ctx, *svc, cleanName)
	}
	if strings.TrimSpace(content) == "" {
		return nil, false, "", errors.New("service has no quadlet configuration to preview")
	}

	files := SplitQuadletUnits(content, cleanName)
	if len(files) == 0 {
		return nil, false, "", errors.New("quadlet configuration produced no unit files")
	}
	valid, dryrun, verr := ValidateQuadletFiles(files)
	return files, valid, dryrun, verr
}

// DemoteQuadlet converts a Quadlet service back to a standalone container managed by Podman.
func (s *WorkloadService) DemoteQuadlet(ctx context.Context, id string) (*Service, error) {
	svc, err := s.GetService(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	cleanName := QuadletUnitName(*svc)
	systemdDir := os.Getenv("QUADLET_DIR")
	if systemdDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdDir = filepath.Join(homeDir, ".config", "containers", "systemd")
		}
	}

	// 1. Discover all candidate systemd units and unit files
	var cands []string
	var filesToDelete []string
	if s.deployer != nil {
		cands, _, filesToDelete = s.deployer.findServiceUnitFilesAndCandidates(*svc)
	} else {
		cands = getQuadletCandidateUnitNames(*svc, cleanName)
	}

	// 2. Stop and disable all candidate systemd services
	if s.deployer != nil && s.deployer.hasUserManager() {
		for _, cand := range cands {
			_, _ = s.deployer.systemctlUser(ctx, "stop", cand)
			_, _ = s.deployer.systemctlUser(ctx, "kill", "--kill-who=all", "--signal=SIGKILL", cand)
			_, _ = s.deployer.systemctlUser(ctx, "disable", cand)
			_, _ = s.deployer.systemctlUser(ctx, "reset-failed", cand)
		}
	}

	// 3. Remove discovered quadlet unit files
	for _, fpath := range filesToDelete {
		_ = os.Remove(fpath)
	}
	if systemdDir != "" {
		_ = os.Remove(filepath.Join(systemdDir, fmt.Sprintf("%s.container", cleanName)))
		_ = os.Remove(filepath.Join(systemdDir, fmt.Sprintf("%s.pod", cleanName)))
		_ = os.Remove(filepath.Join(systemdDir, fmt.Sprintf("pod_%s.pod", cleanName)))
		_ = os.Remove(filepath.Join(systemdDir, fmt.Sprintf("%s.container", svc.Name)))
		if svc.QuadletConfig != "" {
			units := SplitQuadletUnits(svc.QuadletConfig, cleanName)
			for fname := range units {
				_ = os.Remove(filepath.Join(systemdDir, fname))
			}
			parsed := ParseQuadlet(svc.QuadletConfig)
			if parsed.ContainerName != "" {
				_ = os.Remove(filepath.Join(systemdDir, fmt.Sprintf("%s.container", parsed.ContainerName)))
			}
			if parsed.Pod != "" {
				_ = os.Remove(filepath.Join(systemdDir, fmt.Sprintf("%s.pod", parsed.Pod)))
			}
		}
	}
	if s.deployer != nil {
		quadletDir := filepath.Join(s.deployer.dataDir, "quadlets")
		_ = os.Remove(filepath.Join(quadletDir, fmt.Sprintf("%s.container", cleanName)))
		_ = os.Remove(filepath.Join(quadletDir, fmt.Sprintf("%s.pod", cleanName)))
	}

	// 4. Reload daemon and reset failed
	if _, err := exec.LookPath("systemctl"); err == nil {
		_ = exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
		for _, cand := range cands {
			_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", cand).Run()
		}
		_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed").Run()
	}

	// 4. Parse quadlet configuration to preserve image and ports
	if svc.QuadletConfig != "" {
		parsed := ParseQuadlet(svc.QuadletConfig)
		if parsed.Image != "" && ValidateContainerImage(parsed.Image) == nil {
			svc.Image = parsed.Image
		}
		if parsed.Port > 0 {
			svc.Port = parsed.Port
		}
		if parsed.HostPort > 0 {
			svc.HostPort = parsed.HostPort
		}
		if len(parsed.Env) > 0 && len(svc.EnvVars) == 0 {
			for k, v := range parsed.Env {
				svc.EnvVars = append(svc.EnvVars, EnvVar{Key: k, Value: v})
			}
		}
	}

	// 5. Restore origin workload anchor type
	targetType := "image"
	if svc.SourceType == "compose" || strings.TrimSpace(svc.ComposeYaml) != "" {
		targetType = "compose"
	} else if svc.SourceType == "kubernetes" || strings.TrimSpace(svc.K8sYaml) != "" {
		targetType = "kubernetes"
	} else if svc.SourceType == "pod" {
		targetType = "pod"
	} else if svc.SourceType == "git" || svc.SourceType == "dockerfile" || svc.GitRepo != "" {
		targetType = "application"
	} else if svc.SourceType == "image" {
		targetType = "image"
	}

	svc.Type = targetType
	if targetType == "pod" {
		svc.RuntimeTarget = "pod"
		svc.InPod = true
	} else {
		svc.RuntimeTarget = "standalone"
		svc.InPod = false
	}
	svc.QuadletConfig = ""

	if err := s.repo.Update(ctx, *svc); err != nil {
		return nil, err
	}

	// 5. Deploy container directly via Podman
	if s.deployer != nil {
		_, _ = s.deployer.DeployService(ctx, *svc, fmt.Sprintf("demote-%d", time.Now().Unix()))
		svc.Status = s.deployer.GetRuntimeStatus(ctx, *svc)
		_ = s.repo.Update(ctx, *svc)
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.demote", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.demote", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Service '%s' demoted from quadlet (type=%s)", svc.Name, svc.Type), "127.0.0.1")

	return svc, nil
}

// GroupIntoPod groups a service into a Podman pod.
func (s *WorkloadService) GroupIntoPod(ctx context.Context, id, podName string, createPod bool, podPorts []string) (*Service, error) {
	svc, err := s.GetService(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	podSlug := SanitizeQuadletSlug(strings.TrimPrefix(strings.TrimSpace(podName), "pod_"))
	if podSlug == "" {
		return nil, errors.New("pod name cannot be empty")
	}
	// Resolve to the owning pod service's canonical runtime name when possible
	// so members always reference the unique pod_<project>-<service>-<rand> name.
	cleanPodName := PodNameFor(podName)
	if allSvcs, err := s.repo.List(ctx, svc.ProjectID); err == nil {
		want := PodNameFor(podName)
		for _, cand := range allSvcs {
			if cand.Type != "pod" {
				continue
			}
			if PodNameForService(cand) == want ||
				cand.ID == strings.TrimSpace(podName) ||
				cand.Name == strings.TrimSpace(podName) ||
				cand.PodID == strings.TrimSpace(podName) {
				cleanPodName = PodNameForService(cand)
				break
			}
		}
	}

	if s.deployer != nil && s.deployer.podmanClient != nil {
		// Ensure pod exists before placing container inside it
		if _, err := s.deployer.podmanClient.InspectPod(ctx, cleanPodName); err != nil {
			log.Printf("[GroupPod] Resolved target pod '%s' for service '%s' (requested %q); auto-creating with ports %v...", cleanPodName, svc.Name, podName, podPorts)
			_, err := s.deployer.podmanClient.CreatePod(ctx, podman.CreatePodOptions{
				Name:    cleanPodName,
				Ports:   podPorts,
				Network: "gopod-net",
				Labels: map[string]string{
					"io.gopod.project": svc.ProjectID,
				},
			})
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				return nil, fmt.Errorf("failed to create pod: %w", err)
			}
			activity.EmitEngineEvent(ctx, "info", "service", svc.ID, svc.ProjectID, "operator", "pod.autocreate",
				fmt.Sprintf("Pod '%s' auto-created with ports %v for group-pod of '%s'", cleanPodName, podPorts, svc.Name), map[string]any{"pod": cleanPodName, "ports": podPorts, "service": svc.Name})
		} else {
			log.Printf("[GroupPod] Resolved target pod '%s' for service '%s' (requested %q); pod exists, joining.", cleanPodName, svc.Name, podName)
		}
	}

	svc.InPod = true
	svc.PodID = cleanPodName
	svc.RuntimeTarget = "pod"

	if err := s.repo.Update(ctx, *svc); err != nil {
		return nil, err
	}

	if s.deployer != nil {
		groupDepID := fmt.Sprintf("group-pod-%d", time.Now().Unix())
		if s.logManager != nil {
			s.logManager.LogLine(groupDepID, fmt.Sprintf("🔗 Group-into-pod: service '%s' resolved target pod '%s' (requested %q, ports %v).", svc.Name, cleanPodName, podName, podPorts))
		}
		_, _ = s.deployer.DeployService(ctx, *svc, groupDepID, "group-pod")
		svc.Status = s.deployer.GetRuntimeStatus(ctx, *svc)
		_ = s.repo.Update(ctx, *svc)
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.group_pod", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.group_pod", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Service '%s' grouped into pod '%s' (ports %v)", svc.Name, cleanPodName, podPorts), "127.0.0.1")
	activity.EmitEngineEvent(ctx, "info", "service", svc.ID, svc.ProjectID, "operator", "pod.group",
		fmt.Sprintf("Service '%s' grouped into pod '%s'", svc.Name, cleanPodName), map[string]any{"service": svc.Name, "pod": cleanPodName, "ports": podPorts})

	return svc, nil
}

// UngroupFromPod detaches a member service from its pod in BOTH database and
// runtime: the container is removed from the pod's localhost namespace and
// redeployed as a standalone container. Flipping DB flags alone would leave
// the container serving stale traffic inside the pod, so the runtime detach is
// part of the operation.
func (s *WorkloadService) UngroupFromPod(ctx context.Context, id string) (*Service, error) {
	svc, err := s.GetService(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	svc.InPod = false
	svc.PodID = ""
	if svc.RuntimeTarget == "pod" {
		svc.RuntimeTarget = "standalone"
	}
	if err := s.repo.Update(ctx, *svc); err != nil {
		return nil, err
	}

	// Remove the stale copy from the pod before starting the standalone one.
	// deployImage's preflight cleanup targets the same candidate names, so a
	// plain redeploy relocates the container; do an explicit best-effort
	// removal first for immediate truthfulness.
	if s.deployer != nil {
		s.deployer.cleanPreflightConflicts(ctx, *svc, nil)
		ungroupDepID := fmt.Sprintf("ungroup-pod-%d", time.Now().Unix())
		if s.logManager != nil {
			s.logManager.LogLine(ungroupDepID, fmt.Sprintf("🔗 Ungroup-from-pod: service '%s' detached to standalone; redeploying outside pod.", svc.Name))
		}
		log.Printf("[UngroupPod] Service '%s' detached from pod; redeploying as standalone...", svc.Name)
		_, _ = s.deployer.DeployService(ctx, *svc, ungroupDepID, "ungroup-pod")
		svc.Status = s.deployer.GetRuntimeStatus(ctx, *svc)
		_ = s.repo.Update(ctx, *svc)
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.ungroup_pod", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	activity.RecordActivity(ctx, "operator", "service.ungroup_pod", "service", svc.ID, svc.ProjectID, svc.ID, "success",
		fmt.Sprintf("Service '%s' ungrouped from pod to standalone", svc.Name), "127.0.0.1")
	activity.EmitEngineEvent(ctx, "info", "service", svc.ID, svc.ProjectID, "operator", "pod.ungroup",
		fmt.Sprintf("Service '%s' ungrouped from pod to standalone", svc.Name), map[string]any{"service": svc.Name})

	return svc, nil
}
