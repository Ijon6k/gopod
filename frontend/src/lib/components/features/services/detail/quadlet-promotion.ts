// Shared Quadlet promotion logic. The BACKEND preview endpoint
// (POST /api/services/{id}/quadlet/preview) is the single generator — this
// module only joins reviewed unit files into the persisted bundle text and
// preserves the workload's origin type for demote round-trips.

import type { Service } from '$lib/types';

/**
 * Join reviewed unit files into the deployed bundle convention:
 * `# File: <name>` comments joined with `---` (backend SplitQuadletUnits
 * format). Multi-unit bundles route each file through its header; a single
 * file stays plain.
 */
export function joinQuadletFiles(
	files: Record<string, string>,
	activeTab: string,
	promoCode: string
): string {
	if (serviceHasMultipleFiles(files)) {
		const merged: Record<string, string> = { ...files };
		if (activeTab in merged) merged[activeTab] = promoCode;
		const units = Object.entries(merged).map(
			([fname, content]) => `# File: ${fname}\n${content.trim()}`
		);
		return units.join('\n---\n');
	}
	return promoCode;
}

export function serviceHasMultipleFiles(files: Record<string, string>): boolean {
	return Object.keys(files).length > 1;
}

/**
 * Preserve the workload's origin for demote round-trips (the backend restores
 * type from sourceType when demoting back to direct podman supervision).
 */
export function inferQuadletSourceType(
	service: Service,
	originalType: Service['type']
): Service['sourceType'] {
	if (service.sourceType && service.sourceType !== 'git') {
		return service.sourceType;
	}
	if (originalType === 'compose' || (service.composeYaml && service.composeYaml.trim() !== '')) {
		return 'compose';
	}
	if (originalType === 'kubernetes' || (service.k8sYaml && service.k8sYaml.trim() !== '')) {
		return 'kubernetes';
	}
	if (originalType === 'pod') return 'pod';
	if (originalType === 'image') return 'image';
	return originalType;
}
