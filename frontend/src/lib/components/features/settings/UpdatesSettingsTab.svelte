<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { UpdateCheckResponse } from '$lib/api/system/types';
	import {
		Button,
		LayerCard,
		Badge,
		CopyButton,
		ConfirmDialog,
		Checkbox
	} from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import {
		CloudArrowDown,
		ArrowClockwise,
		CheckCircle,
		Warning,
		Package,
		Cpu
	} from 'phosphor-svelte';

	const toast = useKumoToastManager();

	let data = $state<UpdateCheckResponse | null>(null);
	let channel = $state<'latest' | 'dev'>('latest');
	let isChecking = $state(true);
	let isUpdating = $state(false);
	let isReconnecting = $state(false);
	let showConfirmUpdate = $state(false);
	let confirmDevSwitch = $state(false);

	async function checkUpdates(force = false) {
		isChecking = true;
		try {
			const res = await api.system.checkUpdates(channel, force);
			data = res;
			if (!channel && res.channel) channel = res.channel;
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to check for updates'));
		} finally {
			isChecking = false;
		}
	}

	onMount(() => {
		void checkUpdates(false);
	});

	async function handleTriggerUpdate() {
		if (!data || isUpdating) return;
		showConfirmUpdate = false;
		isUpdating = true;
		try {
			const targetTag = channel === 'dev' ? 'dev' : data.latestTag || 'latest';
			await api.system.triggerUpdate({
				tag: targetTag,
				channel,
				confirmSwitchFromDev: data.self.isDevBuild && confirmDevSwitch
			});
			toast.add({
				title: 'Update initiated',
				description: `Pulling ${targetTag} and preparing safe replacement...`,
				variant: 'default'
			});
			isReconnecting = true;
			void waitForHealthReconnect();
		} catch (err: any) {
			isUpdating = false;
			toast.add(errorToastOptions(err, 'Update aborted'));
		}
	}

	async function waitForHealthReconnect() {
		const deadline = Date.now() + 90_000;
		// Give helper/systemd 3s to initiate swap before polling.
		await new Promise((r) => setTimeout(r, 3000));
		while (Date.now() < deadline) {
			try {
				const h = await api.system.health();
				if (h && h.status) {
					await new Promise((r) => setTimeout(r, 800));
					await checkUpdates(true);
					isUpdating = false;
					isReconnecting = false;
					const lastState = data?.lastStatus?.state;
					if (lastState === 'rolled_back' || lastState === 'failed') {
						toast.add({
							title: 'Update rolled back',
							description: data?.lastStatus?.message || 'Previous container restored safely.',
							variant: 'error'
						});
					} else {
						toast.add({
							title: 'GOPOD is back online',
							description: `Running version ${data?.currentVersion || 'updated'}.`,
							variant: 'success'
						});
					}
					return;
				}
			} catch {
				// Expected briefly while container restarts
			}
			await new Promise((r) => setTimeout(r, 2000));
		}
		isUpdating = false;
		isReconnecting = false;
		await checkUpdates(true);
	}

	function modeLabel(mode: string): string {
		switch (mode) {
			case 'quadlet-pod':
				return 'Quadlet Pod (systemd)';
			case 'quadlet-standalone':
				return 'Quadlet Standalone (systemd)';
			case 'compose':
				return 'Podman/Docker Compose';
			case 'container':
				return 'Standalone Container';
			case 'dev-local':
				return 'Local Source Build (Dev)';
			case 'native':
				return 'Native Host Binary';
			default:
				return mode;
		}
	}
</script>

