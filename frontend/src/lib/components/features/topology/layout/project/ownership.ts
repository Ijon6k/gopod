// ──────────────────────────────────────────────
// GOPOD Topology — Container Ownership (framework-free)
// Strict, evidence-based attribution. Explicit labels always win;
// bare names NEVER cross-match across services.
// ──────────────────────────────────────────────

import type { Service, Container, Pod } from '$lib/types';
import { podServiceNames } from '../../pod-naming';

export interface PodAssignment {
	kind: 'pod';
	podName: string;
	podServiceId: string | null;
}

export function isPodMemberService(svc: Pick<Service, 'podId' | 'type'>): boolean {
	return (
		!!svc.podId &&
		svc.type !== 'pod' &&
		svc.type !== 'compose' &&
		svc.type !== 'kubernetes' &&
		svc.type !== 'quadlet'
	);
}

export function isInfraContainer(c: Pick<Container, 'name' | 'image'>): boolean {
	return (
		c.name.endsWith('-infra') ||
		(c.image || '').includes('pause') ||
		(c.image || '').trim() === ''
	);
}

// Strict container -> service ownership. Returns null instead of guessing:
// - io.gopod.service label === service id (set by every GOPOD deploy path)
// - compose project label === compose service id (deployCompose uses -p <serviceId>)
// - unlabeled legacy: exact id or id-prefix only
// A compose service literally named `api` must NEVER capture another
// service's container via the compose *service* name.
export function ownerServiceId(
	c: Pick<Container, 'name' | 'labels'>,
	projServices: Pick<Service, 'id' | 'type'>[],
	projServiceIds: Set<string>
): string | null {
	const labels = c.labels || {};
	const gl = labels['io.gopod.service'];
	if (gl) {
		return projServiceIds.has(gl) ? gl : null;
	}
	const cp = labels['com.docker.compose.project'] || labels['io.podman.compose.project'];
	if (cp) {
		const owner = projServices.find((s) => s.id === cp);
		return owner ? owner.id : null;
	}
	for (const s of projServices) {
		if (!s.id) continue;
		if (c.name === s.id || c.name.startsWith(`${s.id}-`) || c.name.startsWith(`${s.id}_`)) {
			return s.id;
		}
	}
	return null;
}

// Runtime pod membership from the daemon (pods[].containers).
export function buildContainerPodMap(pods: Pick<Pod, 'name' | 'containers'>[]): Map<string, string> {
	const map = new Map<string, string>();
	for (const p of pods) {
		for (const cname of p.containers || []) {
			if (cname && !map.has(cname)) map.set(cname, p.name);
		}
	}
	return map;
}

export function runtimePodService(
	podName: string,
	projServices: Pick<Service, 'id' | 'name' | 'type' | 'podId'>[]
): Pick<Service, 'id' | 'name' | 'type' | 'podId'> | undefined {
	return projServices.find(
		(s) => (s.type === 'pod' || s.type === 'kubernetes') && podServiceNames(s).includes(podName)
	);
}
