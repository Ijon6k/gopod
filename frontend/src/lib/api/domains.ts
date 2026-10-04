import apiClient from './client';
import type { Domain } from '$lib/types';

export const domainsApi = {
	/**
	 * List all domains registered with Caddy ingress
	 */
	async list(): Promise<Domain[]> {
		const res = await apiClient.get<Domain[]>('/domains');
		return res.data;
	},

	/**
	 * Register a new domain and reconfigure Caddy
	 */
	async create(domain: Partial<Domain>): Promise<Domain> {
		const res = await apiClient.post<Domain>('/domains', domain);
		return res.data;
	},

	/**
	 * Update an existing domain configuration
	 */
	async update(id: string, domain: Partial<Domain>): Promise<Domain> {
		const res = await apiClient.put<Domain>(`/domains/${id}`, domain);
		return res.data;
	},

	/**
	 * Delete a domain and reload Caddy ingress
	 */
	async delete(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/domains/${id}`);
		return res.data;
	}
};

export default domainsApi;
