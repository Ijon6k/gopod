// ──────────────────────────────────────────────
// GOPOD — Compose Workload Parser
// Pure utility to parse compose services, workload manifests, and network ports
// ──────────────────────────────────────────────

import type { Workload, Status } from '$lib/types';

export interface ComposeServiceDetail {
	name: string; // The service name in compose, e.g. "archlinux" or "uptime-kuma"
	containerName?: string; // Explicit container_name, e.g. "arch-sandbox"
	image?: string;
	ports: number[]; // Extracted container ports, e.g. [8000]
}

/**
 * Extracts numeric container/target port from a port mapping string.
 * Handles "8000:8000", "0.0.0.0:8080:80", "3000", "3001/tcp", or "target: 80".
 */
export function extractPortNumber(val: string): number | null {
	const clean = val.replace(/['"]/g, '').trim();
	const targetMatch = clean.match(/target:\s*(\d+)/i);
	if (targetMatch) {
		const p = parseInt(targetMatch[1], 10);
		return isNaN(p) ? null : p;
	}
	const withoutProto = clean.replace(/\/(tcp|udp)$/i, '');
	const parts = withoutProto.split(':');
	const lastPart = parts[parts.length - 1]?.trim();
	if (lastPart) {
		const p = parseInt(lastPart, 10);
		return isNaN(p) ? null : p;
	}
	return null;
}

/**
 * Extracts detailed service definitions and internal ports from a docker-compose.yml string.
 */
export function parseComposeServices(yaml: string): ComposeServiceDetail[] {
	if (!yaml || !yaml.trim()) return [];

	try {
		const lines = yaml.split('\n');
		const detected: ComposeServiceDetail[] = [];
		let inServices = false;
		let inPorts = false;
		let serviceIndent = -1;
		let currentName = '';
		let currentImg = '';
		let currentContainerName = '';
		let currentPorts: number[] = [];

		const flushService = () => {
			if (currentName) {
				detected.push({
					name: currentName,
					containerName: currentContainerName || undefined,
					image: currentImg || undefined,
					ports: Array.from(new Set(currentPorts))
				});
			}
		};

		for (const line of lines) {
			const trimmed = line.trim();
			if (!trimmed || trimmed.startsWith('#')) continue;

			const leadingSpaces = line.search(/\S/);

			// Top-level key (0 indentation)
			if (leadingSpaces === 0) {
				if (trimmed.startsWith('services:')) {
					flushService();
					currentName = '';
					inServices = true;
					inPorts = false;
					serviceIndent = -1;
					continue;
				} else {
					// Left services section (e.g. volumes:, networks:, version:)
					flushService();
					currentName = '';
					inServices = false;
					inPorts = false;
					continue;
				}
			}

			if (!inServices) continue;

			// Determine serviceIndent on first child key under services:
			if (serviceIndent === -1) {
				serviceIndent = leadingSpaces;
			}

			// Service declaration line: must be at exactly serviceIndent and end with ':'
			const isServiceDirective =
				trimmed.startsWith('image:') ||
				trimmed.startsWith('ports:') ||
				trimmed.startsWith('volumes:') ||
				trimmed.startsWith('environment:') ||
				trimmed.startsWith('command:') ||
				trimmed.startsWith('container_name:') ||
				trimmed.startsWith('restart:') ||
				trimmed.startsWith('stdin_open:') ||
				trimmed.startsWith('tty:') ||
				trimmed.startsWith('build:') ||
				trimmed.startsWith('depends_on:') ||
				trimmed.startsWith('networks:') ||
				trimmed.startsWith('labels:') ||
				trimmed.startsWith('healthcheck:') ||
				trimmed.startsWith('deploy:') ||
				trimmed.startsWith('entrypoint:');

			if (leadingSpaces === serviceIndent && /^[a-zA-Z0-9_.-]+:\s*$/.test(trimmed) && !isServiceDirective) {
				flushService();
				currentName = trimmed.replace(/:\s*$/, '').trim();
				currentImg = '';
				currentContainerName = '';
				currentPorts = [];
				inPorts = false;
				continue;
			}

			// Directives inside a service (leadingSpaces > serviceIndent)
			if (leadingSpaces > serviceIndent && currentName) {
				if (trimmed.startsWith('ports:')) {
					inPorts = true;
				} else if (inPorts && trimmed.startsWith('-')) {
					const portStr = trimmed.replace(/^-\s*/, '');
					const p = extractPortNumber(portStr);
					if (p) currentPorts.push(p);
				} else if (trimmed.startsWith('image:')) {
					inPorts = false;
					currentImg = trimmed.replace('image:', '').trim().replace(/['"]/g, '');
				} else if (trimmed.startsWith('container_name:')) {
					inPorts = false;
					currentContainerName = trimmed.replace('container_name:', '').trim().replace(/['"]/g, '');
				} else if (trimmed.includes(':') && !trimmed.startsWith('-')) {
					inPorts = false;
				}
			}
		}

		flushService();
		return detected;
	} catch {
		return [];
	}
}

/**
 * Extracts individual workload definitions from a docker-compose.yml string.
 * Supports image, container_name, ports, and service definitions.
 */
export function parseComposeWorkloads(yaml: string, defaultStatus: Status = 'stopped'): Workload[] {
	if (!yaml || !yaml.trim()) return [];

	const services = parseComposeServices(yaml);
	return services.map((s) => ({
		name: s.containerName || s.name,
		image: s.image || '—',
		status: defaultStatus
	}));
}
