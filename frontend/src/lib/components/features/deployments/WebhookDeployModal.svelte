<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button } from '$lib/components/ui';
	import { Modal } from '$lib/components/ui';
	import { api } from '$lib/api';
	import { Switch } from 'kumo-svelte';
	import { Copy, Check, Globe, ArrowClockwise } from 'phosphor-svelte';

	interface Props {
		service: Service;
		open: boolean;
		onclose: () => void;
	}

	let { service, open = $bindable(false), onclose }: Props = $props();

	let autoDeployEnabled = $state(true);
	let copiedUrl = $state(false);
	let copiedCurl = $state(false);

	// The webhook token is not part of service payloads anymore; it is fetched
	// (and rotated) through the dedicated endpoints.
	let webhookToken = $state('');
	let hasWebhook = $state(false);
	let tokenError = $state<string | null>(null);
	let isRotating = $state(false);
	let rotated = $state(false);
	let loadedFor = $state<string | null>(null);

	let webhookUrl = $derived(webhookToken ? api.services.getWebhookUrl(webhookToken) : '');

	let curlCommand = $derived(
		webhookUrl
			? `curl -X POST "${webhookUrl}" \\\n  -H "Content-Type: application/json" \\\n  -d '{"ref": "refs/heads/${service.branch || 'main'}", "commit": "HEAD"}'`
			: ''
	);

	$effect(() => {
		if (open && loadedFor !== service.id) {
			void loadToken();
		}
	});

	async function loadToken() {
		tokenError = null;
		try {
			const res = await api.services.getWebhookToken(service.id);
			webhookToken = res.webhookToken ?? '';
			hasWebhook = res.hasWebhook;
			loadedFor = service.id;
		} catch (err: any) {
			tokenError = err?.message || 'Failed to load the webhook token';
		}
	}

	async function rotateToken() {
		isRotating = true;
		tokenError = null;
		try {
			const res = await api.services.rotateWebhookToken(service.id);
			webhookToken = res.webhookToken ?? '';
			hasWebhook = res.hasWebhook;
			rotated = true;
			setTimeout(() => {
				rotated = false;
			}, 1800);
		} catch (err: any) {
			tokenError = err?.message || 'Failed to rotate the webhook token';
		} finally {
			isRotating = false;
		}
	}

	async function copyUrl() {
		if (!webhookUrl) return;
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
		if (!curlCommand) return;
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
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-kumo-subtle">Webhook URL (POST)</span>
				<Button
					variant="ghost"
					size="sm"
					onclick={rotateToken}
					disabled={isRotating || !hasWebhook}
					class="h-6 gap-1 px-2 text-xs text-kumo-subtle hover:text-kumo-default"
					title="Issue a new token; the previous URL stops working immediately"
				>
					<ArrowClockwise size={12} class={isRotating ? 'animate-spin' : ''} />
					<span>{rotated ? 'Rotated' : 'Rotate token'}</span>
				</Button>
			</div>
			<div
				class="flex items-center gap-2 rounded-lg bg-kumo-recessed p-2 font-mono text-xs text-kumo-default ring ring-kumo-hairline/60"
			>
				<span class="flex-1 truncate">
					{#if webhookUrl}
						{webhookUrl}
					{:else if tokenError}
						<span class="text-kumo-warning">{tokenError}</span>
					{:else}
						<span class="text-kumo-subtle">Loading webhook token…</span>
					{/if}
				</span>
				<Button
					variant="ghost"
					size="sm"
					onclick={copyUrl}
					disabled={!webhookUrl}
					class="h-6 gap-1 px-2 text-xs"
				>
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
					disabled={!curlCommand}
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
				class="m-0 overflow-x-auto rounded-lg bg-[var(--bg-terminal)] p-3 font-mono text-xs leading-relaxed whitespace-pre text-[var(--code-text)] ring ring-kumo-hairline/50">{curlCommand ||
					'Available once the webhook token loads.'}</pre>
		</div>
	</div>
</Modal>
