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

// StartDeploy initiates an asynchronous container deployment with optional commit/image overrides.
func (s *WorkloadService) StartDeploy(ctx context.Context, serviceID, trigger string, overrides ...string) (*Deployment, error) {
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

	var commitOverride, imageOverride string
	if len(overrides) > 0 {
		commitOverride = overrides[0]
	}
	if len(overrides) > 1 {
		imageOverride = overrides[1]
	}

	// Apply rollback commit/image overrides if specified
	if commitOverride != "" {
		svc.GitBranch = commitOverride
		svc.Branch = commitOverride
	}
	if imageOverride != "" {
		svc.Image = imageOverride
	}

	pastDeps, _ := s.repo.ListDeployments(ctx, serviceID)
	nextNumber := len(pastDeps) + 1

	start := time.Now()
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	depID := fmt.Sprintf("dep-%d-%s", start.Unix(), hex.EncodeToString(b))

	commitMsg := "Deployment initiated"
	if trigger == "rollback" {
		if commitOverride != "" {
			commitMsg = fmt.Sprintf("Rollback to commit %s", commitOverride)
		} else if imageOverride != "" {
			commitMsg = fmt.Sprintf("Rollback to image %s", imageOverride)
		} else {
			commitMsg = "Rollback initiated"
		}
	}

	depRecord := Deployment{
		ID:            depID,
		ProjectID:     svc.ProjectID,
		ServiceID:     svc.ID,
		Number:        nextNumber,
		Trigger:       trigger,
		Version:       fmt.Sprintf("#%d", nextNumber),
		CommitHash:    commitOverride,
		CommitMessage: commitMsg,
		Image:         svc.Image,
		Status:        "building",
		StartedAt:     start,
	}

	if err := s.repo.CreateDeployment(ctx, depRecord); err != nil {
		log.Printf("[Deploy] Initial CreateDeployment error: %v", err)
	}

	svc.Status = "deploying"
	_ = s.repo.Update(ctx, *svc)

	bgCtx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	s.activeCancels.Store(depRecord.ID, cancel)
	s.activeServiceDeps.Store(svc.ID, depRecord.ID)

	go func() {
		defer func() {
			cancel()
			s.activeCancels.Delete(depRecord.ID)
			s.activeServiceDeps.Delete(svc.ID)
		}()
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
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(deployErr, context.Canceled) {
			status = "cancelled"
		} else {
			status = "failed"
		}
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
	if status == "cancelled" {
		if s.deployer != nil {
			realStatus := s.deployer.GetRuntimeStatus(ctx, *svc)
			if realStatus != "" {
				svc.Status = realStatus
			} else {
				svc.Status = "stopped"
			}
		} else {
			svc.Status = "stopped"
		}
	} else if deployErr != nil {
		svc.Status = "failed"
	} else if s.deployer != nil {
		realStatus := s.deployer.GetRuntimeStatus(ctx, *svc)
		if realStatus != "" {
			svc.Status = realStatus
		} else {
			svc.Status = "running"
		}
	} else {
		svc.Status = "running"
	}
	_ = s.repo.Update(ctx, *svc)

	depRecord.Status = status
	depRecord.Duration = duration
	depRecord.FinishedAt = &finished

	if err := s.repo.UpdateDeployment(ctx, depRecord); err != nil {
		log.Printf("[Deploy] UpdateDeployment error: %v", err)
	}

	if s.audit != nil {
		action := "service.deploy"
		if depRecord.Trigger == "promote-quadlet" || svc.Type == "quadlet" {
			action = "service.promote_quadlet"
		}
		_ = s.audit.Record(ctx, action, "operator", svc.Name, "deployment", "127.0.0.1", status)
	}
}

// CancelDeployment cancels an in-flight deployment and reconciles service state.
func (s *WorkloadService) CancelDeployment(ctx context.Context, depID string) (*Deployment, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}

	dep, err := s.repo.GetDeployment(ctx, depID)
	if err != nil {
		return nil, err
	}
	if dep == nil {
		return nil, errors.New("deployment not found")
	}

	// Idempotency: if already in a terminal state, return without modifying
	if dep.Status != "building" && dep.Status != "deploying" {
		return dep, nil
	}

	// Invoke active cancellation context if present
	if val, ok := s.activeCancels.Load(depID); ok {
		if cancel, ok := val.(context.CancelFunc); ok {
			cancel()
		}
		s.activeCancels.Delete(depID)
		s.activeServiceDeps.Delete(dep.ServiceID)
	}

	now := time.Now()
	dep.Status = "cancelled"
	dep.Duration = "Cancelled by operator"
	dep.FinishedAt = &now
	_ = s.repo.UpdateDeployment(ctx, *dep)

	if s.logManager != nil {
		s.logManager.LogLine(depID, "🛑 [gopod-engine] Deployment cancelled by operator.")
	}

	// Reconcile parent service runtime status
	svc, _ := s.repo.Get(ctx, dep.ServiceID)
	if svc != nil {
		if s.deployer != nil {
			realStatus := s.deployer.GetRuntimeStatus(ctx, *svc)
			if realStatus != "" {
				svc.Status = realStatus
			} else {
				svc.Status = "stopped"
			}
		} else {
			svc.Status = "stopped"
		}
		_ = s.repo.Update(ctx, *svc)

		if s.audit != nil {
			_ = s.audit.Record(ctx, "service.cancel_deployment", "operator", svc.Name, "deployment", "127.0.0.1", "cancelled")
		}
	}

	return dep, nil
}

// CancelServiceDeployment cancels any active deployment for a service and forces state reconciliation.
func (s *WorkloadService) CancelServiceDeployment(ctx context.Context, serviceID string) (*Service, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}

	svc, err := s.repo.Get(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("service not found")
	}

	// Check if active in memory
	if val, ok := s.activeServiceDeps.Load(serviceID); ok {
		if depID, ok := val.(string); ok {
			_, _ = s.CancelDeployment(ctx, depID)
		}
	}

	// Also cancel any deployments marked building/deploying in database for this service
	deps, _ := s.repo.ListDeployments(ctx, serviceID)
	now := time.Now()
	for _, d := range deps {
		if d.Status == "building" || d.Status == "deploying" {
			d.Status = "cancelled"
			d.Duration = "Cancelled by operator"
			d.FinishedAt = &now
			_ = s.repo.UpdateDeployment(ctx, d)
			if s.logManager != nil {
				s.logManager.LogLine(d.ID, "🛑 [gopod-engine] Deployment cancelled by operator.")
			}
		}
	}

	// Reset service status to real runtime status
	if s.deployer != nil {
		realStatus := s.deployer.GetRuntimeStatus(ctx, *svc)
		if realStatus != "" {
			svc.Status = realStatus
		} else {
			svc.Status = "stopped"
		}
	} else {
		svc.Status = "stopped"
	}
	_ = s.repo.Update(ctx, *svc)

	if s.audit != nil {
		_ = s.audit.Record(ctx, "service.cancel_deployment", "operator", svc.Name, "deployment", "127.0.0.1", "cancelled")
	}

	return svc, nil
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

// GetServiceByWebhookToken finds a service by its secret webhook token or ID.
func (s *WorkloadService) GetServiceByWebhookToken(ctx context.Context, token string) (*Service, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	svc, err := s.repo.GetByWebhookToken(ctx, token)
	if err != nil || svc == nil {
		return s.repo.Get(ctx, token)
	}
	return svc, nil
}

// DeployByWebhook verifies webhook token and triggers deployment asynchronously.
func (s *WorkloadService) DeployByWebhook(ctx context.Context, token string) (*Service, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("unauthorized webhook token")
	}
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
