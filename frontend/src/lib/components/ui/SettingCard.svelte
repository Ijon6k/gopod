<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import type { Component, Snippet } from 'svelte';

	interface Props {
		title: string;
		subtitle?: string;
		icon?: Component<any>;
		badge?: string;
		class?: string;
		children: Snippet;
		headerActions?: Snippet;
		footer?: Snippet;
	}

	let {
		title,
		subtitle,
		icon,
		badge,
		class: className = '',
		children,
		headerActions,
		footer
	}: Props = $props();
</script>

<div
	class={cn(
		'rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden shadow-xs flex flex-col',
		className
	)}
>
	<!-- Card Header -->
	<div class="px-5 py-4 border-b border-[var(--border)] flex items-center justify-between gap-4 bg-[var(--bg-panel)]">
		<div class="flex flex-col gap-0.5 min-w-0">
			<div class="flex items-center gap-2">
				{#if icon}
					{@const IconComp = icon}
					<IconComp size={16} class="text-[var(--accent)] shrink-0" />
				{/if}
				<h3 class="text-sm font-semibold text-[var(--text-primary)] tracking-tight m-0 truncate">
					{title}
				</h3>
				{#if badge}
					<span class="inline-flex items-center text-[11px] font-semibold px-2 py-0.5 rounded border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] font-[var(--font-mono)] shrink-0">
						{badge}
					</span>
				{/if}
			</div>
			{#if subtitle}
				<p class="text-xs text-[var(--text-tertiary)] m-0 leading-normal">
					{subtitle}
				</p>
			{/if}
		</div>

		{#if headerActions}
			<div class="flex items-center gap-2 shrink-0">
				{@render headerActions()}
			</div>
		{/if}
	</div>

	<!-- Card Body -->
	<div class="p-5 flex-1 flex flex-col gap-4 text-xs text-[var(--text-secondary)]">
		{@render children()}
	</div>

	<!-- Optional Card Footer -->
	{#if footer}
		<div class="px-5 py-3 border-t border-[var(--border)] bg-[var(--bg-surface)] shrink-0 flex items-center justify-end gap-2.5">
			{@render footer()}
		</div>
	{/if}
</div>
