export type Status =
	| 'success'
	| 'running'
	| 'stopped'
	| 'degraded'
	| 'failed'
	| 'deploying'
	| 'healthy'
	| 'unhealthy'
	| 'building'
	| 'starting'
	| 'stopping'
	| 'cancelled'
	| 'interrupted'
	| 'queued';

export interface TimeSeriesPoint {
	time?: string;
	label?: string;
	timestamp?: string;
	value: number;
}

export interface NavItem {
	label: string;
	path?: string;
	icon?: string;
	children?: NavItem[];
}
