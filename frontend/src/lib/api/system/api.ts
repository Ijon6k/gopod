import apiClient from '../client';
import type {
	SystemInfoResponse,
	ContainerStatResponse,
	SystemDiskUsage,
	SystemLingerResponse,
	GCPreview,
	GCResult,
	KeyStatusResponse,
	MetricSample,
	PanelSettings,
	AdminUserSummary,
	NotificationChannel,
	UpdateCheckResponse,
	UpdateStatus,
	VersionInfoResponse
} from './types';
import type { SessionSummary, UserPermissions, ProjectScope } from '../auth/types';

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

	async metricsHistory(range: '1h' | '6h' | '24h' | '7d' = '1h'): Promise<MetricSample[]> {
		const res = await apiClient.get<MetricSample[]>('/system/metrics/history', {
			params: { range }
		});
		return res.data;
	},

	async serviceMetricsHistory(
		serviceId: string,
		range: '1h' | '6h' | '24h' | '7d' = '1h'
	): Promise<MetricSample[]> {
		const res = await apiClient.get<MetricSample[]>(
			`/services/${encodeURIComponent(serviceId)}/metrics/history`,
			{ params: { range } }
		);
		return res.data;
	},

	async getPanelSettings(): Promise<PanelSettings> {
		const res = await apiClient.get<PanelSettings>('/system/panel-settings');
		return res.data;
	},

	async updatePanelSettings(data: PanelSettings): Promise<PanelSettings> {
		const res = await apiClient.put<PanelSettings>('/system/panel-settings', data);
		return res.data;
	},

	async listUsers(): Promise<AdminUserSummary[]> {
		const res = await apiClient.get<AdminUserSummary[]>('/users');
		return res.data;
	},

	async createUser(data: {
		name: string;
		email: string;
		password: string;
		role: 'admin' | 'member';
		permissions?: UserPermissions;
		projectIds?: string[];
		projectScopes?: ProjectScope[];
	}): Promise<AdminUserSummary> {
		const res = await apiClient.post<AdminUserSummary>('/users', data);
		return res.data;
	},

	async updateUser(
		id: string,
		data: {
			name?: string;
			role?: 'admin' | 'member';
			permissions?: UserPermissions;
			password?: string;
			projectIds?: string[];
			projectScopes?: ProjectScope[];
		}
	): Promise<AdminUserSummary> {
		const res = await apiClient.put<AdminUserSummary>(`/users/${encodeURIComponent(id)}`, data);
		return res.data;
	},

	async deleteUser(id: string): Promise<void> {
		await apiClient.delete(`/users/${encodeURIComponent(id)}`);
	},

	async listAllSessions(): Promise<SessionSummary[]> {
		const res = await apiClient.get<SessionSummary[]>('/users/sessions');
		return res.data;
	},

	async revokeAdminSession(id: string): Promise<{ success: boolean; revokedCurrent: boolean }> {
		const res = await apiClient.delete<{ success: boolean; revokedCurrent: boolean }>(
			`/users/sessions/${encodeURIComponent(id)}`
		);
		return res.data;
	},

	async version(): Promise<VersionInfoResponse> {
		const res = await apiClient.get<VersionInfoResponse>('/system/version');
		return res.data;
	},

	async checkUpdates(channel?: 'latest' | 'dev', force = false): Promise<UpdateCheckResponse> {
		const res = await apiClient.get<UpdateCheckResponse>('/system/updates', {
			params: {
				...(channel ? { channel } : {}),
				...(force ? { force: 'true' } : {})
			}
		});
		return res.data;
	},

	async triggerUpdate(payload: {
		tag?: string;
		channel?: 'latest' | 'dev';
		confirmSwitchFromDev?: boolean;
	}): Promise<UpdateStatus> {
		const res = await apiClient.post<UpdateStatus>('/system/update', payload);
		return res.data;
	},

	async listNotifications(): Promise<NotificationChannel[]> {
		const res = await apiClient.get<NotificationChannel[]>('/notifications');
		return res.data;
	},

	async createNotification(data: {
		name: string;
		provider: 'discord' | 'telegram';
		webhookUrl?: string;
		botToken?: string;
		chatId?: string;
		events: string[];
		enabled?: boolean;
	}): Promise<NotificationChannel> {
		const res = await apiClient.post<NotificationChannel>('/notifications', data);
		return res.data;
	},

	async updateNotification(
		id: string,
		data: {
			name?: string;
			webhookUrl?: string;
			botToken?: string;
			chatId?: string;
			events?: string[];
			enabled?: boolean;
		}
	): Promise<NotificationChannel> {
		const res = await apiClient.put<NotificationChannel>(
			`/notifications/${encodeURIComponent(id)}`,
			data
		);
		return res.data;
	},

	async deleteNotification(id: string): Promise<void> {
		await apiClient.delete(`/notifications/${encodeURIComponent(id)}`);
	},

	async testNotification(id: string): Promise<{ message: string }> {
		const res = await apiClient.post<{ message: string }>(
			`/notifications/${encodeURIComponent(id)}/test`
		);
		return res.data;
	},

	streamStats(
		onData: (data: { system?: any; stats?: any[] }) => void,
		onError?: (error: Event) => void
	): () => void {
		if (typeof window === 'undefined') return () => {};
		const es = new EventSource('/api/stats/stream', { withCredentials: true });
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
