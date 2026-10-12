import apiClient from '../client';
import type { CaddyAccessLog } from './types';

export interface TrafficFilterParams {
	limit?: number;
	projectId?: string;
	serviceId?: string;
	status?: string;
	timeRange?: string;
	search?: string;
	excludeDashboard?: boolean;
}

export interface TrafficMetricsResponse {
	totalRequests: number;
	successRate: number;
	error4xx: number;
	error5xx: number;
	avgLatency: number;
}

export const trafficApi = {
	async getRequests(limitOrParams?: number | TrafficFilterParams): Promise<CaddyAccessLog[]> {
		const params: Record<string, string> = {};
		if (typeof limitOrParams === 'number') {
			params.limit = String(limitOrParams);
		} else if (limitOrParams) {
			if (limitOrParams.limit) params.limit = String(limitOrParams.limit);
			if (limitOrParams.projectId) params.projectId = limitOrParams.projectId;
			if (limitOrParams.serviceId) params.serviceId = limitOrParams.serviceId;
			if (limitOrParams.status) params.status = limitOrParams.status;
			if (limitOrParams.timeRange) params.timeRange = limitOrParams.timeRange;
			if (limitOrParams.search) params.search = limitOrParams.search;
			if (limitOrParams.excludeDashboard !== undefined) {
				params.excludeDashboard = String(limitOrParams.excludeDashboard);
			}
		} else {
			params.limit = '100';
		}

		const res = await apiClient.get<CaddyAccessLog[]>('/traffic/requests', { params });
		return res.data;
	},

	async getMetrics(filter?: TrafficFilterParams): Promise<TrafficMetricsResponse> {
		const params: Record<string, string> = {};
		if (filter) {
			if (filter.projectId) params.projectId = filter.projectId;
			if (filter.serviceId) params.serviceId = filter.serviceId;
			if (filter.status) params.status = filter.status;
			if (filter.timeRange) params.timeRange = filter.timeRange;
			if (filter.search) params.search = filter.search;
		}
		const res = await apiClient.get<TrafficMetricsResponse>('/traffic/metrics', { params });
		return res.data;
	},

	async requests(limit = 50): Promise<CaddyAccessLog[]> {
		return this.getRequests(limit);
	}
};

export default trafficApi;
