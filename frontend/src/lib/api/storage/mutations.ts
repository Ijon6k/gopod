import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { storageApi } from './api';
import { storageKeys } from './keys';

export function createCreateSnapshotMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (data: { volumeName: string; projectId: string; serviceId: string }) =>
			storageApi.createSnapshot(data),
		onSuccess: (_, variables) => {
			queryClient.invalidateQueries({ queryKey: storageKeys.snapshots(variables.projectId) });
			queryClient.invalidateQueries({ queryKey: storageKeys.snapshots() });
		}
	}));
}

export function createDeleteSnapshotMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => storageApi.deleteSnapshot(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: storageKeys.all });
		}
	}));
}

export function createToggleScheduleMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => storageApi.toggleSchedule(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: storageKeys.all });
		}
	}));
}

export function createDeleteScheduleMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => storageApi.deleteSchedule(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: storageKeys.all });
		}
	}));
}
