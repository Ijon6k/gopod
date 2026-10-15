import apiClient from '../client';
import type { EngineEvent, EngineEventsParams, EngineEventsResponse } from './types';

export const engineEventsApi = {
	async list(params?: EngineEventsParams): Promise<EngineEventsResponse> {
		const query: Record<string, string> = {};
		if (params?.level) query.level = params.level;
		if (params?.serviceId) query.serviceId = params.serviceId;
		if (params?.projectId) query.projectId = params.projectId;
		if (params?.since) query.since = params.since;
		if (params?.limit !== undefined) query.limit = String(params.limit);

		const res = await apiClient.get<EngineEventsResponse | EngineEvent[]>('/engine/events', {
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

export default engineEventsApi;
