<script lang="ts">
	import { Badge } from 'kumo-svelte';
	import type { BadgeVariant } from 'kumo-svelte';
	import { cn } from '$lib/utils/cn';

	interface Props {
		status: string;
		label?: string;
		size?: 'sm' | 'md';
		class?: string;
	}

	let { status, label, size = 'md', class: className = '' }: Props = $props();

	// Map GOPOD statuses to Kumo semantic badge variants
	const variantMap: Record<string, { variant: BadgeVariant; label: string }> = {
		running: { variant: 'success', label: 'Running' },
		healthy: { variant: 'success', label: 'Healthy' },
		active: { variant: 'success', label: 'Active' },
		online: { variant: 'success', label: 'Online' },
		mounted: { variant: 'success', label: 'Mounted' },
		success: { variant: 'success', label: 'Success' },

		stopped: { variant: 'neutral', label: 'Stopped' },
		cancelled: { variant: 'neutral', label: 'Cancelled' },
		unmounted: { variant: 'neutral', label: 'Unmounted' },

		building: { variant: 'info', label: 'Building' },
		deploying: { variant: 'info', label: 'Deploying' },
		starting: { variant: 'info', label: 'Starting' },

		stopping: { variant: 'warning', label: 'Stopping' },
		degraded: { variant: 'warning', label: 'Degraded' },
		pending: { variant: 'warning', label: 'Pending' },
		queued: { variant: 'warning', label: 'Queued' },

		failed: { variant: 'error', label: 'Failed' },
		unhealthy: { variant: 'error', label: 'Unhealthy' },
		error: { variant: 'error', label: 'Error' }
	};

	let info = $derived(
		variantMap[status?.toLowerCase()] ?? {
			variant: 'neutral' as BadgeVariant,
			label: status || 'Unknown'
		}
	);
</script>

<Badge
	variant={info.variant}
	appearance="dot"
	class={cn(size === 'sm' ? 'px-1.5 py-0 text-[11px]' : 'text-xs', className)}
>
	{label ?? info.label}
</Badge>
