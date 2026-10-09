<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		interactive?: boolean;
		padding?: 'none' | 'sm' | 'md' | 'lg';
		class?: string;
		onclick?: (e: MouseEvent) => void;
		children: import('svelte').Snippet;
	}

	let {
		interactive = false,
		padding = 'md',
		class: className = '',
		onclick,
		children
	}: Props = $props();

	const paddings = {
		none: 'p-0',
		sm: 'p-3',
		md: 'p-4',
		lg: 'p-5'
	};
</script>

{#if interactive}
	<button
		type="button"
		{onclick}
		class={cn(
			'group w-full cursor-pointer rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] text-left transition-all duration-150 hover:border-[var(--accent)] hover:bg-[var(--bg-hover)] focus-visible:border-[var(--accent)]',
			paddings[padding],
			className
		)}
	>
		{@render children()}
	</button>
{:else}
	<div
		class={cn(
			'rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]',
			paddings[padding],
			className
		)}
	>
		{@render children()}
	</div>
{/if}
