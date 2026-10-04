import apiClient from './client';
import type { CaddyAccessLog } from '$lib/types';

export const trafficApi = {
	/**
	 * Fetch real-time HTTP requests logged by Caddy reverse proxy
	 */
	async getRequests(): Promise<CaddyAccessLog[]> {
		const res = await apiClient.get<CaddyAccessLog[]>('/traffic/requests');
		return res.data;
	}
};

export default trafficApi;
