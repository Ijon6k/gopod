import apiClient from '../client';
import type { User, AuthStatus, AuthResponse, SetupPayload, LoginPayload } from './types';

export const authApi = {
	async getStatus(): Promise<AuthStatus> {
		const res = await apiClient.get<AuthStatus>('/auth/status');
		return res.data;
	},

	async setup(payload: SetupPayload): Promise<AuthResponse> {
		const res = await apiClient.post<AuthResponse>('/auth/setup', payload);
		return res.data;
	},

	async login(payload: LoginPayload): Promise<AuthResponse> {
		const res = await apiClient.post<AuthResponse>('/auth/login', payload);
		return res.data;
	},

	async logout(): Promise<void> {
		await apiClient.post('/auth/logout');
	},

	async getMe(): Promise<{ user: User }> {
		const res = await apiClient.get<{ user: User }>('/auth/me');
		return res.data;
	},

	async changePassword(payload: {
		currentPassword: string;
		newPassword: string;
	}): Promise<{ message: string }> {
		const res = await apiClient.post<{ message: string }>('/auth/password', payload);
		return res.data;
	}
};

export default authApi;
