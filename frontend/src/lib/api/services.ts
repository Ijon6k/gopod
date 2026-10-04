import apiClient from './client';
import type { Service } from '$lib/types';

export interface DeployResponse {
	status: string;
	serviceId: string;
	containerName: string;
	deployedAt: string;
}

export const servicesApi = {
	/**
	 * List all services, optionally filtered by project ID
	 */
	async list(projectId?: string): Promise<Service[]> {
		const res = await apiClient.get<Service[]>('/services', {
			params: projectId ? { projectId } : undefined
		});
		return res.data;
	},

	/**
	 * Get a specific service by ID
	 */
	async get(id: string): Promise<Service> {
		const res = await apiClient.get<Service>(`/services/${id}`);
		return res.data;
	},

	/**
	 * Create a new service
	 */
	async create(service: Partial<Service>): Promise<Service> {
		const res = await apiClient.post<Service>('/services', service);
		return res.data;
	},

	/**
	 * Delete a service by ID
	 */
	async delete(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/services/${id}`);
		return res.data;
	},

	/**
	 * Trigger a native container deployment for this service
	 */
	async deploy(id: string): Promise<DeployResponse> {
		const res = await apiClient.post<DeployResponse>(`/services/${id}/deploy`);
		return res.data;
	}
};

export default servicesApi;
