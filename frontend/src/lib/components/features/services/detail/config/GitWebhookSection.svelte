<script lang="ts">
	import type { Service } from '$lib/types';
	import { api } from '$lib/api';
	import { Button, CopyButton, Input, Select } from '$lib/components/ui';
	import { Badge, useKumoToastManager } from 'kumo-svelte';
	import { ArrowsClockwise, CheckCircle, GitBranch, Warning } from 'phosphor-svelte';

	interface Props {
		service: Service;
		provider: '' | 'github' | 'gitlab' | 'gitea';
	}

	let { service, provider = $bindable() }: Props = $props();

	const toast = useKumoToastManager();

	let webhookToken = $state('');
	let hasSecret = $state(false);
	let secretError = $state(false);
	let revealedSecret = $state('');
	let loading = $state(true);
	let rotating = $state(false);
	let lastDelivery = $state<{ ts: string; action: string; message: string } | null>(null);

	let webhookUrl = $derived(webhookToken ? api.services.getWebhookUrl(webhookToken) : '');

	$effect(() => {
		const id = service.id;
		let cancelled = false;
		loading = true;
		webhookToken = '';
		hasSecret = false;
		secretError = false;
		revealedSecret = '';
		lastDelivery = null;
		api.services
			.getWebhookToken(id)
			.then((res) => {
				if (cancelled) return;
				webhookToken = res.webhookToken ?? '';
				hasSecret = res.hasWebhookSecret ?? false;
				secretError = res.webhookSecretError ?? false;
			})
			.catch((err) => {
				if (!cancelled) console.warn('Failed to fetch webhook token', err);
			})
			.finally(() => {
				if (!cancelled) loading = false;
			});

		api.engineEvents
			.list({ serviceId: id, limit: 50 })
			.then((res) => {
				if (cancelled) return;
				const event = res.items.find(
					(item) => item.action === 'webhook.received' || item.action === 'webhook.rejected'
				);
				lastDelivery = event
					? { ts: event.ts, action: event.action ?? '', message: event.message }
					: null;
			})
			.catch(() => {
				if (!cancelled) lastDelivery = null;
			});

		return () => {
			cancelled = true;
		};
	});

	async function generateSecret() {
		if (
			hasSecret &&
			!window.confirm(
				'Rotate the signing secret? The provider webhook stops validating until you paste the new value there.'
			)
		) {
			return;
		}
		rotating = true;
		try {
			const res = await api.services.rotateWebhookSecret(service.id);
			revealedSecret = res.webhookSecret;
			hasSecret = true;
			secretError = false;
			service.hasWebhookSecret = true;
			service.webhookSecretError = false;
			toast.add({
				title: 'Signing secret generated',
				description: 'Copy it now. It will not be shown again.',
				variant: 'success'
			});
		} catch (err: any) {
			toast.add({
				title: 'Could not generate secret',
				description: err?.message || 'The signing secret was not changed.',
				variant: 'error'
			});
		} finally {
			rotating = false;
		}
	}
</script>

<div class="bg-kumo-surface flex flex-col gap-3 rounded-lg border border-kumo-line p-4">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-2">
			<GitBranch size={14} class="text-kumo-subtle" />
			<span class="text-xs font-semibold text-kumo-default">Git webhook</span>
			{#if secretError}
				<Badge variant="destructive" size="sm" class="gap-1">
					<Warning size={11} /> Secret unavailable
				</Badge>
			{:else if hasSecret}
				<Badge variant="success" size="sm" class="gap-1">
					<CheckCircle size={11} /> Signed
				</Badge>
			{:else}
				<Badge variant="warning" size="sm" class="gap-1">
					<Warning size={11} /> Unsigned
				</Badge>
			{/if}
		</div>
		<span class="text-[10px] text-kumo-subtle"
			>Pushes on the tracked branch trigger a deployment.</span
		>
	</div>

	<div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
		<div class="flex flex-col gap-1.5">
			<label for="webhook-provider" class="text-[11px] font-medium text-kumo-subtle">
				Provider
			</label>
			<Select
				id="webhook-provider"
				bind:value={provider}
				options={[
					{ value: '', label: 'None (auto-detect)' },
					{ value: 'github', label: 'GitHub' },
					{ value: 'gitlab', label: 'GitLab' },
					{ value: 'gitea', label: 'Gitea' }
				]}
				class="h-8 text-xs"
			/>
			<span class="text-[10px] text-kumo-subtle">
				Saved with the application settings. When unset, the provider is detected from the delivery
				headers.
			</span>
		</div>

		<div class="flex flex-col gap-1.5">
			<span class="text-[11px] font-medium text-kumo-subtle">Webhook URL</span>
			<div class="flex items-center gap-2">
				<Input
					value={webhookUrl}
					readonly
					placeholder={loading ? 'Loading webhook token…' : 'No webhook token yet'}
					class="h-8 flex-1 bg-kumo-base font-mono text-xs text-kumo-subtle"
				/>
				{#if webhookUrl}
					<CopyButton text={webhookUrl} />
				{/if}
			</div>
			<span class="text-[10px] text-kumo-subtle">
				Payload URL for the repository webhook. Content type: application/json.
			</span>
		</div>
	</div>

	<div class="flex flex-col gap-2 border-t border-kumo-hairline pt-3">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-kumo-default">Signing secret</span>
				<span class="text-[11px] text-kumo-subtle">
					{secretError
						? 'The stored secret cannot be decrypted. Deliveries are rejected until you rotate it.'
						: hasSecret
							? 'Signatures are verified before a deployment starts.'
							: 'Without a secret, anyone holding the URL can trigger a deployment.'}
				</span>
			</div>
			<Button
				variant={hasSecret ? 'secondary' : 'primary'}
				size="sm"
				disabled={rotating}
				onclick={generateSecret}
				class="gap-1.5"
			>
				<ArrowsClockwise size={13} class={rotating ? 'animate-spin' : ''} />
				<span>{hasSecret ? 'Rotate secret' : 'Generate secret'}</span>
			</Button>
		</div>

		{#if revealedSecret}
			<div class="flex items-center gap-2">
				<Input value={revealedSecret} readonly class="h-8 flex-1 bg-kumo-base font-mono text-xs" />
				<CopyButton text={revealedSecret} />
				<Button variant="ghost" size="sm" onclick={() => (revealedSecret = '')}>Dismiss</Button>
			</div>
			<span class="text-[10px] text-kumo-danger">
				Copy this value now. It is stored encrypted and will not be shown again.
			</span>
		{/if}

		{#if provider === 'gitlab'}
			<span class="text-[10px] text-kumo-subtle">
				Use this value as GitLab's <strong>Secret token</strong>. GitLab signing tokens (<code
					>webhook-signature</code
				>) are not supported.
			</span>
		{/if}
	</div>

	<div class="flex flex-wrap items-center gap-2 border-t border-kumo-hairline pt-2.5">
		<span class="text-[11px] font-medium text-kumo-subtle">Last delivery</span>
		{#if lastDelivery}
			<Badge
				variant={lastDelivery.action === 'webhook.rejected' ? 'destructive' : 'neutral'}
				size="sm"
				class="font-mono"
			>
				{new Date(lastDelivery.ts).toLocaleString()}
			</Badge>
			<span class="text-[11px] text-kumo-subtle">{lastDelivery.message}</span>
		{:else}
			<span class="text-[11px] text-kumo-subtle">No delivery recorded yet.</span>
		{/if}
	</div>
</div>
