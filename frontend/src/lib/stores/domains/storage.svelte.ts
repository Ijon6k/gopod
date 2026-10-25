import type { VolumeSnapshot, VolumeBackupSchedule, BackupDestination } from '$lib/types';
import { api } from '$lib/api';

export class StorageDomainStore {
	volumeSnapshots = $state<VolumeSnapshot[]>([]);
	volumeSchedules = $state<VolumeBackupSchedule[]>([]);
	backupDestinations = $state<BackupDestination[]>([]);

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

	async restoreVolumeSnapshot(id: string) {
		await api.volumes.restoreSnapshot(id);
	}

	downloadVolumeSnapshot(id: string, filename: string) {
		const url = api.volumes.getDownloadUrl(id);
		const a = document.createElement('a');
		a.href = url;
		a.download = filename;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
	}

	async toggleVolumeSchedule(id: string) {
		const updated = await api.volumes.toggleSchedule(id);
		const idx = this.volumeSchedules.findIndex((s) => s.id === id);
		if (idx !== -1 && updated) {
			this.volumeSchedules[idx] = updated;
		}
	}

	async createVolumeSchedule(input: {
		projectId: string;
		volumeName: string;
		cron: string;
		label?: string;
		retentionCount: number;
		destinationId?: string;
	}): Promise<VolumeBackupSchedule> {
		const created = await api.volumes.createSchedule(input);
		this.volumeSchedules.unshift(created);
		return created;
	}

	// ── S3-compatible destinations ──

	async fetchBackupDestinations() {
		try {
			const dests = await api.volumes.listDestinations();
			if (Array.isArray(dests)) this.backupDestinations = dests;
		} catch (err) {
			console.warn('[StorageDomainStore] Failed to fetch destinations:', err);
		}
	}

	async createBackupDestination(input: Omit<BackupDestination, 'id'>): Promise<BackupDestination> {
		const created = await api.volumes.createDestination(input);
		this.backupDestinations.unshift(created);
		return created;
	}

	async deleteBackupDestination(id: string) {
		await api.volumes.deleteDestination(id);
		this.backupDestinations = this.backupDestinations.filter((d) => d.id !== id);
	}

	async fetchStorageData(projectId = '') {
		try {
			const [snaps, scheds, dests] = await Promise.all([
				api.volumes.listSnapshots(projectId).catch(() => []),
				api.volumes.listSchedules(projectId).catch(() => []),
				api.volumes.listDestinations().catch(() => [])
			]);
			if (Array.isArray(snaps)) this.volumeSnapshots = snaps;
			if (Array.isArray(scheds)) this.volumeSchedules = scheds;
			if (Array.isArray(dests)) this.backupDestinations = dests;
		} catch (err) {
			console.warn('[StorageDomainStore] Failed to fetch storage data:', err);
		}
	}
}
