import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { api } from '$lib/api';
import type { Project, Service, Deployment, Container } from '$lib/types';

/**
 * Reactive TanStack Query for all projects.
 */
export function createProjectsQuery(): CreateQueryResult<Project[], Error> {
	return createQuery(() => ({
		queryKey: ['projects'],
		queryFn: () => api.projects.list(),
		staleTime: 10000
	}));
}

/**
 * Reactive TanStack Query for services (optionally filtered by project).
 */
export function createServicesQuery(projectId?: string): CreateQueryResult<Service[], Error> {
	return createQuery(() => ({
		queryKey: ['services', projectId || 'all'],
		queryFn: () => api.services.list(projectId),
		staleTime: 5000
	}));
}

/**
 * Reactive TanStack Query for service deployments with smart adaptive polling.
 * Automatically polls every 2.5s while building/deploying, then idles.
 */
export function createDeploymentsQuery(
	serviceId: () => string
): CreateQueryResult<Deployment[], Error> {
	return createQuery(() => {
		const sId = serviceId();
		return {
			queryKey: ['deployments', sId],
			queryFn: async () => {
				if (!sId) return [];
				const deps = await api.services.deployments(sId);
				return deps || [];
			},
			enabled: !!sId,
			staleTime: 2000,
			refetchInterval: (query) => {
				const data = query.state.data as Deployment[] | undefined;
				const hasActive = data?.some((d) => d.status === 'building' || d.status === 'deploying');
				return hasActive ? 2500 : false;
			}
		};
	});
}

/**
 * Reactive TanStack Query for runtime containers.
 */
export function createContainersQuery(): CreateQueryResult<Container[], Error> {
	return createQuery(() => ({
		queryKey: ['containers'],
		queryFn: () => api.runtime.containers.list(),
		staleTime: 3000
	}));
}

/**
 * Reactive TanStack Query for host system telemetry.
 */
export function createSystemQuery(): CreateQueryResult<any, Error> {
	return createQuery(() => ({
		queryKey: ['system-info'],
		queryFn: () => api.system.info(),
		staleTime: 15000,
		refetchInterval: 30000
	}));
}
