<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		id?: string;
		type?: 'text' | 'password' | 'email' | 'search' | 'number' | 'url';
		value?: string | number;
		placeholder?: string;
		disabled?: boolean;
		readonly?: boolean;
		required?: boolean;
		autocomplete?: import('svelte/elements').HTMLInputAttributes['autocomplete'];
		error?: string | boolean;
		size?: 'sm' | 'md' | 'lg';
		class?: string;
		oninput?: (e: Event) => void;
		onchange?: (e: Event) => void;
		onkeydown?: (e: KeyboardEvent) => void;
	}

	let {
		id,
		type = 'text',
		value = $bindable(''),
		placeholder = '',
		disabled = false,
		readonly = false,
		required = false,
		autocomplete,
		error = false,
		size = 'md',
		class: className = '',
		oninput,
		onchange,
		onkeydown
	}: Props = $props();
</script>

<input
	{id}
	{type}
	bind:value
	{placeholder}
	{disabled}
	{readonly}
	{required}
	{autocomplete}
	{oninput}
	{onchange}
	{onkeydown}
	class={cn(
		'box-border w-full rounded-[var(--radius-sm)] border border-[var(--border-input)] bg-[var(--bg-input)] font-[var(--font-sans)] text-[var(--text-primary)] transition-all duration-150 placeholder:text-[var(--text-tertiary)]',
		'focus:border-[var(--border-input-focus)] focus:ring-2 focus:ring-[var(--border-input-focus)]/20 focus:outline-none',
		'hover:border-[var(--accent)]/50',
		size === 'sm'
			? 'h-8 px-2.5 text-xs'
			: size === 'lg'
				? 'h-11 px-4 text-base'
				: 'h-9 px-3 text-[13px]',
		disabled && 'cursor-not-allowed bg-[var(--bg-hover)] opacity-50',
		error &&
			'border-[var(--status-red)] focus:border-[var(--status-red)] focus:ring-[var(--status-red)]/20',
		className
	)}
/>
