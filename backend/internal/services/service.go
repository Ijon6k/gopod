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
)

// AuditRecorder records workload actions.
type AuditRecorder interface {
	Record(ctx context.Context, action, actor, target, category, ip, status string) error
}

// WorkloadService encapsulates workload management and deployment orchestration.
type WorkloadService struct {
	repo       Repository
	deployer   *Deployer
	logManager *LogManager
	audit      AuditRecorder
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

// ListServices retrieves services, optionally filtered by projectID.
func (s *WorkloadService) ListServices(ctx context.Context, projectID string) ([]Service, error) {
	if s.repo == nil {
		return []Service{}, nil
	}
	return s.repo.List(ctx, projectID)
}

// GetService retrieves a single service by ID.
func (s *WorkloadService) GetService(ctx context.Context, id string) (*Service, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	return s.repo.Get(ctx, id)
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
	return s.repo.Update(ctx, svc)
}

// DeleteService removes a service.
func (s *WorkloadService) DeleteService(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	err := s.repo.Delete(ctx, id)
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
			return nil, err
		}
	}

	svc.Status = "running"
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
	}

	svc.Status = "stopped"
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
