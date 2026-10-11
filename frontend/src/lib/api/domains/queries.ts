import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { domainsApi } from './api';
import { domainKeys } from './keys';
import type { Domain } from './types';

export function createDomainsQuery(): CreateQueryResult<Domain[], Error> {
	return createQuery(() => ({
		queryKey: domainKeys.all,
		queryFn: () => domainsApi.list(),
		staleTime: 10000
	}));
}
