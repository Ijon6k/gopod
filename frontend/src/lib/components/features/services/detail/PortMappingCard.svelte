<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { Check, Globe, Network } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let containerPort = $state('3000');
	let hostPort = $state('');
	let saveStatus = $state<'idle' | 'saved'>('idle');

	$effect(() => {
		containerPort = String(service.port || 3000);
		hostPort = service.hostPort && service.hostPort > 0 ? String(service.hostPort) : '';
	});

	function handleSave() {
		service.port = parseInt(containerPort, 10) || 3000;
		service.hostPort = parseInt(hostPort, 10) || 0;
		dataStore.updateService(service);
		saveStatus = 'saved';
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}
</script>

<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col">
	<!-- Card Header -->
	<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex items-center justify-between">
		<div class="flex items-center gap-2.5">
			<div class="w-8 h-8 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] border border-[var(--border)] flex items-center justify-center text-[var(--accent)]">
				<Network size={18} />
			</div>
			<div class="flex flex-col gap-0.5">
				<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Port Mapping & Networking</h3>
				<p class="text-xs text-[var(--text-tertiary)] m-0">Configure the internal application listening port and host publishing.</p>
			</div>
		</div>

		<!-- Live Routing Badge -->
		<div class="px-3 py-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] text-xs font-[var(--font-mono)] flex items-center gap-2 text-[var(--text-secondary)]">
			{#if hostPort && parseInt(hostPort, 10) > 0}
				<span class="w-2 h-2 rounded-full bg-[var(--status-green)]"></span>
				<span>Host 0.0.0.0:{hostPort} &rarr; Container :{containerPort || '80'}</span>
			{:else}
				<span class="w-2 h-2 rounded-full bg-[var(--accent)]"></span>
				<span>Caddy Ingress (Internal :{containerPort || '80'})</span>
			{/if}
		</div>
	</div>

	<!-- Content -->
	<div class="p-5 flex flex-col gap-4">
		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<!-- Internal Container Port -->
			<div class="flex flex-col gap-1.5">
				<label for="pm-container-port" class="text-xs font-medium text-[var(--text-secondary)] flex items-center justify-between">
					<span>Internal Container Port *</span>
					<span class="text-[10px] text-[var(--text-tertiary)] font-normal">Target port</span>
				</label>
				<Input
					id="pm-container-port"
					bind:value={containerPort}
					placeholder="80 or 3000"
					class="text-xs font-[var(--font-mono)]"
				/>
				<p class="text-[11px] text-[var(--text-tertiary)] m-0">
					Port where your app listens inside the container (e.g. <strong>80</strong> for Nginx/Whoami, <strong>3000</strong> for Node, <strong>8080</strong> for Java).
				</p>
			</div>

			<!-- Published Host Port -->
			<div class="flex flex-col gap-1.5">
				<label for="pm-host-port" class="text-xs font-medium text-[var(--text-secondary)] flex items-center justify-between">
					<span>Published Host Port (Optional)</span>
					<span class="text-[10px] text-[var(--text-tertiary)] font-normal">Server access</span>
				</label>
				<Input
					id="pm-host-port"
					bind:value={hostPort}
					placeholder="e.g. 8088"
					class="text-xs font-[var(--font-mono)]"
				/>
				<p class="text-[11px] text-[var(--text-tertiary)] m-0">
					Port on host machine for direct IP access (e.g. <code>http://IP:8088</code>). Leave empty to route via Domains tab only.
				</p>
			</div>
		</div>

		<!-- Ingress Notice / Education -->
		<div class="p-3 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] flex items-start gap-2.5 text-xs text-[var(--text-secondary)]">
			<Globe size={16} class="text-[var(--accent)] shrink-0 mt-0.5" />
			<div class="flex flex-col gap-0.5">
				<span class="font-medium text-[var(--text-primary)]">Dokploy & PaaS Ingress Mode</span>
				<span class="text-[11px] text-[var(--text-tertiary)] leading-relaxed">
					If you leave <strong>Published Host Port</strong> empty, your container is protected and reachable exclusively through your domain via Caddy Reverse Proxy. To expose directly on your server's IP without a domain, enter a Host Port like <code>8088</code> or <code>3001</code>.
				</span>
			</div>
		</div>
	</div>

	<!-- Explicit Card Footer -->
	<div class="px-5 py-3 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-end gap-2">
		{#if saveStatus === 'saved'}
			<span class="text-xs text-[var(--status-green)] flex items-center gap-1 mr-2">
				<Check size={14} /> Saved
			</span>
		{/if}
		<Button variant="primary" size="sm" onclick={handleSave}>
			Save Port Settings
		</Button>
	</div>
</div>
