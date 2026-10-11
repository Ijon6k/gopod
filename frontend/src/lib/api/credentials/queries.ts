import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { credentialsApi } from './api';
import { credentialKeys } from './keys';
import type { SSHKey, ContainerRegistry, PodmanSecret } from './types';

export function createSSHKeysQuery(): CreateQueryResult<SSHKey[], Error> {
	return createQuery(() => ({
		queryKey: credentialKeys.sshKeys,
		queryFn: () => credentialsApi.sshKeys.list(),
		staleTime: 15000
	}));
}

export function createRegistriesQuery(): CreateQueryResult<ContainerRegistry[], Error> {
	return createQuery(() => ({
		queryKey: credentialKeys.registries,
		queryFn: () => credentialsApi.registries.list(),
		staleTime: 15000
	}));
}

export function createSecretsQuery(): CreateQueryResult<PodmanSecret[], Error> {
	return createQuery(() => ({
		queryKey: credentialKeys.secrets,
		queryFn: () => credentialsApi.secrets.list(),
		staleTime: 15000
	}));
}
