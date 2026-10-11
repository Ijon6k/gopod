<script lang="ts">
	import type { Service } from '$lib/types';
	import { Input } from '$lib/components/ui';
	import { LayerCard, Button, useKumoToastManager } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import { Check, Globe, Network } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

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
		toast.add({
			title: 'Port Settings Saved',
			description: `Internal listening port set to :${containerPort}.`,
			variant: 'success'
		});
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- Elevated Header (LayerCard.Secondary) with Title & Save Action -->
	<LayerCard.Secondary
		class="flex flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline px-4 py-3"
	>
		<div class="flex items-center gap-2.5">
			<div
				class="flex size-7 shrink-0 items-center justify-center rounded-md bg-kumo-base text-[var(--accent)] ring ring-kumo-line"
			>
				<Network size={15} weight="duotone" />
			</div>
			<div class="flex flex-col">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-kumo-default">Port Mapping & Ingress</span>
					<!-- Live Routing Badge -->
					<div
						class="flex items-center gap-1.5 rounded-sm bg-kumo-base px-2 py-0.5 font-mono text-[10px] text-kumo-subtle ring-1 ring-kumo-hairline"
					>
						{#if hostPort && parseInt(hostPort, 10) > 0}
							<span class="size-1.5 rounded-full bg-kumo-success"></span>
							<span>Host 0.0.0.0:{hostPort} &rarr; :{containerPort || '80'}</span>
						{:else}
							<span class="size-1.5 rounded-full bg-[var(--accent)]"></span>
							<span>Caddy Ingress (:{containerPort || '80'})</span>
						{/if}
					</div>
				</div>
				<span class="text-[11px] text-kumo-subtle">
					Internal container listening port and optional host publishing
				</span>
			</div>
		</div>

		<!-- Action in Header -->
		<div class="flex items-center gap-2">
			{#if saveStatus === 'saved'}
				<span class="flex items-center gap-1 font-mono text-xs text-kumo-success">
					<Check size={13} weight="bold" /> Saved
				</span>
			{/if}
			<Button
				variant="primary"
				size="sm"
				onclick={handleSave}
				class="gap-1.5 font-medium shadow-xs"
			>
				Save Ports
			</Button>
		</div>
	</LayerCard.Secondary>

	<!-- Content Body (LayerCard.Primary) -->
	<LayerCard.Primary class="flex flex-col gap-3.5 p-4">
		<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
			<!-- Internal Container Port -->
			<div class="flex flex-col gap-1">
				<label
					for="pm-container-port"
					class="flex items-center justify-between text-xs font-medium text-kumo-default"
				>
					<span>Internal Container Port <span class="text-kumo-danger">*</span></span>
					<span class="text-[10px] text-kumo-subtle font-mono">Target port</span>
				</label>
				<Input
					id="pm-container-port"
					bind:value={containerPort}
					placeholder="80 or 3000"
					class="text-xs font-mono"
				/>
				<p class="m-0 text-[11px] text-kumo-subtle">
					Port your app listens on inside container (e.g. <strong>80</strong> for Nginx/Whoami,
					<strong>3000</strong> for Node, <strong>8080</strong> for Java).
				</p>
			</div>

			<!-- Published Host Port -->
			<div class="flex flex-col gap-1">
				<label
					for="pm-host-port"
					class="flex items-center justify-between text-xs font-medium text-kumo-default"
				>
					<span>Published Host Port (Optional)</span>
					<span class="text-[10px] text-kumo-subtle font-mono">Direct server IP</span>
				</label>
				<Input
					id="pm-host-port"
					bind:value={hostPort}
					placeholder="e.g. 8088"
					class="text-xs font-mono"
				/>
				<p class="m-0 text-[11px] text-kumo-subtle">
					Direct host port (e.g. <code>http://IP:8088</code>). Leave empty to route via Domains tab
					only.
				</p>
			</div>
		</div>

		<!-- Compact Ingress Architecture Notice -->
		<div
			class="flex items-center gap-2 rounded-md bg-kumo-elevated px-3 py-2 text-xs text-kumo-subtle ring-1 ring-kumo-hairline"
		>
			<Globe size={15} class="shrink-0 text-[var(--accent)]" />
			<div class="flex flex-wrap items-center gap-1 leading-normal">
				<span class="font-medium text-kumo-default">Ingress Architecture:</span>
				<span>
					Leaving Host Port empty keeps the container isolated; Caddy routes traffic via your
					domain. Specifying a host port exposes it directly on your server IP.
				</span>
			</div>
		</div>
	</LayerCard.Primary>
</LayerCard>
