package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

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
	repo           Repository
	deployer       *Deployer
	logManager     *LogManager
	audit          AuditRecorder
	ingressCleaner IngressCleaner
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

func (s *WorkloadService) reconcileRuntimeStatus(ctx context.Context, svc *Service) {
	if svc == nil || s.deployer == nil {
		return
	}
	// Transient deployment states are managed during deploy lifecycle
	if svc.Status == "deploying" || svc.Status == "building" {
		return
	}
	realStatus := s.deployer.GetRuntimeStatus(ctx, *svc)
	if realStatus != "" && realStatus != svc.Status {
		svc.Status = realStatus
		_ = s.repo.Update(ctx, *svc)
	}
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
				continue
			}
			if realSt, ok := statusMap[svc.ID]; ok && realSt != "" && realSt != svc.Status {
				svc.Status = realSt
				_ = s.repo.Update(ctx, *svc)
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

	svc.CreatedAt = time.Now()

	if err := s.repo.Create(ctx, svc); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.create", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	return &svc, nil
}

// UpdateService modifies service configuration.
func (s *WorkloadService) UpdateService(ctx context.Context, svc Service) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}

	if strings.TrimSpace(svc.ComposeYaml) != "" {
		var raw map[string]interface{}
		if err := yaml.Unmarshal([]byte(svc.ComposeYaml), &raw); err != nil {
			return fmt.Errorf("invalid Compose YAML syntax: %w", err)
		}
	}

	return s.repo.Update(ctx, svc)
}

// DeleteService removes a service, stopping and tearing down all running workloads (Dokploy parity).
func (s *WorkloadService) DeleteService(ctx context.Context, id string, deleteVolumes bool) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
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
	}

	svc.Status = "running"
	if err := s.repo.Update(ctx, *svc); err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.restart", "operator", svc.Name, "runtime", "127.0.0.1", "success")
	}
	return svc, nil
}

// StartDeploy initiates an asynchronous container deployment and returns the pending record immediately.
func (s *WorkloadService) StartDeploy(ctx context.Context, serviceID, trigger string) (*Deployment, error) {
	svc, err := s.GetService(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	if trigger == "" {
		trigger = "manual"
	}

	pastDeps, _ := s.repo.ListDeployments(ctx, serviceID)
	nextNumber := len(pastDeps) + 1

	start := time.Now()
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	depID := fmt.Sprintf("dep-%d-%s", start.Unix(), hex.EncodeToString(b))

	depRecord := Deployment{
		ID:            depID,
		ProjectID:     svc.ProjectID,
		ServiceID:     svc.ID,
		Number:        nextNumber,
		Trigger:       trigger,
		Version:       fmt.Sprintf("#%d", nextNumber),
		CommitHash:    "",
		CommitMessage: "Deployment initiated",
		Status:        "building",
		StartedAt:     start,
	}

	if err := s.repo.CreateDeployment(ctx, depRecord); err != nil {
		log.Printf("[Deploy] Initial CreateDeployment error: %v", err)
	}

	svc.Status = "deploying"
	_ = s.repo.Update(ctx, *svc)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
		defer cancel()
		s.executeDeployment(bgCtx, svc, depRecord, start)
	}()

	return &depRecord, nil
}

