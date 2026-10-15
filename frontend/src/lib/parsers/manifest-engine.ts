// ──────────────────────────────────────────────
// GOPOD — Declarative Manifest Validation & Parser Engine
// Robust, AST-based parser using js-yaml (zero regex fragility on scripts/heredocs)
// Single source of truth for K8s Pod and Compose stack inspection.
// ──────────────────────────────────────────────

import { load } from 'js-yaml';

export interface ParsedPort {
	containerPort: number;
	hostPort?: number;
	protocol: 'tcp' | 'udp';
	hostIp?: string;
	raw: string;
}

export interface ParsedWorkload {
	name: string;
	image: string;
	role: 'main' | 'sidecar' | 'init' | 'service';
	ports: ParsedPort[];
	volumeMounts?: string[];
	memory?: string;
	cpu?: string;
	command?: string;
}

export interface ManifestParseResult {
	valid: boolean;
	type: 'kubernetes' | 'compose';
	podName?: string;
	workloads: ParsedWorkload[];
	volumes: string[];
	networks?: string[];
	errors: string[];
	warnings: string[];
}

// Known ports reserved by GOPOD host infrastructure
const RESERVED_HOST_PORTS: Record<number, string> = {
	8080: 'GOPOD Caddy Ingress (HTTP) — use 8081+ or route via domain',
	8443: 'GOPOD Caddy Ingress (HTTPS)',
	8085: 'GOPOD Core API backend',
	2019: 'Caddy Admin API'
};

/**
 * Checks a host port against known reserved ports.
 */
export function checkReservedHostPort(port: number): string | null {
	if (RESERVED_HOST_PORTS[port]) {
		return `Host port ${port} is reserved by ${RESERVED_HOST_PORTS[port]}`;
	}
	return null;
}

/**
 * Parses and validates a Kubernetes Pod YAML manifest.
 */
export function parseKubernetesPodManifest(yamlStr: string): ManifestParseResult {
	const res: ManifestParseResult = {
		valid: false,
		type: 'kubernetes',
		workloads: [],
		volumes: [],
		errors: [],
		warnings: []
	};

	const trimmed = (yamlStr || '').trim();
	if (!trimmed) {
		res.errors.push('Manifest is empty');
		return res;
	}

	let doc: any;
	try {
		doc = load(trimmed);
	} catch (err: any) {
		const line = err?.mark?.line !== undefined ? ` (line ${err.mark.line + 1})` : '';
		res.errors.push(`YAML syntax error${line}: ${err?.reason || err?.message || 'Invalid syntax'}`);
		return res;
	}

	if (!doc || typeof doc !== 'object') {
		res.errors.push('Manifest root must be an object');
		return res;
	}

	if (doc.kind && doc.kind !== 'Pod') {
		res.warnings.push(`Resource kind '${doc.kind}' specified; Podman native play kube operates on 'Pod'`);
	}

	res.podName = doc.metadata?.name || 'pod';

	// Volumes
	if (Array.isArray(doc.spec?.volumes)) {
		for (const v of doc.spec.volumes) {
			if (v && typeof v.name === 'string') {
				res.volumes.push(v.name);
			}
		}
	}

	// Init containers
	if (Array.isArray(doc.spec?.initContainers)) {
		doc.spec.initContainers.forEach((ic: any, idx: number) => {
			const name = ic?.name || `init-${idx + 1}`;
			const image = ic?.image || '';
			if (!image) {
				res.errors.push(`Init container '${name}' is missing an image`);
			}
			res.workloads.push({
				name,
				image,
				role: 'init',
				ports: [],
				memory: ic?.resources?.limits?.memory,
				cpu: ic?.resources?.limits?.cpu
			});
		});
	}

	// Containers
	const containers = doc.spec?.containers;
	if (!Array.isArray(containers) || containers.length === 0) {
		res.errors.push('No containers declared under spec.containers');
		return res;
	}

	containers.forEach((c: any, idx: number) => {
		const name = c?.name || `container-${idx + 1}`;
		const image = c?.image || '';
		if (!image) {
			res.errors.push(`Container '${name}' is missing an image`);
		}

		const ports: ParsedPort[] = [];
		if (Array.isArray(c?.ports)) {
			for (const p of c.ports) {
				const cPort = Number(p?.containerPort);
				if (!cPort || isNaN(cPort) || cPort <= 0 || cPort > 65535) {
					res.errors.push(`Container '${name}' has invalid containerPort (${p?.containerPort})`);
					continue;
				}
				const hPort = p?.hostPort ? Number(p.hostPort) : undefined;
				const proto = String(p?.protocol || 'TCP').toLowerCase() === 'udp' ? 'udp' : 'tcp';
				const hostIp = p?.hostIP || undefined;

				const rawStr = hPort ? `${hPort}:${cPort}${proto === 'udp' ? '/udp' : ''}` : `${cPort}`;
				ports.push({
					containerPort: cPort,
					hostPort: hPort,
					protocol: proto,
					hostIp,
					raw: rawStr
				});

				if (hPort) {
					const collision = checkReservedHostPort(hPort);
					if (collision) {
						res.warnings.push(`Container '${name}': ${collision}`);
					}
				}
			}
		}

		const volumeMounts: string[] = [];
		if (Array.isArray(c?.volumeMounts)) {
			for (const vm of c.volumeMounts) {
				if (vm?.name && vm?.mountPath) {
					volumeMounts.push(`${vm.name} -> ${vm.mountPath}`);
				}
			}
		}

		res.workloads.push({
			name,
			image,
			role: idx === 0 ? 'main' : 'sidecar',
			ports,
			volumeMounts,
			memory: c?.resources?.limits?.memory,
			cpu: c?.resources?.limits?.cpu
		});
	});

	res.valid = res.errors.length === 0;
	return res;
}

