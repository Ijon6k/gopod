<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Badge } from 'kumo-svelte';
	import { Globe, ArrowRight } from 'phosphor-svelte';
	import type { Domain } from '$lib/types';

	interface Props {
		server: { os: string };
		domains: Domain[];
	}

	let { server, domains }: Props = $props();
</script>

<div
	class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
>
	<span class="text-xs font-semibold text-[var(--text-primary)]">Caddy Ingress Capabilities</span>
	<div class="space-y-1.5 text-xs text-[var(--text-secondary)]">
		<div
			class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5"
		>
			<span>Automatic HTTPS</span>
			<Badge variant="success" size="sm">Let's Encrypt / ACME</Badge>
		</div>
		<div
			class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5"
		>
			<span>Reverse Proxy Ports</span>
			<span class="font-mono text-[var(--text-primary)]">80 (HTTP) · 443 (HTTPS)</span>
		</div>
		<div
			class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5"
		>
			<span>Server Platform</span>
			<span class="font-mono text-[var(--text-primary)]">{server.os}</span>
		</div>
	</div>
</div>

<!-- Active Managed Routes -->
<div
	class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
>
	<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
		<span class="flex items-center gap-1.5">
			<Globe size={14} class="text-[var(--accent)]" />
			<span>Active Ingress Routes ({domains.length})</span>
		</span>
	</div>

	<div class="mt-1 flex max-h-48 flex-col gap-1.5 overflow-y-auto">
		{#each domains as d}
			<div
				class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs"
			>
				<span class="truncate font-mono text-xs text-[var(--text-primary)]">{d.hostname}</span>
				<Badge variant={d.tls ? 'success' : 'secondary'} size="sm">
					{d.tls ? 'TLS' : 'HTTP'}
				</Badge>
			</div>
		{/each}
	</div>
</div>

<div class="mt-2">
	<Button
		variant="primary"
		size="sm"
		class="w-full justify-center"
		onclick={() => goto('/networking/domains')}
	>
		<span>Manage Ingress & Caddyfile</span>
		<ArrowRight size={13} />
	</Button>
</div>
