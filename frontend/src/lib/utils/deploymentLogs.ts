import type { Deployment, DeploymentStep } from '$lib/types';

export function getDeploymentSteps(dep: Deployment): DeploymentStep[] {
	if (dep.steps && dep.steps.length > 0) {
		return dep.steps;
	}

	const isFailed = dep.status === 'failed';
	const isDeploying = dep.status === 'deploying' || dep.status === 'building';
	const isCancelled = dep.status === 'cancelled';

	if (isCancelled) {
		return [
			{ id: 'step-1', name: 'Initialize build environment & checkout ref', status: 'success', duration: '0.4s' },
			{ id: 'step-2', name: `Pull container image (${dep.version})`, status: 'running', duration: '12.1s' },
			{ id: 'step-3', name: 'Quadlet generator & user namespace setup', status: 'pending' },
			{ id: 'step-4', name: 'Systemd user daemon-reload & unit activation', status: 'pending' },
			{ id: 'step-5', name: 'Container healthcheck probe', status: 'pending' }
		];
	}

	if (isFailed) {
		return [
			{ id: 'step-1', name: 'Initialize build environment & checkout ref', status: 'success', duration: '0.5s' },
			{ id: 'step-2', name: `Pull container image (${dep.version})`, status: 'success', duration: '28.4s' },
			{ id: 'step-3', name: 'Quadlet generator & user namespace setup', status: 'success', duration: '1.2s' },
			{ id: 'step-4', name: 'Systemd user daemon-reload & unit activation', status: 'success', duration: '2.8s' },
			{ id: 'step-5', name: 'Container healthcheck probe (http://127.0.0.1:port/health)', status: 'failed', duration: '45.0s' }
		];
	}

	if (isDeploying) {
		return [
			{ id: 'step-1', name: 'Initialize build environment & checkout ref', status: 'success', duration: '0.4s' },
			{ id: 'step-2', name: `Pull container image (${dep.version})`, status: 'running', duration: '14.8s' },
			{ id: 'step-3', name: 'Quadlet generator & user namespace setup', status: 'pending' },
			{ id: 'step-4', name: 'Systemd user daemon-reload & unit activation', status: 'pending' },
			{ id: 'step-5', name: 'Container healthcheck probe', status: 'pending' }
		];
	}

	return [
		{ id: 'step-1', name: 'Initialize build environment & checkout ref', status: 'success', duration: '0.4s' },
		{ id: 'step-2', name: `Pull container image (${dep.version})`, status: 'success', duration: '19.2s' },
		{ id: 'step-3', name: 'Quadlet generator & user namespace setup', status: 'success', duration: '1.1s' },
		{ id: 'step-4', name: 'Systemd user daemon-reload & unit activation', status: 'success', duration: '1.8s' },
		{ id: 'step-5', name: 'Container healthcheck probe (HTTP 200 OK)', status: 'success', duration: '2.5s' }
	];
}

