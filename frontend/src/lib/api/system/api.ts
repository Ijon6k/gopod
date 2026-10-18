import apiClient from '../client';
import type {
	SystemInfoResponse,
	ContainerStatResponse,
	SystemDiskUsage,
	SystemLingerResponse,
	GCPreview,
	GCResult
} from './types';

export const systemApi = {
	async health(): Promise<{ status: string; timestamp: string; socket: string }> {
		const res = await apiClient.get('/health');
		return res.data;
	},

	async info(): Promise<SystemInfoResponse> {
		const res = await apiClient.get<SystemInfoResponse>('/system');
		return res.data;
	},

	async linger(): Promise<SystemLingerResponse> {
		const res = await apiClient.get<SystemLingerResponse>('/system/linger');
		return res.data;
	},

	async getLinger(): Promise<SystemLingerResponse> {
		const res = await apiClient.get<SystemLingerResponse>('/system/linger');
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

	async gcPreview(): Promise<GCPreview> {
		const res = await apiClient.get<GCPreview>('/system/gc/preview');
		return res.data;
	},

	async runGC(
		scope: string,
		includeVolumes = false,
		volumes: string[] = []
	): Promise<GCResult> {
		const res = await apiClient.post<GCResult>('/system/gc', { scope, includeVolumes, volumes });
		return res.data;
	},

	streamStats(
		onData: (data: { system?: any; stats?: any[] }) => void,
		onError?: (error: Event) => void
	): () => void {
		if (typeof window === 'undefined') return () => {};
		const token = typeof localStorage !== 'undefined' ? localStorage.getItem('gopod_token') : null;
		const url = token
			? `/api/stats/stream?token=${encodeURIComponent(token)}`
			: '/api/stats/stream';
		const es = new EventSource(url, { withCredentials: true });
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
