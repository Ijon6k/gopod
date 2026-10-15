package ingress

import (
	"context"
	"fmt"

	"gopod/internal/activity"
)

// emitEngineEvent is a best-effort forwarder that never fails reconciliation.
func emitEngineEvent(ctx context.Context, level, scope, serviceID, projectID, actor, action, message string, data any) {
	defer func() { _ = recover() }()
	activity.EmitEngineEvent(ctx, level, scope, serviceID, projectID, actor, action, message, data)
}

// emitIngressReconcileEvent records a best-effort reconcile summary.
func emitIngressReconcileEvent(ctx context.Context, domainCount int, reloaded bool) {
	defer func() { _ = recover() }()
	emitEngineEvent(ctx, "info", "system", "", "", "engine", "ingress.reconcile",
		fmt.Sprintf("Ingress reconciled: %d domain(s), reloaded=%v", domainCount, reloaded),
		map[string]any{"domains": domainCount, "reloaded": reloaded})
}
