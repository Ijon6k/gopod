import apiClient from '../client';
import type { Project } from './types';

export const projectsApi = {
	async list(): Promise<Project[]> {
		const res = await apiClient.get<Project[]>('/projects');
		return res.data;
	},

	async get(id: string): Promise<Project> {
		const res = await apiClient.get<Project>(`/projects/${id}`);
		return res.data;
	},

	async create(project: Partial<Project>): Promise<Project> {
		const res = await apiClient.post<Project>('/projects', project);
		return res.data;
	},

	async delete(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/projects/${id}`);
		return res.data;
	}
};

export default projectsApi;
