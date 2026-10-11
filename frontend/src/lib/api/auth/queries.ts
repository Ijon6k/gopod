import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { authApi } from './api';
import { authKeys } from './keys';
import type { AuthStatus, User } from './types';

export function createAuthStatusQuery(): CreateQueryResult<AuthStatus, Error> {
	return createQuery(() => ({
		queryKey: authKeys.status,
		queryFn: () => authApi.getStatus(),
		staleTime: 30000
	}));
}

export function createMeQuery(): CreateQueryResult<{ user: User }, Error> {
	return createQuery(() => ({
		queryKey: authKeys.me,
		queryFn: () => authApi.getMe(),
		staleTime: 60000
	}));
}
