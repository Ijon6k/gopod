<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Badge } from 'kumo-svelte';
	import { CopyButton } from '$lib/components/ui';
	import { Globe, Lock, LockOpen, ArrowSquareOut } from 'phosphor-svelte';
	import type { TopologyNode } from '../types';

	interface Props {
		node: TopologyNode;
		raw?: any;
	}

	let { node, raw }: Props = $props();
</script>

<div
	class="flex flex-col gap-2.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
>
	<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
		<span class="flex items-center gap-1.5">
			<Globe size={14} class="text-[var(--accent)]" />
			<span>Ingress Endpoint</span>
		</span>
		<Badge variant={raw?.tls ? 'success' : 'secondary'} size="sm">
			{#if raw?.tls}
				<span class="inline-flex items-center gap-1"><Lock size={10} /> TLS Active</span>
			{:else}
				<span class="inline-flex items-center gap-1"><LockOpen size={10} /> HTTP Only</span>
			{/if}
		</Badge>
	</div>

	<div
		class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5"
	>
		<a
			href={`https://${node.title}`}
			target="_blank"
			rel="noreferrer"
			class="flex items-center gap-1 truncate font-mono text-xs text-[var(--accent)] hover:underline"
		>
			<span>https://{node.title}</span>
			<ArrowSquareOut size={12} class="shrink-0" />
		</a>
		<CopyButton text={`https://${node.title}`} size={13} />
	</div>

	<div class="grid grid-cols-2 gap-2 text-xs text-[var(--text-secondary)]">
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
		>
			<span class="block text-[10px] text-[var(--text-tertiary)]">Upstream Port</span>
			<span class="font-mono text-[var(--text-primary)]"
				>:{raw?.containerPort || raw?.proxyPort || 80}</span
			>
		</div>
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
		>
			<span class="block text-[10px] text-[var(--text-tertiary)]">Caddy TLS Gateway</span>
			<span class="font-mono text-[var(--text-primary)]">ports 80, 443</span>
		</div>
	</div>
</div>

<div class="mt-2 flex flex-col gap-2">
	<Button
		variant="primary"
		size="sm"
		class="w-full justify-center"
		href={`https://${node.title}`}
		external={true}
	>
		<span>Visit Live Site</span>
		<ArrowSquareOut size={13} />
	</Button>
	<Button
		variant="secondary"
		size="sm"
		class="w-full justify-center"
		onclick={() => goto('/networking/domains')}
	>
		<span>Manage Domain Settings</span>
	</Button>
</div>
