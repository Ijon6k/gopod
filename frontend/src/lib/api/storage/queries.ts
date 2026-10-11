import { createQuery, type CreateQueryResult } from '@tanstack/svelte-query';
import { storageApi } from './api';
import { storageKeys } from './keys';
import type { VolumeSnapshot, VolumeBackupSchedule } from './types';

export function createSnapshotsQuery(
	projectId?: () => string | undefined
): CreateQueryResult<VolumeSnapshot[], Error> {
	return createQuery(() => {
		const pId = projectId?.();
		return {
			queryKey: storageKeys.snapshots(pId),
			queryFn: () => storageApi.listSnapshots(pId),
			staleTime: 10000
		};
	});
}

export function createSchedulesQuery(
	projectId?: () => string | undefined
): CreateQueryResult<VolumeBackupSchedule[], Error> {
	return createQuery(() => {
		const pId = projectId?.();
		return {
			queryKey: storageKeys.schedules(pId),
			queryFn: () => storageApi.listSchedules(pId),
			staleTime: 10000
		};
	});
}
