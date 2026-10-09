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

<div
	class="flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]"
>
	<!-- Card Header -->
	<div class="flex items-center justify-between border-b border-[var(--border-subtle)] px-5 py-4">
		<div class="flex items-center gap-2.5">
			<div
				class="flex h-8 w-8 items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--accent)]"
			>
				<Network size={18} />
			</div>
			<div class="flex flex-col gap-0.5">
				<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
					Port Mapping & Networking
				</h3>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">
					Configure the internal application listening port and host publishing.
				</p>
			</div>
		</div>

		<!-- Live Routing Badge -->
		<div
			class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-1 text-xs font-[var(--font-mono)] text-[var(--text-secondary)]"
		>
			{#if hostPort && parseInt(hostPort, 10) > 0}
				<span class="h-2 w-2 rounded-full bg-[var(--status-green)]"></span>
				<span>Host 0.0.0.0:{hostPort} &rarr; Container :{containerPort || '80'}</span>
			{:else}
				<span class="h-2 w-2 rounded-full bg-[var(--accent)]"></span>
				<span>Caddy Ingress (Internal :{containerPort || '80'})</span>
			{/if}
		</div>
	</div>

	<!-- Content -->
	<div class="flex flex-col gap-4 p-5">
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<!-- Internal Container Port -->
			<div class="flex flex-col gap-1.5">
				<label
					for="pm-container-port"
					class="flex items-center justify-between text-xs font-medium text-[var(--text-secondary)]"
				>
					<span>Internal Container Port *</span>
					<span class="text-[10px] font-normal text-[var(--text-tertiary)]">Target port</span>
				</label>
				<Input
					id="pm-container-port"
					bind:value={containerPort}
					placeholder="80 or 3000"
					class="text-xs font-[var(--font-mono)]"
				/>
				<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
					Port where your app listens inside the container (e.g. <strong>80</strong> for
					Nginx/Whoami, <strong>3000</strong> for Node, <strong>8080</strong> for Java).
				</p>
			</div>

			<!-- Published Host Port -->
			<div class="flex flex-col gap-1.5">
				<label
					for="pm-host-port"
					class="flex items-center justify-between text-xs font-medium text-[var(--text-secondary)]"
				>
					<span>Published Host Port (Optional)</span>
					<span class="text-[10px] font-normal text-[var(--text-tertiary)]">Server access</span>
				</label>
				<Input
					id="pm-host-port"
					bind:value={hostPort}
					placeholder="e.g. 8088"
					class="text-xs font-[var(--font-mono)]"
				/>
				<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
					Port on host machine for direct IP access (e.g. <code>http://IP:8088</code>). Leave empty
					to route via Domains tab only.
				</p>
			</div>
		</div>

		<!-- Ingress Notice / Education -->
		<div
			class="flex items-start gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 text-xs text-[var(--text-secondary)]"
		>
			<Globe size={16} class="mt-0.5 shrink-0 text-[var(--accent)]" />
			<div class="flex flex-col gap-0.5">
				<span class="font-medium text-[var(--text-primary)]">Dokploy & PaaS Ingress Mode</span>
				<span class="text-[11px] leading-relaxed text-[var(--text-tertiary)]">
					If you leave <strong>Published Host Port</strong> empty, your container is protected and
					reachable exclusively through your domain via Caddy Reverse Proxy. To expose directly on
					your server's IP without a domain, enter a Host Port like <code>8088</code> or
					<code>3001</code>.
				</span>
			</div>
		</div>
	</div>

	<!-- Explicit Card Footer -->
	<div
		class="flex items-center justify-end gap-2 border-t border-[var(--border)] bg-[var(--bg-panel)] px-5 py-3"
	>
		{#if saveStatus === 'saved'}
			<span class="mr-2 flex items-center gap-1 text-xs text-[var(--status-green)]">
				<Check size={14} /> Saved
			</span>
		{/if}
		<Button variant="primary" size="sm" onclick={handleSave}>Save Port Settings</Button>
	</div>
</div>
