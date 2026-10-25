import apiClient from '../client';
import type { VolumeSnapshot, VolumeBackupSchedule, BackupDestination } from './types';

export const storageApi = {
	async listSnapshots(projectId?: string): Promise<VolumeSnapshot[]> {
		const res = await apiClient.get<VolumeSnapshot[]>('/volumes/snapshots', {
			params: projectId ? { projectId } : undefined
		});
		return res.data;
	},

	async createSnapshot(data: {
		volumeName: string;
		projectId: string;
		serviceId: string;
	}): Promise<VolumeSnapshot> {
		const res = await apiClient.post<VolumeSnapshot>('/volumes/snapshot', data);
		return res.data;
	},

	async deleteSnapshot(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/volumes/snapshots/${id}`);
		return res.data;
	},

	async restoreSnapshot(id: string): Promise<{ message: string }> {
		const res = await apiClient.post<{ message: string }>(`/volumes/snapshots/${id}/restore`);
		return res.data;
	},

	getDownloadUrl(id: string): string {
		return `/api/volumes/snapshots/${id}/download`;
	},

	async listSchedules(projectId?: string): Promise<VolumeBackupSchedule[]> {
		const res = await apiClient.get<VolumeBackupSchedule[]>('/volumes/schedules', {
			params: projectId ? { projectId } : undefined
		});
		return res.data;
	},

	async createSchedule(data: {
		projectId: string;
		volumeName: string;
		cron: string;
		label?: string;
		retentionCount: number;
		destinationId?: string;
	}): Promise<VolumeBackupSchedule> {
		const res = await apiClient.post<VolumeBackupSchedule>('/volumes/schedules', data);
		return res.data;
	},

	async toggleSchedule(id: string): Promise<VolumeBackupSchedule> {
		const res = await apiClient.post<VolumeBackupSchedule>(`/volumes/schedules/${id}/toggle`);
		return res.data;
	},

	async deleteSchedule(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/volumes/schedules/${id}`);
		return res.data;
	},

	// ── S3-compatible destinations ──

	async listDestinations(): Promise<BackupDestination[]> {
		const res = await apiClient.get<BackupDestination[]>('/volumes/destinations');
		return res.data;
	},

	async createDestination(data: Omit<BackupDestination, 'id'>): Promise<BackupDestination> {
		const res = await apiClient.post<BackupDestination>('/volumes/destinations', data);
		return res.data;
	},

	async deleteDestination(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/volumes/destinations/${id}`);
		return res.data;
	},

	/** Probe a not-yet-saved destination (create form). */
	async testDestination(data: Omit<BackupDestination, 'id'>): Promise<void> {
		await apiClient.post('/volumes/destinations/test', data);
	},

	/** Probe a saved destination by id (optional fresh secret). */
	async testSavedDestination(id: string, secretKey?: string): Promise<void> {
		await apiClient.post(`/volumes/destinations/${id}/test`, { secretKey });
	}
};

export default storageApi;
