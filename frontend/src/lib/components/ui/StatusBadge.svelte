<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		status: string;
		label?: string;
		size?: 'sm' | 'md';
		class?: string;
	}

	let { status, label, size = 'md', class: className = '' }: Props = $props();

	const config: Record<string, { color: string; dotClass: string; label: string }> = {
		running: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#3D9970]', label: 'Running' },
		healthy: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#3D9970]', label: 'Healthy' },
		stopped: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#5E6678]', label: 'Stopped' },
		degraded: {
			color: 'text-[var(--text-secondary)]',
			dotClass: 'bg-[#C98A2E]',
			label: 'Degraded'
		},
		building: {
			color: 'text-[var(--text-secondary)]',
			dotClass: 'bg-[#4A90D9]',
			label: 'Building'
		},
		deploying: {
			color: 'text-[var(--text-secondary)]',
			dotClass: 'bg-[#4A90D9]',
			label: 'Deploying'
		},
		failed: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#C94040]', label: 'Failed' },
		unhealthy: {
			color: 'text-[var(--text-secondary)]',
			dotClass: 'bg-[#C94040]',
			label: 'Unhealthy'
		},
		active: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#3D9970]', label: 'Active' },
		pending: {
			color: 'text-[var(--text-secondary)]',
			dotClass: 'bg-[#C98A2E]',
			label: 'Pending'
		},
		error: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#C94040]', label: 'Error' },
		cancelled: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#5E6678]', label: 'Cancelled' },
		queued: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#C98A2E]', label: 'Queued' },
		online: { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#3D9970]', label: 'Online' },
		mounted: {
			color: 'text-[var(--text-secondary)]',
			dotClass: 'bg-[#3D9970]',
			label: 'Mounted'
		},
		unmounted: {
			color: 'text-[var(--text-secondary)]',
			dotClass: 'bg-[#5E6678]',
			label: 'Unmounted'
		}
	};

	let c = $derived(config[status] ?? { color: 'text-[var(--text-secondary)]', dotClass: 'bg-[#5E6678]', label: status });
	let dotSize = $derived(size === 'sm' ? 'w-[5px] h-[5px]' : 'w-1.5 h-1.5');
	let fontSize = $derived(size === 'sm' ? 'text-xs' : 'text-base');
</script>

<span class={cn('inline-flex items-center gap-[5px] font-normal', fontSize, c.color, className)}>
	<span
		class={cn('rounded-full shrink-0', dotSize, c.dotClass)}
		style="box-shadow: 0 0 0 2px color-mix(in srgb, {c.dotClass === 'bg-[#3D9970]' ? '#3D9970' : c.dotClass === 'bg-[#C94040]' ? '#C94040' : c.dotClass === 'bg-[#C98A2E]' ? '#C98A2E' : c.dotClass === 'bg-[#4A90D9]' ? '#4A90D9' : '#5E6678'} 13%, transparent);"
	></span>
	{label ?? c.label}
</span>
