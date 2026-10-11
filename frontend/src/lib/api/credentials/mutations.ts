import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { credentialsApi } from './api';
import { credentialKeys } from './keys';
import type { SSHKey, ContainerRegistry, PodmanSecret } from './types';

export function createCreateSSHKeyMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (key: Partial<SSHKey>) => credentialsApi.sshKeys.create(key),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: credentialKeys.sshKeys });
		}
	}));
}

export function createDeleteSSHKeyMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => credentialsApi.sshKeys.delete(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: credentialKeys.sshKeys });
		}
	}));
}

export function createCreateRegistryMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (reg: Partial<ContainerRegistry>) => credentialsApi.registries.create(reg),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: credentialKeys.registries });
		}
	}));
}

export function createDeleteRegistryMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => credentialsApi.registries.delete(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: credentialKeys.registries });
		}
	}));
}

export function createCreateSecretMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (sec: Partial<PodmanSecret>) => credentialsApi.secrets.create(sec),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: credentialKeys.secrets });
		}
	}));
}

export function createDeleteSecretMutation() {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: string) => credentialsApi.secrets.delete(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: credentialKeys.secrets });
		}
	}));
}