// Deploy triggers synchronous container deployment.
func (s *WorkloadService) Deploy(ctx context.Context, serviceID, trigger string) (*Deployment, error) {
	svc, err := s.GetService(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	if trigger == "" {
		trigger = "manual"
	}

	pastDeps, _ := s.repo.ListDeployments(ctx, serviceID)
	nextNumber := len(pastDeps) + 1

	start := time.Now()
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	depID := fmt.Sprintf("dep-%d-%s", start.Unix(), hex.EncodeToString(b))

	depRecord := Deployment{
		ID:            depID,
		ProjectID:     svc.ProjectID,
		ServiceID:     svc.ID,
		Number:        nextNumber,
		Trigger:       trigger,
		Version:       fmt.Sprintf("#%d", nextNumber),
		CommitHash:    "",
		CommitMessage: "Deployment initiated",
		Status:        "building",
		StartedAt:     start,
	}

	_ = s.repo.CreateDeployment(ctx, depRecord)
	svc.Status = "deploying"
	_ = s.repo.Update(ctx, *svc)

	s.executeDeployment(ctx, svc, depRecord, start)
	return &depRecord, nil
}

func (s *WorkloadService) executeDeployment(ctx context.Context, svc *Service, depRecord Deployment, start time.Time) {
	if s.deployer == nil {
		depRecord.Status = "failed"
		depRecord.FinishedAt = &start
		_ = s.repo.UpdateDeployment(ctx, depRecord)
		return
	}

	result, deployErr := s.deployer.DeployService(ctx, *svc, depRecord.ID, depRecord.Trigger)
	finished := time.Now()
	duration := finished.Sub(start).Round(time.Millisecond).String()

	status := "success"
	if deployErr != nil {
		status = "failed"
	}

	if result != nil {
		if result.CommitHash != "" {
			depRecord.CommitHash = result.CommitHash
		}
		if result.CommitMessage != "" {
			depRecord.CommitMessage = result.CommitMessage
		}
		if result.Version != "" {
			depRecord.Version = result.Version
		}
		if result.Image != "" {
			depRecord.Image = result.Image
		}
	}

	// Update service status
	svc.Status = "running"
	if deployErr != nil {
		svc.Status = "failed"
	}
	_ = s.repo.Update(ctx, *svc)

	depRecord.Status = status
	depRecord.Duration = duration
	depRecord.FinishedAt = &finished

	if err := s.repo.UpdateDeployment(ctx, depRecord); err != nil {
		log.Printf("[Deploy] UpdateDeployment error: %v", err)
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.deploy", "operator", svc.Name, "deployment", "127.0.0.1", status)
	}
}

// GetDeployment retrieves deployment record by ID.
func (s *WorkloadService) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	return s.repo.GetDeployment(ctx, id)
}

// GetDeploymentLogs retrieves text logs for a specific deployment.
func (s *WorkloadService) GetDeploymentLogs(ctx context.Context, id string) (string, error) {
	if s.logManager == nil {
		return "", nil
	}
	return s.logManager.GetLogs(id)
}

// SubscribeLogs provides real-time streaming channel for a deployment.
func (s *WorkloadService) SubscribeLogs(deploymentID string) (<-chan string, func()) {
	if s.logManager == nil {
		ch := make(chan string)
		close(ch)
		return ch, func() {}
	}
	return s.logManager.Subscribe(deploymentID)
}

// GetServiceByWebhookToken finds a service by its secret webhook token.
func (s *WorkloadService) GetServiceByWebhookToken(ctx context.Context, token string) (*Service, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	return s.repo.GetByWebhookToken(ctx, token)
}

// DeployByWebhook verifies webhook token and triggers deployment asynchronously.
func (s *WorkloadService) DeployByWebhook(ctx context.Context, token string) (*Service, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	svc, err := s.repo.GetByWebhookToken(ctx, token)
	if err != nil || svc == nil {
		return nil, errors.New("unauthorized webhook token")
	}

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		log.Printf("🔔 Webhook deploy triggered for service '%s' (%s)", svc.Name, svc.ID)
		_, err := s.Deploy(bgCtx, svc.ID, "webhook")
		if err != nil {
			log.Printf("❌ Webhook deploy failed for '%s': %v", svc.Name, err)
		} else {
			log.Printf("✅ Webhook deploy succeeded for '%s'", svc.Name)
		}
	}()

	return svc, nil
}

// ListDeployments returns recent deployment runs for a service.
func (s *WorkloadService) ListDeployments(ctx context.Context, serviceID string) ([]Deployment, error) {
	if s.repo == nil {
		return []Deployment{}, nil
	}
	return s.repo.ListDeployments(ctx, serviceID)
}
