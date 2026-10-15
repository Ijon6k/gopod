// ──────────────────────────────────────────────
// GOPOD — Pod Port Parsers & Serializers
// Manages declarative YAML synchronization between visual UI and Compose/Kube specs
// ──────────────────────────────────────────────

export type PortHostIp = '0.0.0.0' | '127.0.0.1';

export const DEFAULT_PORT_HOST_IP: PortHostIp = '0.0.0.0';

export interface PortBindingItem {
	id: string;
	hostPort: number;
	podPort: number;
	protocol: 'tcp' | 'udp';
	hostIp: PortHostIp;
	label?: string;
}

export function normalizePortHostIp(raw: unknown): PortHostIp {
	return raw === '127.0.0.1' ? '127.0.0.1' : '0.0.0.0';
}

export function portBindingId(
	hostPort: number,
	podPort: number,
	protocol: 'tcp' | 'udp',
	hostIp: PortHostIp = DEFAULT_PORT_HOST_IP
): string {
	return `port-${normalizePortHostIp(hostIp)}-${hostPort}-${podPort}-${protocol}`;
}

function parsePortToken(raw: string): Omit<PortBindingItem, 'id'> | null {
	const val = raw.replace(/['"]/g, '').trim();
	if (!val) return null;
	const protoMatch = val.match(/\/(tcp|udp)$/i);
	const protocol: 'tcp' | 'udp' =
		protoMatch && protoMatch[1].toLowerCase() === 'udp' ? 'udp' : 'tcp';
	const clean = val.replace(/\/(tcp|udp)$/i, '');
	const parts = clean.split(':').map((p) => p.trim());
	let hostIp: PortHostIp = DEFAULT_PORT_HOST_IP;
	let hostPort = 0;
	let podPort = 0;
	if (parts.length === 1) {
		const p = parseInt(parts[0], 10);
		hostPort = p;
		podPort = p;
	} else if (parts.length === 2) {
		hostPort = parseInt(parts[0], 10);
		podPort = parseInt(parts[1], 10);
	} else if (parts.length === 3 && parts[0].includes('.')) {
		// ip:host:container — per-binding host IP (0.0.0.0 vs 127.0.0.1);
		// any other IP normalizes to the 0.0.0.0 default.
		hostIp = normalizePortHostIp(parts[0]);
		hostPort = parseInt(parts[1], 10);
		podPort = parseInt(parts[2], 10);
	} else if (parts.length >= 3) {
		// legacy "IP:h:c" — host IP is owned by infra, keep last two segments
		hostPort = parseInt(parts[parts.length - 2], 10);
		podPort = parseInt(parts[parts.length - 1], 10);
	}
	if (!hostPort || !podPort || hostPort < 1 || hostPort > 65535 || podPort < 1 || podPort > 65535) {
		return null;
	}
	return { hostPort, podPort, protocol, hostIp };
}

/**
 * Extracts all host:container port bindings from a compose.yaml string.
 * Indent-aware: only reads ports: blocks nested inside services.<name>.
 */
export function parseDetailedPorts(yaml: string): PortBindingItem[] {
	if (!yaml || !yaml.trim()) return [];
	const items: PortBindingItem[] = [];
	const seen = new Set<string>();
	const lines = yaml.split('\n');
	let inServices = false;
	let serviceIndent = -1;
	let portsIndent = -1;

	for (const line of lines) {
		if (!line.trim() || line.trim().startsWith('#')) continue;
		const indent = line.search(/\S/);
		const trimmed = line.trim();

		if (indent === 0) {
			inServices = trimmed.startsWith('services:');
			serviceIndent = -1;
			portsIndent = -1;
			continue;
		}
		if (!inServices) continue;

		if (serviceIndent === -1 && indent > 0 && /^[A-Za-z0-9_.-]+:\s*$/.test(trimmed)) {
			serviceIndent = indent;
			portsIndent = -1;
			continue;
		}
		if (serviceIndent !== -1 && indent <= serviceIndent && /^[A-Za-z0-9_.-]+:\s*$/.test(trimmed)) {
			portsIndent = -1;
			continue;
		}

		if (serviceIndent !== -1 && indent > serviceIndent && trimmed.startsWith('ports:')) {
			portsIndent = indent;
			continue;
		}

		if (portsIndent !== -1) {
			if (trimmed.startsWith('-')) {
				const parsed = parsePortToken(trimmed.replace(/^-\s*/, ''));
				if (parsed) {
					const id = portBindingId(
						parsed.hostPort,
						parsed.podPort,
						parsed.protocol,
						parsed.hostIp
					);
					if (!seen.has(id)) {
						seen.add(id);
						items.push({ id, ...parsed });
					}
				}
				continue;
			}
			// A mapping key at ports level or shallower ends the ports block
			if (trimmed.includes(':') && indent <= portsIndent) {
				portsIndent = -1;
			}
		}
	}
	return items;
}

/**
 * Updates or generates compose.yaml for a Pod, preserving existing member services while syncing ports.
 */
export function serializePodComposeYaml(
	serviceName: string,
	ports: PortBindingItem[],
	existingYaml?: string
): string {
	const cleanName = serviceName.trim().replace(/^pod_/, '') || 'pod-service';
	// Default bind (0.0.0.0) serializes as short "host:container"; only a
	// loopback-only binding carries the explicit IP prefix.
	const portLines = ports.map((p) => {
		const ip = normalizePortHostIp(p.hostIp);
		const prefix = ip === DEFAULT_PORT_HOST_IP ? '' : `${ip}:`;
		return `      - "${prefix}${p.hostPort}:${p.podPort}${p.protocol === 'udp' ? '/udp' : ''}"`;
	});

	if (!existingYaml || !existingYaml.trim()) {
		if (portLines.length === 0) {
			return `version: "3.8"\nservices:\n  ${cleanName}:\n    restart: always\n`;
		}
		return `version: "3.8"\nservices:\n  ${cleanName}:\n    ports:\n${portLines.join('\n')}\n    restart: always\n`;
	}

	// If existing yaml has services, replace or inject the ports block
	const lines = existingYaml.split('\n');
	const output: string[] = [];
	let inTargetService = false;
	let inPorts = false;
	let portsWritten = false;

	for (let i = 0; i < lines.length; i++) {
		const line = lines[i];
		const trimmed = line.trim();
		const indent = line.search(/\S/);

		if (trimmed.startsWith('services:')) {
			output.push(line);
			continue;
		}

		if (indent === 2 && trimmed.endsWith(':')) {
			const sName = trimmed.replace(/:$/, '').trim();
			if (sName === cleanName || sName === `pod_${cleanName}`) {
				inTargetService = true;
				output.push(line);
				// Inject updated ports
				if (portLines.length > 0) {
					output.push('    ports:');
					portLines.forEach((pl) => output.push(pl));
					portsWritten = true;
				} else {
					portsWritten = true;
				}
				continue;
			} else {
				inTargetService = false;
			}
		}

		if (inTargetService) {
			if (trimmed.startsWith('ports:')) {
				inPorts = true;
				continue;
			}
			if (inPorts) {
				if (trimmed.startsWith('-')) {
					continue; // skip old port definitions
				} else {
					inPorts = false;
				}
			}
		}

		output.push(line);
	}

	if (!portsWritten && portLines.length > 0) {
		// If the target service wasn't found in existing yaml, append it
		if (!existingYaml.includes('services:')) {
			return `version: "3.8"\nservices:\n  ${cleanName}:\n    ports:\n${portLines.join('\n')}\n\n${existingYaml}`;
		}
		output.push(`  ${cleanName}:`);
		output.push('    ports:');
		portLines.forEach((pl) => output.push(pl));
	}

	return output.join('\n');
}

export interface PodMemberDef {
	name: string;
	image?: string;
	port?: number;
}

/**
 * Generates Kubernetes Pod YAML manifest with member containers and hostPort bindings.
 */
export function serializePodK8sYaml(
	serviceName: string,
	ports: PortBindingItem[],
	members?: PodMemberDef[]
): string {
	const cleanName =
		serviceName
			.trim()
			.replace(/^pod_/, '')
			.toLowerCase()
			.replace(/[^a-z0-9-]+/g, '-')
			.replace(/^-+|-+$/g, '') || 'pod-service';

	let containerBlocks = '';
	if (members && members.length > 0) {
		const blocks = members.map((m) => {
			// Find host port bindings mapped to this member container's port
			const memberPorts = ports.filter((p) => m.port && p.podPort === m.port);
			let portSection = '';
			if (memberPorts.length > 0) {
				const portLines = memberPorts.map(
					(p) =>
						`        - containerPort: ${p.podPort}\n          hostPort: ${p.hostPort}\n          protocol: ${p.protocol.toUpperCase()}`
				);
				portSection = `\n      ports:\n${portLines.join('\n')}`;
			} else if (m.port && m.port > 0) {
				portSection = `\n      ports:\n        - containerPort: ${m.port}`;
			}
			return `    - name: ${m.name}\n      image: ${m.image || 'docker.io/library/alpine:latest'}${portSection}`;
		});
		containerBlocks = blocks.join('\n');
	} else {
		const portDefs = ports.map(
			(p) =>
				`        - containerPort: ${p.podPort}\n          hostPort: ${p.hostPort}\n          protocol: ${p.protocol.toUpperCase()}`
		);
		containerBlocks =
			portDefs.length > 0
				? `    - name: infra\n      image: registry.k8s.io/pause:3.9\n      ports:\n${portDefs.join('\n')}`
				: `    - name: infra\n      image: registry.k8s.io/pause:3.9`;
	}

	return `apiVersion: v1
kind: Pod
metadata:
  name: ${cleanName}
spec:
  containers:
${containerBlocks}
`;
}

/**
 * Extracts PortBindingItem list from Kubernetes Pod manifest YAML string.
 */
export function parseK8sPodPorts(yaml: string): PortBindingItem[] {
	if (!yaml || !yaml.trim()) return [];
	const items: PortBindingItem[] = [];
	const seen = new Set<string>();
	const portBlocks = yaml.split(/-\s*containerPort:/i);
	for (let i = 1; i < portBlocks.length; i++) {
		const block = portBlocks[i];
		const cpMatch = block.match(/^\s*(\d+)/);
		const hpMatch = block.match(/hostPort:\s*(\d+)/i);
		const protoMatch = block.match(/protocol:\s*(TCP|UDP)/i);
		if (cpMatch) {
			const podPort = parseInt(cpMatch[1], 10);
			const hostPort = hpMatch ? parseInt(hpMatch[1], 10) : podPort;
			const protocol: 'tcp' | 'udp' =
				protoMatch && protoMatch[1].toLowerCase() === 'udp' ? 'udp' : 'tcp';
			if (!hostPort || !podPort || hostPort < 1 || hostPort > 65535 || podPort < 1 || podPort > 65535) {
				continue;
			}
			const id = portBindingId(hostPort, podPort, protocol, DEFAULT_PORT_HOST_IP);
			if (seen.has(id)) continue;
			seen.add(id);
			items.push({ id, hostPort, podPort, protocol, hostIp: DEFAULT_PORT_HOST_IP });
		}
	}
	return items;
}

/** Normalized set key for drift comparison: "ip:host:container[/udp]" (missing ip -> 0.0.0.0). */
export function normalizePortKey(p: PortBindingItem): string {
	const ip = normalizePortHostIp(p.hostIp);
	return `${ip}:${p.hostPort}:${p.podPort}${p.protocol === 'udp' ? '/udp' : ''}`;
}
