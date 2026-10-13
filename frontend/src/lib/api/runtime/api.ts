import apiClient from '../client';
import type { Pod, Image, Volume, Network, PodmanContainerItem, ContainerLogsResponse, QuadletUnit } from './types';

export const runtimeApi = {
	containers: {
		async list(): Promise<PodmanContainerItem[]> {
			const res = await apiClient.get<PodmanContainerItem[]>('/containers');
			return res.data;
		},

		async start(id: string): Promise<{ message: string; id: string }> {
			const res = await apiClient.post<{ message: string; id: string }>(`/containers/${id}/start`);
			return res.data;
		},

		async stop(id: string): Promise<{ message: string; id: string }> {
			const res = await apiClient.post<{ message: string; id: string }>(`/containers/${id}/stop`);
			return res.data;
		},

		async restart(id: string): Promise<{ message: string; id: string }> {
			const res = await apiClient.post<{ message: string; id: string }>(
				`/containers/${id}/restart`
			);
			return res.data;
		},

		async delete(id: string, force = false): Promise<{ message: string; id: string }> {
			const res = await apiClient.delete<{ message: string; id: string }>(`/containers/${id}`, {
				params: { force }
			});
			return res.data;
		},

		async prune(): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>('/containers/prune');
			return res.data;
		},

		async logs(id: string, tail = 100): Promise<ContainerLogsResponse> {
			const res = await apiClient.get<ContainerLogsResponse>(`/containers/${id}/logs`, {
				params: { tail }
			});
			return res.data;
		},

		getTerminalWsUrl(id: string, cols = 80, rows = 24, cmd = 'sh'): string {
			if (typeof window === 'undefined') return '';
			const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
			const token =
				typeof localStorage !== 'undefined' ? localStorage.getItem('gopod_token') || '' : '';
			const tokenParam = token ? `&token=${encodeURIComponent(token)}` : '';
			return `${protocol}//${window.location.host}/api/containers/${encodeURIComponent(id)}/exec?cmd=${encodeURIComponent(cmd)}&cols=${cols}&rows=${rows}${tokenParam}`;
		}
	},

	pods: {
		async list(): Promise<Pod[]> {
			const res = await apiClient.get<Pod[]>('/pods');
			return res.data;
		},

		async create(pod: { name: string; ports?: string[]; network?: string; labels?: Record<string, string> }): Promise<{ id: string; name: string }> {
			const res = await apiClient.post<{ id: string; name: string }>('/pods', pod);
			return res.data;
		},

		async start(id: string): Promise<{ status: string; id: string }> {
			const res = await apiClient.post<{ status: string; id: string }>(`/pods/${id}/start`);
			return res.data;
		},

		async stop(id: string): Promise<{ status: string; id: string }> {
			const res = await apiClient.post<{ status: string; id: string }>(`/pods/${id}/stop`);
			return res.data;
		},

		async restart(id: string): Promise<{ status: string; id: string }> {
			const res = await apiClient.post<{ status: string; id: string }>(`/pods/${id}/restart`);
			return res.data;
		},

		async delete(id: string, force = false): Promise<{ status: string; id: string }> {
			const res = await apiClient.delete<{ status: string; id: string }>(`/pods/${id}`, {
				params: { force }
			});
			return res.data;
		}
	},

	images: {
		async list(): Promise<Image[]> {
			const res = await apiClient.get<Image[]>('/images');
			return res.data;
		},

		async prune(all = false): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>('/images/prune', null, {
				params: { all }
			});
			return res.data;
		}
	},

	volumes: {
		async list(): Promise<Volume[]> {
			const res = await apiClient.get<Volume[]>('/volumes');
			return res.data;
		},

		async prune(): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>('/volumes/prune');
			return res.data;
		}
	},

	networks: {
		async list(): Promise<Network[]> {
			const res = await apiClient.get<Network[]>('/networks');
			return res.data;
		}
	},

	quadlets: {
		async list(): Promise<QuadletUnit[]> {
			const res = await apiClient.get<QuadletUnit[]>('/quadlets');
			return res.data;
		},

		async start(name: string): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>(`/quadlets/${encodeURIComponent(name)}/start`);
			return res.data;
		},

		async stop(name: string): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>(`/quadlets/${encodeURIComponent(name)}/stop`);
			return res.data;
		},

		async restart(name: string): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>(`/quadlets/${encodeURIComponent(name)}/restart`);
			return res.data;
		},

		async reload(): Promise<{ message: string }> {
			const res = await apiClient.post<{ message: string }>('/quadlets/reload');
			return res.data;
		},

		async getContent(name: string): Promise<{ name: string; content: string }> {
			const res = await apiClient.get<{ name: string; content: string }>(
				`/quadlets/${encodeURIComponent(name)}/content`
			);
			return res.data;
		},

		async delete(name: string): Promise<{ message: string }> {
			const res = await apiClient.delete<{ message: string }>(
				`/quadlets/${encodeURIComponent(name)}`
			);
			return res.data;
		}
	}
};

export default runtimeApi;
