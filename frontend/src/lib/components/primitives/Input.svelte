<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		id?: string;
		type?: 'text' | 'password' | 'email' | 'search' | 'number';
		value?: string;
		placeholder?: string;
		disabled?: boolean;
		class?: string;
		oninput?: (e: Event) => void;
	}

	let {
		id,
		type = 'text',
		value = $bindable(''),
		placeholder = '',
		disabled = false,
		class: className = '',
		oninput
	}: Props = $props();
</script>

<input
	{id}
	{type}
	bind:value
	{placeholder}
	{disabled}
	{oninput}
	class={cn(
		'w-full border-none outline-none bg-transparent text-[var(--text-primary)] font-[var(--font-sans)] text-base placeholder:text-[var(--text-muted)] focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 shadow-none',
		disabled && 'opacity-50 cursor-not-allowed',
		className
	)}
/>
