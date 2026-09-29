<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		variant?: 'default' | 'success' | 'warning' | 'error' | 'info' | 'muted';
		size?: 'sm' | 'md';
		dot?: boolean;
		class?: string;
		children: import('svelte').Snippet;
	}

	let {
		variant = 'default',
		size = 'md',
		dot = false,
		class: className = '',
		children
	}: Props = $props();

	const base = 'inline-flex items-center font-normal';

	const variantColors = {
		default: 'text-[var(--text-secondary)]',
		success: 'text-[var(--status-green)]',
		warning: 'text-[var(--status-amber)]',
		error: 'text-[var(--status-red)]',
		info: 'text-[var(--status-blue)]',
		muted: 'text-[var(--text-muted)]'
	};

	const dotColors: Record<string, string> = {
		default: 'bg-[var(--text-tertiary)]',
		success: 'bg-[var(--status-green)]',
		warning: 'bg-[var(--status-amber)]',
		error: 'bg-[var(--status-red)]',
		info: 'bg-[var(--status-blue)]',
		muted: 'bg-[var(--text-muted)]'
	};

	const sizes = {
		sm: 'text-[11px] gap-[5px]',
		md: 'text-xs gap-[5px]'
	};

	let dotSize = $derived(size === 'sm' ? 'w-[5px] h-[5px]' : 'w-1.5 h-1.5');
</script>

<span class={cn(base, variantColors[variant], sizes[size], className)}>
	{#if dot}
		<span class={cn('rounded-full shrink-0', dotSize, dotColors[variant])}></span>
	{/if}
	{@render children()}
</span>
