<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		variant?: 'default' | 'accent' | 'mono';
		size?: 'sm' | 'md';
		class?: string;
		children: import('svelte').Snippet;
	}

	let { variant = 'default', size = 'sm', class: className = '', children }: Props = $props();

	const variants = {
		default:
			'bg-[rgba(255,255,255,0.035)] text-[var(--text-secondary)] border-[var(--border-subtle)]',
		accent:
			'bg-[rgba(105,115,168,0.10)] text-[var(--text-primary)] border-[rgba(105,115,168,0.22)]',
		mono: 'bg-[rgba(255,255,255,0.025)] text-[var(--text-tertiary)] border-[var(--border-subtle)] font-[var(--font-mono)]'
	};

	const sizes = {
		sm: 'px-2 py-0.5 text-[11px] leading-tight',
		md: 'px-2.5 py-1 text-xs leading-normal'
	};
</script>

<span
	class={cn(
		'inline-flex items-center gap-1 rounded-[var(--radius-sm)] border font-normal tracking-normal whitespace-nowrap select-none',
		variants[variant],
		sizes[size],
		className
	)}
>
	{@render children()}
</span>
