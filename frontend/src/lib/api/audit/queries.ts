import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { auditApi } from './api';
import { auditKeys } from './keys';
import type { AuditLog } from './types';

export function createAuditQuery(
	category?: () => string | undefined
): CreateQueryResult<AuditLog[], Error> {
	return createQuery(() => {
		const cat = category?.();
		return {
			queryKey: auditKeys.list(cat),
			queryFn: () => auditApi.list(cat),
			staleTime: 10000
		};
	});
}
