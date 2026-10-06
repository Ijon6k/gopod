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

	createVolumeSnapshot(volumeName: string, projectId: string, serviceId: string): VolumeSnapshot {
		const dateStr = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 16);
		const newSnapshot: VolumeSnapshot = {
			id: `snap-${Date.now()}`,
			volumeName,
			projectId,
			serviceId,
			filename: `${volumeName}_${dateStr}.tar.zst`,
			size: 'Calculating...',
			sizeBytes: 1024 * 1024 * 50,
			createdAt: new Date().toISOString(),
			timeAgo: 'Just now',
			status: 'completed',
			compression: 'zstd'
		};
		this.volumeSnapshots.unshift(newSnapshot);
		api.volumes.createSnapshot({ volumeName, projectId, serviceId }).catch(() => null);
		return newSnapshot;
	}

	deleteVolumeSnapshot(id: string) {
		this.volumeSnapshots = this.volumeSnapshots.filter((s) => s.id !== id);
		api.volumes.deleteSnapshot(id).catch(() => null);
	}

	toggleVolumeSchedule(id: string) {
		const sched = this.volumeSchedules.find((s) => s.id === id);
		if (sched) {
			sched.enabled = !sched.enabled;
		}
		api.volumes.toggleSchedule(id).catch(() => null);
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
