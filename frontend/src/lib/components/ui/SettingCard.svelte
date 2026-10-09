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
		'flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs',
		className
	)}
>
	<!-- Card Header -->
	<div
		class="flex items-center justify-between gap-4 border-b border-[var(--border)] bg-[var(--bg-panel)] px-5 py-4"
	>
		<div class="flex min-w-0 flex-col gap-0.5">
			<div class="flex items-center gap-2">
				{#if icon}
					{@const IconComp = icon}
					<IconComp size={16} class="shrink-0 text-[var(--accent)]" />
				{/if}
				<h3 class="m-0 truncate text-sm font-semibold tracking-tight text-[var(--text-primary)]">
					{title}
				</h3>
				{#if badge}
					<span
						class="inline-flex shrink-0 items-center rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-0.5 text-[11px] font-[var(--font-mono)] font-semibold text-[var(--text-secondary)]"
					>
						{badge}
					</span>
				{/if}
			</div>
			{#if subtitle}
				<p class="m-0 text-xs leading-normal text-[var(--text-tertiary)]">
					{subtitle}
				</p>
			{/if}
		</div>

		{#if headerActions}
			<div class="flex shrink-0 items-center gap-2">
				{@render headerActions()}
			</div>
		{/if}
	</div>

	<!-- Card Body -->
	<div class="flex flex-1 flex-col gap-4 p-5 text-xs text-[var(--text-secondary)]">
		{@render children()}
	</div>

	<!-- Optional Card Footer -->
	{#if footer}
		<div
			class="flex shrink-0 items-center justify-end gap-2.5 border-t border-[var(--border)] bg-[var(--bg-surface)] px-5 py-3"
		>
			{@render footer()}
		</div>
	{/if}
</div>
