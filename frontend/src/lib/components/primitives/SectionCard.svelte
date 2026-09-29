<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { CaretDown, CaretRight } from 'phosphor-svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		description?: string;
		collapsible?: boolean;
		open?: boolean;
		class?: string;
		headerClass?: string;
		contentClass?: string;
		headerActions?: Snippet;
		children: Snippet;
		footer?: Snippet;
	}

	let {
		title,
		description,
		collapsible = false,
		open = $bindable(true),
		class: className = '',
		headerClass = '',
		contentClass = '',
		headerActions,
		children,
		footer
	}: Props = $props();

	function toggle() {
		if (collapsible) {
			open = !open;
		}
	}
</script>

<div
	class={cn(
		'rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden transition-colors',
		className
	)}
>
	<!-- Header -->
	{#if collapsible}
		<button
			type="button"
			onclick={toggle}
			class={cn(
				'w-full flex items-center justify-between p-4 bg-transparent hover:bg-[var(--bg-hover)] transition-colors border-0 cursor-pointer text-left',
				headerClass
			)}
		>
			<div class="flex flex-col gap-0.5 min-w-0 pr-3">
				<span class="text-sm font-medium text-[var(--text-primary)]">
					{title}
				</span>
				{#if description}
					<span class="text-xs text-[var(--text-tertiary)] leading-relaxed">
						{description}
					</span>
				{/if}
			</div>

			<div class="flex items-center gap-3 shrink-0">
				{#if headerActions}
					<div onclick={(e) => e.stopPropagation()} role="presentation">
						{@render headerActions()}
					</div>
				{/if}
				<span class="text-[var(--text-tertiary)]">
					{#if open}
						<CaretDown size={15} />
					{:else}
						<CaretRight size={15} />
					{/if}
				</span>
			</div>
		</button>
	{:else}
		<div
			class={cn(
				'flex items-center justify-between p-4',
				open ? 'border-b border-[var(--border-subtle)]' : '',
				headerClass
			)}
		>
			<div class="flex flex-col gap-0.5 min-w-0 pr-3">
				<span class="text-sm font-medium text-[var(--text-primary)]">
					{title}
				</span>
				{#if description}
					<span class="text-xs text-[var(--text-tertiary)] leading-relaxed">
						{description}
					</span>
				{/if}
			</div>

			{#if headerActions}
				<div class="shrink-0 flex items-center gap-2">
					{@render headerActions()}
				</div>
			{/if}
		</div>
	{/if}

	<!-- Content Body -->
	{#if open}
		<div class={cn(collapsible ? 'p-4 pt-0 flex flex-col gap-4 border-t border-[var(--border-subtle)] mt-2' : 'p-4 flex flex-col gap-4', contentClass)}>
			{@render children()}
		</div>
	{/if}

	<!-- Optional Footer -->
	{#if footer && open}
		<div class="p-3.5 bg-[var(--bg-surface)] border-t border-[var(--border-subtle)] flex items-center justify-between">
			{@render footer()}
		</div>
	{/if}
</div>
