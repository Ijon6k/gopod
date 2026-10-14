import type { Service } from '$lib/types';

export interface ComposeWorkloadItem {
	name: string;
	image: string;
	containerName: string;
	ports: string[];
	volumes: string[];
	environment: string[];
	restart: string;
	exec: string;
}

export function slugifyName(name: string): string {
	return name
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9_-]/g, '-')
		.replace(/-+/g, '-')
		.replace(/^-|-$/g, '');
}

export function parseAllComposeWorkloads(yamlStr: string): ComposeWorkloadItem[] {
	const items: ComposeWorkloadItem[] = [];
	if (!yamlStr) return items;

	const lines = yamlStr.split('\n');
	let inServices = false;
	let currentItem: ComposeWorkloadItem | null = null;
	let inPorts = false;
	let inVolumes = false;
	let inEnv = false;

	for (const line of lines) {
		const rawTrimmed = line.trim();
		if (!rawTrimmed || rawTrimmed.startsWith('#')) continue;

		if (line.match(/^services:\s*$/i)) {
			inServices = true;
			continue;
		}

		if (inServices) {
			if (line.match(/^[a-zA-Z0-9_-]+:\s*$/) && !line.startsWith(' ') && !line.startsWith('\t')) {
				inServices = false;
				if (currentItem) items.push(currentItem);
				currentItem = null;
				continue;
			}

			const svcMatch = line.match(/^(?: {2}|\t)([a-zA-Z0-9_-]+):\s*$/);
			if (svcMatch) {
				if (currentItem) items.push(currentItem);
				currentItem = {
					name: svcMatch[1],
					image: '',
					containerName: svcMatch[1],
					ports: [],
					volumes: [],
					environment: [],
					restart: 'always',
					exec: ''
				};
				inPorts = false;
				inVolumes = false;
				inEnv = false;
				continue;
			}

			if (currentItem) {
				const trimmed = rawTrimmed;
				if (trimmed.startsWith('image:')) {
					currentItem.image = trimmed
						.slice(6)
						.trim()
						.replace(/^['"]|['"]$/g, '');
				} else if (trimmed.startsWith('container_name:')) {
					currentItem.containerName = trimmed
						.slice(15)
						.trim()
						.replace(/^['"]|['"]$/g, '');
				} else if (trimmed.startsWith('command:')) {
					const rawCmd = trimmed.slice(8).trim();
					currentItem.exec = rawCmd
						.replace(/[[\]"',]/g, ' ')
						.replace(/\s+/g, ' ')
						.trim();
				} else if (trimmed.startsWith('restart:')) {
					const rawRestart = trimmed
						.slice(8)
						.trim()
						.replace(/^['"]|['"]$/g, '')
						.toLowerCase();
					if (rawRestart === 'unless-stopped' || rawRestart === 'always') {
						currentItem.restart = 'always';
					} else if (rawRestart === 'on-failure') {
						currentItem.restart = 'on-failure';
					} else if (rawRestart === 'no') {
						currentItem.restart = 'no';
					} else {
						currentItem.restart = 'always';
					}
				} else if (trimmed === 'ports:') {
					inPorts = true;
					inVolumes = false;
					inEnv = false;
				} else if (trimmed === 'volumes:') {
					inVolumes = true;
					inPorts = false;
					inEnv = false;
				} else if (trimmed === 'environment:') {
					inEnv = true;
					inPorts = false;
					inVolumes = false;
				} else if (inPorts && trimmed.startsWith('-')) {
					const p = trimmed
						.slice(1)
						.trim()
						.replace(/^['"]|['"]$/g, '');
					if (p) currentItem.ports.push(p);
				} else if (inVolumes && trimmed.startsWith('-')) {
					const v = trimmed
						.slice(1)
						.trim()
						.replace(/^['"]|['"]$/g, '');
					if (v) currentItem.volumes.push(v);
				} else if (inEnv && trimmed.startsWith('-')) {
					const e = trimmed
						.slice(1)
						.trim()
						.replace(/^['"]|['"]$/g, '');
					if (e) currentItem.environment.push(e);
				} else if (!trimmed.startsWith('-') && trimmed.includes(':')) {
					inPorts = false;
					inVolumes = false;
					inEnv = false;
				}
			}
		}
	}

	if (currentItem) items.push(currentItem);
	return items;
}

export function parseComposeWorkload(yamlStr: string): ComposeWorkloadItem | null {
	const items = parseAllComposeWorkloads(yamlStr);
	return items.length > 0 ? items[0] : null;
}

export function getSanitizedImage(svc: Service): string {
	if (svc.composeYaml) {
		const parsed = parseComposeWorkload(svc.composeYaml);
		if (parsed?.image && !parsed.image.includes(' ')) {
			return parsed.image;
		}
	}
	const raw = (svc.image || '').trim();
	if (
		raw &&
		!raw.includes(' ') &&
		!raw.endsWith('.yaml') &&
		!raw.endsWith('.yml') &&
		!raw.startsWith('.') &&
		!raw.startsWith('/')
	) {
		if (!raw.includes(':') && !raw.includes('/')) {
			return `${raw}:latest`;
		}
		return raw;
	}
	const clean = slugifyName(svc.name) || 'app';
	return `docker.io/library/${clean}:latest`;
}
