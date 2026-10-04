<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { Lightning, Copy, Check } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let autoDeploy = $state(true);
	let watchPaths = $state('');
	let webhookUrl = $derived(`https://gopod.my.id/api/webhooks/deploy/${service.id}?token=${service.webhookToken || 'sec_82194'}`);
	let copied = $state(false);
	let saveStatus = $state<'idle' | 'saved'>('idle');

	$effect(() => {
		autoDeploy = service.autoDeploy ?? true;
		watchPaths = service.watchPaths || '';
	});

	function handleCopy() {
		navigator.clipboard.writeText(webhookUrl);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	function handleSave() {
		service.autoDeploy = autoDeploy;
		service.watchPaths = watchPaths.trim();
		dataStore.updateService(service);
		saveStatus = 'saved';
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}
</script>

<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col">
	<!-- Card Header -->
	<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex items-center justify-between">
		<div class="flex flex-col gap-0.5">
			<div class="flex items-center gap-2">
				<Lightning size={16} class="text-[var(--accent)]" />
				<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Deployment Triggers & Webhook</h3>
			</div>
			<p class="text-xs text-[var(--text-tertiary)] m-0">Automate deployments when changes are pushed to your Git repository.</p>
		</div>
	</div>

	<!-- Content -->
	<div class="p-5 flex flex-col gap-4">
		<div class="flex flex-col gap-1.5">
			<div class="flex items-center justify-between">
				<label for="trig-webhook-url" class="text-xs font-medium text-[var(--text-secondary)]">Webhook URL</label>
				<button
					type="button"
					onclick={handleCopy}
					class="flex items-center gap-1 text-[11px] text-[var(--accent)] hover:underline bg-transparent border-0 cursor-pointer p-0"
				>
					{#if copied}
						<Check size={12} class="text-[var(--status-green)]" /> Copied!
					{:else}
						<Copy size={12} /> Copy Webhook URL
					{/if}
				</button>
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
	</div>

	<!-- Card Footer -->
	<div class="px-5 py-3 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-end gap-2">
		{#if saveStatus === 'saved'}
			<span class="text-xs text-[var(--status-green)] flex items-center gap-1 mr-2">
				<Check size={14} /> Saved
			</span>
		{/if}
		<Button variant="primary" size="sm" onclick={handleSave}>
			Save Triggers
		</Button>
	</div>
</div>
