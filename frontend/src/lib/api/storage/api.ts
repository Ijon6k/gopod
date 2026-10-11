import apiClient from '../client';
import type { VolumeSnapshot, VolumeBackupSchedule } from './types';

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

	async listSchedules(projectId?: string): Promise<VolumeBackupSchedule[]> {
		const res = await apiClient.get<VolumeBackupSchedule[]>('/volumes/schedules', {
			params: projectId ? { projectId } : undefined
		});
		return res.data;
	},

	async toggleSchedule(id: string): Promise<VolumeBackupSchedule> {
		const res = await apiClient.post<VolumeBackupSchedule>(`/volumes/schedules/${id}/toggle`);
		return res.data;
	},

	async deleteSchedule(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/volumes/schedules/${id}`);
		return res.data;
	}
};

export default storageApi;
