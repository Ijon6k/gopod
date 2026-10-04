import apiClient from './client';
import type { Project } from '$lib/types';

export const projectsApi = {
	/**
	 * List all projects
	 */
	async list(): Promise<Project[]> {
		const res = await apiClient.get<Project[]>('/projects');
		return res.data;
	},

	/**
	 * Get a specific project by ID
	 */
	async get(id: string): Promise<Project> {
		const res = await apiClient.get<Project>(`/projects/${id}`);
		return res.data;
	},

	/**
	 * Create a new project
	 */
	async create(project: Partial<Project>): Promise<Project> {
		const res = await apiClient.post<Project>('/projects', project);
		return res.data;
	},

	/**
	 * Delete a project by ID
	 */
	async delete(id: string): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(`/projects/${id}`);
		return res.data;
	}
};

export default projectsApi;
