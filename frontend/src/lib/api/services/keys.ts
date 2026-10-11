export const serviceKeys = {
	all: ['services'] as const,
	byProject: (projectId?: string) => ['services', projectId || 'all'] as const,
	detail: (id: string) => ['services', 'detail', id] as const,
	deployments: (serviceId: string) => ['deployments', serviceId] as const,
	deploymentDetail: (id: string) => ['deployments', 'detail', id] as const
};
