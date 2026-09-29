<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { Input } from '$lib/components/primitives';
	import { MagnifyingGlass } from 'phosphor-svelte';

	interface Props {
		value?: string;
		placeholder?: string;
		class?: string;
	}

	let {
		value = $bindable(''),
		placeholder = 'Search…',
		class: className = ''
	}: Props = $props();
</script>

<label
	class={cn(
		'flex items-center gap-2 px-2.5 py-[5px] border-b border-[var(--border)] text-[var(--text-tertiary)] focus-within:border-[var(--accent)] transition-colors',
		className
	)}
>
	<MagnifyingGlass size={14} />
	<span class="sr-only">Search</span>
	<Input bind:value {placeholder} class="text-base" />
</label>
