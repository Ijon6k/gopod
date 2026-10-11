export const domainKeys = {
	all: ['domains'] as const,
	detail: (id: string) => ['domains', id] as const
};
