<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		status: string;
		label?: string;
		size?: 'sm' | 'md';
		class?: string;
	}

	let { status, label, size = 'md', class: className = '' }: Props = $props();

	interface StatusConfig {
		color: string;
		dotClass: string;
		shadowColor: string;
		label: string;
	}

	const greenConfig: StatusConfig = {
		color: 'text-[var(--text-secondary)]',
		dotClass: 'bg-[var(--status-green)]',
		shadowColor: 'var(--status-green-muted)',
		label: 'Healthy'
	};

	const redConfig: StatusConfig = {
		color: 'text-[var(--text-secondary)]',
		dotClass: 'bg-[var(--status-red)]',
		shadowColor: 'var(--status-red-muted)',
		label: 'Failed'
	};

	const amberConfig: StatusConfig = {
		color: 'text-[var(--text-secondary)]',
		dotClass: 'bg-[var(--status-amber)]',
		shadowColor: 'var(--status-amber-muted)',
		label: 'Pending'
	};

	const blueConfig: StatusConfig = {
		color: 'text-[var(--text-secondary)]',
		dotClass: 'bg-[var(--status-blue)]',
		shadowColor: 'var(--accent-muted)',
		label: 'Deploying'
	};

	const grayConfig: StatusConfig = {
		color: 'text-[var(--text-secondary)]',
		dotClass: 'bg-[var(--status-gray)]',
		shadowColor: 'rgba(101, 106, 116, 0.15)',
		label: 'Stopped'
	};

	const config: Record<string, StatusConfig> = {
		running: { ...greenConfig, label: 'Running' },
		healthy: { ...greenConfig, label: 'Healthy' },
		active: { ...greenConfig, label: 'Active' },
		online: { ...greenConfig, label: 'Online' },
		mounted: { ...greenConfig, label: 'Mounted' },

		stopped: { ...grayConfig, label: 'Stopped' },
		cancelled: { ...grayConfig, label: 'Cancelled' },
		unmounted: { ...grayConfig, label: 'Unmounted' },

		building: { ...blueConfig, label: 'Building' },
		deploying: { ...blueConfig, label: 'Deploying' },

		degraded: { ...amberConfig, label: 'Degraded' },
		pending: { ...amberConfig, label: 'Pending' },
		queued: { ...amberConfig, label: 'Queued' },

		failed: { ...redConfig, label: 'Failed' },
		unhealthy: { ...redConfig, label: 'Unhealthy' },
		error: { ...redConfig, label: 'Error' }
	};

	let c = $derived(config[status] ?? { ...grayConfig, label: status });
	let dotSize = $derived(size === 'sm' ? 'w-[5px] h-[5px]' : 'w-1.5 h-1.5');
	let fontSize = $derived(size === 'sm' ? 'text-xs' : 'text-base');
</script>

<span class={cn('inline-flex items-center gap-[5px] font-normal', fontSize, c.color, className)}>
	<span
		class={cn('shrink-0 rounded-full', dotSize, c.dotClass)}
		style="box-shadow: 0 0 0 2px {c.shadowColor};"
	></span>
	{label ?? c.label}
</span>
