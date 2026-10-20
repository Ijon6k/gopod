// Pure validation helpers for the Advanced tab. Every deploy-able value the UI
// collects flows through here so invalid input surfaces INLINE instead of
// being coerced silently (the old save path turned garbage into "512MB",
// "2048" pids, or "0" cpu). The swap grammar mirrors the backend
// podman.NormalizeMemorySwap so the UI and the engine agree on accepted forms.

/** Parse a CPU limit in cores: '' means unlimited (0). Returns NaN when invalid. */
export function parseCpuLimit(raw: string): number {
	const s = (raw || '').trim();
	if (s === '') return 0;
	if (!/^\d+(?:\.\d+)?$/.test(s)) return NaN;
	const n = parseFloat(s);
	if (n <= 0 || n > 64) return NaN;
	return n;
}

/** Parse a memory limit to MB: '' means unlimited (0). Accepts bare MB and G units. */
export function parseMemoryMb(raw: string): number {
	const s = (raw || '').trim().toUpperCase();
	if (s === '') return 0;
	const m = s.match(/^(\d+(?:\.\d+)?)\s*(MB|M|GB|G|KB|K)?$/);
	if (!m) return NaN;
	const num = parseFloat(m[1]);
	const unit = m[2] || 'MB';
	if (num <= 0) return NaN;
	if (unit.startsWith('K')) {
		if (num < 1024) return NaN; // sub-MB values cannot round to a settable MB int
		return Math.round(num / 1024);
	}
	if (unit.startsWith('G')) return Math.round(num * 1024);
	return Math.round(num);
}

/** Parse a pids limit: '' means unlimited (0, matches the backend contract). */
export function parsePidsLimit(raw: string): number {
	const s = (raw || '').trim();
	if (s === '') return 0;
	if (!/^\d+$/.test(s)) return NaN;
	return parseInt(s, 10);
}

export interface SwapResult {
	value: string;
	error: string;
}

/**
 * Validate + canonicalize a --memory-swap value exactly like the backend:
 * '', '0', 'default' → '' (omit), 'unlimited' → '-1', sizes validated
 * (b/kb/mb/gb or k/m/g suffix). Returns {value:'', error} on invalid input.
 */
export function normalizeSwapLimit(raw: string): SwapResult {
	const s = (raw || '').trim();
	if (s === '' || s === '0' || s === 'default') return { value: '', error: '' };
	if (s.toLowerCase() === 'unlimited') return { value: '-1', error: '' };
	if (s === '-1') return { value: s, error: '' };

	const lowered = s.toLowerCase();
	let body = s;
	for (const suffix of ['kb', 'mb', 'gb', 'b', 'k', 'm', 'g']) {
		if (s.length > suffix.length && lowered.endsWith(suffix)) {
			body = s.slice(0, s.length - suffix.length).trim();
			break;
		}
	}
	if (body === '') {
		return { value: '', error: `invalid memory swap value "${s}": missing size` };
	}
	for (const ch of body) {
		if ((ch < '0' || ch > '9') && ch !== '.') {
			return {
				value: '',
				error: `invalid memory swap value "${s}": use a size like "512MB", "1G", or "unlimited"`
			};
		}
	}
	return { value: s.replace(/ /g, ''), error: '' };
}

const CAP_TOKEN = /^[A-Za-z][A-Za-z0-9_]*$/;

/** Split + validate a comma-separated capability list. null = format error. */
export function parseCapList(raw: string): string[] | null {
	const items = (raw || '')
		.split(',')
		.map((c) => c.trim())
		.filter(Boolean);
	for (const item of items) {
		if (!CAP_TOKEN.test(item)) return null;
	}
	return items;
}

export interface VolumeRow {
	source: string;
	target: string;
	options: string;
}

const NAMED_VOLUME = /^[A-Za-z0-9][A-Za-z0-9_.-]*$/;
const OPT_TOKEN = /^[A-Za-z0-9=+_.:-]+$/;

/** Validate one `source:target[:options]` row. Returns '' when valid. */
export function validateVolumeRow(row: VolumeRow): string {
	const source = (row.source || '').trim();
	if (source === '') return 'Source is required (named volume or absolute host path)';
	if (!source.startsWith('/') && !source.startsWith('~') && !NAMED_VOLUME.test(source)) {
		return 'Source must be an absolute host path or a named volume';
	}
	const target = (row.target || '').trim();
	if (target === '') return 'Target is required';
	if (!target.startsWith('/')) return 'Target must be an absolute container path';
	for (const opt of (row.options || '').split(',')) {
		const token = opt.trim();
		if (token === '') continue;
		if (!OPT_TOKEN.test(token)) return `Invalid mount option "${token}"`;
	}
	return '';
}

const USERNS_TOKEN = /^[A-Za-z0-9:_=.-]+$/;

/**
 * Validate a --userns value: '' stays invalid-free (unset). podman accepts
 * tokens like keep-id, host, auto, size=N:M — anything with shell characters
 * is rejected up front.
 */
export function validateUserNamespace(raw: string): string {
	const s = (raw || '').trim();
	if (s === '') return '';
	if (!USERNS_TOKEN.test(s)) return 'Namespace values use letters, digits and ":", "=", ".", "-"';
	return '';
}

/** Validate a comma-separated device list: each entry must be a /dev path. */
export function validateDeviceList(raw: string): string {
	const items = (raw || '')
		.split(',')
		.map((d) => d.trim())
		.filter(Boolean);
	for (const item of items) {
		if (!item.startsWith('/dev/')) {
			return `Devices must be host paths under /dev/ — "${item}" is not`;
		}
	}
	return '';
}
