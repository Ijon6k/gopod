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
		try {
			const created = await api.credentials.sshKeys.create(key);
			this.sshKeys.unshift(created);
			return created;
		} catch (err) {
			const fallback: SSHKey = {
				...key,
				id: `key-${Date.now()}`,
				createdAt: new Date().toISOString()
			};
			this.sshKeys.unshift(fallback);
			return fallback;
		}
	}

	async deleteSSHKey(id: string) {
		this.sshKeys = this.sshKeys.filter((k) => k.id !== id);
		await api.credentials.sshKeys.delete(id).catch((err) => console.warn(err));
	}

	async addRegistry(reg: Omit<ContainerRegistry, 'id' | 'createdAt'>): Promise<ContainerRegistry> {
		try {
			const created = await api.credentials.registries.create(reg);
			this.registries.unshift(created);
			return created;
		} catch (err) {
			const fallback: ContainerRegistry = {
				...reg,
				id: `reg-${Date.now()}`,
				createdAt: new Date().toISOString()
			};
			this.registries.unshift(fallback);
			return fallback;
		}
	}

	async deleteRegistry(id: string) {
		this.registries = this.registries.filter((r) => r.id !== id);
		await api.credentials.registries.delete(id).catch((err) => console.warn(err));
	}

	async addSecret(name: string, value = ''): Promise<PodmanSecret> {
		try {
			const created = await api.credentials.secrets.create({ name, value, driver: 'file' } as any);
			this.podmanSecrets.unshift(created);
			return created;
		} catch (err) {
			const fallback: PodmanSecret = {
				id: `sec-${Date.now()}`,
				name,
				createdAt: new Date().toISOString(),
				driver: 'file'
			};
			this.podmanSecrets.unshift(fallback);
			return fallback;
		}
	}
}
