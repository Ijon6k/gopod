export type LogLevel = 'INFO' | 'WARN' | 'ERROR' | 'DEBUG';

export interface AnsiSpan {
	text: string;
	colorClass: string;
	bold?: boolean;
	dim?: boolean;
}

export interface FormattedLogEntry {
	id: number;
	time: string;
	sortTs: number | null;
	container: string;
	level: LogLevel;
	text: string;
	spans: AnsiSpan[];
	prettyJson: string | null;
}

const ESC = String.fromCharCode(0x1b);
const BEL = String.fromCharCode(0x07);

// Full ANSI escape matcher constructed dynamically so ESLint no-control-regex stays clean.
const ANSI_ALL_RE = new RegExp(
	`${ESC}(?:\\[[0-9;?]*[ -/]*[@-~]|\\][^${BEL}${ESC}]*(?:${BEL}|${ESC}\\\\)|[@-Z\\\\-_])`,
	'g'
);
const ANSI_SGR_RE = new RegExp(`${ESC}\\[([0-9;]*)m`, 'g');
const LEADING_ISO_TS_RE = new RegExp(
	`^(?:${ESC}\\[[0-9;]*m)*\\d{4}-\\d{2}-\\d{2}[T\\s]\\d{2}:\\d{2}:\\d{2}(?:\\.\\d+)?(?:Z|[+-]\\d{2}:?\\d{2}|\\s+UTC)?(?:${ESC}\\[[0-9;]*m)*\\s+`
);

function stripResidualControlChars(input: string): string {
	let out = '';
	for (let i = 0; i < input.length; i++) {
		const code = input.charCodeAt(i);
		if (code === 13) continue; // normalize \r out
		if ((code < 32 && code !== 9 && code !== 10 && code !== 27) || code === 127) {
			continue;
		}
		out += input[i];
	}
	return out;
}

const SGR_FG_CLASS: Record<number, string> = {
	30: 'text-[var(--text-tertiary)]',
	31: 'text-[var(--status-red)]',
	32: 'text-[var(--status-green)]',
	33: 'text-[var(--status-amber)]',
	34: 'text-[var(--status-blue)]',
	35: 'text-[var(--accent-text)]',
	36: 'text-[var(--status-blue)]',
	37: 'text-[var(--text-primary)]',
	90: 'text-[var(--text-tertiary)]',
	91: 'text-[var(--status-red)]',
	92: 'text-[var(--status-green)]',
	93: 'text-[var(--status-amber)]',
	94: 'text-[var(--status-blue)]',
	95: 'text-[var(--accent-text)]',
	96: 'text-[var(--status-blue)]',
	97: 'text-[var(--text-primary)]'
};

export function stripAnsi(input: string): string {
	return stripResidualControlChars(input).replace(ANSI_ALL_RE, '');
}

/**
 * Parse ANSI SGR color sequences into declarative spans using semantic CSS tokens.
 * Non-SGR escape codes are stripped safely so terminal UI noise never clutters output.
 */
export function parseAnsiSpans(rawInput: string): AnsiSpan[] {
	const cleaned = stripResidualControlChars(rawInput);
	const spans: AnsiSpan[] = [];
	let colorClass = '';
	let bold = false;
	let dim = false;
	let lastIndex = 0;

	const pushSegment = (segment: string) => {
		const normalized = segment.replace(ANSI_ALL_RE, '');
		if (!normalized) return;
		spans.push({
			text: normalized,
			colorClass,
			bold: bold || undefined,
			dim: dim || undefined
		});
	};

	ANSI_SGR_RE.lastIndex = 0;
	let match: RegExpExecArray | null;
	while ((match = ANSI_SGR_RE.exec(cleaned)) !== null) {
		if (match.index > lastIndex) {
			pushSegment(cleaned.slice(lastIndex, match.index));
		}
		lastIndex = match.index + match[0].length;

		const rawCodes = match[1] ? match[1].split(';') : ['0'];
		for (const rawCode of rawCodes) {
			const code = Number(rawCode);
			if (Number.isNaN(code) || code === 0) {
				colorClass = '';
				bold = false;
				dim = false;
			} else if (code === 1) {
				bold = true;
			} else if (code === 2) {
				dim = true;
			} else if (code === 22) {
				bold = false;
				dim = false;
			} else if (code === 39) {
				colorClass = '';
			} else if (SGR_FG_CLASS[code]) {
				colorClass = SGR_FG_CLASS[code];
			}
		}
	}

	if (lastIndex < cleaned.length) {
		pushSegment(cleaned.slice(lastIndex));
	}

	if (spans.length === 0 && cleaned.length > 0) {
		const fallback = stripAnsi(cleaned);
		if (fallback) spans.push({ text: fallback, colorClass: '' });
	}
	return spans;
}

