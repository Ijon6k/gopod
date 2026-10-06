import apiClient from './client';
import type { AuditLog } from '$lib/types';

export const auditApi = {
	/**
	 * List audit logs with optional category filter
	 */
	async list(category?: string, limit = 100): Promise<AuditLog[]> {
		const params: Record<string, string> = {};
		if (category && category !== 'all') {
			params.category = category;
		}
		if (limit) {
			params.limit = String(limit);
		}
		const res = await apiClient.get<AuditLog[]>('/audit-log', { params });
		return res.data;
	}
};

export default auditApi;
