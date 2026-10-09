<script lang="ts">
	import { Button } from '$lib/components/primitives';
	import { Modal } from '$lib/components/ui';
	import { api } from '$lib/api';
	import type { SystemDiskUsage } from '$lib/api/system';
	import {
		Broom,
		CircleNotch,
		CheckCircle,
		Warning,
		ImageSquare,
		Cube,
		Database,
		Wrench
	} from 'phosphor-svelte';

	import { formatBytes } from '$lib/utils/format';

	interface Props {
		open: boolean;
		onclose: () => void;
		onpruned?: (reclaimedGB: number) => void;
	}

	let { open = $bindable(false), onclose, onpruned }: Props = $props();

	let pruneImages = $state(true);
	let pruneContainers = $state(true);
	let pruneVolumes = $state(false);
	let pruneBuildCache = $state(true);

	let isPruning = $state(false);
	let pruneSuccess = $state(false);
	let currentStep = $state('');
	let dfData = $state<SystemDiskUsage[]>([]);
	let isLoadingDf = $state(false);

	async function loadDiskUsage() {
		try {
			isLoadingDf = true;
			const data = await api.system.df();
			if (Array.isArray(data)) {
				dfData = data;
			}
		} catch (err) {
			console.error('Failed to load Podman disk usage:', err);
		} finally {
			isLoadingDf = false;
		}
	}

	$effect(() => {
		if (open) {
			loadDiskUsage();
		}
	});

	let imageUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('image')));
	let containerUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('container')));
	let volumeUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('volume')));

	let totalReclaimableBytes = $derived.by(() => {
		let total = 0;
		if (pruneImages && imageUsage) total += imageUsage.rawReclaimable || 0;
		if (pruneContainers && containerUsage) total += containerUsage.rawReclaimable || 0;
		if (pruneVolumes && volumeUsage) total += volumeUsage.rawReclaimable || 0;
		return total;
	});

	let anySelected = $derived(pruneImages || pruneContainers || pruneVolumes || pruneBuildCache);

	async function executePrune() {
		if (!anySelected) return;
		isPruning = true;
		pruneSuccess = false;

		const reclaimedGB = parseFloat((totalReclaimableBytes / (1024 * 1024 * 1024)).toFixed(2));

		try {
			if (pruneContainers || pruneBuildCache) {
				currentStep = 'Pruning stopped containers & cache (podman system prune)...';
				await api.system.prune();
			}
			if (pruneImages) {
				currentStep = 'Pruning unused container images (podman image prune -a)...';
				await api.runtime.images.prune(true);
			}
			if (pruneVolumes) {
				currentStep = 'Pruning unreferenced volumes (podman volume prune)...';
				await api.runtime.volumes.prune();
			}

			currentStep = 'Storage catalog optimized.';
			pruneSuccess = true;
			onpruned?.(reclaimedGB);

			setTimeout(() => {
				pruneSuccess = false;
				onclose();
			}, 1400);
		} catch (err) {
			console.error('Podman storage prune error:', err);
		} finally {
			isPruning = false;
		}
	}
</script>

