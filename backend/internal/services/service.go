package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopod/internal/podman"
	"gopkg.in/yaml.v3"
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
}

// NewWorkloadService creates a new services domain service.
func NewWorkloadService(repo Repository, deployer *Deployer, logMgr *LogManager) *WorkloadService {
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
		randBytes := make([]byte, 3)
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
	return &svc, nil
}

// UpdateService modifies service configuration and syncs declarative units.
func (s *WorkloadService) UpdateService(ctx context.Context, svc Service) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
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

	if err := s.repo.Update(ctx, svc); err != nil {
		return err
	}

	// Real Quadlet unit compilation to disk and systemd daemon-reload
	if (svc.Type == "quadlet" || svc.RuntimeTarget == "quadlet" || strings.TrimSpace(svc.QuadletConfig) != "") && s.deployer != nil {
		cleanName := QuadletUnitName(svc)
		content := s.deployer.prepareQuadletContent(svc, cleanName)
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
			_ = os.WriteFile(filepath.Join(quadletDir, fname), []byte(uContent), 0644)
			if systemdDir != "" {
				_ = os.WriteFile(filepath.Join(systemdDir, fname), []byte(uContent), 0644)
			}
		}

		if _, err := exec.LookPath("systemctl"); err == nil {
			_ = exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
			if svc.Status == "running" {
				targetSvc := fmt.Sprintf("%s.service", cleanName)
				for fname := range files {
					if strings.HasSuffix(fname, ".pod") {
						targetSvc = fmt.Sprintf("%s-pod.service", strings.TrimSuffix(fname, ".pod"))
						break
					}
				}
				_ = exec.CommandContext(ctx, "systemctl", "--user", "restart", targetSvc).Run()
			}
		}
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.update", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}

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
	if _, err := exec.LookPath("systemctl"); err == nil {
		for _, cand := range cands {
			_ = exec.CommandContext(ctx, "systemctl", "--user", "stop", cand).Run()
			_ = exec.CommandContext(ctx, "systemctl", "--user", "kill", "--kill-who=all", "--signal=SIGKILL", cand).Run()
			_ = exec.CommandContext(ctx, "systemctl", "--user", "disable", cand).Run()
			_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", cand).Run()
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

	cleanPodName := strings.TrimSpace(podName)
	if cleanPodName == "" {
		return nil, errors.New("pod name cannot be empty")
	}

	if createPod && s.deployer != nil && s.deployer.podmanClient != nil {
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
	}

	svc.InPod = true
	svc.PodID = cleanPodName
	svc.RuntimeTarget = "pod"

	if err := s.repo.Update(ctx, *svc); err != nil {
		return nil, err
	}

	if s.deployer != nil {
		_, _ = s.deployer.DeployService(ctx, *svc, fmt.Sprintf("group-pod-%d", time.Now().Unix()), "group-pod")
		svc.Status = s.deployer.GetRuntimeStatus(ctx, *svc)
		_ = s.repo.Update(ctx, *svc)
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.group_pod", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}

	return svc, nil
}
