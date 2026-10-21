import type { SSHKey, ContainerRegistry, PodmanSecret, GitToken } from '$lib/types';
import { api } from '$lib/api';

export class CredentialsDomainStore {
	sshKeys = $state<SSHKey[]>([]);
	registries = $state<ContainerRegistry[]>([]);
	podmanSecrets = $state<PodmanSecret[]>([]);
	gitTokens = $state<GitToken[]>([]);

	async fetchCredentials() {
		try {
			const [keys, regs, secs, tokens] = await Promise.all([
				api.credentials.sshKeys.list().catch(() => []),
				api.credentials.registries.list().catch(() => []),
				api.credentials.secrets.list().catch(() => []),
				api.credentials.gitTokens.list().catch(() => [])
			]);
			if (Array.isArray(keys)) this.sshKeys = keys;
			if (Array.isArray(regs)) this.registries = regs;
			if (Array.isArray(secs)) this.podmanSecrets = secs;
			if (Array.isArray(tokens)) this.gitTokens = tokens;
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

	async addGitToken(token: Omit<GitToken, 'id' | 'createdAt'>): Promise<GitToken> {
		const created = await api.credentials.gitTokens.create(token);
		this.gitTokens.unshift(created);
		return created;
	}

	async deleteGitToken(id: string) {
		await api.credentials.gitTokens.delete(id);
		this.gitTokens = this.gitTokens.filter((t) => t.id !== id);
	}
}