<div class="flex flex-col gap-6">
	<!-- GOPOD Release & Self-Update Card -->
	<LayerCard class="flex flex-col gap-5 p-5">
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-subtle)] pb-3"
		>
			<div class="flex items-center gap-2.5">
				<CloudArrowDown size={18} class="text-[var(--accent)]" />
				<div class="flex flex-col gap-0.5">
					<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
						GOPOD Version &amp; Updates
					</h3>
					<p class="m-0 text-xs text-[var(--text-tertiary)]">
						Tracks official releases from <span class="font-medium text-[var(--text-secondary)]">github.com/ijon6k/gopod</span> and <span class="font-medium text-[var(--text-secondary)]">ghcr.io/ijon6k/gopod</span>.
					</p>
				</div>
			</div>

			<div class="flex items-center gap-2">
				<div
					class="inline-flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
				>
					<button
						type="button"
						onclick={() => {
							channel = 'latest';
							checkUpdates(true);
						}}
						class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {channel === 'latest'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						Latest Stable
					</button>
					<button
						type="button"
						onclick={() => {
							channel = 'dev';
							checkUpdates(true);
						}}
						class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {channel === 'dev'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						Dev (:dev)
					</button>
				</div>

				<Button variant="secondary" size="sm" onclick={() => checkUpdates(true)} disabled={isChecking}>
					<ArrowClockwise size={14} class={isChecking ? 'animate-spin' : ''} />
					<span>Check now</span>
				</Button>
			</div>
		</div>

		{#if isChecking && !data}
			<div class="h-28 animate-pulse rounded-[var(--radius-sm)] bg-[var(--bg-surface)]"></div>
		{:else if data}
			<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
				<div class="flex flex-col gap-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3">
					<span class="text-[11px] text-[var(--text-tertiary)]">Installed version</span>
					<div class="flex items-center gap-2">
						<span class="text-sm font-semibold text-[var(--text-primary)]">{data.currentVersion}</span>
						{#if data.buildCommit}
							<Badge variant="neutral">{data.buildCommit}</Badge>
						{/if}
					</div>
				</div>

				<div class="flex flex-col gap-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3">
					<span class="text-[11px] text-[var(--text-tertiary)]">Upstream ({data.channel})</span>
					<div class="flex items-center gap-2">
						<span class="text-sm font-semibold text-[var(--text-primary)]">
							{data.latestVersion || (channel === 'dev' ? 'dev' : 'No release published yet')}
						</span>
						{#if data.updateAvailable}
							<Badge variant="warning">Update available</Badge>
						{:else if !data.checkError}
							<Badge variant="success">Up to date</Badge>
						{/if}
					</div>
				</div>

				<div class="flex flex-col gap-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3">
					<span class="text-[11px] text-[var(--text-tertiary)]">Deployment mode</span>
					<div class="flex items-center gap-2">
						<Package size={14} class="text-[var(--accent)]" />
						<span class="text-xs font-medium text-[var(--text-primary)]">{modeLabel(data.self.mode)}</span>
					</div>
				</div>
			</div>

			{#if data.checkError}
				<div class="rounded-[var(--radius-sm)] border border-[var(--status-amber)]/30 bg-[var(--status-amber-muted)] p-3 text-xs text-[var(--status-amber)]">
					Could not reach GitHub / GHCR upstream: {data.checkError}
				</div>
			{/if}

			{#if data.lastStatus && data.lastStatus.state !== 'idle'}
				<div
					class="flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border p-3 text-xs {data.lastStatus.state === 'failed' || data.lastStatus.state === 'rolled_back'
						? 'border-[var(--status-red)]/30 bg-[var(--status-red-muted)] text-[var(--status-red)]'
						: data.lastStatus.state === 'updating'
							? 'border-[var(--accent)]/40 bg-[var(--accent-muted)] text-[var(--accent-text)]'
							: 'border-[var(--status-green)]/30 bg-[var(--status-green-muted)] text-[var(--status-green)]'}"
				>
					<span>
						<strong>Last update ({data.lastStatus.state}):</strong> {data.lastStatus.message}
					</span>
				</div>
			{/if}

			{#if isReconnecting}
				<div class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--accent)]/30 bg-[var(--accent-muted)] p-3 text-xs text-[var(--accent-text)]">
					<ArrowClockwise size={14} class="animate-spin" />
					<span>Applying update and waiting for health check recovery…</span>
				</div>
			{/if}

			{#if data.releaseNotes}
				<div class="flex flex-col gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-panel)] p-3">
					<span class="text-[11px] font-semibold text-[var(--text-tertiary)]">Release Notes</span>
					<pre class="m-0 max-h-40 overflow-auto whitespace-pre-wrap font-sans text-xs leading-relaxed text-[var(--text-secondary)]">{data.releaseNotes}</pre>
				</div>
			{/if}

			{#if data.self.canAutoUpdate}
				<div class="flex flex-wrap items-center justify-between gap-3 border-t border-[var(--border-subtle)] pt-3">
					<span class="text-xs text-[var(--text-secondary)]">
						{#if data.self.mode.startsWith('quadlet')}
							Pulls image over Podman socket and performs safe unit restart via host systemd D-Bus.
						{:else if data.self.mode === 'native'}
							Downloads release binary, verifies SHA-256 against checksums.txt, and atomically swaps executable.
						{:else}
							Pre-stages replacement container, verifies /api/health, and auto-rolls back on failure.
						{/if}
					</span>
					<Button
						variant={data.updateAvailable ? 'primary' : 'secondary'}
						size="sm"
						disabled={isUpdating}
						onclick={() => (showConfirmUpdate = true)}
					>
						<CloudArrowDown size={14} />
						<span>{data.updateAvailable ? `Update to ${channel === 'dev' ? 'dev' : data.latestTag || 'latest'}` : 'Re-pull & Refresh'}</span>
					</Button>
				</div>
			{:else}
				<div class="flex flex-col gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3.5">
					<div class="flex items-start gap-2 text-xs text-[var(--text-secondary)]">
						<Warning size={15} class="mt-0.5 shrink-0 text-[var(--status-amber)]" />
						<span>{data.self.autoUpdateReason || 'Automatic in-panel update unavailable for this setup.'}</span>
					</div>

					{#if data.self.manualHint}
						<div class="flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-2 font-mono text-xs text-[var(--text-primary)]">
							<span class="truncate">{data.self.manualHint}</span>
							<CopyButton text={data.self.manualHint} />
						</div>
					{/if}

					{#if data.self.isDevBuild && data.self.podmanSocketOk && data.self.containerName}
						<div class="flex flex-wrap items-center justify-between gap-3 border-t border-[var(--border-subtle)] pt-2.5">
							<label class="flex cursor-pointer items-center gap-2 text-xs text-[var(--text-secondary)] select-none">
								<Checkbox bind:checked={confirmDevSwitch} />
								<span>Switch local dev container to GHCR release image ({data.imageRepo})</span>
							</label>
							<Button
								variant="secondary"
								size="sm"
								disabled={!confirmDevSwitch || isUpdating}
								onclick={() => (showConfirmUpdate = true)}
							>
								Switch &amp; Update
							</Button>
						</div>
					{/if}
				</div>
			{/if}
		{/if}
	</LayerCard>

	<!-- Host Podman Engine Compatibility Card -->
	{#if data}
		<LayerCard class="flex flex-col gap-4 p-5">
			<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
				<div class="flex items-center gap-2">
					<Cpu size={18} class="text-[var(--accent)]" />
					<div class="flex flex-col gap-0.5">
						<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Podman Host Engine</h3>
						<p class="m-0 text-xs text-[var(--text-tertiary)]">
							Host container engine compatibility check and OS-level upgrade instructions.
						</p>
					</div>
				</div>
				{#if data.podman.compatible}
					<Badge variant="success">
						<CheckCircle size={12} class="mr-1" /> Compatible (v5+)
					</Badge>
				{:else}
					<Badge variant="warning">Upgrade recommended</Badge>
				{/if}
			</div>

			<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
				<div class="flex flex-col gap-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3">
					<span class="text-[11px] text-[var(--text-tertiary)]">Installed Podman</span>
					<span class="text-sm font-semibold text-[var(--text-primary)]">v{data.podman.version || 'unknown'}</span>
				</div>
				<div class="flex flex-col gap-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3">
					<span class="text-[11px] text-[var(--text-tertiary)]">Host OS</span>
					<span class="text-sm font-medium text-[var(--text-primary)]">{data.podman.os || 'Linux'}</span>
				</div>
				<div class="flex flex-col gap-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3">
					<span class="text-[11px] text-[var(--text-tertiary)]">Socket mode</span>
					<span class="text-sm font-medium text-[var(--text-primary)]">
						{data.podman.rootless ? 'Rootless' : 'Rootful'}
					</span>
				</div>
			</div>

			{#if data.podman.warning}
				<div class="rounded-[var(--radius-sm)] border border-[var(--status-amber)]/30 bg-[var(--status-amber-muted)] p-3 text-xs text-[var(--status-amber)]">
					{data.podman.warning}
				</div>
			{/if}

			<div class="flex flex-col gap-1.5">
				<span class="text-xs font-medium text-[var(--text-secondary)]">
					Upgrade Podman on host (requires host OS privileges — not executed from unprivileged container):
				</span>
				<div class="flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-2 font-mono text-xs text-[var(--text-primary)]">
					<span class="truncate">{data.podman.upgradeHint}</span>
					<CopyButton text={data.podman.upgradeHint} />
				</div>
			</div>
		</LayerCard>
	{/if}
</div>

<ConfirmDialog
	open={showConfirmUpdate}
	title="Update GOPOD instance?"
	description={`GOPOD will pull ${data?.imageRepo}:${channel === 'dev' ? 'dev' : data?.latestTag || 'latest'} and restart safely with healthcheck verification (auto-rollback if startup fails).`}
	confirmText="Start update"
	oncancel={() => (showConfirmUpdate = false)}
	onconfirm={handleTriggerUpdate}
/>

