<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { MagnifyingGlass, X } from 'phosphor-svelte';

	interface Props {
		value?: string;
		placeholder?: string;
		class?: string;
		oninput?: (e: Event) => void;
	}

	let {
		value = $bindable(''),
		placeholder = 'Search…',
		class: className = '',
		oninput
	}: Props = $props();
</script>

<div
	class={cn(
		'flex items-center gap-2 px-3 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border-input)] bg-[var(--bg-input)] text-[var(--text-tertiary)] focus-within:border-[var(--border-input-focus)] focus-within:ring-2 focus-within:ring-[var(--border-input-focus)]/20 transition-all duration-150',
		className
	)}
>
	<MagnifyingGlass size={15} class="shrink-0 text-[var(--text-tertiary)]" />
	<span class="sr-only">Search</span>
	<input
		type="search"
		bind:value
		{placeholder}
		{oninput}
		class="w-full bg-transparent border-0 outline-none text-[13px] text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)] font-[var(--font-sans)] leading-normal"
	/>
	{#if value}
		<button
			type="button"
			onclick={() => (value = '')}
			class="p-0.5 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] bg-transparent border-0 cursor-pointer"
			aria-label="Clear search"
		>
			<X size={13} />
		</button>
	{/if}
</div>
