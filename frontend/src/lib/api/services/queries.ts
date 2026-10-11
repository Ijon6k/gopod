import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { servicesApi } from './api';
import { serviceKeys } from './keys';
import type { Service, Deployment } from './types';

export function createServicesQuery(
	projectId?: () => string | undefined
): CreateQueryResult<Service[], Error> {
	return createQuery(() => {
		const pId = projectId?.();
		return {
			queryKey: serviceKeys.byProject(pId),
			queryFn: () => servicesApi.list(pId),
			staleTime: 5000
		};
	});
}

export function createServiceQuery(id: () => string): CreateQueryResult<Service, Error> {
	return createQuery(() => {
		const sId = id();
		return {
			queryKey: serviceKeys.detail(sId),
			queryFn: () => servicesApi.get(sId),
			enabled: Boolean(sId),
			staleTime: 5000
		};
	});
}

export function createDeploymentsQuery(
	serviceId: () => string
): CreateQueryResult<Deployment[], Error> {
	return createQuery(() => {
		const sId = serviceId();
		return {
			queryKey: serviceKeys.deployments(sId),
			queryFn: async () => {
				if (!sId) return [];
				const deps = await servicesApi.deployments(sId);
				return deps || [];
			},
			enabled: Boolean(sId),
			staleTime: 2000,
			refetchInterval: (query) => {
				const data = query.state.data as Deployment[] | undefined;
				const hasActive = data?.some((d) => d.status === 'building' || d.status === 'deploying');
				return hasActive ? 2500 : false;
			}
		};
	});
}
