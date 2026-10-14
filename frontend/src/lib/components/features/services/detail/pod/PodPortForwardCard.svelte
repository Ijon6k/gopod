<script lang="ts">
	import type { Service } from '$lib/types';
	import { LayerCard, Badge, Button, Input, useKumoToastManager } from 'kumo-svelte';
	import { Package, PlugsConnected, Globe } from 'phosphor-svelte';
	import { dataStore } from '$lib/data';

	interface Props {
		service: Service;
		podName: string;
	}

	let { service, podName }: Props = $props();

	const toast = useKumoToastManager();

	let isEditingPorts = $state(false);
	let exposedHostPort = $state('');
	let exposedTargetPort = $state('');
	let isSavingPorts = $state(false);

	$effect(() => {
		exposedHostPort = String(service.hostPort || service.port || 8080);
		exposedTargetPort = String(service.port || 80);
	});

	async function handleSavePorts() {
		isSavingPorts = true;
		const hPort = parseInt(exposedHostPort.trim(), 10) || 0;
		const tPort = parseInt(exposedTargetPort.trim(), 10) || 80;

		service.hostPort = hPort;
		service.port = tPort;

		try {
			await dataStore.updateService(service);
			toast.add({
				title: 'Pod Ports Saved',
				description: `Exposed port updated to ${hPort}:${tPort}`,
				variant: 'success'
			});
			isEditingPorts = false;
		} catch (err: any) {
			toast.add({
				title: 'Failed to Save Ports',
				description: err?.message || 'Error updating ports',
				variant: 'error'
			});
		} finally {
			isSavingPorts = false;
		}
	}
</script>

<LayerCard class="overflow-hidden p-0 shadow-xs">
	<LayerCard.Secondary
		class="flex items-center justify-between border-b border-[var(--color-kumo-hairline)] px-5 py-3.5"
	>
		<div class="flex items-center gap-2.5">
			<Package size={16} class="text-[var(--accent)]" />
			<h3 class="m-0 text-sm font-semibold text-[var(--color-kumo-default)]">
				Pod Infrastructure & Exposed Ports
			</h3>
		</div>
		<Badge variant="neutral" class="font-mono text-xs">network: gopod-net</Badge>
	</LayerCard.Secondary>

	<div class="grid grid-cols-1 gap-6 p-5 sm:grid-cols-2 md:grid-cols-4">
		<!-- Pod Name -->
		<div class="flex flex-col gap-1">
			<span class="text-xs text-[var(--color-kumo-subtle)]">Pod Name</span>
			<div class="flex items-center gap-2">
				<span class="font-mono text-sm font-semibold text-[var(--color-kumo-default)]">
					{podName}
				</span>
			</div>
			<span class="text-[11px] text-[var(--color-kumo-subtle)]">Podman namespace</span>
		</div>

		<!-- Network -->
		<div class="flex flex-col gap-1">
			<span class="text-xs text-[var(--color-kumo-subtle)]">Container Network</span>
			<div class="flex items-center gap-1.5">
				<PlugsConnected size={14} class="text-[var(--accent)]" />
				<span class="font-mono text-sm font-medium text-[var(--color-kumo-default)]">
					gopod-net
				</span>
			</div>
			<span class="text-[11px] text-[var(--color-kumo-subtle)]">Shared Netavark bridge</span>
		</div>

		<!-- Exposed Host Port -->
		<div class="flex flex-col gap-1 sm:col-span-2">
			<div class="flex items-center justify-between">
				<span class="text-xs text-[var(--color-kumo-subtle)]">Exposed Host Port Binding</span>
				{#if !isEditingPorts}
					<button
						type="button"
						onclick={() => (isEditingPorts = true)}
						class="cursor-pointer border-0 bg-transparent text-[11px] text-[var(--accent)] hover:underline"
					>
						Edit Port
					</button>
				{/if}
			</div>

			{#if isEditingPorts}
				<div class="flex items-center gap-2">
					<div class="w-28">
						<Input
							bind:value={exposedHostPort}
							placeholder="Host Port"
							class="h-8 font-mono text-xs"
							aria-label="Host Port"
						/>
					</div>
					<span class="text-xs font-medium text-[var(--color-kumo-subtle)]">:</span>
					<div class="w-28">
						<Input
							bind:value={exposedTargetPort}
							placeholder="Pod Port"
							class="h-8 font-mono text-xs"
							aria-label="Pod Port"
						/>
					</div>
					<Button variant="primary" size="sm" onclick={handleSavePorts} disabled={isSavingPorts}>
						Save
					</Button>
					<Button variant="secondary" size="sm" onclick={() => (isEditingPorts = false)}>
						Cancel
					</Button>
				</div>
			{:else}
				<div class="flex items-center gap-2">
					<Globe size={14} class="text-[var(--color-kumo-subtle)]" />
					<span class="font-mono text-sm font-semibold text-[var(--color-kumo-default)]">
						{service.hostPort || service.port || 8080}:{service.port || 80}
					</span>
					<span class="text-xs text-[var(--color-kumo-subtle)]">(Host ➔ Pod)</span>
				</div>
				<span class="text-[11px] text-[var(--color-kumo-subtle)]">
					Traffic routed to internal member port
				</span>
			{/if}
		</div>
	</div>
</LayerCard>
