import type { SSHKey, ContainerRegistry, PodmanSecret } from '$lib/types';
import { api } from '$lib/api';

export class CredentialsDomainStore {
	sshKeys = $state<SSHKey[]>([]);
	registries = $state<ContainerRegistry[]>([]);
	podmanSecrets = $state<PodmanSecret[]>([]);

	async fetchCredentials() {
		try {
			const [keys, regs, secs] = await Promise.all([
				api.credentials.sshKeys.list().catch(() => []),
				api.credentials.registries.list().catch(() => []),
				api.credentials.secrets.list().catch(() => [])
			]);
			if (Array.isArray(keys)) this.sshKeys = keys;
			if (Array.isArray(regs)) this.registries = regs;
			if (Array.isArray(secs)) this.podmanSecrets = secs;
		} catch (err) {
			console.warn('[CredentialsDomainStore] Failed to fetch credentials:', err);
		}
	}

	async addSSHKey(key: Omit<SSHKey, 'id' | 'createdAt'>): Promise<SSHKey> {
		const created = await api.credentials.sshKeys.create(key);
		this.sshKeys.unshift(created);
		return created;
	}

	async deleteSSHKey(id: string) {
		await api.credentials.sshKeys.delete(id);
		this.sshKeys = this.sshKeys.filter((k) => k.id !== id);
	}

	async addRegistry(reg: Omit<ContainerRegistry, 'id' | 'createdAt'>): Promise<ContainerRegistry> {
		const created = await api.credentials.registries.create(reg);
		this.registries.unshift(created);
		return created;
	}

	async deleteRegistry(id: string) {
		await api.credentials.registries.delete(id);
		this.registries = this.registries.filter((r) => r.id !== id);
	}

	async addSecret(name: string, value = ''): Promise<PodmanSecret> {
		const created = await api.credentials.secrets.create({ name, value, driver: 'file' } as any);
		this.podmanSecrets.unshift(created);
		return created;
	}
}
