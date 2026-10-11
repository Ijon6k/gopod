export const trafficKeys = {
	all: ['traffic'] as const,
	requests: (limit?: number) => ['traffic', 'requests', limit || 50] as const
};
