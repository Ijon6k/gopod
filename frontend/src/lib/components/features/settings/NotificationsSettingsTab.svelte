<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { NotificationChannel } from '$lib/api/system/types';
	import {
		Button,
		Field,
		Input,
		Select,
		Switch,
		Checkbox,
		LayerCard,
		Badge,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		ConfirmDialog
	} from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { BellRinging, Plus, PaperPlaneTilt, Trash } from 'phosphor-svelte';

	const toast = useKumoToastManager();

	const availableEvents = [
		{ id: 'deploy.succeeded', label: 'Deployment succeeded' },
		{ id: 'deploy.failed', label: 'Deployment failed' },
		{ id: 'backup.failed', label: 'Backup / S3 mirror failed' },
		{ id: 'container.alert', label: 'Container health alert' }
	];

	let channels = $state<NotificationChannel[]>([]);
	let isLoading = $state(true);
	let showForm = $state(false);
	let isSaving = $state(false);
	let testingId = $state<string | null>(null);
	let deletingChannel = $state<NotificationChannel | null>(null);

	let formName = $state('');
	let formProvider = $state<'discord' | 'telegram'>('discord');
	let formWebhookUrl = $state('');
	let formBotToken = $state('');
	let formChatId = $state('');
	let formEvents = $state<string[]>(['deploy.failed', 'backup.failed']);

	async function loadChannels() {
		isLoading = true;
		try {
			channels = await api.system.listNotifications();
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to load notification channels'));
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		void loadChannels();
	});

	function toggleEvent(ev: string) {
		if (formEvents.includes(ev)) {
			formEvents = formEvents.filter((e) => e !== ev);
		} else {
			formEvents = [...formEvents, ev];
		}
	}

	async function handleCreate() {
		if (isSaving) return;
		isSaving = true;
		try {
			const created = await api.system.createNotification({
				name: formName.trim(),
				provider: formProvider,
				webhookUrl: formProvider === 'discord' ? formWebhookUrl.trim() : undefined,
				botToken: formProvider === 'telegram' ? formBotToken.trim() : undefined,
				chatId: formProvider === 'telegram' ? formChatId.trim() : undefined,
				events: formEvents,
				enabled: true
			});
			channels = [created, ...channels];
			formName = '';
			formWebhookUrl = '';
			formBotToken = '';
			formChatId = '';
			showForm = false;
			toast.add({
				title: 'Notification channel added',
				description: `"${created.name}" (${created.provider}) saved and encrypted.`,
				variant: 'default'
			});
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to add notification channel'));
		} finally {
			isSaving = false;
		}
	}

	async function handleToggleEnabled(ch: NotificationChannel) {
		try {
			const updated = await api.system.updateNotification(ch.id, { enabled: !ch.enabled });
			channels = channels.map((item) => (item.id === updated.id ? updated : item));
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to update channel state'));
		}
	}

	async function handleTest(ch: NotificationChannel) {
		testingId = ch.id;
		try {
			await api.system.testNotification(ch.id);
			toast.add({
				title: 'Test alert delivered',
				description: `Notification sent cleanly to ${ch.name}.`,
				variant: 'default'
			});
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Notification delivery failed'));
		} finally {
			testingId = null;
		}
	}

	async function confirmDelete() {
		if (!deletingChannel) return;
		try {
			await api.system.deleteNotification(deletingChannel.id);
			channels = channels.filter((c) => c.id !== deletingChannel?.id);
			toast.add({
				title: 'Channel deleted',
				description: `Removed "${deletingChannel.name}".`,
				variant: 'default'
			});
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to remove channel'));
		} finally {
			deletingChannel = null;
		}
	}
</script>

