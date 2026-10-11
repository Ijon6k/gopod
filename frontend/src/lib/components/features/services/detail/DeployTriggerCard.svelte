<script lang="ts">
	import type { Service } from '$lib/types';
	import { Input, CopyButton } from '$lib/components/ui';
	import { LayerCard, Button, Switch, useKumoToastManager } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { Lightning, Check } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	let autoDeploy = $state(true);
	let watchPaths = $state('');
	let webhookUrl = $derived(api.services.getWebhookUrl(service.webhookToken || service.id));
	let saveStatus = $state<'idle' | 'saved'>('idle');

	$effect(() => {
		autoDeploy = service.autoDeploy ?? true;
		watchPaths = service.watchPaths || '';
	});

	function handleSave() {
		service.autoDeploy = autoDeploy;
		service.watchPaths = watchPaths.trim();
		dataStore.updateService(service);
		saveStatus = 'saved';
		toast.add({
			title: 'Triggers Saved',
			description: 'Webhook and auto-deploy settings updated.',
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
				<Lightning size={15} weight="duotone" />
			</div>
			<div class="flex flex-col">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-kumo-default">Deployment Triggers</span>
					<span
						class="rounded-sm bg-kumo-base px-1.5 py-0.5 font-mono text-[10px] text-kumo-subtle ring-1 ring-kumo-hairline"
					>
						{autoDeploy ? 'Auto-Deploy Enabled' : 'Manual Only'}
					</span>
				</div>
				<span class="text-[11px] text-kumo-subtle">
					Git webhook triggers and repository path filters
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
				Save Triggers
			</Button>
		</div>
	</LayerCard.Secondary>

	<!-- Content Body (LayerCard.Primary) -->
	<LayerCard.Primary class="flex flex-col gap-3.5 p-4">
		<!-- Webhook URL Row with Copy Button -->
		<div class="flex flex-col gap-1">
			<div class="flex items-center justify-between">
				<label for="trig-webhook-url" class="text-xs font-medium text-kumo-default">
					Webhook Endpoint
				</label>
				<CopyButton text={webhookUrl} label="Copy Webhook URL" variant="inline" />
			</div>

			<div
				class="flex items-center justify-between rounded-md bg-kumo-elevated px-3 py-1.5 text-xs font-mono text-kumo-subtle ring-1 ring-kumo-hairline select-all break-all"
			>
				<span>{webhookUrl}</span>
			</div>
			<span class="text-[11px] text-kumo-subtle">
				Paste into GitHub or GitLab webhook settings (payload: JSON, event: Push).
			</span>
		</div>

		<!-- 2-column: Auto-deploy Toggle + Watch Paths -->
		<div class="grid grid-cols-1 items-center gap-3 pt-1 sm:grid-cols-2">
			<div class="flex items-center gap-2.5">
				<Switch
					size="sm"
					id="auto-deploy-switch"
					bind:checked={autoDeploy}
					aria-label="Automatic deployment on git push"
				/>
				<div class="flex flex-col">
					<label
						for="auto-deploy-switch"
						class="cursor-pointer text-xs font-medium text-kumo-default"
					>
						Auto-deploy on Git Push
					</label>
					<span class="text-[11px] text-kumo-subtle">
						Trigger fresh build and rollout on every commit
					</span>
				</div>
			</div>

			<div class="flex flex-col gap-1">
				<label for="trig-watch-paths" class="text-xs font-medium text-kumo-default">
					Watch Paths (Optional)
				</label>
				<Input
					id="trig-watch-paths"
					bind:value={watchPaths}
					placeholder="src/**, packages/backend/**"
					class="text-xs font-mono"
				/>
			</div>
		</div>
	</LayerCard.Primary>
</LayerCard>
