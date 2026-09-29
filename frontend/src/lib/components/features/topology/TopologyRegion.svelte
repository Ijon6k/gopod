<script lang="ts">
	import type { TopologyRegion } from './types';
	import { cn } from '$lib/utils/cn';

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
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	{onclick}
	{onmouseenter}
	{onmouseleave}
	style="transform: translate3d({region.x}px, {region.y}px, 0); width: {region.width}px; height: {region.height}px;"
	class={cn(
		'absolute top-0 left-0 transition-colors duration-150 select-none pointer-events-auto overflow-hidden',
		region.type === 'project'
			? 'rounded-[18px] border border-[var(--border)] bg-[var(--bg-canvas-region-project)]'
			: 'rounded-[14px] border border-dashed border-[var(--border-canvas-region-pod)] bg-[var(--bg-canvas-region-pod)]',
		isHovered &&
			(region.type === 'project'
				? 'border-[var(--accent)]/40 bg-[var(--bg-canvas-region-project-hover)]'
				: 'border-[var(--accent)]/50 bg-[var(--bg-canvas-region-pod-hover)]'),
		isSelected &&
			'border-[var(--accent)] ring-1 ring-[var(--accent)]/40'
	)}
>
	<!-- Region Header / Tag with clean, non-wrapping layout -->
	<div class="h-10 px-3.5 flex items-center justify-between text-left border-b border-[var(--border-subtle)]">
		<div class="flex items-center gap-2 min-w-0">
			<span
				class={cn(
					'w-1.5 h-1.5 rounded-full shrink-0',
					region.type === 'project' ? 'bg-[var(--accent)]' : 'bg-[var(--text-secondary)]'
				)}
			></span>
			<span
				class="text-[11px] font-mono font-medium tracking-wide text-[var(--text-secondary)] uppercase truncate"
			>
				{region.label}
			</span>
		</div>

		{#if region.sublabel}
			<span
				class="text-[10px] text-[var(--text-tertiary)] font-mono shrink-0 ml-2 px-1.5 py-0.5 rounded bg-[var(--bg-hover)] border border-[var(--border-subtle)]"
			>
				{region.sublabel}
			</span>
		{/if}
	</div>
</div>
