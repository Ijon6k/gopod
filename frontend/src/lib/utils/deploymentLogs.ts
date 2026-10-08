import type { Deployment, DeploymentStep } from '$lib/types';

export function getDeploymentSteps(dep: Deployment): DeploymentStep[] {
	if (dep.steps && dep.steps.length > 0) {
		return dep.steps;
	}

	const isFailed = dep.status === 'failed';
	const isBuilding = dep.status === 'deploying' || dep.status === 'building';
	const isCancelled = dep.status === 'cancelled';
	const trigger = dep.trigger || 'manual';

	if (trigger === 'compose') {
		return [
			{ id: 'step-1', name: 'Initialize workspace & compose.yaml', status: 'success' },
			{
				id: 'step-2',
				name: 'Podman compose orchestration (up -d)',
				status: isBuilding ? 'running' : isFailed ? 'failed' : 'success',
				duration: !isBuilding ? dep.duration : undefined
			},
			{
				id: 'step-3',
				name: 'Container health probe & network proxy',
				status: isBuilding ? 'pending' : isFailed ? 'failed' : 'success'
			}
		];
	}

	if (dep.commit && dep.commit !== '—') {
		return [
			{ id: 'step-1', name: 'Checkout git repository ref', status: 'success' },
			{
				id: 'step-2',
				name: 'Build rootless container image (podman build)',
				status: isBuilding ? 'running' : isFailed ? 'failed' : 'success',
				duration: !isBuilding ? dep.duration : undefined
			},
			{
				id: 'step-3',
				name: 'Unit activation & readiness check',
				status: isBuilding ? 'pending' : isFailed ? 'failed' : 'success'
			}
		];
	}

	// Default / Container Image rollout
	return [
		{ id: 'step-1', name: 'Validate target runtime & manifest', status: 'success' },
		{
			id: 'step-2',
			name: `Reconcile container image (${dep.version || 'latest'})`,
			status: isBuilding ? 'running' : isFailed ? 'failed' : 'success',
			duration: !isBuilding ? dep.duration : undefined
		},
		{
			id: 'step-3',
			name: 'Service healthcheck & status check',
			status: isBuilding ? 'pending' : isFailed ? 'failed' : 'success'
		}
	];
}

export function getDeploymentLogs(dep: Deployment): string[] {
	if (dep.logs && dep.logs.length > 0) {
		return dep.logs;
	}

	const timeStr = dep.startedAt ? dep.startedAt.substring(11, 19) : new Date().toTimeString().substring(0, 8);
	const sName = dep.serviceName || dep.serviceId;

	if (dep.status === 'cancelled') {
		return [
			`[${timeStr}] [gopod-engine] Deployment #${dep.number} cancelled by operator for '${sName}'.`
		];
	}

	if (dep.status === 'failed') {
		return [
			`[${timeStr}] [gopod-engine] Deployment #${dep.number} triggered for service '${sName}'`,
			`[${timeStr}] [gopod-engine] Execution failed. Review podman runtime logs for details.`
		];
	}

	if (dep.status === 'deploying' || dep.status === 'building') {
		return [
			`[${timeStr}] [gopod-engine] Deployment #${dep.number} running for service '${sName}'...`,
			`[${timeStr}] [gopod-engine] Streaming output from backend runtime...`
		];
	}

	return [
		`[${timeStr}] [gopod-engine] Deployment #${dep.number} completed for '${sName}' (duration: ${dep.duration || '0s'}).`
	];
}
