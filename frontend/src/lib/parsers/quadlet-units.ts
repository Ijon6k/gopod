// Pure Quadlet unit text helpers for the structured editor.
//
// The BACKEND generator (internal/services) is the single source of truth for
// GENERATING units; these functions only PARSE unit text into editor state and
// REWRITE specific directives on apply. Multi-unit bundles use the deployed
// convention: chunks introduced by `# File: <name>` comments joined with a
// `---` separator line (mirrors backend SplitQuadletUnits).

export interface QuadletUnitFile {
	name: string;
	content: string;
	kind: 'pod' | 'container' | 'other';
}

export function splitQuadletText(config: string): QuadletUnitFile[] {
	const text = (config || '').replace(/\r\n/g, '\n').trim();
	if (!text) return [];

	const chunks = text
		.split(/^---\s*$/m)
		.map((chunk) => chunk.trim())
		.filter(Boolean);

	return chunks.map((chunk) => {
		const lines = chunk.split('\n');
		let name = '';
		let bodyStart = 0;
		if (lines[0] && /^#\s*File:\s*\S+/i.test(lines[0])) {
			name = lines[0].replace(/^#\s*File:\s*/i, '').trim();
			bodyStart = 1;
		}
		const content = lines.slice(bodyStart).join('\n').trim();
		const kind: QuadletUnitFile['kind'] = name.endsWith('.pod')
			? 'pod'
			: classifyUnit(content, name);
		return { name, content, kind };
	});
}

function classifyUnit(content: string, name: string): QuadletUnitFile['kind'] {
	const haystack = (content + '\n' + name).toLowerCase();
	if (haystack.includes('[pod]') || name.endsWith('.pod')) return 'pod';
	if (haystack.includes('[container]')) return 'container';
	return 'other';
}

export function unitFallbackName(unit: QuadletUnitFile): string {
	const m = unit.content.match(/^ContainerName\s*=\s*([^\r\n#]+)/im);
	if (m?.[1]?.trim()) return m[1].trim();
	const p = unit.content.match(/^PodName\s*=\s*([^\r\n#]+)/im);
	if (p?.[1]?.trim()) return p[1].trim();
	return unit.name || 'unit';
}

/** Read ALL directive values for one key (case-insensitive), quotes stripped. */
export function readDirectives(content: string, key: string): string[] {
	const out: string[] = [];
	for (const line of (content || '').split('\n')) {
		const trimmed = line.trim();
		if (trimmed.startsWith('#') || trimmed.startsWith(';')) continue;
		const eq = trimmed.indexOf('=');
		if (eq <= 0) continue;
		const k = trimmed.slice(0, eq).trim();
		if (k.toLowerCase() !== key.toLowerCase()) continue;
		let v = trimmed.slice(eq + 1).trim();
		if (
			v.length >= 2 &&
			((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'")))
		) {
			v = v.slice(1, -1);
		}
		out.push(v);
	}
	return out;
}

function readFirst(content: string, key: string): string {
	const values = readDirectives(content, key);
	return values[0] ?? '';
}

export function writeDirectiveLine(key: string, value: string): string {
	if (key === 'Environment') {
		return `Environment="${value}"`;
	}
	return `${key}=${value}`;
}

export interface UnitEdits {
	// key -> list of values; empty list removes every line of that key
	values: Record<string, string[]>;
	// key -> single switch values written as true/remove (e.g. ReadOnly)
	switches?: Record<string, boolean>;
}

const SECTION_KEYS: string[] = ['Container', 'Pod'];

/**
 * Applies directive edits to one unit body: every line of an edited key is
 * removed, then the new values are inserted right after the [Container] or
 * [Pod] section header (never outside their section). Unknown content,
 * comments, and other keys are left untouched.
 */
export function applyUnitEdits(content: string, edits: UnitEdits): string {
	let lines = (content || '').split('\n');

	for (const [key, values] of Object.entries(edits.values)) {
		// 1. remove existing lines for this key (case-insensitive directive match)
		lines = lines.filter((line) => {
			const trimmed = line.trim();
			if (trimmed.startsWith('#') || trimmed.startsWith(';')) return true;
			const eq = trimmed.indexOf('=');
			if (eq <= 0) return true;
			return trimmed.slice(0, eq).trim().toLowerCase() !== key.toLowerCase();
		});
		if (values.length === 0) continue;

		// 2. insert after the [Container]/[Pod] section header, else before
		//    [Service]/[Install], else append at the end.
		let insertAt = -1;
		for (const section of SECTION_KEYS) {
			const idx = lines.findIndex((l) => l.trim() === `[${section}]`);
			if (idx !== -1) {
				insertAt = idx + 1;
				break;
			}
		}
		if (insertAt === -1) {
			const serviceIdx = lines.findIndex((l) => l.trim() === '[Service]');
			const installIdx = lines.findIndex((l) => l.trim() === '[Install]');
			if (serviceIdx !== -1) insertAt = serviceIdx;
			else if (installIdx !== -1) insertAt = installIdx;
			else insertAt = lines.length;
		}
		const newLines = values.map((v) => writeDirectiveLine(key, v));
		lines.splice(insertAt, 0, ...newLines);
	}

	for (const [key, on] of Object.entries(edits.switches ?? {})) {
		// remove and re-insert like value keys; `false` removes only
		lines = lines.filter((line) => {
			const trimmed = line.trim();
			if (trimmed.startsWith('#') || trimmed.startsWith(';')) return true;
			const eq = trimmed.indexOf('=');
			if (eq <= 0) return true;
			return trimmed.slice(0, eq).trim().toLowerCase() !== key.toLowerCase();
		});
		if (on) {
			let insertAt = lines.length;
			for (const section of SECTION_KEYS) {
				const idx = lines.findIndex((l) => l.trim() === `[${section}]`);
				if (idx !== -1) {
					insertAt = idx + 1;
					break;
				}
			}
			lines.splice(insertAt, 0, `${key}=true`);
		}
	}

	return lines.join('\n');
}

/** Join edited units back into the deployed multi-unit text convention. */
export function joinQuadletUnits(units: QuadletUnitFile[]): string {
	return units
		.filter((u) => u.kind !== 'other' || u.content.trim() !== '')
		.map((u) => {
			const header = u.name ? `# File: ${u.name}\n` : '';
			return `${header}${u.content.trim()}`;
		})
		.join('\n---\n');
}

/** Parse the structured fields of a .container unit for the editor card. */
export interface ContainerCardState {
	image: string;
	containerName: string;
	pod: string;
	env: { key: string; value: string }[];
	volumes: { source: string; target: string; options: string }[];
	tmpfs: string[];
	capAdd: string;
	capDrop: string;
	addDevice: string;
	healthCmd: string;
	healthInterval: string;
	healthOnFailure: string;
	pidsLimit: string;
	readOnly: boolean;
	readOnlyTmpfs: boolean;
	securityLabelDisable: boolean;
	userNs: string;
	chownUser: string;
	labels: { key: string; value: string }[];
	cpuLimit: string;
	memoryLimit: string;
	podmanArgs: string;
	exec: string;
}

export function readContainerCard(content: string): ContainerCardState {
	const env = readDirectives(content, 'Environment').map((v) => {
		const eq = v.indexOf('=');
		if (eq === -1) return { key: v, value: '' };
		return { key: v.slice(0, eq), value: v.slice(eq + 1) };
	});
	const volumes = readDirectives(content, 'Volume').map((v) => {
		const parts = v.split(':');
		if (parts.length === 1) return { source: parts[0], target: '', options: '' };
		if (parts.length === 2) return { source: parts[0], target: parts[1], options: '' };
		return { source: parts[0], target: parts[1], options: parts.slice(2).join(':') };
	});
	const podmanArgs = readFirst(content, 'PodmanArgs');
	const cpuLimit = readArgValue(podmanArgs, 'cpu-quota');
	const memoryLimit = readArgValue(podmanArgs, 'memory');

	return {
		image: readFirst(content, 'Image'),
		containerName: readFirst(content, 'ContainerName'),
		pod: readFirst(content, 'Pod'),
		env,
		volumes,
		tmpfs: readDirectives(content, 'Tmpfs'),
		capAdd: readDirectives(content, 'AddCapability').join(', '),
		capDrop: readDirectives(content, 'DropCapability').join(', '),
		addDevice: readDirectives(content, 'AddDevice').join(', '),
		healthCmd: readFirst(content, 'HealthCmd'),
		healthInterval: readFirst(content, 'HealthInterval'),
		healthOnFailure: readFirst(content, 'HealthOnFailure'),
		pidsLimit: readFirst(content, 'PidsLimit'),
		readOnly: readFirst(content, 'ReadOnly').toLowerCase() === 'true',
		readOnlyTmpfs: readFirst(content, 'ReadOnlyTmpfs').toLowerCase() === 'true',
		securityLabelDisable: readFirst(content, 'SecurityLabelDisable').toLowerCase() === 'true',
		userNs: readFirst(content, 'UserNS'),
		chownUser: readFirst(content, 'Chown'),
		labels: /^Label\s*=/im.test(content)
			? readDirectives(content, 'Label').map((v) => {
					const eq = v.indexOf('=');
					if (eq === -1) return { key: v, value: '' };
					return { key: v.slice(0, eq), value: v.slice(eq + 1) };
				})
			: [],
		cpuLimit,
		memoryLimit,
		podmanArgs: podmanArgs,
		exec: readFirst(content, 'Exec')
	};
}

function readArgValue(args: string, flag: string): string {
	const re = new RegExp(`--${flag}(?:=([^-\\s]+)|(?:\\s+)([^-\\s]+))`);
	const m = (args || '').match(re);
	if (!m) return '';
	return m[1] ?? m[2] ?? '';
}

/**
 * Builds the apply-able edits for a .container unit from its card state.
 * CPU/memory are NOT quadlet keys — they ride the REAL key PodmanArgs with the
 * REAL podman flags --cpu-quota/--memory (podman-systemd.unit has no dedicated
 * key for them).
 */
export function containerCardEdits(state: ContainerCardState): UnitEdits {
	const values: Record<string, string[]> = {};

	if (state.podmanArgs.trim() || state.cpuLimit.trim() || state.memoryLimit.trim()) {
		const baseArgs = stripArgs(state.podmanArgs, ['cpu-quota', 'memory']);
		const extra: string[] = [];
		if (state.cpuLimit.trim()) extra.push(`--cpu-quota=${state.cpuLimit.trim()}`);
		if (state.memoryLimit.trim()) extra.push(`--memory=${state.memoryLimit.trim()}`);
		const merged = [baseArgs.trim(), ...extra].filter(Boolean).join(' ');
		if (merged.trim()) values['PodmanArgs'] = [merged];
	}

	if (state.env.length > 0) {
		values['Environment'] = state.env
			.filter((e) => e.key.trim())
			.map((e) => `${e.key.trim()}=${e.value}`);
	}
	if (state.volumes.length > 0) {
		values['Volume'] = state.volumes
			.filter((v) => v.source.trim() && v.target.trim())
			.map((v) =>
				v.options.trim()
					? `${v.source.trim()}:${v.target.trim()}:${v.options.trim()}`
					: `${v.source.trim()}:${v.target.trim()}:Z`
			);
	}
	if (state.tmpfs.length > 0) {
		values['Tmpfs'] = state.tmpfs.filter((t) => t.trim());
	}
	if (state.capAdd.trim())
		values['AddCapability'] = state.capAdd
			.split(',')
			.map((c) => c.trim())
			.filter(Boolean);
	if (state.capDrop.trim())
		values['DropCapability'] = state.capDrop
			.split(',')
			.map((c) => c.trim())
			.filter(Boolean);
	if (state.addDevice.trim())
		values['AddDevice'] = state.addDevice
			.split(',')
			.map((d) => d.trim())
			.filter(Boolean);

	if (state.healthCmd.trim()) values['HealthCmd'] = [state.healthCmd.trim()];
	if (state.healthInterval.trim()) values['HealthInterval'] = [state.healthInterval.trim()];
	if (state.healthOnFailure.trim()) values['HealthOnFailure'] = [state.healthOnFailure.trim()];
	if (state.pidsLimit.trim()) values['PidsLimit'] = [state.pidsLimit.trim()];
	if (state.userNs.trim()) values['UserNS'] = [state.userNs.trim()];

	const switches: Record<string, boolean> = {
		ReadOnly: state.readOnly,
		ReadOnlyTmpfs: state.readOnly && state.readOnlyTmpfs,
		SecurityLabelDisable: state.securityLabelDisable
	};

	return { values, switches };
}

function stripArgs(args: string, flags: string[]): string {
	let out = args || '';
	for (const flag of flags) {
		out = out.replace(new RegExp(`\\s*--${flag}(?:=[^-\\s]+|\\s+[^-\\s]+)?`), '');
	}
	return out;
}

/** Parse the structured fields of a .pod unit for the editor card. */
export interface PodCardState {
	podName: string;
	network: string[];
	publishPorts: string[];
	stopTimeout: string;
	labels: { key: string; value: string }[];
	shmSize: string;
	exitPolicy: string;
}

export function readPodCard(content: string): PodCardState {
	return {
		podName: readFirst(content, 'PodName'),
		network: readDirectives(content, 'Network'),
		publishPorts: readDirectives(content, 'PublishPort'),
		stopTimeout: readFirst(content, 'StopTimeout'),
		labels: readDirectives(content, 'Label').map((v) => {
			const eq = v.indexOf('=');
			if (eq === -1) return { key: v, value: '' };
			return { key: v.slice(0, eq), value: v.slice(eq + 1) };
		}),
		shmSize: readFirst(content, 'ShmSize'),
		exitPolicy: readFirst(content, 'ExitPolicy')
	};
}

export function podCardEdits(state: PodCardState): UnitEdits {
	const values: Record<string, string[]> = {};
	if (state.publishPorts.length > 0)
		values['PublishPort'] = state.publishPorts.filter((p) => p.trim());
	if (state.stopTimeout.trim()) values['StopTimeout'] = [state.stopTimeout.trim()];
	if (state.shmSize.trim()) values['ShmSize'] = [state.shmSize.trim()];
	if (state.exitPolicy.trim()) values['ExitPolicy'] = [state.exitPolicy.trim()];
	// Network= and the io.gopod.* labels are ownership plumbing maintained by
	// the generator; editing them here would break service discovery.
	return { values };
}
