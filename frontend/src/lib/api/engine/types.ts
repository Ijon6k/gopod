export interface EngineEvent {
	id: string;
	ts: string;
	level: 'info' | 'warn' | 'error' | 'debug' | string;
	scope: string;
	serviceId?: string;
	projectId?: string;
	actor?: string;
	action?: string;
	message: string;
	data?: Record<string, unknown>;
}

export interface EngineEventsParams {
	level?: string;
	serviceId?: string;
	projectId?: string;
	since?: string;
	limit?: number;
}

export interface EngineEventsResponse {
	items: EngineEvent[];
	total: number;
}
