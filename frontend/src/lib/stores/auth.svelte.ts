// ──────────────────────────────────────────────
// GOPOD Auth Store — Reactive Authentication & Setup State
// Svelte 5 Runes ($state) implementation
// ──────────────────────────────────────────────

import { authApi, type User, type SetupPayload, type LoginPayload } from '$lib/api/auth';

class AuthStore {
	user = $state<User | null>(null);
	isAuthenticated = $state(false);
	isInitialized = $state<boolean | null>(null); // null until first check completes
	isLoading = $state(false);
	error = $state<string | null>(null);

	async checkStatus(): Promise<{ initialized: boolean; authenticated: boolean; user?: User }> {
		this.isLoading = true;
		this.error = null;
		try {
			const status = await authApi.getStatus();
			this.isInitialized = status.initialized;
			this.isAuthenticated = status.authenticated;
			this.user = status.user || null;
			return status;
		} catch (err: any) {
			console.warn('[AuthStore] checkStatus failed:', err.message);
			// If backend unreachable or error, preserve reasonable defaults
			return { initialized: this.isInitialized ?? true, authenticated: this.isAuthenticated };
		} finally {
			this.isLoading = false;
		}
	}

	async setup(payload: SetupPayload): Promise<User> {
		this.isLoading = true;
		this.error = null;
		try {
			const res = await authApi.setup(payload);
			if (typeof window !== 'undefined' && res.token) {
				localStorage.setItem('gopod_token', res.token);
			}
			this.user = res.user;
			this.isAuthenticated = true;
			this.isInitialized = true;
			return res.user;
		} catch (err: any) {
			this.error = err.message || 'Failed to complete initial administrator setup';
			throw err;
		} finally {
			this.isLoading = false;
		}
	}

	async login(payload: LoginPayload): Promise<User> {
		this.isLoading = true;
		this.error = null;
		try {
			const res = await authApi.login(payload);
			if (typeof window !== 'undefined' && res.token) {
				localStorage.setItem('gopod_token', res.token);
			}
			this.user = res.user;
			this.isAuthenticated = true;
			return res.user;
		} catch (err: any) {
			this.error = err.message || 'Invalid email or password';
			throw err;
		} finally {
			this.isLoading = false;
		}
	}

	async logout(): Promise<void> {
		this.isLoading = true;
		try {
			await authApi.logout();
		} catch (err) {
			console.warn('[AuthStore] logout request failed:', err);
		} finally {
			if (typeof window !== 'undefined') {
				localStorage.removeItem('gopod_token');
			}
			this.user = null;
			this.isAuthenticated = false;
			this.isLoading = false;
		}
	}

	clearError() {
		this.error = null;
	}
}

export const authStore = new AuthStore();
export default authStore;