<div class="flex flex-col gap-6">
	<LayerCard class="flex flex-col gap-4 p-5">
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-subtle)] pb-3"
		>
			<div class="flex items-center gap-2">
				<BellRinging size={16} class="text-[var(--accent)]" />
				<div class="flex flex-col gap-0.5">
					<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
						Alerting &amp; Webhook Notifications
					</h3>
					<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
						Send real-time deployment outcomes and scheduled backup alerts to Discord or Telegram.
						Secrets are encrypted at rest with the cluster master key.
					</p>
				</div>
			</div>
			<Button variant="primary" size="sm" onclick={() => (showForm = !showForm)}>
				<Plus size={14} />
				<span>Add channel</span>
			</Button>
		</div>

		{#if showForm}
			<div
				class="flex flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)] p-4"
			>
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
					<Field label="Channel label">
						<Input bind:value={formName} placeholder="Homelab Ops Discord" class="text-xs" />
					</Field>
					<Field label="Provider">
						<Select
							value={formProvider}
							onValueChange={(v: any) => {
								if (v === 'discord' || v === 'telegram') formProvider = v;
							}}
							options={[
								{ label: 'Discord Webhook', value: 'discord' },
								{ label: 'Telegram Bot', value: 'telegram' }
							]}
							size="sm"
						/>
					</Field>
				</div>

				{#if formProvider === 'discord'}
					<Field label="Discord Webhook URL (HTTPS)">
						<Input
							bind:value={formWebhookUrl}
							type="password"
							placeholder="https://discord.com/api/webhooks/..."
							class="font-mono text-xs"
						/>
					</Field>
				{:else}
					<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
						<Field label="Bot token">
							<Input
								bind:value={formBotToken}
								type="password"
								placeholder="123456:ABC-DEF..."
								class="font-mono text-xs"
							/>
						</Field>
						<Field label="Chat ID">
							<Input
								bind:value={formChatId}
								placeholder="-1001234567890"
								class="font-mono text-xs"
							/>
						</Field>
					</div>
				{/if}

				<div class="flex flex-col gap-2">
					<span class="text-xs font-medium text-[var(--text-secondary)]">Subscribed events</span>
					<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
						{#each availableEvents as ev (ev.id)}
							<label
								class="flex cursor-pointer items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-2 text-xs select-none"
							>
								<Checkbox
									checked={formEvents.includes(ev.id)}
									onCheckedChange={() => toggleEvent(ev.id)}
								/>
								<span class="text-[var(--text-primary)]">{ev.label}</span>
							</label>
						{/each}
					</div>
				</div>

				<div class="flex justify-end gap-2 border-t border-[var(--border-subtle)] pt-3">
					<Button variant="secondary" size="sm" onclick={() => (showForm = false)}>Cancel</Button>
					<Button variant="primary" size="sm" onclick={handleCreate} disabled={isSaving}>
						{isSaving ? 'Saving...' : 'Save channel'}
					</Button>
				</div>
			</div>
		{/if}

		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<Table class="w-full text-left text-[13px]">
				<TableHeader
					class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
				>
					<TableRow>
						<TableHead class="px-3.5 py-2.5 text-xs font-semibold text-[var(--text-tertiary)]"
							>Channel</TableHead
						>
						<TableHead class="px-3 py-2.5 text-xs font-semibold text-[var(--text-tertiary)]"
							>Provider</TableHead
						>
						<TableHead class="px-3 py-2.5 text-xs font-semibold text-[var(--text-tertiary)]"
							>Events</TableHead
						>
						<TableHead class="px-3 py-2.5 text-xs font-semibold text-[var(--text-tertiary)]"
							>Enabled</TableHead
						>
						<TableHead
							class="px-3.5 py-2.5 text-right text-xs font-semibold text-[var(--text-tertiary)]"
							>Actions</TableHead
						>
					</TableRow>
				</TableHeader>
				<TableBody class="divide-y divide-[var(--border-subtle)]">
					{#if isLoading}
						<TableRow>
							<TableCell colspan={5} class="py-6 text-center text-xs text-[var(--text-tertiary)]">
								Loading notification channels…
							</TableCell>
						</TableRow>
					{:else if channels.length === 0}
						<TableRow>
							<TableCell colspan={5} class="py-6 text-center text-xs text-[var(--text-tertiary)]">
								No alert channels configured yet. Add Discord or Telegram above to receive deploy
								and backup notifications.
							</TableCell>
						</TableRow>
					{:else}
						{#each channels as ch (ch.id)}
							<TableRow class="hover:bg-[var(--bg-table-row-hover)]">
								<TableCell class="px-3.5 py-2.5">
									<div class="flex flex-col">
										<span class="font-medium text-[var(--text-primary)]">{ch.name}</span>
										<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
											{ch.provider === 'discord'
												? ch.webhookHint || 'Webhook configured'
												: `Chat ${ch.chatId}`}
										</span>
									</div>
								</TableCell>
								<TableCell class="px-3 py-2.5">
									<Badge variant="neutral">{ch.provider}</Badge>
								</TableCell>
								<TableCell class="px-3 py-2.5 text-xs text-[var(--text-secondary)]">
									{(ch.events || []).join(', ')}
								</TableCell>
								<TableCell class="px-3 py-2.5">
									<Switch checked={ch.enabled} onCheckedChange={() => handleToggleEnabled(ch)} />
								</TableCell>
								<TableCell class="px-3.5 py-2.5 text-right">
									<div class="inline-flex items-center gap-1.5">
										<Button
											variant="secondary"
											size="sm"
											onclick={() => handleTest(ch)}
											disabled={testingId === ch.id}
										>
											<PaperPlaneTilt size={13} />
											<span>{testingId === ch.id ? 'Sending...' : 'Test'}</span>
										</Button>
										<button
											type="button"
											onclick={() => (deletingChannel = ch)}
											class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] hover:bg-[var(--status-red-muted)] hover:text-[var(--status-red)]"
											title="Delete channel"
											aria-label="Delete channel"
										>
											<Trash size={14} />
										</button>
									</div>
								</TableCell>
							</TableRow>
						{/each}
					{/if}
				</TableBody>
			</Table>
		</div>
	</LayerCard>
</div>

<ConfirmDialog
	open={deletingChannel !== null}
	title="Delete notification channel?"
	description={deletingChannel ? `Stop sending alerts to "${deletingChannel.name}"?` : ''}
	confirmText="Delete channel"
	variant="danger"
	oncancel={() => (deletingChannel = null)}
	onconfirm={confirmDelete}
/>
