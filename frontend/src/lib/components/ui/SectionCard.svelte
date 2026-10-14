<script lang="ts">
	import { LayerCard } from 'kumo-svelte';
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

<!-- Kumo Flat Section Card -->
<LayerCard class={cn('overflow-hidden p-0 shadow-none', className)}>
	<!-- Header -->
	{#if collapsible}
		<button
			type="button"
			onclick={toggle}
			class={cn(
				'flex w-full cursor-pointer items-center justify-between border-0 bg-transparent p-4 text-left hover:bg-[var(--bg-hover)]',
				open && 'border-b border-[var(--border)]',
				headerClass
			)}
		>
			<div class="flex min-w-0 flex-col gap-0.5 pr-3">
				<span class="text-sm font-semibold text-[var(--text-primary)]">
					{title}
				</span>
				{#if description}
					<span class="text-xs leading-normal text-[var(--text-tertiary)]">
						{description}
					</span>
				{/if}
			</div>

			<div class="flex shrink-0 items-center gap-2">
				{#if headerActions}
					<div onclick={(e) => e.stopPropagation()} role="presentation">
						{@render headerActions()}
					</div>
				{/if}
				<span class="text-[var(--text-tertiary)]">
					{#if open}
						<CaretDown size={14} />
					{:else}
						<CaretRight size={14} />
					{/if}
				</span>
			</div>
		</button>
	{:else}
		<div
			class={cn(
				'flex items-center justify-between p-4',
				open && 'border-b border-[var(--border)]',
				headerClass
			)}
		>
			<div class="flex min-w-0 flex-col gap-0.5 pr-3">
				<span class="text-sm font-semibold text-[var(--text-primary)]">
					{title}
				</span>
				{#if description}
					<span class="text-xs leading-normal text-[var(--text-tertiary)]">
						{description}
					</span>
				{/if}
			</div>

			{#if headerActions}
				<div class="flex shrink-0 items-center gap-2">
					{@render headerActions()}
				</div>
			{/if}
		</div>
	{/if}

	<!-- Content Body -->
	{#if open}
		<div class={cn('flex flex-col gap-4 p-4 text-xs text-[var(--text-secondary)]', contentClass)}>
			{@render children()}
		</div>
	{/if}

	<!-- Optional Footer -->
	{#if footer && open}
		<div
			class="flex items-center justify-between border-t border-[var(--border)] bg-[var(--bg-surface)] px-4 py-3 text-xs"
		>
			{@render footer()}
		</div>
	{/if}
</LayerCard>
