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
	/** Remote mirror state. Empty remoteStatus = never pushed. */
	remoteStatus?: 'synced' | 'local-only' | 'failed' | string;
	remoteProvider?: string;
	remoteDestinationId?: string;
	remoteKey?: string;
	checksum?: string;
	remoteError?: string;
	destinationId?: string;
}

/** A generic S3-compatible object store (AWS S3, R2, MinIO, Wasabi, B2...). */
export interface BackupDestination {
	id: string;
	name: string;
	endpoint: string;
	region?: string;
	accessKey: string;
	/** Write-only: every read returns an empty string. */
	secretKey?: string;
	bucket: string;
	prefix?: string;
	pathStyle?: boolean;
	createdAt?: string;
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
	/** Optional S3-compatible mirror; empty = local-only snapshots. */
	destinationId?: string;
}
