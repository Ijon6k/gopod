import apiClient from '../client';
import type { CaddyAccessLog } from './types';

export const trafficApi = {
	async getRequests(limit = 50): Promise<CaddyAccessLog[]> {
		const res = await apiClient.get<CaddyAccessLog[]>('/traffic/requests', {
			params: { limit: String(limit) }
		});
		return res.data;
	},

	async requests(limit = 50): Promise<CaddyAccessLog[]> {
		return this.getRequests(limit);
	}
};

export default trafficApi;
