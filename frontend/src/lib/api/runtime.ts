import apiClient from './client';
import type { Container, Pod, Image, Volume, Network } from '$lib/types';

export interface PodmanContainerItem {
	id: string;
	names: string[];
	image: string;
	status: string;
	state: string;
	created: string;
	ports: string;
	stats?: {
		id: string;
		name: string;
		cpuPercent: number;
		memPercent: number;
		memDisplay: string;
		netDisplay: string;
		pids: number;
	};
}

export interface ContainerLogsResponse {
	id: string;
	logs: string;
	tail: number;
}

export const runtimeApi = {
	containers: {
		/**
		 * List all containers from Podman engine
		 */
		async list(): Promise<PodmanContainerItem[]> {
			const res = await apiClient.get<PodmanContainerItem[]>('/containers');
			return res.data;
		},

		/**
		 * Start a container by ID
		 */
		async start(id: string): Promise<{ message: string; id: string }> {
			const res = await apiClient.post<{ message: string; id: string }>(`/containers/${id}/start`);
			return res.data;
		},

		/**
		 * Stop a container by ID
		 */
		async stop(id: string): Promise<{ message: string; id: string }> {
			const res = await apiClient.post<{ message: string; id: string }>(`/containers/${id}/stop`);
			return res.data;
		},

		/**
		 * Restart a container by ID
		 */
		async restart(id: string): Promise<{ message: string; id: string }> {
			const res = await apiClient.post<{ message: string; id: string }>(
				`/containers/${id}/restart`
			);
			return res.data;
		},

		/**
		 * Delete/remove a container by ID
		 */
		async delete(id: string, force = false): Promise<{ message: string; id: string }> {
			const res = await apiClient.delete<{ message: string; id: string }>(`/containers/${id}`, {
				params: { force }
			});
			return res.data;
		},

		/**
		 * Fetch container stdout/stderr logs
		 */
		async logs(id: string, tail = 100): Promise<ContainerLogsResponse> {
			const res = await apiClient.get<ContainerLogsResponse>(`/containers/${id}/logs`, {
				params: { tail }
			});
			return res.data;
		}
	},

	pods: {
		/**
		 * List all Podman pods
		 */
		async list(): Promise<Pod[]> {
			const res = await apiClient.get<Pod[]>('/pods');
			return res.data;
		}
	},

	images: {
		/**
		 * List local container images
		 */
		async list(): Promise<Image[]> {
			const res = await apiClient.get<Image[]>('/images');
			return res.data;
		},

		/**
		 * Prune unused images
		 */
		async prune(all = false): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>('/images/prune', null, {
				params: { all }
			});
			return res.data;
		}
	},

	volumes: {
		/**
		 * List Podman storage volumes
		 */
		async list(): Promise<Volume[]> {
			const res = await apiClient.get<Volume[]>('/volumes');
			return res.data;
		},

		/**
		 * Prune unused storage volumes
		 */
		async prune(): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>('/volumes/prune');
			return res.data;
		}
	},

	networks: {
		/**
		 * List Podman container networks
		 */
		async list(): Promise<Network[]> {
			const res = await apiClient.get<Network[]>('/networks');
			return res.data;
		}
	}
};

export default runtimeApi;
