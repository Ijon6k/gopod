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
		'flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-input)] bg-[var(--bg-input)] px-3 py-1.5 text-[var(--text-tertiary)] transition-all duration-150 focus-within:border-[var(--border-input-focus)] focus-within:ring-2 focus-within:ring-[var(--border-input-focus)]/20',
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
		class="w-full border-0 bg-transparent text-[13px] leading-normal font-[var(--font-sans)] text-[var(--text-primary)] outline-none placeholder:text-[var(--text-tertiary)]"
	/>
	{#if value}
		<button
			type="button"
			onclick={() => (value = '')}
			class="cursor-pointer border-0 bg-transparent p-0.5 text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
			aria-label="Clear search"
		>
			<X size={13} />
		</button>
	{/if}
</div>
