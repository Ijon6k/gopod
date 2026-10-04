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

// Response interceptor for consistent error extraction
apiClient.interceptors.response.use(
	(response) => response,
	(error: AxiosError<{ error?: string; message?: string }>) => {
		const errorMessage =
			error.response?.data?.error ||
			error.response?.data?.message ||
			error.message ||
			'An unexpected API error occurred';
		
		console.warn(`[API] ${error.config?.method?.toUpperCase()} ${error.config?.url} failed:`, errorMessage);
		return Promise.reject(new Error(errorMessage));
	}
);

export default apiClient;
