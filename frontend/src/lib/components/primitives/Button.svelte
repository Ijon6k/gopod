<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
		size?: 'sm' | 'md';
		disabled?: boolean;
		type?: 'button' | 'submit';
		class?: string;
		title?: string;
		ariaLabel?: string;
		onclick?: (e: MouseEvent) => void;
		children: import('svelte').Snippet;
	}

	let {
		variant = 'secondary',
		size = 'md',
		disabled = false,
		type = 'button',
		class: className = '',
		title,
		ariaLabel,
		onclick,
		children
	}: Props = $props();

	const base =
		'inline-flex items-center gap-1.5 font-medium rounded-[var(--radius-sm)] cursor-pointer whitespace-nowrap transition-opacity transition-colors duration-150 font-[var(--font-sans)]';

	const variants = {
		primary: 'bg-[var(--accent)] text-white border-none hover:bg-[var(--accent-hover)]',
		secondary:
			'bg-[var(--bg-surface)] text-[var(--text-primary)] border border-[var(--border)] hover:bg-[var(--bg-hover)]',
		ghost: 'bg-transparent text-[var(--text-secondary)] border-none hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]',
		danger:
			'bg-[rgba(201,64,64,0.12)] text-[#C94040] border border-[rgba(201,64,64,0.2)] hover:bg-[rgba(201,64,64,0.18)]'
	};

	const sizes = {
		sm: 'px-2.5 py-1 text-xs',
		md: 'px-3.5 py-1.5 text-base'
	};
</script>

<button
	{type}
	{disabled}
	{onclick}
	{title}
	aria-label={ariaLabel}
	class={cn(base, variants[variant], sizes[size], disabled && 'opacity-50 cursor-not-allowed', className)}
>
	{@render children()}
</button>
