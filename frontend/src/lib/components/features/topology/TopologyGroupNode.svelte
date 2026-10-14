<script lang="ts">
	import type { NodeProps } from '@xyflow/svelte';
	import type { TopologyNode } from './types';
	import { Badge as KumoBadge } from 'kumo-svelte';
	import { cn } from '$lib/utils/cn';
	import { Stack, FileText, Folder, ShareNetwork, AppWindow } from 'phosphor-svelte';

	let { data, selected }: NodeProps = $props();
	let node = $derived(data.node as TopologyNode);

	let groupType = $derived(node.groupType || 'service');

	function getStatusDotColor(status?: string) {
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

	let borderStyle = $derived.by(() => {
		if (groupType === 'pod') {
			return 'border-2 border-dashed border-[var(--status-blue)] bg-[var(--color-kumo-base)] shadow-xs';
		}
		if (groupType === 'compose') {
			return 'border border-[var(--color-kumo-line)] bg-[var(--color-kumo-base)] shadow-xs';
		}
		if (groupType === 'quadlet') {
			return 'border border-[var(--accent)]/50 bg-[var(--color-kumo-base)] shadow-xs';
		}
		if (groupType === 'project') {
			return 'border border-[var(--color-kumo-line)] bg-[var(--color-kumo-base)] shadow-sm';
		}
		return 'border border-[var(--color-kumo-line)] bg-[var(--color-kumo-base)] shadow-xs';
	});

	let badgeVariant = $derived.by(() => {
		if (groupType === 'pod') return 'info';
		if (groupType === 'compose') return 'secondary';
		if (groupType === 'quadlet') return 'accent';
		return 'outline';
	});
</script>

<div
	style="width: {node.width}px; height: {node.height}px;"
	class={cn(
		'relative rounded-[var(--radius-card)] p-3.5 text-left select-none',
		borderStyle,
		selected && 'border-[var(--accent)] shadow-md ring-2 ring-[var(--accent)]'
	)}
>
	<!-- Group Header (Drag Handle) styled strictly with Kumo Semantic Tokens -->
	<div
		class="drag-handle -mx-3.5 -mt-3.5 mb-2 flex cursor-grab items-center justify-between rounded-t-[calc(var(--radius-card)-1px)] border-b border-[var(--color-kumo-line)] bg-[var(--color-kumo-elevated)] px-3.5 py-2.5 active:cursor-grabbing"
	>
		<div class="flex min-w-0 items-center gap-2">
			<div
				class={cn(
					'flex h-6 w-6 shrink-0 items-center justify-center rounded-[var(--radius-sm)] border border-[var(--color-kumo-line)] bg-[var(--color-kumo-base)] shadow-2xs',
					groupType === 'pod' ? 'text-[var(--status-blue)]' : 'text-[var(--text-secondary)]'
				)}
			>
				{#if groupType === 'pod'}
					<ShareNetwork size={13} weight="bold" />
				{:else if groupType === 'compose'}
					<Stack size={13} weight="bold" />
				{:else if groupType === 'quadlet'}
					<FileText size={13} weight="bold" />
				{:else if groupType === 'project'}
					<Folder size={13} weight="bold" />
				{:else}
					<AppWindow size={13} weight="bold" />
				{/if}
			</div>

			<div class="min-w-0">
				<div class="flex items-center gap-1.5">
					<span
						class="max-w-[210px] truncate text-xs font-[var(--font-sans)] font-semibold text-[var(--text-primary)]"
					>
						{node.title}
					</span>
					{#if node.status}
						<span class={cn('h-1.5 w-1.5 shrink-0 rounded-full', getStatusDotColor(node.status))}
						></span>
					{/if}
				</div>
				{#if node.subtitle}
					<span class="block max-w-[210px] truncate text-xs text-[var(--text-tertiary)]">
						{node.subtitle}
					</span>
				{/if}
			</div>
		</div>

		<div class="flex shrink-0 items-center gap-1.5">
			{#if node.monoDetail}
				<span
					class="rounded-[var(--radius-xs)] border border-[var(--color-kumo-line)] bg-[var(--color-kumo-base)] px-1.5 py-0.5 font-mono text-xs font-medium text-[var(--text-secondary)]"
				>
					{node.monoDetail}
				</span>
			{/if}
			{#if node.badge}
				<KumoBadge variant={badgeVariant as any} size="sm">
					{node.badge}
				</KumoBadge>
			{/if}
		</div>
	</div>

	<!-- Child Content Zone rendered within parent group boundary -->
	<div class="pointer-events-none relative mt-2 h-[calc(100%-42px)] w-full"></div>
</div>
