import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { authApi } from './api';
import { authKeys } from './keys';
import type { SetupPayload, LoginPayload } from './types';

export function createSetupMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (payload: SetupPayload) => authApi.setup(payload),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: authKeys.status });
		}
	}));
}

export function createLoginMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (payload: LoginPayload) => authApi.login(payload),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: authKeys.status });
			queryClient.invalidateQueries({ queryKey: authKeys.me });
		}
	}));
}

export function createLogoutMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: () => authApi.logout(),
		onSuccess: () => {
			queryClient.clear();
		}
	}));
}
