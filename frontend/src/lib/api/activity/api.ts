import apiClient from '../client';
import type { ActivityItem, ActivityListParams, ActivityListResponse } from './types';

export const activityApi = {
	async list(params?: ActivityListParams): Promise<ActivityListResponse> {
		const query: Record<string, string> = {};
		if (params?.projectId) query.projectId = params.projectId;
		if (params?.serviceId) query.serviceId = params.serviceId;
		if (params?.action) query.action = params.action;
		if (params?.status) query.status = params.status;
		if (params?.limit !== undefined) query.limit = String(params.limit);
		if (params?.offset !== undefined) query.offset = String(params.offset);

		const res = await apiClient.get<ActivityListResponse | ActivityItem[]>('/activity', {
			params: query
		});
		if (Array.isArray(res.data)) {
			return { items: res.data, total: res.data.length };
		}
		return {
			items: res.data.items ?? [],
			total: typeof res.data.total === 'number' ? res.data.total : (res.data.items?.length ?? 0)
		};
	}
};

export default activityApi;