export function getDeploymentLogs(dep: Deployment): string[] {
	if (dep.logs && dep.logs.length > 0) {
		return dep.logs;
	}

	const timeStr = dep.startedAt ? dep.startedAt.substring(11, 19) : '10:14:02';
	const sName = dep.serviceName || dep.serviceId;
	const commitShort = dep.commit || '4b89c02';
	const branch = dep.branch || 'main';
	const image = dep.image || `ghcr.io/joko/${sName}:${dep.version || 'latest'}`;

	if (dep.status === 'cancelled') {
		return [
			`[${timeStr}] [gopod-engine] Deployment #${dep.number} triggered for service '${sName}'`,
			`[${timeStr}] [gopod-engine] Strategy: Quadlet Rootless Podman Rolling Update`,
			`[${timeStr}] [git] Fetching repository origin/${branch}...`,
			`[${timeStr}] [git] Checked out ref: ${commitShort} (${dep.commitMessage})`,
			`[${timeStr}] [podman] Pulling image '${image}'...`,
			`[${timeStr}] [podman] Resolving manifest list...`,
			`[${timeStr}] [podman] Downloading blob sha256:8f41ab... (42.5MB / 112MB)`,
			`[${timeStr}] [gopod-engine] Cancellation signal SIGTERM received from operator.`,
			`[${timeStr}] [podman] Aborted container image pull.`,
			`[${timeStr}] [gopod-engine] Deployment #${dep.number} cancelled.`
		];
	}

	if (dep.status === 'failed') {
		return [
			`[${timeStr}] [gopod-engine] Deployment #${dep.number} triggered for service '${sName}'`,
			`[${timeStr}] [gopod-engine] Strategy: Quadlet Rootless Podman Rolling Update`,
			`[${timeStr}] [git] Fetching repository origin/${branch}...`,
			`[${timeStr}] [git] Checked out commit ${commitShort}: ${dep.commitMessage}`,
			`[${timeStr}] [podman] Pulling container image '${image}'...`,
			`[${timeStr}] [podman] Image is up to date for ${image}`,
			`[${timeStr}] [quadlet] Validating ~/.config/containers/systemd/${sName}.container`,
			`[${timeStr}] [quadlet] UserNS: keep-id | AutoUpdate: registry`,
			`[${timeStr}] [systemd] Executing: systemctl --user daemon-reload`,
			`[${timeStr}] [systemd] Executing: systemctl --user restart ${sName}.service`,
			`[${timeStr}] [systemd] Unit ${sName}.service entered active (running) state.`,
			`[${timeStr}] [healthcheck] Starting health probes on container port...`,
			`[${timeStr}] [healthcheck] Probe 1/5: Connection refused (127.0.0.1:3000)`,
			`[${timeStr}] [healthcheck] Probe 2/5: Connection refused (127.0.0.1:3000)`,
			`[${timeStr}] [healthcheck] Probe 3/5: HTTP 502 Bad Gateway`,
			`[${timeStr}] [healthcheck] Probe 4/5: HTTP 502 Bad Gateway`,
			`[${timeStr}] [healthcheck] Probe 5/5: Timeout waiting for healthy response`,
			`[${timeStr}] [error] Health check failed after 5 retries (45s).`,
			`[${timeStr}] [rollback] Initiating automatic rollback to previous stable unit.`,
			`[${timeStr}] [gopod-engine] Deployment #${dep.number} marked as FAILED.`
		];
	}

	if (dep.status === 'deploying' || dep.status === 'building') {
		return [
			`[${timeStr}] [gopod-engine] Deployment #${dep.number} triggered for service '${sName}'`,
			`[${timeStr}] [gopod-engine] Strategy: Quadlet Rootless Podman Rolling Update`,
			`[${timeStr}] [git] Fetching repository origin/${branch}...`,
			`[${timeStr}] [git] Checked out commit ${commitShort}: ${dep.commitMessage}`,
			`[${timeStr}] [podman] Pulling container image '${image}'...`,
			`[${timeStr}] [podman] Downloading blob sha256:7a4f9... [===>                ] 24%`,
			`[${timeStr}] [podman] Downloading blob sha256:12c0e... [========>           ] 52%`,
			`[${timeStr}] [podman] Extracting layers into rootless storage overlay...`
		];
	}

	return [
		`[${timeStr}] [gopod-engine] Deployment #${dep.number} triggered for service '${sName}'`,
		`[${timeStr}] [gopod-engine] Strategy: Quadlet Rootless Podman Rolling Update`,
		`[${timeStr}] [git] Fetching repository origin/${branch}...`,
		`[${timeStr}] [git] HEAD is now at ${commitShort} "${dep.commitMessage}"`,
		`[${timeStr}] [podman] Pulling container image '${image}'...`,
		`[${timeStr}] [podman] Copying blob sha256:69b4... done`,
		`[${timeStr}] [podman] Copying blob sha256:c18a... done`,
		`[${timeStr}] [podman] Writing manifest to image destination`,
		`[${timeStr}] [podman] Stored image: sha256:${commitShort}9012beef`,
		`[${timeStr}] [quadlet] Parsing Quadlet configuration ~/.config/containers/systemd/${sName}.container`,
		`[${timeStr}] [quadlet] Applying Cgroups v2 limits: CPU=2.0, Memory=512MB`,
		`[${timeStr}] [systemd] systemctl --user daemon-reload`,
		`[${timeStr}] [systemd] Reloaded /run/user/1000/systemd/generator/ units in 14ms`,
		`[${timeStr}] [systemd] systemctl --user restart ${sName}.service`,
		`[${timeStr}] [healthcheck] Waiting for container readiness probe...`,
		`[${timeStr}] [healthcheck] Probe 1/5: HTTP 200 OK (latency: 18ms)`,
		`[${timeStr}] [healthcheck] Healthcheck passed. Container is healthy.`,
		`[${timeStr}] [proxy] Routing updated in internal reverse proxy -> port 3000`,
		`[${timeStr}] [gopod-engine] Deployment #${dep.number} deployed successfully in ${dep.duration || '34s'}.`
	];
}
