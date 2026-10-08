import apiClient from './client';
import type { VolumeSnapshot, VolumeBackupSchedule } from '$lib/types';

export const volumesApi = {
	/**
	 * List all volume snapshot archives
	 */
	async listSnapshots(projectId?: string): Promise<VolumeSnapshot[]> {
		const res = await apiClient.get<VolumeSnapshot[]>('/volumes/snapshots', {
			params: projectId ? { projectId } : undefined
		});
		return res.data;
	},

	/**
	 * Create a new compressed volume snapshot (.tar.zst)
	 */
	async createSnapshot(data: {
		volumeName: string;
		projectId: string;
		serviceId: string;
	}): Promise<VolumeSnapshot> {
		const res = await apiClient.post<VolumeSnapshot>('/volumes/snapshot', data);
		return res.data;
	},

	/**
	 * Delete a volume snapshot archive
	 */
	async deleteSnapshot(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/volumes/snapshots/${id}`);
		return res.data;
	},

	/**
	 * List automated volume backup schedules
	 */
	async listSchedules(projectId?: string): Promise<VolumeBackupSchedule[]> {
		const res = await apiClient.get<VolumeBackupSchedule[]>('/volumes/schedules', {
			params: projectId ? { projectId } : undefined
		});
		return res.data;
	},

	/**
	 * Toggle active status of a backup schedule
	 */
	async toggleSchedule(id: string): Promise<VolumeBackupSchedule> {
		const res = await apiClient.post<VolumeBackupSchedule>(`/volumes/schedules/${id}/toggle`);
		return res.data;
	},

	/**
	 * Delete an automated backup schedule
	 */
	async deleteSchedule(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/volumes/schedules/${id}`);
		return res.data;
	}
};

export default volumesApi;
