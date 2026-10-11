import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { runtimeApi } from './api';
import { runtimeKeys } from './keys';

export function createStartContainerMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => runtimeApi.containers.start(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: runtimeKeys.containers });
		}
	}));
}

export function createStopContainerMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => runtimeApi.containers.stop(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: runtimeKeys.containers });
		}
	}));
}

export function createRestartContainerMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => runtimeApi.containers.restart(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: runtimeKeys.containers });
		}
	}));
}

export function createDeleteContainerMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: ({ id, force }: { id: string; force?: boolean }) =>
			runtimeApi.containers.delete(id, force),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: runtimeKeys.containers });
		}
	}));
}

export function createPruneImagesMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (all?: boolean) => runtimeApi.images.prune(all),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: runtimeKeys.images });
		}
	}));
}

export function createPruneVolumesMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: () => runtimeApi.volumes.prune(),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: runtimeKeys.volumes });
		}
	}));
}
