import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { trafficApi } from './api';
import { trafficKeys } from './keys';
import type { CaddyAccessLog } from './types';

export function createTrafficRequestsQuery(
	limit?: () => number | undefined
): CreateQueryResult<CaddyAccessLog[], Error> {
	return createQuery(() => {
		const l = limit?.() ?? 50;
		return {
			queryKey: trafficKeys.requests(l),
			queryFn: () => trafficApi.getRequests(l),
			staleTime: 3000,
			refetchInterval: 5000
		};
	});
}
