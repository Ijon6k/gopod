import apiClient from '../client';
import type { SystemInfoResponse, ContainerStatResponse, SystemDiskUsage } from './types';

export const systemApi = {
	async health(): Promise<{ status: string; timestamp: string; socket: string }> {
		const res = await apiClient.get('/health');
		return res.data;
	},

	async info(): Promise<SystemInfoResponse> {
		const res = await apiClient.get<SystemInfoResponse>('/system');
		return res.data;
	},

	async stats(): Promise<ContainerStatResponse[]> {
		const res = await apiClient.get<ContainerStatResponse[]>('/stats');
		return res.data;
	},

	async prune(): Promise<{ message: string }> {
		const res = await apiClient.post<{ message: string }>('/system/prune');
		return res.data;
	},

	async df(): Promise<SystemDiskUsage[]> {
		const res = await apiClient.get<SystemDiskUsage[]>('/system/df');
		return res.data;
	},

	streamStats(
		onData: (data: { system?: any; stats?: any[] }) => void,
		onError?: (error: Event) => void
	): () => void {
		if (typeof window === 'undefined') return () => {};
		const es = new EventSource('/api/stats/stream');
		es.onmessage = (event) => {
			try {
				const parsed = JSON.parse(event.data);
				onData(parsed);
			} catch {}
		};
		if (onError) {
			es.onerror = onError;
		}
		return () => es.close();
	}
};

export default systemApi;
