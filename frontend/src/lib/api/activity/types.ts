export interface ActivityItem {
	id: string;
	ts: string;
	actor: string;
	action: string;
	entity: string;
	entityId?: string;
	projectId?: string;
	serviceId?: string;
	status: 'success' | 'failed' | 'info' | string;
	detail?: string;
	ip?: string;
}

export interface ActivityListParams {
	projectId?: string;
	serviceId?: string;
	action?: string;
	status?: string;
	limit?: number;
	offset?: number;
}

export interface ActivityListResponse {
	items: ActivityItem[];
	total: number;
}
