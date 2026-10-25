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

// ── Session expiry (401) handling ──
// The client cannot import the auth store directly (auth store -> api ->
// client would be a circular import), so the session owner registers a
// callback at startup. Every 401 from a protected API route notifies that
// callback (synchronously, at least once per concurrent failure batch — the
// handler is idempotent, so races only cause redundant teardowns). A stale
// token therefore stops pollers and bounces to /login instead of hammering
// the backend forever.
let unauthorizedHandler: (() => void) | null = null;
let sessionExpiryHandling = false;

/** Registers the callback invoked when a protected API call reports an expired session. */
export function setUnauthorizedHandler(handler: () => void): void {
	unauthorizedHandler = handler;
}

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
		const status = error.response?.status;
		const errorMessage =
			error.response?.data?.error ||
			error.response?.data?.message ||
			error.message ||
			'An unexpected API error occurred';

		// 401 on an auth API call itself (bad password) is a normal rejection,
		// not a session-expiry signal — never trigger the logout path for it.
		const requestUrl: string = error.config?.url ?? '';
		if (
			status === 401 &&
			unauthorizedHandler &&
			!sessionExpiryHandling &&
			!requestUrl.includes('/auth/')
		) {
			sessionExpiryHandling = true;
			try {
				unauthorizedHandler();
			} finally {
				sessionExpiryHandling = false;
			}
		}

		console.warn(
			`[API] ${error.config?.method?.toUpperCase()} ${error.config?.url} failed (${error.response?.status}):`,
			errorMessage
		);
		return Promise.reject(new ApiError(errorMessage, error.response?.status, error.response?.data));
	}
);

export default apiClient;
