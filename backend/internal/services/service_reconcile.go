package services

import (
	"context"
	"log"
	"time"
)

func (s *WorkloadService) reconcileRuntimeStatus(ctx context.Context, svc *Service) {
	if svc == nil || s.deployer == nil {
		return
	}
	if svc.Status == "deploying" || svc.Status == "building" {
		// Check if actively deploying in-memory
		if _, active := s.activeServiceDeps.Load(svc.ID); active {
			return
		}

		// Verify database state for stale deployments
		deps, err := s.repo.ListDeployments(ctx, svc.ID)
		hasActive := false
		if err == nil && len(deps) > 0 {
			now := time.Now()
			for _, d := range deps {
				if d.Status == "building" || d.Status == "deploying" {
					// Auto-heal abandoned deployments older than 15 minutes
					if time.Since(d.StartedAt) > 15*time.Minute {
						d.Status = "interrupted"
						d.Duration = "Timed out"
						d.FinishedAt = &now
						_ = s.repo.UpdateDeployment(ctx, d)
					} else {
						hasActive = true
					}
				}
			}
		}
		if hasActive {
			return
		}
	}
	realStatus := s.deployer.GetRuntimeStatus(ctx, *svc)
	if realStatus != "" && realStatus != svc.Status {
		svc.Status = realStatus
		_ = s.repo.Update(ctx, *svc)
	}
}

// ReconcileOrphanedDeployments cleans up stale in-flight state left behind by server restarts or crashes.
func (s *WorkloadService) ReconcileOrphanedDeployments(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}

	services, err := s.repo.List(ctx, "")
	if err != nil {
		return err
	}

	now := time.Now()
	for _, svc := range services {
		// Clean up stale deployments
		deps, _ := s.repo.ListDeployments(ctx, svc.ID)
		for _, d := range deps {
			if d.Status == "building" || d.Status == "deploying" {
				d.Status = "interrupted"
				d.Duration = "Engine restarted"
				d.FinishedAt = &now
				_ = s.repo.UpdateDeployment(ctx, d)
				if s.logManager != nil {
					s.logManager.LogLine(d.ID, "⚠️ [gopod-engine] Engine restarted while deployment was running. Marked as interrupted.")
				}
			}
		}

		if svc.Status == "deploying" || svc.Status == "building" {
			if s.deployer != nil {
				realStatus := s.deployer.GetRuntimeStatus(ctx, svc)
				if realStatus != "" {
					svc.Status = realStatus
				} else {
					svc.Status = "stopped"
				}
			} else {
				svc.Status = "stopped"
			}
			_ = s.repo.Update(ctx, svc)
			log.Printf("[Reconcile] Reset stuck service %s (%s) status to %s on startup", svc.Name, svc.ID, svc.Status)
		}
	}

	return nil
}
