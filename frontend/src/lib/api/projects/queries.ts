import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { projectsApi } from './api';
import { projectKeys } from './keys';
import type { Project } from './types';

export function createProjectsQuery(): CreateQueryResult<Project[], Error> {
	return createQuery(() => ({
		queryKey: projectKeys.all,
		queryFn: () => projectsApi.list(),
		staleTime: 10000
	}));
}

export function createProjectQuery(id: () => string): CreateQueryResult<Project, Error> {
	return createQuery(() => {
		const projectId = id();
		return {
			queryKey: projectKeys.detail(projectId),
			queryFn: () => projectsApi.get(projectId),
			enabled: Boolean(projectId),
			staleTime: 10000
		};
	});
}
