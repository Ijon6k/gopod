package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"gopod/internal/podman"
)

func TestAwaitReplacementReady_HealthcheckTransitions(t *testing.T) {
	var calls atomic.Int32
	d := &Deployer{
		readinessTimeout:      500 * time.Millisecond,
		readinessPollInterval: 5 * time.Millisecond,
		readinessGracePeriod:  10 * time.Millisecond,
		readinessInspectFn: func(ctx context.Context, idOrName string) (*podman.ContainerInspectState, error) {
			n := calls.Add(1)
			if n < 3 {
				return &podman.ContainerInspectState{Status: "running", Running: true, HasHealth: true, HealthStatus: "starting"}, nil
			}
			return &podman.ContainerInspectState{Status: "running", Running: true, HasHealth: true, HealthStatus: "healthy"}, nil
		},
	}

	var logs []string
	err := d.awaitReplacementReady(context.Background(), "svc-next", func(s string) { logs = append(logs, s) })
	if err != nil {
		t.Fatalf("expected healthy transition to pass, got: %v", err)
	}
	if calls.Load() < 3 {
		t.Fatalf("expected at least 3 polls before healthy, got %d", calls.Load())
	}
}

func TestAwaitReplacementReady_FailsUnhealthyOrExited(t *testing.T) {
	t.Run("unhealthy", func(t *testing.T) {
		d := &Deployer{
			readinessTimeout:      300 * time.Millisecond,
			readinessPollInterval: 5 * time.Millisecond,
			readinessGracePeriod:  10 * time.Millisecond,
			readinessInspectFn: func(ctx context.Context, idOrName string) (*podman.ContainerInspectState, error) {
				return &podman.ContainerInspectState{Status: "running", Running: true, HasHealth: true, HealthStatus: "unhealthy"}, nil
			},
		}
		if err := d.awaitReplacementReady(context.Background(), "svc-bad", func(string) {}); err == nil {
			t.Fatal("expected unhealthy container to fail readiness gate")
		}
	})

	t.Run("exited prematurely", func(t *testing.T) {
		d := &Deployer{
			readinessTimeout:      300 * time.Millisecond,
			readinessPollInterval: 5 * time.Millisecond,
			readinessGracePeriod:  10 * time.Millisecond,
			readinessInspectFn: func(ctx context.Context, idOrName string) (*podman.ContainerInspectState, error) {
				return &podman.ContainerInspectState{Status: "exited", Running: false, ExitCode: 1}, nil
			},
		}
		if err := d.awaitReplacementReady(context.Background(), "svc-crash", func(string) {}); err == nil {
			t.Fatal("expected exited replacement container to fail readiness gate")
		}
	})

	t.Run("no healthcheck passes after grace", func(t *testing.T) {
		d := &Deployer{
			readinessTimeout:      300 * time.Millisecond,
			readinessPollInterval: 5 * time.Millisecond,
			readinessGracePeriod:  15 * time.Millisecond,
			readinessInspectFn: func(ctx context.Context, idOrName string) (*podman.ContainerInspectState, error) {
				return &podman.ContainerInspectState{Status: "running", Running: true, HasHealth: false}, nil
			},
		}
		if err := d.awaitReplacementReady(context.Background(), "svc-plain", func(string) {}); err != nil {
			t.Fatalf("expected running container without healthcheck to pass after grace: %v", err)
		}
	})
}

