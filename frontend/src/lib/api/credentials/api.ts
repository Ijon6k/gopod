import apiClient from '../client';
import type { SSHKey, ContainerRegistry, PodmanSecret, GitToken } from './types';

export const credentialsApi = {
	gitTokens: {
		async list(): Promise<GitToken[]> {
			const res = await apiClient.get<GitToken[]>('/credentials/git-tokens');
			return res.data;
		},
		async create(token: Partial<GitToken>): Promise<GitToken> {
			const res = await apiClient.post<GitToken>('/credentials/git-tokens', token);
			return res.data;
		},
		async delete(id: string): Promise<{ message: string }> {
			const res = await apiClient.delete<{ message: string }>(`/credentials/git-tokens/${id}`);
			return res.data;
		}
	},
	sshKeys: {
		async list(): Promise<SSHKey[]> {
			const res = await apiClient.get<SSHKey[]>('/credentials/ssh-keys');
			return res.data;
		},
		async create(key: Partial<SSHKey>): Promise<SSHKey> {
			const res = await apiClient.post<SSHKey>('/credentials/ssh-keys', key);
			return res.data;
		},
		async generate(name: string): Promise<SSHKey> {
			const res = await apiClient.post<SSHKey>('/credentials/ssh-keys/generate', { name });
			return res.data;
		},
		async delete(id: string): Promise<{ message: string }> {
			const res = await apiClient.delete<{ message: string }>(`/credentials/ssh-keys/${id}`);
			return res.data;
		}
	},

	registries: {
		async list(): Promise<ContainerRegistry[]> {
			const res = await apiClient.get<ContainerRegistry[]>('/credentials/registries');
			return res.data;
		},
		async create(reg: Partial<ContainerRegistry>): Promise<ContainerRegistry> {
			const res = await apiClient.post<ContainerRegistry>('/credentials/registries', reg);
			return res.data;
		},
		async delete(id: string): Promise<{ message: string }> {
			const res = await apiClient.delete<{ message: string }>(`/credentials/registries/${id}`);
			return res.data;
		}
	},

	secrets: {
		async list(): Promise<PodmanSecret[]> {
			const res = await apiClient.get<PodmanSecret[]>('/credentials/secrets');
			return res.data;
		},
		async create(sec: Partial<PodmanSecret>): Promise<PodmanSecret> {
			const res = await apiClient.post<PodmanSecret>('/credentials/secrets', sec);
			return res.data;
		},
		async delete(id: string): Promise<{ message: string }> {
			const res = await apiClient.delete<{ message: string }>(`/credentials/secrets/${id}`);
			return res.data;
		}
	}
};

export default credentialsApi;
