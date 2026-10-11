export const auditKeys = {
	all: ['audit'] as const,
	list: (category?: string) => ['audit', category || 'all'] as const
};
