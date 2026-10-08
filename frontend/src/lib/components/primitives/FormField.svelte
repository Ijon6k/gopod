<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import type { Snippet } from 'svelte';

	interface Props {
		label?: string;
		id?: string;
		forId?: string;
		description?: string;
		error?: string | null;
		required?: boolean;
		class?: string;
		children: Snippet;
	}

	let {
		label,
		id,
		forId,
		description,
		error,
		required = false,
		class: className = '',
		children
	}: Props = $props();

	let resolvedId = $derived(forId || id);
</script>

<div class={cn('flex flex-col gap-1.5', className)}>
	{#if label}
		<label for={resolvedId} class="text-xs font-medium text-[var(--text-secondary)] flex items-center gap-1">
			<span>{label}</span>
			{#if required}
				<span class="text-[var(--status-red)] text-[11px]">*</span>
			{/if}
		</label>
	{/if}

	{@render children()}

	{#if error}
		<span class="text-[11px] text-[var(--status-red)] leading-tight">{error}</span>
	{:else if description}
		<span class="text-[11px] text-[var(--text-tertiary)] leading-tight">{description}</span>
	{/if}
</div>
