/** Runtime (containers/pods/images/volumes/networks/quadlets) shared helpers. */

// Rows carry gopod ownership labels or compose labels; a row with neither
// is foreign to this deployment and is flagged wherever a delete is offered.
export function isGopodManaged(labels?: Record<string, string> | null): boolean {
	if (!labels) return false;
	return Boolean(
		labels['io.gopod.name'] ||
			labels['io.gopod.service'] ||
			labels['io.gopod.project'] ||
			labels['com.docker.compose.project'] ||
			labels['io.podman.compose.project']
	);
}

/** Splits a Podman ports blob ("0.0.0.0:8080->80/tcp\n…") into cell lines. */
export function parsePorts(ports: string | undefined | null): string[] {
	if (!ports || ports === '—') return [];
	return ports
		.split(/[\n,]+/)
		.map((p) => p.trim().replaceAll('→', ':').replaceAll('->', ':'))
		.filter(Boolean);
}

/**
 * Shortens a volume mount path for table cells: the tail two segments of the
 * path stay visible (the identifying part), the rest collapses into an
 * ellipsis. The full path stays available via title/copy on the same cell.
 */
export function shortenMountPath(path: string | undefined | null): string {
	if (!path || path === '—') return '';
	const trimmed = path.replace(/\/+$/, '');
	if (trimmed.length <= 42) return trimmed;
	const segments = trimmed.split('/').filter(Boolean);
	if (segments.length > 2) {
		const tail = segments.slice(-2).join('/');
		return `…/${tail}`;
	}
	return `…${trimmed.slice(-(40 - 1))}`;
}
