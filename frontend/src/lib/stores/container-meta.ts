// ──────────────────────────────────────────────
// GOPOD — Container metadata resolution (pure)
// Resolves a live container to its owning project/service via GOPOD labels,
// compose labels, podman systemd units, and name prefixes. Pure functions:
// the DataStore passes its current snapshot as context, so live stat ticks
// resolve against fresh data without coupling this module to any store.
// ──────────────────────────────────────────────

import type { Container, PortMapping, Project, Service } from '$lib/types';

export interface ContainerMetaContext {
	services: Service[];
	projects: Project[];
	getProjectServices(projectId: string): Service[];
}

export function resolveContainerMetadata(
	name: string,
	labels: Record<string, string> | undefined,
	ctx: ContainerMetaContext
): { projId: string; projName: string; servId: string; servName: string } {
	let projName = 'System Host';
	let projId = 'system';
	let servName = name;
	let servId = name;

	// 1. Explicit GOPOD metadata labels
	if (labels && labels['io.gopod.project']) {
		projId = labels['io.gopod.project'];
		servId = labels['io.gopod.service'] || name;
		servName = labels['io.gopod.name'] || name;
		projName = ctx.projects.find((p) => p.id === projId)?.name || projId;
		return { projId, projName, servId, servName };
	}

	// 2. Compose Project labels (Docker & Podman Compose parity)
	const composeProj =
		labels?.['com.docker.compose.project'] || labels?.['io.podman.compose.project'];
	const composeServ =
		labels?.['com.docker.compose.service'] || labels?.['io.podman.compose.service'];

	if (composeProj) {
		const matched = ctx.services.find((s: Service) => {
			if (!s) return false;
			if (s.id === composeProj || s.name === composeProj) return true;
			if (composeProj === `${s.projectId}-${s.name}` || composeProj === `${s.projectId}-${s.id}`)
				return true;
			if (s.id && (composeProj.startsWith(`${s.id}-`) || composeProj.startsWith(`${s.id}_`)))
				return true;
			if (s.projectId && composeProj.startsWith(`${s.projectId}-`)) {
				const rest = composeProj.slice(s.projectId.length + 1);
				return (
					rest === s.id ||
					rest === s.name ||
					rest.startsWith(`${s.id}-`) ||
					rest.startsWith(`${s.name}-`)
				);
			}
			return false;
		});
		if (matched) {
			servId = matched.id;
			servName = composeServ ? `${matched.name} (${composeServ})` : matched.name;
			projId = matched.projectId;
			projName = ctx.projects.find((p) => p.id === projId)?.name || projId;
			return { projId, projName, servId, servName };
		}
	}

	// 3. Compose working directory or config file paths
	const composeWorkDir =
		labels?.['com.docker.compose.project.working_dir'] ||
		labels?.['com.docker.compose.project.config_files'];
	if (composeWorkDir) {
		for (const p of ctx.projects) {
			if (composeWorkDir.includes(`/${p.id}/`) || composeWorkDir.includes(`/${p.name}/`)) {
				projId = p.id;
				projName = p.name;
				const pServices = ctx.getProjectServices(p.id);
				for (const s of pServices) {
					if (
						composeWorkDir.includes(`/${s.id}/`) ||
						composeWorkDir.endsWith(`/${s.id}`) ||
						composeWorkDir.includes(`/${s.id}.`) ||
						composeWorkDir.includes(`/${s.name}/`) ||
						composeWorkDir.endsWith(`/${s.name}`) ||
						(composeProj &&
							(s.id === composeProj ||
								s.name === composeProj ||
								composeProj === `${s.projectId}-${s.name}`))
					) {
						servId = s.id;
						servName = composeServ ? `${s.name} (${composeServ})` : s.name;
						return { projId, projName, servId, servName };
					}
				}
				break;
			}
		}
	}

	// 4. Podman systemd unit label (e.g. podman-compose@self-hosted-redis.service)
	const systemdUnit = labels?.['PODMAN_SYSTEMD_UNIT'];
	if (systemdUnit) {
		const unitMatch = systemdUnit.match(/podman-compose@([^.]+)\.service/);
		if (unitMatch) {
			const unitName = unitMatch[1];
			const matched = ctx.services.find((s) => {
				if (s.id === unitName || s.name === unitName) return true;
				if (unitName === `${s.projectId}-${s.name}` || unitName === `${s.projectId}-${s.id}`)
					return true;
				if (s.projectId && unitName.startsWith(`${s.projectId}-`)) {
					const rest = unitName.slice(s.projectId.length + 1);
					return (
						rest === s.id ||
						rest === s.name ||
						rest.startsWith(`${s.id}-`) ||
						rest.startsWith(`${s.name}-`)
					);
				}
				return false;
			});
			if (matched) {
				servId = matched.id;
				servName = composeServ || matched.name;
				projId = matched.projectId;
				projName = ctx.projects.find((p) => p.id === projId)?.name || projId;
				return { projId, projName, servId, servName };
			}
		}
	}

	// 5. Container name prefixes / exact matching with services
	const matchedByPrefix = ctx.services.find(
		(s: Service) =>
			(s.id && (name === s.id || name.startsWith(`${s.id}-`) || name.startsWith(`${s.id}_`))) ||
			(s.name &&
				(name === s.name || name.startsWith(`${s.name}-`) || name.startsWith(`${s.name}_`)))
	);
	if (matchedByPrefix) {
		servId = matchedByPrefix.id;
		servName = matchedByPrefix.name;
		projId = matchedByPrefix.projectId;
		projName = ctx.projects.find((p) => p.id === projId)?.name || projId;
		return { projId, projName, servId, servName };
	}

	// 6. Project name prefix in container name (e.g., self-hosted-redis-...)
	const parts = name.split(/[-_]/);
	if (parts.length >= 2) {
		const potentialProj = ctx.projects.find(
			(p) => p.id === parts[0] || p.name.toLowerCase() === parts[0]
		);
		if (potentialProj) {
			projId = potentialProj.id;
			projName = potentialProj.name;
			const matchedServInProj = ctx
				.getProjectServices(potentialProj.id)
				.find((s) => name.includes(s.name) || name.includes(s.id));
			if (matchedServInProj) {
				servId = matchedServInProj.id;
				servName = matchedServInProj.name;
			} else {
				servName = parts.slice(1).join('-');
				servId = `${projId}-${servName}`;
			}
		}
	}

	return { projId, projName, servId, servName };
}

