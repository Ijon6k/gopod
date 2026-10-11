export const storageKeys = {
	all: ['volumes'] as const,
	snapshots: (projectId?: string) => ['volumes', 'snapshots', projectId || 'all'] as const,
	schedules: (projectId?: string) => ['volumes', 'schedules', projectId || 'all'] as const
};
