<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from 'kumo-svelte';
	import { CopyButton } from '$lib/components/ui';
	import { Cpu, Database, Terminal } from 'phosphor-svelte';
	import type { TopologyNode } from '../types';

	interface Props {
		node: TopologyNode;
		cnt: any;
		raw?: any;
	}

	let { node, cnt, raw }: Props = $props();
</script>

<!-- Telemetry Deck -->
<div class="grid grid-cols-2 gap-2">
	<div
		class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs"
	>
		<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
			<Cpu size={13} />
			<span>CPU Usage</span>
		</div>
		<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
			{(cnt?.cpu || raw?.cpu || 0).toFixed(1)}%
		</div>
	</div>

	<div
		class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs"
	>
		<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
			<Database size={13} />
			<span>Memory</span>
		</div>
		<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
			{cnt?.memory || raw?.memory || 0} MB
		</div>
	</div>
</div>

<!-- Container Image Specification -->
<div
	class="flex flex-col gap-1.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
>
	<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
		<span>Container Image</span>
		<CopyButton text={cnt?.image || node.subtitle || ''} size={12} label="Copy" variant="inline" />
	</div>
	<div
		class="mt-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 font-mono text-xs break-all text-[var(--text-primary)]"
	>
		{cnt?.image || node.subtitle}
	</div>
</div>

<!-- Runtime Diagnostics -->
<div
	class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
>
	<span class="text-xs font-semibold text-[var(--text-primary)]">Runtime State</span>
	<div class="grid grid-cols-2 gap-2 text-xs">
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
		>
			<span class="block text-[10px] text-[var(--text-tertiary)]">Uptime</span>
			<span class="font-mono text-[var(--text-primary)]">{cnt?.uptime || '—'}</span>
		</div>
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
		>
			<span class="block text-[10px] text-[var(--text-tertiary)]">Restart Policy</span>
			<span class="font-mono text-[var(--text-primary)]">always</span>
		</div>
	</div>

	{#if cnt?.ports}
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 text-xs"
		>
			<span class="block text-[10px] text-[var(--text-tertiary)]">Port Forwarding</span>
			<span class="font-mono break-all text-[var(--text-primary)]">
				{Array.isArray(cnt.ports) ? cnt.ports.join(', ') : cnt.ports}
			</span>
		</div>
	{/if}
</div>

<!-- Action Buttons -->
<div class="mt-2 flex flex-col gap-2">
	<Button
		variant="primary"
		size="sm"
		class="w-full justify-center"
		onclick={() => goto(`/runtime/containers?search=${node.title}`)}
	>
		<Terminal size={13} />
		<span>Inspect in Runtime Table</span>
	</Button>
	{#if node.projectId && node.serviceId}
		<Button
			variant="secondary"
			size="sm"
			class="w-full justify-center"
			onclick={() => goto(`/projects/${node.projectId}/services/${node.serviceId}`)}
		>
			<span>Parent Service Details</span>
		</Button>
	{/if}
</div>
