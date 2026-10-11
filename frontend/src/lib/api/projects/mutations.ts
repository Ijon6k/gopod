import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { projectsApi } from './api';
import { projectKeys } from './keys';
import type { Project } from './types';

export function createCreateProjectMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (data: Partial<Project>) => projectsApi.create(data),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: projectKeys.all });
		}
	}));
}

export function createDeleteProjectMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => projectsApi.delete(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: projectKeys.all });
		}
	}));
}
