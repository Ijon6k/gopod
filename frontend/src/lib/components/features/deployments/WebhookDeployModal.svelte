<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button } from '$lib/components/primitives';
	import { Modal } from '$lib/components/ui';
	import { Copy, Check, Globe } from 'phosphor-svelte';

	interface Props {
		service: Service;
		open: boolean;
		onclose: () => void;
	}

	let { service, open = $bindable(false), onclose }: Props = $props();

	let autoDeployEnabled = $state(true);
	let copiedUrl = $state(false);
	let copiedCurl = $state(false);

	let webhookUrl = $derived(
		`http://localhost:5173/api/v1/deploy/webhook/${service.projectId}/${service.id}?token=gpd_sec_${service.id.substring(0, 6)}`
	);

	let curlCommand = $derived(
		`curl -X POST "${webhookUrl}" \\\n  -H "Content-Type: application/json" \\\n  -d '{"ref": "refs/heads/${service.branch || 'main'}", "commit": "HEAD"}'`
	);

	async function copyUrl() {
		try {
			await navigator.clipboard.writeText(webhookUrl);
			copiedUrl = true;
			setTimeout(() => {
				copiedUrl = false;
			}, 1800);
		} catch (err) {
			console.error('Failed to copy webhook url', err);
		}
	}

	async function copyCurl() {
		try {
			await navigator.clipboard.writeText(curlCommand);
			copiedCurl = true;
			setTimeout(() => {
				copiedCurl = false;
			}, 1800);
		} catch (err) {
			console.error('Failed to copy curl command', err);
		}
	}
</script>

{#snippet modalFooter()}
	<Button variant="secondary" size="sm" onclick={onclose}>
		Done
	</Button>
{/snippet}

<Modal
	{open}
	{onclose}
	title="Webhook & CI/CD Deployment Trigger"
	subtitle="Trigger zero-downtime rolling updates via webhook or CI/CD pipelines."
	icon={Globe}
	size="lg"
	footer={modalFooter}
>
	<!-- Auto deploy toggle -->
	<div class="flex items-center justify-between p-3.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
		<div class="flex flex-col gap-0.5">
			<span class="font-medium text-[var(--text-primary)]">Auto-deploy on Git push</span>
			<span class="text-[var(--text-tertiary)]">
				Automatically pull container and reload Quadlet unit when new commits hit <code class="font-[var(--font-mono)] text-[var(--text-secondary)]">{service.branch || 'main'}</code>.
			</span>
		</div>

		<label class="relative inline-flex items-center cursor-pointer">
			<input
				type="checkbox"
				bind:checked={autoDeployEnabled}
				class="sr-only peer"
			/>
			<div class="w-9 h-5 bg-[var(--bg-surface)] border border-[var(--border)] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-[var(--accent)]"></div>
		</label>
	</div>

	<!-- Webhook Endpoint URL -->
	<div class="flex flex-col gap-1.5">
		<span class="font-medium text-[var(--text-secondary)]">Webhook URL (POST)</span>
		<div class="flex items-center gap-2 p-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] font-[var(--font-mono)] text-[11px] text-[var(--text-primary)]">
			<span class="flex-1 truncate">{webhookUrl}</span>
			<Button variant="ghost" size="sm" onclick={copyUrl} class="h-6 gap-1 px-2 text-[10.5px]">
				{#if copiedUrl}
					<Check size={11} class="text-[var(--status-green)]" />
					<span class="text-[var(--status-green)]">Copied</span>
				{:else}
					<Copy size={11} />
					<span>Copy</span>
				{/if}
			</Button>
		</div>
	</div>

	<!-- cURL Command Example -->
	<div class="flex flex-col gap-1.5">
		<div class="flex items-center justify-between">
			<span class="font-medium text-[var(--text-secondary)]">cURL Trigger Command</span>
			<button
				type="button"
				onclick={copyCurl}
				class="inline-flex items-center gap-1 text-[11px] text-[var(--text-tertiary)] hover:text-[var(--text-primary)] cursor-pointer border-0 bg-transparent"
			>
				{#if copiedCurl}
					<Check size={11} class="text-[var(--status-green)]" />
					<span class="text-[var(--status-green)]">Copied</span>
				{:else}
					<Copy size={11} />
					<span>Copy cURL</span>
				{/if}
			</button>
		</div>

		<pre class="p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-terminal)] font-[var(--font-mono)] text-[11.5px] leading-relaxed text-[var(--code-text)] overflow-x-auto whitespace-pre m-0">{curlCommand}</pre>
	</div>
</Modal>