export function tryPrettyJson(
	plainLine: string
): { formatted: string; obj: Record<string, unknown> } | null {
	const trimmed = plainLine.trim();
	if (!trimmed.startsWith('{') || !trimmed.endsWith('}')) return null;
	try {
		const parsed = JSON.parse(trimmed);
		if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
			return {
				formatted: JSON.stringify(parsed, null, 2),
				obj: parsed as Record<string, unknown>
			};
		}
	} catch {
		return null;
	}
	return null;
}

function normalizeLevelFromString(raw: unknown): LogLevel | null {
	if (typeof raw !== 'string') return null;
	const upper = raw.toUpperCase();
	if (
		upper === 'ERROR' ||
		upper === 'ERR' ||
		upper === 'FATAL' ||
		upper === 'PANIC' ||
		upper === 'CRIT'
	) {
		return 'ERROR';
	}
	if (upper === 'WARN' || upper === 'WARNING') {
		return 'WARN';
	}
	if (upper === 'DEBUG' || upper === 'TRACE') {
		return 'DEBUG';
	}
	if (upper === 'INFO' || upper === 'LOG' || upper === 'NOTICE') {
		return 'INFO';
	}
	return null;
}

function detectLevel(plain: string, jsonObj?: Record<string, unknown>): LogLevel {
	if (jsonObj) {
		const fromField =
			normalizeLevelFromString(jsonObj.level) ||
			normalizeLevelFromString(jsonObj.severity) ||
			normalizeLevelFromString(jsonObj.lvl);
		if (fromField) return fromField;
	}

	if (/\b(ERROR|FATAL|PANIC|CRITICAL|EXCEPTION|ECONNREFUSED|FAIL(?:ED)?)\b/i.test(plain)) {
		return 'ERROR';
	}
	if (/\b(WARN|WARNING|DEPRECATED)\b/i.test(plain)) {
		return 'WARN';
	}
	if (/\b(DEBUG|TRACE)\b/.test(plain)) {
		return 'DEBUG';
	}
	return 'INFO';
}

function formatClockFromDate(d: Date): string {
	const hh = String(d.getHours()).padStart(2, '0');
	const mm = String(d.getMinutes()).padStart(2, '0');
	const ss = String(d.getSeconds()).padStart(2, '0');
	const ms = String(d.getMilliseconds()).padStart(3, '0');
	return `${hh}:${mm}:${ss}.${ms}`;
}

