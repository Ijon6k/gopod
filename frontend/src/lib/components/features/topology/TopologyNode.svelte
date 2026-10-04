<script lang="ts">
	import type { TopologyNode } from './types';
	import { cn } from '$lib/utils/cn';
	import {
		Globe,
		AppWindow,
		Cube,
		HardDrive,
		TreeStructure,
		ShareNetwork,
		ShieldCheck,
		Cloud,
		Database,
		FileText,
		Stack
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

	let isDatabase = $derived.by(() => {
		if (node.type === 'service' || node.type === 'container') {
			const raw = node.raw as any;
			if (raw?.type === 'database') return true;
			const name = (node.title || '').toLowerCase();
			const image = (raw?.image || node.subtitle || '').toLowerCase();
			return (
				name.includes('redis') ||
				name.includes('postgres') ||
				name.includes('mysql') ||
				name.includes('mongo') ||
				name.includes('db') ||
				image.includes('postgres') ||
				image.includes('redis') ||
				image.includes('mysql') ||
				image.includes('mongo')
			);
		}
		return false;
	});

	let isQuadlet = $derived.by(() => {
		const raw = node.raw as any;
		return (
			raw?.type === 'quadlet' ||
			raw?.runtimeTarget === 'quadlet' ||
			raw?.isQuadlet === true ||
			(node.badge || '').toLowerCase() === 'quadlet' ||
			node.title.endsWith('.container') ||
			node.title.endsWith('.service') ||
			(node.subtitle || '').toLowerCase().includes('systemd')
		);
	});

	let isCompose = $derived.by(() => {
		const raw = node.raw as any;
		return raw?.type === 'compose' || node.badge === 'compose';
	});

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

	let cleanMonoDetail = $derived.by(() => {
		if (!node.monoDetail) return '';
		return node.monoDetail.replace(/^port\s*:\s*/i, ':');
	});
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	{onclick}
	{onmouseenter}
	{onmouseleave}
	style="transform: translate3d({node.x}px, {node.y}px, 0); width: {node.width}px; height: {node.height}px;"
	class={cn(
		'absolute top-0 left-0 cursor-pointer select-none rounded-[var(--radius-sm)] px-3 py-2 transition-all duration-150',
		'bg-[var(--bg-canvas-node)] border border-[var(--border-canvas-node)] text-left flex flex-col justify-between shadow-xs',
		isHovered &&
			'border-[var(--accent)] bg-[var(--bg-canvas-node-hover)] shadow-sm translate-y-[-1px]',
		isSelected &&
			'border-[var(--accent)] ring-2 ring-[var(--accent)]/30 bg-[var(--bg-canvas-node-selected)]'
	)}
>
	<!-- Top Row: Icon, Title, Status Indicator -->
	<div class="flex items-center justify-between gap-1.5 w-full min-w-0">
		<div class="flex items-center gap-2 min-w-0">
			<span class="text-[var(--text-secondary)] shrink-0">
				{#if node.type === 'domain'}
					<Globe size={14} class="text-[var(--accent)]" />
				{:else if isDatabase}
					<Database size={14} class="text-[var(--status-amber)]" />
				{:else if isQuadlet}
					<FileText size={14} class="text-[var(--accent)]" />
				{:else if isCompose}
					<Stack size={14} class="text-[var(--accent)]" />
				{:else if node.type === 'service'}
					<AppWindow size={14} />
				{:else if node.type === 'container'}
					<Cube size={14} />
				{:else if node.type === 'volume'}
					<HardDrive size={14} class="text-[var(--text-tertiary)]" />
				{:else if node.type === 'network'}
					<ShareNetwork size={14} class="text-[var(--text-secondary)]" />
				{:else if node.type === 'proxy'}
					<ShieldCheck size={14} class="text-[var(--status-green)]" />
				{:else}
					<Cloud size={14} />
				{/if}
			</span>
			<span
				class="text-[13px] font-semibold text-[var(--text-primary)] truncate font-[var(--font-sans)] leading-snug"
				title={node.title}
			>
				{node.title}
			</span>
		</div>

		{#if node.status}
			<div class="flex items-center justify-center shrink-0 w-3 h-3 relative" title={`Status: ${node.status}`}>
				{#if node.status === 'running' || node.status === 'active' || node.status === 'healthy'}
					<span class="absolute w-2 h-2 rounded-full bg-[var(--status-green)] opacity-35 animate-ping"></span>
				{/if}
				<span class={cn('w-1.5 h-1.5 rounded-full z-10', getStatusColor(node.status))}></span>
			</div>
		{/if}
	</div>

	<!-- Bottom Row: Subtitle/Type and Monospace Detail -->
	<div class="flex items-center justify-between gap-1.5 w-full text-[11px] mt-0.5">
		<span class="text-[var(--text-tertiary)] truncate text-[11px]" title={node.subtitle}>
			{node.subtitle || (isDatabase ? 'Database' : isQuadlet ? 'Quadlet Service' : node.type)}
		</span>

		{#if cleanMonoDetail}
			<span
				class="text-[10px] font-mono text-[var(--text-secondary)] bg-[var(--bg-canvas-pill)] px-1.5 py-0.5 rounded border border-[var(--bg-canvas-pill-border)] shrink-0"
			>
				{cleanMonoDetail}
			</span>
		{/if}
	</div>
</div>
