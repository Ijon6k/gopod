import apiClient from '../client';
import type {
	SystemInfoResponse,
	ContainerStatResponse,
	SystemDiskUsage,
	SystemLingerResponse,
	GCPreview,
	GCResult,
	KeyStatusResponse
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

	async reloadQuadlets(): Promise<{ message: string }> {
		const res = await apiClient.post<{ message: string }>('/quadlets/reload');
		return res.data;
	},

	async getKeyStatus(): Promise<KeyStatusResponse> {
		const res = await apiClient.get<KeyStatusResponse>('/system/security/key-status');
		return res.data;
	},

	async listBackups(): Promise<import('../storage/types').VolumeSnapshot[]> {
		const res = await apiClient.get<import('../storage/types').VolumeSnapshot[]>('/system/backups');
		return res.data;
	},

	/** Runs the backend offline integrity inspection of one stored archive. */
	async verifyBackup(id: string): Promise<import('./types').SystemBackupVerification> {
		const res = await apiClient.get<import('./types').SystemBackupVerification>(
			`/system/backups/${encodeURIComponent(id)}/verify`
		);
		return res.data;
	},

	async createBackup(destinationId?: string): Promise<import('../storage/types').VolumeSnapshot> {
		const res = await apiClient.post<import('../storage/types').VolumeSnapshot>('/system/backup', {
			destinationId
		});
		return res.data;
	},

	async getBackupSchedule(): Promise<import('../storage/types').VolumeBackupSchedule | null> {
		const res = await apiClient.get<import('../storage/types').VolumeBackupSchedule | null>(
			'/system/backup/schedule'
		);
		return res.data;
	},

	async setBackupSchedule(data: {
		cron: string;
		retentionCount: number;
		destinationId?: string;
		enabled: boolean;
	}): Promise<import('../storage/types').VolumeBackupSchedule> {
		const res = await apiClient.post<import('../storage/types').VolumeBackupSchedule>(
			'/system/backup/schedule',
			data
		);
		return res.data;
	},

	async gcPreview(): Promise<GCPreview> {
		const res = await apiClient.get<GCPreview>('/system/gc/preview');
		return res.data;
	},

	async runGC(scope: string, includeVolumes = false, volumes: string[] = []): Promise<GCResult> {
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
			// Close on the first error; EventSource auto-reconnects while the
			// caller also refetches, which produced parallel reconnect plus
			// refetch storms after any transport drop. The caller now owns the
			// reconnect (backoff) entirely.
			es.onerror = (event) => {
				es.close();
				onError(event);
			};
		}
		return () => es.close();
	}
};

export default systemApi;
