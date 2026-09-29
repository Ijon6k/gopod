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
