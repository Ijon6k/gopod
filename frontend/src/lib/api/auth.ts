import apiClient from './client';

export interface User {
	id: string;
	email: string;
	name: string;
	role: string;
	createdAt: string;
}

export interface AuthStatus {
	initialized: boolean;
	authenticated: boolean;
	user?: User;
}

export interface AuthResponse {
	user: User;
	token: string;
}

export interface SetupPayload {
	name: string;
	email: string;
	password: string;
}

export interface LoginPayload {
	email: string;
	password: string;
}

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
	}
};

export default authApi;
