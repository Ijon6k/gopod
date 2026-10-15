// ──────────────────────────────────────────────
// GOPOD — Pod Membership Matching (framework-free, unit-tested)
// Single source of truth for "which services are members of this pod".
// Strict PodID matching only. Empty PodIDs NEVER match: '' === '' once
// claimed every standalone service for a fresh, never-deployed pod.
// ──────────────────────────────────────────────

import { slugLower } from '../../../topology/pod-naming';

export interface PodServiceLike {
	id: string;
	name: string;
	podId?: string;
	type?: string;
	projectId?: string;
}

export interface MemberCandidateLike {
	id: string;
	name?: string;
	podId?: string;
	type?: string;
	projectId?: string;
}

export function podRuntimeName(svc: Pick<PodServiceLike, 'id' | 'name' | 'podId'>): string {
	if (svc.podId && svc.podId.trim()) {
		const raw = svc.podId.trim();
		return raw.startsWith('pod_') ? raw : `pod_${slugLower(raw)}`;
	}
	if (svc.id && svc.id.trim()) {
		return `pod_${slugLower(svc.id)}`;
	}
	return `pod_${slugLower(svc.name || 'pod-service')}`;
}

/** True when the candidate is a member of the given pod service. */
export function isPodMember(
	podSvc: PodServiceLike,
	candidate: MemberCandidateLike,
	podName?: string
): boolean {
	if (!candidate.podId || !candidate.podId.trim()) return false;
	if (candidate.id === podSvc.id) return false;
	if (candidate.type === 'pod' || candidate.type === 'compose') return false;
	if (podSvc.projectId && candidate.projectId && candidate.projectId !== podSvc.projectId) {
		return false;
	}
	const expectedPod = `pod_${slugLower(podSvc.id || podSvc.name)}`;
	const resolvedPodName = podName || podRuntimeName(podSvc);
	const svcPodId = podSvc.podId?.trim() || '';
	const p = candidate.podId.trim();
	return (
		(svcPodId !== '' &&
			(p === podSvc.id ||
				p === svcPodId ||
				p === podSvc.name ||
				p === `pod_${podSvc.name}` ||
				p === expectedPod ||
				p === resolvedPodName ||
				`pod_${slugLower(p)}` === expectedPod ||
				`pod_${slugLower(p)}` === resolvedPodName)) ||
		// Legacy: member PodID stored as the pod's display name variants
		// while the pod service itself was never deployed (no podId yet).
		(svcPodId === '' &&
			(p === podSvc.id ||
				p === podSvc.name ||
				p === `pod_${podSvc.name}` ||
				p === expectedPod ||
				p === resolvedPodName ||
				`pod_${slugLower(p)}` === expectedPod ||
				`pod_${slugLower(p)}` === resolvedPodName))
	);
}

/** All member services of a pod service within a service list. */
export function findPodMembers<T extends MemberCandidateLike>(
	podSvc: PodServiceLike,
	services: T[],
	podName?: string
): T[] {
	return services.filter((s) => isPodMember(podSvc, s, podName));
}