function extractTimeAndBody(
	rawLine: string,
	plainLine: string,
	defaultContainer: string
): { time: string; sortTs: number | null; container: string; bodyRaw: string; bodyPlain: string } {
	let container = defaultContainer;
	let workPlain = plainLine;
	let workRaw = rawLine;

	// Strip [container_name] prefix when aggregated streams inject one
	const prefixMatch = workPlain.match(/^\[([^\]]+)\]\s+(.*)/);
	if (prefixMatch && !/^\d+$/.test(prefixMatch[1])) {
		container = prefixMatch[1];
		workPlain = prefixMatch[2];
		workRaw = workRaw.replace(/^\[[^\]]+\]\s+/, '');
	}

	// RFC3339 / ISO / Postgres UTC timestamps at line start (e.g. "2026-10-28 08:00:12.039 UTC ...")
	const isoMatch = workPlain.match(
		/^(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2}|\s+UTC)?)\s+(.*)$/
	);
	if (isoMatch) {
		const rawTs = isoMatch[1].trim();
		const normalizedTs = rawTs.endsWith(' UTC')
			? rawTs.slice(0, -4).replace(' ', 'T') + 'Z'
			: rawTs.replace(' ', 'T');
		const parsed = Date.parse(normalizedTs);
		const timePartMatch = rawTs.match(/(\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?)/);
		return {
			time: timePartMatch ? timePartMatch[1] : rawTs,
			sortTs: Number.isNaN(parsed) ? null : parsed,
			container,
			bodyPlain: isoMatch[2] || workPlain,
			bodyRaw: workRaw.replace(LEADING_ISO_TS_RE, '')
		};
	}

	// Syslog format ("Oct 28 08:00:12 host unit[pid]: msg")
	const syslogMatch = workPlain.match(
		/^([A-Z][a-z]{2}\s+\d+\s+\d{2}:\d{2}:\d{2})\s+\S+\s+([^:]+):\s*(.*)$/
	);
	if (syslogMatch) {
		return {
			time: syslogMatch[1],
			sortTs: null,
			container: syslogMatch[2] || container,
			bodyPlain: syslogMatch[3] || workPlain,
			bodyRaw: syslogMatch[3] || workPlain
		};
	}

	// Deployment manager clock prefix ("[15:04:05] msg")
	const clockMatch = workPlain.match(/^\[(\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?)\]\s+(.*)$/);
	if (clockMatch) {
		return {
			time: clockMatch[1],
			sortTs: null,
			container,
			bodyPlain: clockMatch[2] || workPlain,
			bodyRaw: workRaw.replace(/^\[\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?\]\s+/, '')
		};
	}

	return {
		time: '—',
		sortTs: null,
		container,
		bodyPlain: workPlain,
		bodyRaw: workRaw
	};
}

export function parseFormattedLogLines(
	raw: string,
	defaultContainer = 'container'
): FormattedLogEntry[] {
	if (!raw || !raw.trim()) return [];
	const normalized = stripResidualControlChars(raw.replace(/\r\n/g, '\n'));
	const lines = normalized.split('\n');
	// Trim trailing empty newline produced by line-buffered output
	if (lines.length > 0 && lines[lines.length - 1] === '') {
		lines.pop();
	}

	const entries: FormattedLogEntry[] = [];
	for (let i = 0; i < lines.length; i++) {
		const rawLine = lines[i];
		const plain = stripAnsi(rawLine);
		if (!plain.trim()) continue;

		const extracted = extractTimeAndBody(rawLine, plain, defaultContainer);
		let time = extracted.time;
		let sortTs = extracted.sortTs;
		const bodyPlain = extracted.bodyPlain;
		const bodyRaw = extracted.bodyRaw;

		const jsonInfo = tryPrettyJson(bodyPlain);
		if (jsonInfo) {
			const tsRaw = jsonInfo.obj.ts ?? jsonInfo.obj.timestamp ?? jsonInfo.obj.time;
			if (typeof tsRaw === 'number' && Number.isFinite(tsRaw)) {
				const ms = tsRaw < 1e12 ? tsRaw * 1000 : tsRaw;
				const d = new Date(ms);
				if (!Number.isNaN(d.getTime())) {
					time = formatClockFromDate(d);
					sortTs = ms;
				}
			} else if (typeof tsRaw === 'string') {
				const ms = Date.parse(tsRaw);
				if (!Number.isNaN(ms)) {
					time = formatClockFromDate(new Date(ms));
					sortTs = ms;
				}
			}
		}

		const level = detectLevel(bodyPlain, jsonInfo?.obj);
		const spans = parseAnsiSpans(bodyRaw);

		entries.push({
			id: i,
			time,
			sortTs,
			container: extracted.container,
			level,
			text: bodyPlain,
			spans,
			prettyJson: jsonInfo ? jsonInfo.formatted : null
		});
	}

	return entries;
}
