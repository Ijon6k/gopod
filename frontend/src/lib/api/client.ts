import axios, { type AxiosInstance, type AxiosError } from 'axios';

// Base API Client configured for GoPod backend
export const apiClient: AxiosInstance = axios.create({
	baseURL: '/api',
	timeout: 15000,
	withCredentials: true,
	headers: {
		'Content-Type': 'application/json',
		Accept: 'application/json'
	}
});

// Request interceptor to attach Bearer token if present
apiClient.interceptors.request.use((config) => {
	if (typeof window !== 'undefined') {
		const token = localStorage.getItem('gopod_token');
		if (token && config.headers) {
			config.headers.Authorization = `Bearer ${token}`;
		}
	}
	return config;
});

export class ApiError extends Error {
	status?: number;
	data?: any;

	constructor(message: string, status?: number, data?: any) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.data = data;
	}
}

// Response interceptor for consistent error extraction with status code preservation
apiClient.interceptors.response.use(
	(response) => response,
	(error: AxiosError<{ error?: string; message?: string }>) => {
		const errorMessage =
			error.response?.data?.error ||
			error.response?.data?.message ||
			error.message ||
			'An unexpected API error occurred';
		
		console.warn(`[API] ${error.config?.method?.toUpperCase()} ${error.config?.url} failed (${error.response?.status}):`, errorMessage);
		return Promise.reject(new ApiError(errorMessage, error.response?.status, error.response?.data));
	}
);

export default apiClient;
