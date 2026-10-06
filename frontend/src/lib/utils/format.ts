/**
 * Format memory value to human readable string.
 * e.g., 1240 MB → "1.2 GB", 180 MB → "180 MB"
 */
export function formatMemory(mb: number): string {
	if (mb >= 1024) {
		return `${(mb / 1024).toFixed(1)} GB`;
	}
	return `${mb} MB`;
}

/**
 * Format CPU percentage.
 */
export function formatCpu(value: number): string {
	return `${value.toFixed(1)}%`;
}

/**
 * Format storage value.
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
	const date = typeof dateInput === 'string' || typeof dateInput === 'number' ? new Date(dateInput) : dateInput;
	if (isNaN(date.getTime())) return 'Recently';

	const diff = Math.floor((Date.now() - date.getTime()) / 1000);
	if (diff < 30) return 'Just now';
	if (diff < 60) return `${diff}s ago`;
	if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
	if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
	return `${Math.floor(diff / 86400)}d ago`;
}

/**
 * Format raw bytes into human-readable string (B, KB, MB, GB).
 */
export function formatBytes(bytes: number, decimals = 1): string {
	if (!+bytes) return '0 B';
	const k = 1024;
	const dm = decimals < 0 ? 0 : decimals;
	const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}
