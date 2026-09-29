<script lang="ts">
	import type { TopologyNode } from './types';
	import { cn } from '$lib/utils/cn';
	import {
		Globe,
		AppWindow,
		Cube,
		HardDrive,
		TreeStructure,
		ShieldCheck,
		Cloud
	} from 'phosphor-svelte';

	interface Props {
		node: TopologyNode;
		isSelected?: boolean;
		isHovered?: boolean;
		onclick?: () => void;
		onmouseenter?: () => void;
		onmouseleave?: () => void;
	}

	let {
		node,
		isSelected = false,
		isHovered = false,
		onclick,
		onmouseenter,
		onmouseleave
	}: Props = $props();

	function getStatusColor(status?: string) {
		switch (status) {
			case 'running':
			case 'active':
			case 'healthy':
				return 'bg-[var(--status-green)]';
			case 'degraded':
				return 'bg-[var(--status-amber)]';
			case 'failed':
			case 'unhealthy':
				return 'bg-[var(--status-red)]';
			default:
				return 'bg-[var(--status-gray)]';
		}
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	{onclick}
	{onmouseenter}
	{onmouseleave}
	style="transform: translate3d({node.x}px, {node.y}px, 0); width: {node.width}px; height: {node.height}px;"
	class={cn(
		'absolute top-0 left-0 cursor-pointer select-none rounded-[10px] px-3 py-2.5 transition-all duration-150',
		'bg-[var(--bg-canvas-node)] border border-[var(--border-canvas-node)] text-left flex flex-col justify-between',
		isHovered &&
			'border-[var(--accent)] bg-[var(--bg-canvas-node-hover)] translate-y-[-1px]',
		isSelected &&
			'border-[var(--accent)] ring-1.5 ring-[var(--accent)] bg-[var(--bg-canvas-node-selected)]'
	)}
>
	<!-- Top Row: Icon, Title, Status Indicator -->
	<div class="flex items-center justify-between gap-1.5 w-full">
		<div class="flex items-center gap-2 min-w-0">
			<span class="text-[var(--text-secondary)] opacity-85 shrink-0">
				{#if node.type === 'domain'}
					<Globe size={14} />
				{:else if node.type === 'service'}
					<AppWindow size={14} />
				{:else if node.type === 'container'}
					<Cube size={14} />
				{:else if node.type === 'volume'}
					<HardDrive size={14} />
				{:else if node.type === 'network'}
					<TreeStructure size={14} />
				{:else if node.type === 'proxy'}
					<ShieldCheck size={14} />
				{:else}
					<Cloud size={14} />
				{/if}
			</span>
			<span
				class="text-base font-medium text-[var(--text-primary)] truncate font-[var(--font-sans)] leading-snug"
				title={node.title}
			>
				{node.title}
			</span>
		</div>

		{#if node.status}
			<div class="flex items-center justify-center shrink-0 w-3 h-3 relative" title={`Status: ${node.status}`}>
				{#if node.status === 'running' || node.status === 'active' || node.status === 'healthy'}
					<span class="absolute w-2 h-2 rounded-full bg-[var(--status-green)] opacity-40 animate-ping"></span>
				{/if}
				<span class={cn('w-1.5 h-1.5 rounded-full z-10', getStatusColor(node.status))}></span>
			</div>
		{/if}
	</div>

	<!-- Bottom Row: Subtitle/Type and Monospace Detail -->
	<div class="flex items-center justify-between gap-2 w-full text-[11px] mt-0.5">
		<span class="text-[var(--text-tertiary)] truncate text-[11px]">
			{node.subtitle}
		</span>

		{#if node.monoDetail}
			<span
				class="text-[10px] font-mono text-[var(--bg-canvas-pill-text)] bg-[var(--bg-canvas-pill)] px-1.5 py-0.5 rounded border border-[var(--bg-canvas-pill-border)] shrink-0"
			>
				{node.monoDetail}
			</span>
		{/if}
	</div>
</div>
