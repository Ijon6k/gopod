export const runtimeKeys = {
	all: ['runtime'] as const,
	containers: ['runtime', 'containers'] as const,
	containerDetail: (id: string) => ['runtime', 'containers', id] as const,
	containerLogs: (id: string) => ['runtime', 'containers', id, 'logs'] as const,
	pods: ['runtime', 'pods'] as const,
	images: ['runtime', 'images'] as const,
	volumes: ['runtime', 'volumes'] as const,
	networks: ['runtime', 'networks'] as const
};
