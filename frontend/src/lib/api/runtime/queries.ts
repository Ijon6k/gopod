import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { runtimeApi } from './api';
import { runtimeKeys } from './keys';
import type { Pod, Image, Volume, Network, PodmanContainerItem, ContainerLogsResponse } from './types';

export function createContainersQuery(): CreateQueryResult<PodmanContainerItem[], Error> {
	return createQuery(() => ({
		queryKey: runtimeKeys.containers,
		queryFn: () => runtimeApi.containers.list(),
		staleTime: 3000
	}));
}

export function createContainerLogsQuery(
	containerId: () => string,
	tail = 100
): CreateQueryResult<ContainerLogsResponse, Error> {
	return createQuery(() => {
		const id = containerId();
		return {
			queryKey: runtimeKeys.containerLogs(id),
			queryFn: () => runtimeApi.containers.logs(id, tail),
			enabled: Boolean(id),
			staleTime: 2000
		};
	});
}

export function createPodsQuery(): CreateQueryResult<Pod[], Error> {
	return createQuery(() => ({
		queryKey: runtimeKeys.pods,
		queryFn: () => runtimeApi.pods.list(),
		staleTime: 5000
	}));
}

export function createImagesQuery(): CreateQueryResult<Image[], Error> {
	return createQuery(() => ({
		queryKey: runtimeKeys.images,
		queryFn: () => runtimeApi.images.list(),
		staleTime: 10000
	}));
}

export function createVolumesQuery(): CreateQueryResult<Volume[], Error> {
	return createQuery(() => ({
		queryKey: runtimeKeys.volumes,
		queryFn: () => runtimeApi.volumes.list(),
		staleTime: 10000
	}));
}

export function createNetworksQuery(): CreateQueryResult<Network[], Error> {
	return createQuery(() => ({
		queryKey: runtimeKeys.networks,
		queryFn: () => runtimeApi.networks.list(),
		staleTime: 10000
	}));
}
