export type VolumeStatus = 'mounted' | 'unmounted';

export interface VolumeSnapshot {
	id: string;
	volumeName: string;
	projectId: string;
	serviceId: string;
	filename: string;
	fileName?: string;
	size: string;
	sizeBytes?: number;
	rawSize?: number;
	createdAt: string;
	timeAgo?: string;
	status?: 'completed' | 'in_progress' | 'failed' | string;
	compression?: 'zstd' | 'gzip' | string;
}

export interface VolumeBackupSchedule {
	id: string;
	projectId: string;
	volumeName: string;
	serviceId?: string;
	cron: string;
	label?: string;
	retentionCount?: number;
	keepCount?: number;
	enabled: boolean;
	lastRun?: string;
	nextRun?: string;
}
