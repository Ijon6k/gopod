import apiClient from './client';
import type { SSHKey, ContainerRegistry, PodmanSecret } from '$lib/types';

export const credentialsApi = {
	sshKeys: {
		async list(): Promise<SSHKey[]> {
			const res = await apiClient.get<SSHKey[]>('/credentials/ssh-keys');
			return res.data;
		},
		async create(key: Partial<SSHKey>): Promise<SSHKey> {
			const res = await apiClient.post<SSHKey>('/credentials/ssh-keys', key);
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
