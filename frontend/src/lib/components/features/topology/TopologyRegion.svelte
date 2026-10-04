<script lang="ts">
	import type { TopologyRegion } from './types';
	import { cn } from '$lib/utils/cn';
	import { Folder, SquaresFour } from 'phosphor-svelte';

	interface Props {
		region: TopologyRegion;
		isSelected?: boolean;
		isHovered?: boolean;
		onclick?: () => void;
		onmouseenter?: () => void;
		onmouseleave?: () => void;
	}

	let {
		region,
		isSelected = false,
		isHovered = false,
		onclick,
		onmouseenter,
		onmouseleave
	}: Props = $props();

	let isPod = $derived(region.type === 'pod');
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	{onclick}
	{onmouseenter}
	{onmouseleave}
	style="transform: translate3d({region.x}px, {region.y}px, 0); width: {region.width}px; height: {region.height}px;"
	class={cn(
		'absolute top-0 left-0 transition-all duration-150 select-none pointer-events-auto overflow-hidden',
		isPod
			? 'rounded-[16px] border-2 border-dashed border-[var(--accent)]/35 bg-[var(--bg-canvas-region-pod)]'
			: 'rounded-[16px] border border-[var(--border)] bg-[var(--bg-canvas-region-project)] shadow-xs',
		isHovered &&
			(isPod
				? 'border-[var(--accent)]/70 bg-[var(--bg-canvas-region-pod-hover)] shadow-sm'
				: 'border-[var(--accent)]/50 bg-[var(--bg-canvas-region-project-hover)] shadow-sm'),
		isSelected &&
			'border-[var(--accent)] ring-2 ring-[var(--accent)]/30'
	)}
>
	<!-- Region Header / Tag with clean, non-wrapping layout -->
	<div class="h-9 px-3.5 flex items-center justify-between text-left border-b border-[var(--border-subtle)] bg-[var(--bg-panel)]/40">
		<div class="flex items-center gap-2 min-w-0">
			{#if isPod}
				<SquaresFour size={14} class="text-[var(--accent)] shrink-0" />
			{:else}
				<Folder size={14} class="text-[var(--text-secondary)] shrink-0" />
			{/if}
			<span
				class="text-xs font-semibold text-[var(--text-primary)] font-[var(--font-sans)] truncate"
			>
				{isPod ? `Pod · ${region.label.replace(/^pod\s*[:·-]?\s*/i, '')}` : region.label}
			</span>
		</div>

		{#if region.sublabel}
			<span
				class="text-[10.5px] text-[var(--text-tertiary)] shrink-0 ml-2 px-1.5 py-0.5 rounded bg-[var(--bg-surface)] border border-[var(--border-subtle)]"
			>
				{region.sublabel}
			</span>
		{/if}
	</div>
</div>
