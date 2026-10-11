import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { servicesApi } from './api';
import { serviceKeys } from './keys';
import type { Service } from './types';

export function createCreateServiceMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (service: Partial<Service>) => servicesApi.create(service),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: serviceKeys.all });
		}
	}));
}

export function createUpdateServiceMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: ({ id, data }: { id: string; data: Partial<Service> }) =>
			servicesApi.update(id, data),
		onSuccess: (_, variables) => {
			queryClient.invalidateQueries({ queryKey: serviceKeys.all });
			queryClient.invalidateQueries({ queryKey: serviceKeys.detail(variables.id) });
		}
	}));
}

export function createDeleteServiceMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: ({ id, deleteVolumes }: { id: string; deleteVolumes?: boolean }) =>
			servicesApi.delete(id, deleteVolumes),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: serviceKeys.all });
		}
	}));
}

export function createDeployServiceMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: ({
			id,
			trigger,
			commit,
			image
		}: {
			id: string;
			trigger?: string;
			commit?: string;
			image?: string;
		}) => servicesApi.deploy(id, trigger, commit, image),
		onSuccess: (_, variables) => {
			queryClient.invalidateQueries({ queryKey: serviceKeys.deployments(variables.id) });
			queryClient.invalidateQueries({ queryKey: serviceKeys.detail(variables.id) });
		}
	}));
}

export function createStartServiceMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => servicesApi.start(id),
		onSuccess: (_, id) => {
			queryClient.invalidateQueries({ queryKey: serviceKeys.all });
			queryClient.invalidateQueries({ queryKey: serviceKeys.detail(id) });
		}
	}));
}

export function createStopServiceMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => servicesApi.stop(id),
		onSuccess: (_, id) => {
			queryClient.invalidateQueries({ queryKey: serviceKeys.all });
			queryClient.invalidateQueries({ queryKey: serviceKeys.detail(id) });
		}
	}));
}

export function createRestartServiceMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => servicesApi.restart(id),
		onSuccess: (_, id) => {
			queryClient.invalidateQueries({ queryKey: serviceKeys.all });
			queryClient.invalidateQueries({ queryKey: serviceKeys.detail(id) });
		}
	}));
}
