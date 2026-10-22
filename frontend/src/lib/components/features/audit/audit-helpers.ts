// ──────────────────────────────────────────────
// GOPOD — Audit log shared helpers
// Pure formatting / routing helpers reused by the activity and console views.
// ──────────────────────────────────────────────
import { goto } from '$app/navigation';
import { ApiError } from '$lib/api';
import type { ActivityItem, Project, Service } from '$lib/types';

export type LevelFilter = 'all' | 'info' | 'warn' | 'error' | 'debug';

/** Backend feature toggle: a 404 means the endpoint isn't deployed yet. */
export function isNotFound(err: unknown): boolean {
	if (err instanceof ApiError) return err.status === 404;
	return (err as { status?: number })?.status === 404;
}

export function timeAgo(ts: string): string {
	const t = new Date(ts).getTime();
	if (Number.isNaN(t)) return ts || '-';
	const diffSec = Math.max(0, Math.floor((Date.now() - t) / 1000));
	if (diffSec < 10) return 'just now';
	if (diffSec < 60) return `${diffSec}s ago`;
	const diffMin = Math.floor(diffSec / 60);
	if (diffMin < 60) return `${diffMin}m ago`;
	const diffHrs = Math.floor(diffMin / 60);
	if (diffHrs < 24) return `${diffHrs}h ago`;
	const diffDays = Math.floor(diffHrs / 24);
	if (diffDays < 30) return `${diffDays}d ago`;
	return new Date(ts).toLocaleDateString();
}

export function formatTime(ts: string): string {
	const d = new Date(ts);
	if (Number.isNaN(d.getTime())) return ts || '-';
	return d.toLocaleString();
}

export function shortId(id?: string): string {
	if (!id) return '-';
	return id.length > 8 ? id.slice(0, 8) : id;
}

export function resolveServiceName(id: string | undefined, services: Service[]): string {
	if (!id) return '-';
	return services.find((s) => s.id === id)?.name ?? shortId(id);
}

export function resolveProjectName(id: string | undefined, projects: Project[]): string {
	if (!id) return '-';
	return projects.find((p) => p.id === id)?.name ?? shortId(id);
}

export function activityStatusVariant(status: string): 'success' | 'error' | 'info' | 'neutral' {
	const s = (status || '').toLowerCase();
	if (s === 'success') return 'success';
	if (s === 'failed' || s === 'error') return 'error';
	if (s === 'info') return 'info';
	return 'neutral';
}

export function levelVariant(level: string): 'info' | 'warning' | 'error' | 'neutral' | 'success' {
	const l = (level || '').toLowerCase();
	if (l === 'info') return 'info';
	if (l === 'warn' || l === 'warning') return 'warning';
	if (l === 'error') return 'error';
	if (l === 'debug') return 'neutral';
	return 'neutral';
}

/** Entity cell text for an activity row ('Service: name' / raw entity fallback). */
export function entityTarget(item: ActivityItem, services: Service[], projects: Project[]): string | null {
	if (item.serviceId) return `Service: ${resolveServiceName(item.serviceId, services)}`;
	if (item.projectId) return `Project: ${resolveProjectName(item.projectId, projects)}`;
	if (item.entity) return item.entityId ? `${item.entity}: ${shortId(item.entityId)}` : item.entity;
	return null;
}

export function hasEntityLink(item: ActivityItem): boolean {
	return Boolean(item.serviceId || item.projectId);
}

/** Deep link into the owning service / project page. */
export function openEntity(item: ActivityItem, services: Service[]): void {
	if (item.serviceId && item.projectId) {
		goto(`/projects/${item.projectId}/services/${item.serviceId}`);
		return;
	}
	if (item.serviceId) {
		const svc = services.find((s) => s.id === item.serviceId);
		if (svc?.projectId) {
			goto(`/projects/${svc.projectId}/services/${item.serviceId}`);
			return;
		}
	}
	if (item.projectId) {
		goto(`/projects/${item.projectId}`);
	}
}