/**
 * Resolve a named Podman volume to its owning project/service using the same
 * ownership conventions as containers. Named volumes created by GoPod deploys
 * carry io.gopod.* labels; compose deploys run with the service id as the
 * compose project name, which makes the engine name volumes
 * `<serviceId>_<volume>` (podman-compose) or `<serviceId>-<volume>`.
 * Anything else stays in the global "Host Storage" bucket: ownership cannot
 * be proven for unlabeled volumes created by manual podman commands.
 */
export function resolveVolumeMetadata(
	name: string,
	labels: Record<string, string> | undefined,
	ctx: ContainerMetaContext
): { projId: string; projName: string; servId: string; servName: string } {
	let projName = 'Host Storage';
	let projId = 'system';
	let servName = 'volume';
	let servId = 'volume';

	// 1. Explicit GOPOD volume labels (attached before first use at deploy).
	if (labels && labels['io.gopod.project']) {
		projId = labels['io.gopod.project'];
		projName = ctx.projects.find((p) => p.id === projId)?.name || projId;
		servId = labels['io.gopod.service'] || servId;
		servName = ctx.services.find((s) => s.id === servId)?.name || servId;
		return { projId, projName, servId, servName };
	}

	// 2. Compose naming convention: the compose project name IS the service id,
	// so the engine prefixes every created volume with it. Match the service id
	// followed by the separator the tooling used (never a bare prefix).
	const matchByService = ctx.services.find((s) => {
		if (!s.id) return false;
		return name === s.id || name.startsWith(`${s.id}_`) || name.startsWith(`${s.id}-`);
	});
	if (matchByService) {
		servId = matchByService.id;
		servName = matchByService.name;
		projId = matchByService.projectId;
		projName = ctx.projects.find((p) => p.id === projId)?.name || projId;
		return { projId, projName, servId, servName };
	}

	return { projId, projName, servId, servName };
}

export function parseContainerPortMappings(containers: Container[]): PortMapping[] {
	const result: PortMapping[] = [];
	for (const c of containers) {
		if (!c.ports || c.ports === '—') continue;
		const portItems = c.ports
			.split(/[\n,]+/)
			.map((s) => s.trim())
			.filter(Boolean);
		for (const item of portItems) {
			const arrowMatch = item.match(
				/^(?:([0-9a-fA-F.:*]+):)?(\d+)\s*(?:->|:)\s*(\d+)(?:\/(\w+))?$/
			);
			if (arrowMatch) {
				const bindIp = arrowMatch[1] || '0.0.0.0';
				const hostPort = parseInt(arrowMatch[2], 10);
				const containerPort = parseInt(arrowMatch[3], 10);
				const proto = (arrowMatch[4] || 'tcp').toLowerCase();
				const isPublic =
					bindIp === '0.0.0.0' || bindIp === '::' || bindIp === '*' || bindIp === '[::]';
				result.push({
					id: `${c.id}-${hostPort}-${proto}`,
					hostPort,
					containerPort,
					protocol: proto,
					serviceId: c.serviceId || c.id,
					serviceName: c.serviceName || c.projectName || c.name,
					containerName: c.name,
					bindAddress: bindIp,
					isPublic,
					status: c.status === 'running' ? 'listening' : 'idle'
				});
			} else {
				const exposeMatch = item.match(/^(\d+)(?:\/(\w+))?$/);
				if (exposeMatch) {
					const containerPort = parseInt(exposeMatch[1], 10);
					const proto = (exposeMatch[2] || 'tcp').toLowerCase();
					result.push({
						id: `${c.id}-${containerPort}-${proto}`,
						hostPort: containerPort,
						containerPort: containerPort,
						protocol: proto,
						serviceId: c.serviceId || c.id,
						serviceName: c.serviceName || c.projectName || c.name,
						containerName: c.name,
						bindAddress: 'Internal network',
						isPublic: false,
						status: c.status === 'running' ? 'listening' : 'idle'
					});
				}
			}
		}
	}
	return result;
}

/**
 * Extract a machine-readable health signal from the podman human status
 * string ("Up 2 minutes (unhealthy)"). Returns '' when the container engine
 * supplies no health info — no fabricated states.
 */
export function extractContainerHealth(status: string, state = ''): Container['health'] {
	const s = (status || '').toLowerCase();
	if (s.startsWith('restarting') || /restarting/.test(s)) return 'restarting';
	if (state === 'restarting') return 'restarting';
	const m = /\((unhealthy|healthy|starting)\)/.exec(s);
	return (m?.[1] as Container['health']) ?? '';
}
