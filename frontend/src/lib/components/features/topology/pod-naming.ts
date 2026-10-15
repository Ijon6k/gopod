// ──────────────────────────────────────────────
// GOPOD — Canonical Pod Naming (framework-free)
// Must mirror backend PodNameForService / PodNameFor exactly:
// pod_<lowercase slug of service ID>, slug truncated to 48 chars.
// ──────────────────────────────────────────────

export function slugLower(name: string): string {
	return (name || '')
		.replace(/^pod_/, '')
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9_-]+/g, '-');
}

export function canonicalPodNameFor(svc: { id?: string; name?: string }): string {
	const base = svc.id && svc.id.trim() ? svc.id : svc.name || 'pod-service';
	let slug = slugLower(base);
	if (slug.length > 48) slug = slug.slice(0, 48).replace(/-+$/, '');
	return `pod_${slug || 'pod-service'}`;
}

// Every runtime name a pod service may own, preferred first
// (canonical, persisted podId, legacy name forms).
export function podServiceNames(svc: { id?: string; name?: string; podId?: string }): string[] {
	const out: string[] = [];
	const add = (n?: string) => {
		if (n && !out.includes(n)) out.push(n);
	};
	add(canonicalPodNameFor(svc));
	if (svc.podId) {
		add(svc.podId);
		add(`pod_${slugLower(svc.podId)}`);
	}
	add(`pod_${slugLower(svc.name || '')}`);
	add(svc.name);
	add(svc.id);
	return out;
}
