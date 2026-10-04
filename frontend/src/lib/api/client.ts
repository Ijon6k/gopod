import axios, { type AxiosInstance, type AxiosError } from 'axios';

// Base API Client configured for GoPod backend
export const apiClient: AxiosInstance = axios.create({
	baseURL: '/api',
	timeout: 15000,
	headers: {
		'Content-Type': 'application/json',
		Accept: 'application/json'
	}
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
