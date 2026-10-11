import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { domainsApi } from './api';
import { domainKeys } from './keys';
import type { Domain } from './types';

export function createCreateDomainMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (domain: Partial<Domain>) => domainsApi.create(domain),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: domainKeys.all });
		}
	}));
}

export function createUpdateDomainMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: ({ id, data }: { id: string; data: Partial<Domain> }) =>
			domainsApi.update(id, data),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: domainKeys.all });
		}
	}));
}

export function createDeleteDomainMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => domainsApi.delete(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: domainKeys.all });
		}
	}));
}
