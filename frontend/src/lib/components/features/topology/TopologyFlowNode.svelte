<script lang="ts">
	import { Handle, Position, type NodeProps } from '@xyflow/svelte';
	import type { TopologyNode } from './types';
	import { Badge as KumoBadge } from 'kumo-svelte';
	import { cn } from '$lib/utils/cn';
	import {
		Globe,
		AppWindow,
		Cube,
		HardDrive,
		ShareNetwork,
		ShieldCheck,
		Cloud,
		Database,
		FileText,
		Stack,
		Plugs,
		ArrowsLeftRight
	} from 'phosphor-svelte';

	let { data, selected }: NodeProps = $props();
	let node = $derived(data.node as TopologyNode);

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

	// Flow connectivity rules:
	// - Internet and Entrypoint Ports have NO target handle (traffic originates here)
	// - Storage/Volume and Network leaf nodes have NO outgoing source handle
	let hasTargetHandle = $derived(node.type !== 'internet' && node.type !== 'port');
	let hasSourceHandle = $derived(node.type !== 'volume' && node.type !== 'network');
</script>

<!-- Left Ingress Connection Handle -->
{#if hasTargetHandle}
	<Handle
		type="target"
		position={Position.Left}
		class="!border-2 !border-[var(--bg-canvas-node)]"
	/>
{/if}

<!-- Node Card Surface with Kumo Design Tokens -->
<div
	style="width: {node.width}px; height: {node.height}px;"
	class={cn(
		'cursor-pointer rounded-[var(--radius-sm)] px-3.5 py-2.5 transition-all duration-150 select-none',
		'flex flex-col justify-between border border-[var(--border-canvas-node)] bg-[var(--bg-canvas-node)] text-left shadow-xs',
		'hover:translate-y-[-1px] hover:border-[var(--accent)] hover:bg-[var(--bg-canvas-node-hover)] hover:shadow-sm',
		selected &&
			'border-[var(--accent)] bg-[var(--bg-canvas-node-selected)] ring-2 ring-[var(--accent)]/30'
	)}
>
	<!-- Top Row: Icon, Title, Status Indicator -->
	<div class="flex w-full min-w-0 items-center justify-between gap-1.5">
		<div class="flex min-w-0 items-center gap-2">
			<span class="shrink-0 text-[var(--text-secondary)]">
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
				{:else if node.type === 'port'}
					<Plugs
						size={14}
						class={node.title.startsWith('127.')
							? 'text-[var(--status-blue)]'
							: 'text-[var(--status-amber)]'}
					/>
				{:else}
					<Cloud size={14} />
				{/if}
			</span>
			<span
				class="truncate text-sm leading-snug font-[var(--font-sans)] font-medium text-[var(--text-primary)]"
				title={node.title}
			>
				{node.title}
			</span>
		</div>

		{#if node.status}
			<div
				class="relative flex h-3 w-3 shrink-0 items-center justify-center"
				title={`Status: ${node.status}`}
			>
				{#if node.status === 'running' || node.status === 'active' || node.status === 'healthy'}
					<span
						class="absolute h-2 w-2 animate-ping rounded-full bg-[var(--status-green)] opacity-35"
					></span>
				{/if}
				<span class={cn('z-10 h-1.5 w-1.5 rounded-full', getStatusColor(node.status))}></span>
			</div>
		{/if}
	</div>

	<!-- Bottom Row: Subtitle/Type and Monospace Detail Badge -->
	<div class="mt-0.5 flex w-full items-center justify-between gap-1.5 text-xs">
		<span class="truncate text-xs text-[var(--text-tertiary)]" title={node.subtitle}>
			{node.subtitle || (isDatabase ? 'Database' : isQuadlet ? 'Quadlet service' : node.type)}
		</span>

		{#if cleanMonoDetail}
			<KumoBadge
				variant="secondary"
				class="shrink-0 !rounded border border-[var(--border-subtle)] !px-1.5 !py-0 !font-mono !text-[10px] font-normal text-[var(--text-secondary)]"
			>
				{cleanMonoDetail}
			</KumoBadge>
		{/if}
	</div>
</div>

<!-- Right Egress Connection Handle -->
{#if hasSourceHandle}
	<Handle
		type="source"
		position={Position.Right}
		class="!border-2 !border-[var(--bg-canvas-node)]"
	/>
{/if}
