<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input, CopyButton } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { SettingCard } from '$lib/components/ui';
	import { Lightning, Check } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let autoDeploy = $state(true);
	let watchPaths = $state('');
	let webhookUrl = $derived(
		typeof window !== 'undefined'
			? `${window.location.protocol}//${window.location.host}/api/deploy/webhook/${service.webhookToken || service.id}`
			: `/api/deploy/webhook/${service.webhookToken || service.id}`
	);
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
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}
</script>

{#snippet cardFooter()}
	{#if saveStatus === 'saved'}
		<span class="text-xs text-[var(--status-green)] flex items-center gap-1 mr-2">
			<Check size={14} /> Saved
		</span>
	{/if}
	<Button variant="primary" size="sm" onclick={handleSave}>
		Save Triggers
	</Button>
{/snippet}

<SettingCard
	title="Deployment Triggers & Webhook"
	subtitle="Automate deployments when changes are pushed to your Git repository."
	icon={Lightning}
	footer={cardFooter}
>
	<div class="flex flex-col gap-1.5">
		<div class="flex items-center justify-between">
			<label for="trig-webhook-url" class="text-xs font-medium text-[var(--text-secondary)]">Webhook URL</label>
			<CopyButton
				text={webhookUrl}
				label="Copy Webhook URL"
				variant="inline"
			/>
		</div>

		<div class="p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] font-[var(--font-mono)] text-[11px] text-[var(--text-secondary)] break-all select-all">
			{webhookUrl}
		</div>
		<span class="text-[11px] text-[var(--text-tertiary)]">
			Paste this into GitHub / GitLab repository webhook settings (Content type: <code>application/json</code>, Event: Push).
		</span>
	</div>

	<div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2 border-t border-[var(--border-subtle)]">
		<label class="flex items-center gap-2.5 text-xs text-[var(--text-secondary)] cursor-pointer">
			<input type="checkbox" bind:checked={autoDeploy} class="accent-[var(--accent)] cursor-pointer" />
			<span>Automatic deployment on git push</span>
		</label>

		<div class="flex flex-col gap-1">
			<label for="trig-watch-paths" class="text-xs font-medium text-[var(--text-secondary)]">Watch Paths (Optional)</label>
			<Input
				id="trig-watch-paths"
				bind:value={watchPaths}
				placeholder="src/**, packages/backend/**"
				class="text-xs font-[var(--font-mono)]"
			/>
		</div>
	</div>
</SettingCard>