/**
 * Parses and validates a Docker Compose YAML manifest.
 */
export function parseComposeManifest(yamlStr: string): ManifestParseResult {
	const res: ManifestParseResult = {
		valid: false,
		type: 'compose',
		workloads: [],
		volumes: [],
		networks: [],
		errors: [],
		warnings: []
	};

	const trimmed = (yamlStr || '').trim();
	if (!trimmed) {
		res.errors.push('Compose YAML is empty');
		return res;
	}

	let doc: any;
	try {
		doc = load(trimmed);
	} catch (err: any) {
		const line = err?.mark?.line !== undefined ? ` (line ${err.mark.line + 1})` : '';
		res.errors.push(`YAML syntax error${line}: ${err?.reason || err?.message || 'Invalid syntax'}`);
		return res;
	}

	if (!doc || typeof doc !== 'object') {
		res.errors.push('Compose root must be an object');
		return res;
	}

	const services = doc.services;
	if (!services || typeof services !== 'object' || Object.keys(services).length === 0) {
		res.errors.push("No services declared under 'services:' section");
		return res;
	}

	if (doc.volumes && typeof doc.volumes === 'object') {
		res.volumes = Object.keys(doc.volumes);
	}
	if (doc.networks && typeof doc.networks === 'object') {
		res.networks = Object.keys(doc.networks);
	}

	for (const [sName, sDef] of Object.entries<any>(services)) {
		if (!sDef || typeof sDef !== 'object') continue;

		const image = sDef.image || (sDef.build ? '(local build)' : '');
		if (!image) {
			res.warnings.push(`Service '${sName}' has neither 'image' nor 'build' specified`);
		}

		const ports: ParsedPort[] = [];
		if (Array.isArray(sDef.ports)) {
			for (const p of sDef.ports) {
				const pStr = String(p).trim();
				const m = pStr.match(/^(?:([0-9.]+):)?([0-9]+):([0-9]+)(?:\/(tcp|udp))?$/i);
				if (m) {
					const hostIp = m[1];
					const hPort = Number(m[2]);
					const cPort = Number(m[3]);
					const proto = (m[4] || 'tcp').toLowerCase() === 'udp' ? 'udp' : 'tcp';
					ports.push({
						containerPort: cPort,
						hostPort: hPort,
						protocol: proto,
						hostIp,
						raw: pStr
					});

					const collision = checkReservedHostPort(hPort);
					if (collision) {
						res.warnings.push(`Service '${sName}': ${collision}`);
					}
				} else {
					ports.push({
						containerPort: 0,
						protocol: 'tcp',
						raw: pStr
					});
				}
			}
		}

		res.workloads.push({
			name: sName,
			image,
			role: 'service',
			ports
		});
	}

	res.valid = res.errors.length === 0;
	return res;
}
