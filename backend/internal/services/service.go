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
	repo     Repository
	deployer *Deployer
	audit    AuditRecorder
}

// NewWorkloadService creates a new services domain service.
func NewWorkloadService(repo Repository, deployer *Deployer) *WorkloadService {
	return &WorkloadService{
		repo:     repo,
		deployer: deployer,
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
		svc.ID = strings.ToLower(svc.ProjectID + "-" + strings.ReplaceAll(svc.Name, " ", "-"))
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

// Deploy triggers a container deployment or rebuild and records the deployment history.
func (s *WorkloadService) Deploy(ctx context.Context, serviceID string) (string, error) {
	svc, err := s.GetService(ctx, serviceID)
	if err != nil {
		return "", err
	}
	if svc == nil {
		return "", errors.New("service not found")
	}

	start := time.Now()
	depID := fmt.Sprintf("dep-%d", start.Unix())
	depRecord := Deployment{
		ID:            depID,
		ProjectID:     svc.ProjectID,
		ServiceID:     svc.ID,
		Version:       "v1",
		CommitHash:    "HEAD",
		CommitMessage: "Manual trigger or webhook",
		Status:        "building",
		StartedAt:     start,
	}
	_ = s.repo.CreateDeployment(ctx, depRecord)

	if s.deployer == nil {
		return "", errors.New("deployer not configured")
	}

	result, err := s.deployer.DeployService(ctx, *svc)
	finished := time.Now()
	duration := finished.Sub(start).Round(time.Millisecond).String()

	status := "success"
	if err != nil {
		status = "failed"
	}

	containerName := ""
	if result != nil {
		containerName = result.ContainerName
		if result.CommitHash != "" {
			depRecord.CommitHash = result.CommitHash
		}
		if result.CommitMessage != "" {
			depRecord.CommitMessage = result.CommitMessage
		}
	}

	// Update service status
	svc.Status = "running"
	if err != nil {
		svc.Status = "error"
	}
	_ = s.repo.Update(ctx, *svc)

	depRecord.Status = status
	depRecord.Duration = duration
	depRecord.FinishedAt = &finished
	_ = s.repo.CreateDeployment(ctx, depRecord)

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.deploy", "operator", svc.Name, "deployment", "127.0.0.1", status)
	}

	if err != nil {
		return containerName, err
	}
	return containerName, nil
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
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		log.Printf("🔔 Webhook deploy triggered for service '%s' (%s)", svc.Name, svc.ID)
		_, err := s.Deploy(bgCtx, svc.ID)
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