{#snippet modalFooter()}
	<Button variant="secondary" size="sm" onclick={onclose}>Cancel</Button>

	<Button variant="primary" size="sm" disabled={!anySelected} onclick={executePrune}>
		<Broom size={14} />
		<span
			>Prune Selected {totalReclaimableBytes > 0
				? `(~${formatBytes(totalReclaimableBytes)})`
				: ''}</span
		>
	</Button>
{/snippet}

<Modal
	{open}
	{onclose}
	title="Prune Podman Storage"
	subtitle="Reclaim unreferenced storage layers and disk cache safely."
	icon={Broom}
	size="lg"
	showClose={!isPruning}
	footer={!isPruning && !pruneSuccess ? modalFooter : undefined}
>
	{#if pruneSuccess}
		<div class="flex flex-col items-center justify-center gap-3 py-8 text-center">
			<div
				class="flex h-12 w-12 items-center justify-center rounded-full bg-[var(--status-green-muted)] text-[var(--status-green)]"
			>
				<CheckCircle size={28} />
			</div>
			<div class="flex flex-col gap-1">
				<h4 class="text-base font-semibold text-[var(--text-primary)]">
					Storage Cleaned Successfully
				</h4>
				<p class="text-xs text-[var(--text-secondary)]">
					Recovered <strong class="font-mono text-[var(--status-green)]"
						>{totalReclaimableBytes > 0
							? formatBytes(totalReclaimableBytes)
							: 'storage cache'}</strong
					> of disk space.
				</p>
			</div>
		</div>
	{:else if isPruning}
		<div class="flex flex-col items-center justify-center gap-3 py-8 text-center">
			<CircleNotch size={32} class="animate-spin text-[var(--accent)]" />
			<div class="flex flex-col gap-1">
				<span class="text-sm font-medium text-[var(--text-primary)]">Executing Podman Prune</span>
				<span class="animate-pulse font-mono text-xs text-[var(--text-tertiary)]"
					>{currentStep}</span
				>
			</div>
		</div>
	{:else}
		<p class="text-xs leading-relaxed text-[var(--text-secondary)]">
			Select the categories of unused container artifacts you want to remove. Active workloads and
			mounted volumes will not be affected.
		</p>

		<!-- Options Checklist -->
		<div class="flex flex-col gap-2.5">
			<!-- Option 1: Unused Images -->
			<label
				class="flex cursor-pointer items-start gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 transition-colors hover:bg-[var(--bg-hover)]"
			>
				<input
					type="checkbox"
					bind:checked={pruneImages}
					class="mt-1 cursor-pointer rounded accent-[var(--accent)]"
				/>
				<div class="min-w-0 flex-1">
					<div class="flex items-center justify-between gap-2">
						<div class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
							<ImageSquare size={14} class="text-[var(--accent)]" />
							<span>Unused & Dangling Images</span>
						</div>
						<span class="font-mono text-xs font-semibold text-[var(--status-green)]">
							{imageUsage?.reclaimable ? `~${imageUsage.reclaimable}` : '0 B'}
						</span>
					</div>
					<p class="mt-0.5 text-[11px] text-[var(--text-tertiary)]">
						Deletes untagged <code class="text-[10px]">&lt;none&gt;</code> layers and unreferenced
						images ({imageUsage?.total ?? 0} total images).
					</p>
				</div>
			</label>

			<!-- Option 2: Stopped Containers -->
			<label
				class="flex cursor-pointer items-start gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 transition-colors hover:bg-[var(--bg-hover)]"
			>
				<input
					type="checkbox"
					bind:checked={pruneContainers}
					class="mt-1 cursor-pointer rounded accent-[var(--accent)]"
				/>
				<div class="min-w-0 flex-1">
					<div class="flex items-center justify-between gap-2">
						<div class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
							<Cube size={14} class="text-[var(--text-secondary)]" />
							<span>Stopped / Exited Containers</span>
						</div>
						<span class="font-mono text-xs font-semibold text-[var(--status-green)]">
							{containerUsage?.reclaimable ? `~${containerUsage.reclaimable}` : '0 B'}
						</span>
					</div>
					<p class="mt-0.5 text-[11px] text-[var(--text-tertiary)]">
						Removes ephemeral writable layers from {Math.max(
							0,
							(containerUsage?.total ?? 0) - (containerUsage?.active ?? 0)
						)} stopped/exited containers.
					</p>
				</div>
			</label>

			<!-- Option 3: Build Cache -->
			<label
				class="flex cursor-pointer items-start gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 transition-colors hover:bg-[var(--bg-hover)]"
			>
				<input
					type="checkbox"
					bind:checked={pruneBuildCache}
					class="mt-1 cursor-pointer rounded accent-[var(--accent)]"
				/>
				<div class="min-w-0 flex-1">
					<div class="flex items-center justify-between gap-2">
						<div class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
							<Wrench size={14} class="text-[var(--accent)]" />
							<span>Buildah / Build Cache</span>
						</div>
						<span class="font-mono text-xs font-semibold text-[var(--status-green)]"> Clean </span>
					</div>
					<p class="mt-0.5 text-[11px] text-[var(--text-tertiary)]">
						Removes intermediate image build caches created during git push or Dockerfile builds.
					</p>
				</div>
			</label>

			<!-- Option 4: Dangling Volumes (Caution) -->
			<label
				class="flex cursor-pointer items-start gap-3 rounded-[var(--radius-sm)] border border-[var(--status-amber)]/30 bg-[var(--bg-panel)] p-3 transition-colors hover:bg-[var(--bg-hover)]"
			>
				<input
					type="checkbox"
					bind:checked={pruneVolumes}
					class="mt-1 cursor-pointer rounded accent-[var(--status-amber)]"
				/>
				<div class="min-w-0 flex-1">
					<div class="flex items-center justify-between gap-2">
						<div class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
							<Database size={14} class="text-[var(--status-amber)]" />
							<span>Unused Volumes (Caution)</span>
						</div>
						<span class="font-mono text-xs font-semibold text-[var(--status-amber)]">
							{volumeUsage?.reclaimable ? `~${volumeUsage.reclaimable}` : '0 B'}
						</span>
					</div>
					<p class="mt-0.5 flex items-center gap-1 text-[11px] text-[var(--status-amber)]/90">
						<Warning size={12} class="shrink-0" />
						<span
							>Destructive: Removes local storage volumes not actively mounted ({Math.max(
								0,
								(volumeUsage?.total ?? 0) - (volumeUsage?.active ?? 0)
							)} unused).</span
						>
					</p>
				</div>
			</label>
		</div>

		<!-- Calculation Summary Bar -->
		<div
			class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 text-xs"
		>
			<span class="font-medium text-[var(--text-secondary)]">Estimated Recoverable Space:</span>
			<span
				class="font-mono text-sm font-bold {totalReclaimableBytes > 0
					? 'text-[var(--status-green)]'
					: 'text-[var(--text-tertiary)]'}"
			>
				{totalReclaimableBytes > 0
					? `~${formatBytes(totalReclaimableBytes)}`
					: isLoadingDf
						? 'Calculating...'
						: '0 B'}
			</span>
		</div>
	{/if}
</Modal>
