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
	 * Update an existing service
	 */
	async update(id: string, service: Partial<Service>): Promise<Service> {
		const res = await apiClient.put<Service>(`/services/${id}`, service);
		return res.data;
	},

	/**
	 * Delete a service by ID
	 */
	async delete(id: string, deleteVolumes = false): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/services/${id}${deleteVolumes ? '?delete_volumes=true' : ''}`);
		return res.data;
	},

	/**
	 * Trigger a native container deployment for this service
	 */
	async deploy(id: string, trigger = 'manual'): Promise<DeployResponse> {
		const res = await apiClient.post<DeployResponse>(`/services/${id}/deploy`, { trigger });
		return res.data;
	},

	/**
	 * Power on service workload
	 */
	async start(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/start`);
		return res.data;
	},

	/**
	 * Power off / halt service workload
	 */
	async stop(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/stop`);
		return res.data;
	},

	/**
	 * Gracefully restart service workload
	 */
	async restart(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/restart`);
		return res.data;
	},

	/**
	 * List deployment rollout records for a service
	 */
	async deployments(id: string): Promise<any[]> {
		const res = await apiClient.get<any[]>(`/services/${id}/deployments`);
		return res.data;
	},

	/**
	 * Get a specific deployment record by ID
	 */
	async getDeployment(id: string): Promise<any> {
		const res = await apiClient.get<any>(`/deployments/${id}`);
		return res.data;
	},

	/**
	 * Get plain text / stored logs for a specific deployment
	 */
	async deploymentLogs(id: string): Promise<{ deploymentId: string; logs: string }> {
		const res = await apiClient.get<{ deploymentId: string; logs: string }>(`/deployments/${id}/logs`);
		return res.data;
	}
};

export default servicesApi;
