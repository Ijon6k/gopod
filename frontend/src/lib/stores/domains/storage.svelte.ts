import type { VolumeSnapshot, VolumeBackupSchedule } from '$lib/types';
import { api } from '$lib/api';

export class StorageDomainStore {
	volumeSnapshots = $state<VolumeSnapshot[]>([]);
	volumeSchedules = $state<VolumeBackupSchedule[]>([]);

	getProjectVolumeSnapshots(projectId: string): VolumeSnapshot[] {
		return this.volumeSnapshots.filter((s) => s.projectId === projectId);
	}

	getProjectVolumeSchedules(projectId: string): VolumeBackupSchedule[] {
		return this.volumeSchedules.filter((s) => s.projectId === projectId);
	}

	async createVolumeSnapshot(
		volumeName: string,
		projectId: string,
		serviceId: string
	): Promise<VolumeSnapshot> {
		const created = await api.volumes.createSnapshot({ volumeName, projectId, serviceId });
		this.volumeSnapshots.unshift(created);
		return created;
	}

	async deleteVolumeSnapshot(id: string) {
		await api.volumes.deleteSnapshot(id);
		this.volumeSnapshots = this.volumeSnapshots.filter((s) => s.id !== id);
	}

	async toggleVolumeSchedule(id: string) {
		const updated = await api.volumes.toggleSchedule(id);
		const idx = this.volumeSchedules.findIndex((s) => s.id === id);
		if (idx !== -1 && updated) {
			this.volumeSchedules[idx] = updated;
		}
	}

	async fetchStorageData(projectId = '') {
		try {
			const [snaps, scheds] = await Promise.all([
				api.volumes.listSnapshots(projectId).catch(() => []),
				api.volumes.listSchedules(projectId).catch(() => [])
			]);
			if (Array.isArray(snaps)) this.volumeSnapshots = snaps;
			if (Array.isArray(scheds)) this.volumeSchedules = scheds;
		} catch (err) {
			console.warn('[StorageDomainStore] Failed to fetch storage data:', err);
		}
	}
}
