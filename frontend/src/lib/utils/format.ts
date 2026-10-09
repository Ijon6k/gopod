/**
 * GOPOD Centralized Formatting & Measurement Utilities
 * Single Source of Truth for memory, bytes, CPU, storage, percentages, and semantic status colors.
 */

/**
 * Format memory value in MB to human-readable string.
 * e.g., 1240 MB → "1.2 GB", 180 MB → "180 MB"
 */
export function formatMemory(mb: number): string {
	if (!mb || mb <= 0) return '0 MB';
	if (mb >= 1024) {
		return `${(mb / 1024).toFixed(1)} GB`;
	}
	return `${Math.round(mb)} MB`;
}

/**
 * Convert bytes to GB with precision.
 */
export function bytesToGB(bytes?: number): number {
	if (!bytes || bytes <= 0) return 0;
	return parseFloat((bytes / (1024 * 1024 * 1024)).toFixed(2));
}

/**
 * Parse reclaimable percentage string (e.g. "45 MB (25%)" → 25).
 */
export function parsePct(reclaimStr?: string): number {
	if (!reclaimStr) return 0;
	const match = reclaimStr.match(/\((\d+)%\)/);
	return match ? parseInt(match[1], 10) : 0;
}

/**
 * Parse network rate string (e.g. "12.4 MB", "350 KB") to numeric megabytes.
 */
export function parseNetToMb(netStr: string): number {
	if (!netStr || netStr === '—') return 0;
	const parts = netStr.match(/([0-9.]+)\s*([a-zA-Z]+)/);
	if (!parts) return 0;
	const val = parseFloat(parts[1]);
	const unit = parts[2].toLowerCase();
	if (unit.startsWith('g')) return val * 1024;
	if (unit.startsWith('m')) return val;
	if (unit.startsWith('k')) return val / 1024;
	return val / (1024 * 1024);
}

/**
 * Format CPU percentage to one decimal place.
 */
export function formatCpu(value: number): string {
	return `${(value || 0).toFixed(1)}%`;
}

/**
 * Format storage value in GB.
 */
export function formatStorage(gb: number, total?: number): string {
	if (total !== undefined) {
		return `${gb.toFixed(1)} / ${total} GB`;
	}
	return `${gb.toFixed(1)} GB`;
}

/**
 * Pluralize a word based on count.
 */
export function pluralize(count: number, singular: string, plural?: string): string {
	return count === 1 ? singular : (plural ?? `${singular}s`);
}

/**
 * Format timestamp into human-readable relative time (e.g., "Just now", "5m ago", "2h ago").
 */
export function formatTimeAgo(dateInput: string | number | Date | null | undefined): string {
	if (!dateInput) return '—';
	const date =
		typeof dateInput === 'string' || typeof dateInput === 'number'
			? new Date(dateInput)
			: dateInput;
	if (isNaN(date.getTime())) return 'Recently';

	const diff = Math.floor((Date.now() - date.getTime()) / 1000);
	if (diff < 30) return 'Just now';
	if (diff < 60) return `${diff}s ago`;
	if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
	if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
	return `${Math.floor(diff / 86400)}d ago`;
}

/**
 * Format raw bytes into human-readable string (B, KB, MB, GB, TB).
 */
export function formatBytes(bytes: number, decimals = 1): string {
	if (!+bytes || bytes <= 0) return '0 B';
	const k = 1024;
	const dm = decimals < 0 ? 0 : decimals;
	const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

/**
 * Compute semantic background color token for CPU consumption.
 */
export function getCpuColor(cpu: number): string {
	if (cpu >= 5.0) return 'bg-[var(--status-red)]';
	if (cpu >= 2.0) return 'bg-[var(--status-amber)]';
	return 'bg-[var(--accent)]';
}

/**
 * Calculate memory consumption percentage against limit.
 */
export function getMemPercent(used: number, limit: number): number {
	return Math.min(Math.round(((used || 0) / (limit || 512)) * 100), 100);
}

/**
 * Compute semantic background color token for memory consumption percentage.
 */
export function getMemColor(percent: number): string {
	if (percent >= 85) return 'bg-[var(--status-red)]';
	if (percent >= 70) return 'bg-[var(--status-amber)]';
	return 'bg-[var(--status-green)]';
}
