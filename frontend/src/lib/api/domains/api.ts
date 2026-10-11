import apiClient from '../client';
import type { Domain } from './types';

export const domainsApi = {
	async list(): Promise<Domain[]> {
		const res = await apiClient.get<Domain[]>('/domains');
		return res.data;
	},

	async create(domain: Partial<Domain>): Promise<Domain> {
		const res = await apiClient.post<Domain>('/domains', domain);
		return res.data;
	},

	async update(id: string, domain: Partial<Domain>): Promise<Domain> {
		const res = await apiClient.put<Domain>(`/domains/${id}`, domain);
		return res.data;
	},

	async delete(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/domains/${id}`);
		return res.data;
	}
};

export default domainsApi;
