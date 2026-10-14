<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button } from '$lib/components/ui';
	import { Modal } from '$lib/components/ui';
	import { api } from '$lib/api';
	import { Switch } from 'kumo-svelte';
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

	let webhookUrl = $derived(api.services.getWebhookUrl(service.webhookToken || service.id));

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
	<Button variant="secondary" size="sm" onclick={onclose}>Done</Button>
{/snippet}

<Modal
	{open}
	{onclose}
	title="Webhook deployment trigger"
	subtitle="Trigger zero-downtime rolling updates via webhook or CI/CD pipelines."
	icon={Globe}
	size="lg"
	footer={modalFooter}
>
	<div class="flex flex-col gap-4">
		<!-- Auto deploy toggle -->
		<div
			class="flex items-center justify-between rounded-lg bg-kumo-base p-3.5 ring ring-kumo-line"
		>
			<div class="flex flex-col gap-0.5">
				<span class="text-sm font-medium text-kumo-default">Auto-deploy on git push</span>
				<span class="text-xs text-kumo-subtle">
					Automatically pull container and reload Quadlet unit when new commits hit <code
						class="rounded bg-kumo-recessed px-1 py-0.5 font-mono text-xs text-kumo-default"
						>{service.branch || 'main'}</code
					>.
				</span>
			</div>

			<Switch size="sm" bind:checked={autoDeployEnabled} aria-label="Auto-deploy on git push" />
		</div>

		<!-- Webhook Endpoint URL -->
		<div class="flex flex-col gap-1.5">
			<span class="text-xs font-medium text-kumo-subtle">Webhook URL (POST)</span>
			<div
				class="flex items-center gap-2 rounded-lg bg-kumo-recessed p-2 font-mono text-xs text-kumo-default ring ring-kumo-hairline/60"
			>
				<span class="flex-1 truncate">{webhookUrl}</span>
				<Button variant="ghost" size="sm" onclick={copyUrl} class="h-6 gap-1 px-2 text-xs">
					{#if copiedUrl}
						<Check size={12} class="text-kumo-success" />
						<span class="text-kumo-success">Copied</span>
					{:else}
						<Copy size={12} />
						<span>Copy</span>
					{/if}
				</Button>
			</div>
		</div>

		<!-- cURL Command Example -->
		<div class="flex flex-col gap-1.5">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-kumo-subtle">cURL trigger command</span>
				<Button
					variant="ghost"
					size="sm"
					onclick={copyCurl}
					class="h-6 gap-1 px-2 text-xs text-kumo-subtle hover:text-kumo-default"
				>
					{#if copiedCurl}
						<Check size={12} class="text-kumo-success" />
						<span class="text-kumo-success">Copied</span>
					{:else}
						<Copy size={12} />
						<span>Copy command</span>
					{/if}
				</Button>
			</div>

			<pre
				class="m-0 overflow-x-auto rounded-lg bg-[var(--bg-terminal)] p-3 font-mono text-xs leading-relaxed whitespace-pre text-[var(--code-text)] ring ring-kumo-hairline/50">{curlCommand}</pre>
		</div>
	</div>
</Modal>
