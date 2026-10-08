// ──────────────────────────────────────────────
// GOPOD — Compose Workload Parser
// Pure utility to parse compose services and workload manifests
// ──────────────────────────────────────────────

import type { Workload, Status } from '$lib/types';

/**
 * Extracts individual workload definitions from a docker-compose.yml string.
 * Supports image, container_name, ports, and service definitions.
 */
export function parseComposeWorkloads(yaml: string, defaultStatus: Status = 'stopped'): Workload[] {
	if (!yaml || !yaml.trim()) return [];

	try {
		const lines = yaml.split('\n');
		const detected: Workload[] = [];
		let inServices = false;
		let currentName = '';
		let currentImg = '';
		let currentContainerName = '';

		for (const line of lines) {
			const trimmed = line.trim();
			if (!trimmed || trimmed.startsWith('#')) continue;

			if (trimmed.startsWith('services:')) {
				inServices = true;
				continue;
			}

			// If another root-level key starts (volumes:, networks:), exit services block
			if (inServices && !line.startsWith(' ') && !line.startsWith('\t') && trimmed.includes(':')) {
				if (!trimmed.startsWith('services:')) {
					inServices = false;
				}
			}

			if (inServices) {
				// Detect service declaration: e.g. "web:", "db_redis:"
				if (
					/^[a-zA-Z0-9_.-]+:$/.test(trimmed) &&
					!trimmed.startsWith('image:') &&
					!trimmed.startsWith('ports:') &&
					!trimmed.startsWith('volumes:') &&
					!trimmed.startsWith('environment:') &&
					!trimmed.startsWith('command:') &&
					!trimmed.startsWith('container_name:') &&
					!trimmed.startsWith('restart:') &&
					!trimmed.startsWith('build:') &&
					!trimmed.startsWith('depends_on:') &&
					!trimmed.startsWith('networks:')
				) {
					if (currentName) {
						detected.push({
							name: currentContainerName || currentName,
							image: currentImg || '—',
							status: defaultStatus
						});
					}
					currentName = trimmed.replace(':', '');
					currentImg = '';
					currentContainerName = '';
				} else if (trimmed.startsWith('image:')) {
					currentImg = trimmed.replace('image:', '').trim().replace(/['"]/g, '');
				} else if (trimmed.startsWith('container_name:')) {
					currentContainerName = trimmed.replace('container_name:', '').trim().replace(/['"]/g, '');
				}
			}
		}

		if (currentName) {
			detected.push({
				name: currentContainerName || currentName,
				image: currentImg || '—',
				status: defaultStatus
			});
		}

		return detected;
	} catch {
		return [];
	}
}
