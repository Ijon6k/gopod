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
