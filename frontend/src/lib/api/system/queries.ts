import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { systemApi } from './api';
import { systemKeys } from './keys';
import type { SystemInfoResponse, ContainerStatResponse, SystemDiskUsage } from './types';

export function createSystemQuery(): CreateQueryResult<SystemInfoResponse, Error> {
	return createQuery(() => ({
		queryKey: systemKeys.info,
		queryFn: () => systemApi.info(),
		staleTime: 15000,
		refetchInterval: 30000
	}));
}

export function createSystemStatsQuery(): CreateQueryResult<ContainerStatResponse[], Error> {
	return createQuery(() => ({
		queryKey: systemKeys.stats,
		queryFn: () => systemApi.stats(),
		staleTime: 2500,
		refetchInterval: 5000
	}));
}

export function createSystemDfQuery(): CreateQueryResult<SystemDiskUsage[], Error> {
	return createQuery(() => ({
		queryKey: systemKeys.df,
		queryFn: () => systemApi.df(),
		staleTime: 10000
	}));
}

export function createHealthQuery(): CreateQueryResult<
	{ status: string; timestamp: string; socket: string },
	Error
> {
	return createQuery(() => ({
		queryKey: systemKeys.health,
		queryFn: () => systemApi.health(),
		staleTime: 10000
	}));
}
