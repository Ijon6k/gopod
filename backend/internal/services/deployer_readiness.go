package services

import (
	"context"
	"fmt"
	"time"

	"gopod/internal/podman"
)

const (
	defaultReadinessTimeout      = 60 * time.Second
	defaultReadinessPollInterval = 750 * time.Millisecond
	defaultReadinessGracePeriod  = 1500 * time.Millisecond
)

func (d *Deployer) inspectContainerState(ctx context.Context, idOrName string) (*podman.ContainerInspectState, error) {
	if d.readinessInspectFn != nil {
		return d.readinessInspectFn(ctx, idOrName)
	}
	if d.podmanClient == nil {
		return &podman.ContainerInspectState{Status: "running", Running: true}, nil
	}
	return d.podmanClient.InspectContainerState(ctx, idOrName)
}

// awaitReplacementReady blocks until the newly launched start-first replacement
// container passes its image HEALTHCHECK (when present) or proves stable past a
// short startup grace check so a crash-looping container never causes us to
// retire a healthy predecessor.
func (d *Deployer) awaitReplacementReady(ctx context.Context, containerName string, logLine func(string)) error {
	if d.podmanClient == nil && d.readinessInspectFn == nil {
		return nil
	}

	timeout := d.readinessTimeout
	if timeout <= 0 {
		timeout = defaultReadinessTimeout
	}
	poll := d.readinessPollInterval
	if poll <= 0 {
		poll = defaultReadinessPollInterval
	}
	grace := d.readinessGracePeriod
	if grace <= 0 {
		grace = defaultReadinessGracePeriod
	}

	start := time.Now()
	deadline := start.Add(timeout)
	loggedHealthWait := false

	for {
		st, err := d.inspectContainerState(ctx, containerName)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out inspecting replacement '%s': %w", containerName, err)
			}
		} else {
			if !st.Running && st.Status != "created" {
				return fmt.Errorf("replacement container '%s' exited prematurely (status=%s, exitCode=%d)", containerName, st.Status, st.ExitCode)
			}
			if st.HasHealth {
				switch st.HealthStatus {
				case "healthy":
					logLine(fmt.Sprintf("💚 Replacement container '%s' passed HEALTHCHECK.", containerName))
					return nil
				case "unhealthy":
					return fmt.Errorf("replacement container '%s' reported HEALTHCHECK unhealthy", containerName)
				default:
					if !loggedHealthWait {
						logLine(fmt.Sprintf("🩺 Waiting for replacement container '%s' HEALTHCHECK...", containerName))
						loggedHealthWait = true
					}
				}
			} else if st.Running && time.Since(start) >= grace {
				logLine(fmt.Sprintf("✅ Replacement container '%s' stabilized (running).", containerName))
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for replacement container '%s' to become ready", containerName)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}
}

func (d *Deployer) rollbackFailedReplacement(ctx context.Context, plan containerRedeployPlan, logLine func(string)) {
	if plan.mode != redeployStartFirst || plan.runName == "" {
		return
	}
	logLine(fmt.Sprintf("🛡️ Keeping healthy predecessor '%s' active and removing failed replacement '%s'.", plan.predecessorID, plan.runName))
	if d.podmanClient != nil {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = d.podmanClient.StopContainer(cleanCtx, plan.runName)
		_ = d.podmanClient.DeleteContainer(cleanCtx, plan.runName, true)
	}
}

