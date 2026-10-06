import apiClient from './client';
import type { CaddyAccessLog } from '$lib/types';

export const trafficApi = {
	/**
	 * Fetch real-time HTTP requests logged by Caddy reverse proxy
	 */
	async getRequests(limit = 50): Promise<CaddyAccessLog[]> {
		const res = await apiClient.get<CaddyAccessLog[]>('/traffic/requests', {
			params: { limit: String(limit) }
		});
		return res.data;
	},

	/**
	 * Alias for getRequests with optional limit
	 */
	async requests(limit = 50): Promise<CaddyAccessLog[]> {
		return this.getRequests(limit);
	}
};

export default trafficApi;
