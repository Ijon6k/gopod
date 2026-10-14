<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Badge } from 'kumo-svelte';
	import { CopyButton } from '$lib/components/ui';
	import { Plugs, ArrowRight } from 'phosphor-svelte';
	import type { TopologyNode } from '../types';

	interface Props {
		node: TopologyNode;
	}

	let { node }: Props = $props();

	let isPublic = $derived(node.title.startsWith('0.0.0.0') || !node.title.startsWith('127.'));
</script>

<div
	class="flex flex-col gap-2.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
>
	<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
		<span class="flex items-center gap-1.5">
			<Plugs
				size={14}
				class={isPublic ? 'text-[var(--status-amber)]' : 'text-[var(--status-blue)]'}
			/>
			<span>Host Port Mapping</span>
		</span>
		<Badge variant={isPublic ? 'warning' : 'info'} size="sm">
			{isPublic ? '0.0.0.0 WAN Exposed' : '127.0.0.1 Loopback'}
		</Badge>
	</div>

	<div
		class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-2"
	>
		<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">{node.title}</span>
		<CopyButton text={node.title} size={13} />
	</div>

	<div class="text-xs leading-relaxed text-[var(--text-secondary)]">
		{#if isPublic}
			Directly published to host interface. Traffic connects straight to container, bypassing Caddy
			reverse proxy.
		{:else}
			Bound to host loopback interface only. Secure from WAN; accessible via localhost or SSH port
			forwarding.
		{/if}
	</div>
</div>

{#if node.projectId && node.serviceId}
	<div class="mt-2">
		<Button
			variant="primary"
			size="sm"
			class="w-full justify-center"
			onclick={() => goto(`/projects/${node.projectId}/services/${node.serviceId}`)}
		>
			<span>Inspect Target Service</span>
			<ArrowRight size={13} />
		</Button>
	</div>
{/if}
