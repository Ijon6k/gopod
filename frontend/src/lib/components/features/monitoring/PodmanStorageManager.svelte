<script lang="ts">
	import { dataStore } from '$lib/stores/data.svelte';
	import { ConfirmDialog, Button } from '$lib/components/ui';
	import PruneStorageModal from './PruneStorageModal.svelte';
	import StorageHeaderDeck from './StorageHeaderDeck.svelte';
	import StorageDonutView from './StorageDonutView.svelte';
	import StorageBarView from './StorageBarView.svelte';
	import StorageSubsystemTable from './StorageSubsystemTable.svelte';
	import StorageConsumersTable from './StorageConsumersTable.svelte';
	import { api } from '$lib/api';
	import type { SystemDiskUsage, GCPreview } from '$lib/types';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { onMount } from 'svelte';
	import {
		ImageSquare,
		Cube,
		Database,
		CheckCircle,
		Broom,
		ArrowClockwise,
		CircleNotch
	} from 'phosphor-svelte';
	import { bytesToGB, formatBytes } from '$lib/utils/format';

	// Prune modal and confirmation states
	let isPruneModalOpen = $state(false);
	let isVolumeConfirmOpen = $state(false);
	let recentReclaimedText = $state<string | null>(null);
	let dfData = $state<SystemDiskUsage[]>([]);
	let isLoadingDf = $state(false);
	let chartMode = $state<'donut' | 'bar'>('donut');
	let pruningTarget = $state<string | null>(null);

	// Cleanup engine (scheduled nightly; never touches named volumes)
	let gcPreview = $state<GCPreview | null>(null);
	let gcRunning = $state(false);
	let isLoadingGcPreview = $state(false);

	// Same toast pattern as the runtime row actions: destructive actions
	// surface failures in the UI, not only in the console.
	const toast = useKumoToastManager();

	let images = $derived(dataStore.images);
	let volumes = $derived(dataStore.volumes);
	let containers = $derived(dataStore.containers);

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

	onMount(() => {
		loadDiskUsage();
		loadGCPreview();
	});

	async function loadGCPreview() {
		try {
			isLoadingGcPreview = true;
			gcPreview = await api.system.gcPreview();
		} catch (err) {
			console.error('Failed to load cleanup engine preview:', err);
		} finally {
			isLoadingGcPreview = false;
		}
	}

	async function runEngineCleanup() {
		try {
			gcRunning = true;
			const prevBytes = totalRawSize;
			const result = await api.system.runGC('all');
			await loadDiskUsage();
			await loadGCPreview();
			const reclaimed = Math.max(0, prevBytes - totalRawSize);
			if (result.skipped) {
				recentReclaimedText = null;
			} else {
				recentReclaimedText = reclaimed > 0 ? formatBytes(reclaimed) : 'storage cache';
			}
		} catch (err) {
			console.error('Cleanup engine run failed:', err);
			toast.add(
				errorToastOptions(
					'Cleanup engine failed',
					'The cleanup run did not finish — check server logs for details.'
				)
			);
		} finally {
			gcRunning = false;
			if (recentReclaimedText) {
				setTimeout(() => {
					recentReclaimedText = null;
				}, 6000);
			}
		}
	}

	let runningContainers = $derived(containers.filter((c) => c.status === 'running').length);
	let stoppedContainers = $derived(containers.filter((c) => c.status !== 'running').length);

	let imageUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('image')));
	let volumeUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('volume')));
	let containerUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('container')));

	// Canonical color tokens - synchronized across chart, cards, and table.
	// Raw hex on purpose: they feed ECharts canvas series colors, which cannot
	// resolve CSS variables; the token-based tailwind classes above are used
	// everywhere outside the chart.
	const COLOR_IMAGES = '#6366f1';
	const COLOR_VOLUMES = '#0ea5e9';
	const COLOR_CONTAINERS = '#10b981';

	let storageCategories = $derived([
		{
			id: 'images',
			name: 'Container images',
			icon: ImageSquare,
			totalCount: imageUsage?.total ?? images.length,
			activeCount: imageUsage?.active ?? images.length,
			inactiveCount: imageUsage ? Math.max(0, imageUsage.total - imageUsage.active) : 0,
			activeText: `${imageUsage?.active ?? images.length} in use`,
			inactiveText: `${imageUsage ? Math.max(0, imageUsage.total - imageUsage.active) : 0} unreferenced`,
			rawSize: imageUsage?.rawSize ?? images.length * 350000000,
			rawReclaimable: imageUsage?.rawReclaimable ?? 0,
			sizeDisplay:
				imageUsage?.size ||
				(images.length > 0 ? `${bytesToGB(images.length * 350000000)} GB` : '0 B'),
			sizeGB: imageUsage
				? bytesToGB(imageUsage.rawSize)
				: parseFloat((images.length * 0.35).toFixed(2)),
			reclaimableDisplay: imageUsage?.reclaimable || '0 B',
			reclaimableGB: imageUsage ? bytesToGB(imageUsage.rawReclaimable) : 0,
			hexColor: COLOR_IMAGES,
			colorClass: 'bg-[var(--accent)]',
			textClass: 'text-[var(--accent)]',
			viewPath: '/runtime/images',
			viewLabel: 'images'
		},
		{
			id: 'volumes',
			name: 'Local volumes',
			icon: Database,
			totalCount: volumeUsage?.total ?? volumes.length,
			activeCount: volumeUsage?.active ?? volumes.length,
			inactiveCount: volumeUsage ? Math.max(0, volumeUsage.total - volumeUsage.active) : 0,
			activeText: `${volumeUsage?.active ?? volumes.length} in use`,
			inactiveText: `${volumeUsage ? Math.max(0, volumeUsage.total - volumeUsage.active) : 0} unreferenced`,
			rawSize: volumeUsage?.rawSize ?? volumes.length * 25000000,
			rawReclaimable: volumeUsage?.rawReclaimable ?? 0,
			sizeDisplay: volumeUsage?.size || '0 B',
			sizeGB: volumeUsage
				? bytesToGB(volumeUsage.rawSize)
				: parseFloat((volumes.length * 0.025).toFixed(2)),
			reclaimableDisplay: volumeUsage?.reclaimable || '0 B',
			reclaimableGB: volumeUsage ? bytesToGB(volumeUsage.rawReclaimable) : 0,
			hexColor: COLOR_VOLUMES,
			colorClass: 'bg-[var(--status-blue)]',
			textClass: 'text-[var(--status-blue)]',
			viewPath: '/runtime/volumes',
			viewLabel: 'volumes'
		},
		{
			id: 'containers',
			name: 'Containers (layers)',
			icon: Cube,
			totalCount: containerUsage?.total ?? containers.length,
			activeCount: containerUsage?.active ?? runningContainers,
			inactiveCount: containerUsage
				? Math.max(0, containerUsage.total - containerUsage.active)
				: stoppedContainers,
			activeText: `${containerUsage?.active ?? runningContainers} running`,
			inactiveText: `${containerUsage ? Math.max(0, containerUsage.total - containerUsage.active) : stoppedContainers} stopped`,
			rawSize: containerUsage?.rawSize ?? containers.length * 10000000,
			rawReclaimable: containerUsage?.rawReclaimable ?? 0,
			sizeDisplay: containerUsage?.size || '0 B',
			sizeGB: containerUsage
				? bytesToGB(containerUsage.rawSize)
				: parseFloat((containers.length * 0.01).toFixed(2)),
			reclaimableDisplay: containerUsage?.reclaimable || '0 B',
			reclaimableGB: containerUsage ? bytesToGB(containerUsage.rawReclaimable) : 0,
			hexColor: COLOR_CONTAINERS,
			colorClass: 'bg-[var(--status-green)]',
			textClass: 'text-[var(--status-green)]',
			viewPath: '/runtime/containers',
			viewLabel: 'containers'
		}
	]);

	let totalRawSize = $derived(
		dfData.length > 0
			? dfData.reduce((acc, c) => acc + (c.rawSize || 0), 0)
			: storageCategories.reduce((acc, c) => acc + c.rawSize, 0)
	);
	let totalRawReclaimable = $derived(dfData.reduce((acc, c) => acc + (c.rawReclaimable || 0), 0));

	let totalPodmanDisplay = $derived(
		dfData.length > 0
			? formatBytes(totalRawSize)
			: `${storageCategories.reduce((acc, c) => acc + c.sizeGB, 0).toFixed(1)} GB`
	);
	let totalReclaimableDisplay = $derived(
		dfData.length > 0 ? formatBytes(totalRawReclaimable) : '0 B'
	);

	let totalPodmanNum = $derived(storageCategories.reduce((acc, c) => acc + c.sizeGB, 0));

	let topConsumers = $derived.by(() => {
		const items: {
			name: string;
			type: string;
			size: string;
			detail: string;
			reclaimable: boolean;
		}[] = [];
		for (const img of images.slice(0, 5)) {
			items.push({
				name: `${img.name}:${img.tag}`,
				type: 'image',
				size: img.size,
				detail: 'Container image',
				reclaimable: false
			});
		}
		for (const vol of volumes.slice(0, 5)) {
			items.push({
				name: vol.name,
				type: 'volume',
				size: vol.size,
				detail: vol.mount || 'Storage volume',
				reclaimable: false
			});
		}
		return items;
	});

	// Prune Action Handlers
	async function executePruneAll() {
		try {
			pruningTarget = 'all';
			const prevBytes = totalRawSize;
			await api.system.prune();
			await loadDiskUsage();
			const reclaimed = Math.max(0, prevBytes - totalRawSize);
			recentReclaimedText = reclaimed > 0 ? formatBytes(reclaimed) : 'storage cache';
		} catch (err) {
			console.error('Failed to prune system:', err);
			toast.add(
				errorToastOptions(
					'System prune failed',
					'Could not prune the system — check server logs for details.'
				)
			);
		} finally {
			pruningTarget = null;
			if (recentReclaimedText) {
				setTimeout(() => {
					recentReclaimedText = null;
				}, 6000);
			}
		}
	}

	async function executePruneCategory(type: 'images' | 'containers' | 'volumes') {
		if (type === 'volumes') {
			isVolumeConfirmOpen = true;
			return;
		}

		try {
			pruningTarget = type;
			const prevBytes = totalRawSize;
			if (type === 'images') {
				await api.runtime.images.prune(true);
			} else if (type === 'containers') {
				await api.runtime.containers.prune();
			}
			await loadDiskUsage();
			const reclaimed = Math.max(0, prevBytes - totalRawSize);
			recentReclaimedText = reclaimed > 0 ? formatBytes(reclaimed) : `${type} cache`;
		} catch (err) {
			console.error(`Failed to prune ${type}:`, err);
			toast.add(
				errorToastOptions(
					'Prune failed',
					`Could not prune ${type} — check server logs for details.`
				)
			);
		} finally {
			pruningTarget = null;
			if (recentReclaimedText) {
				setTimeout(() => {
					recentReclaimedText = null;
				}, 6000);
			}
		}
	}

	async function confirmVolumePrune() {
		try {
			pruningTarget = 'volumes';
			isVolumeConfirmOpen = false;
			const prevBytes = totalRawSize;
			await api.runtime.volumes.prune();
			await loadDiskUsage();
			const reclaimed = Math.max(0, prevBytes - totalRawSize);
			recentReclaimedText = reclaimed > 0 ? formatBytes(reclaimed) : 'unreferenced volumes';
		} catch (err) {
			console.error('Failed to prune volumes:', err);
			toast.add(
				errorToastOptions(
					'Volume prune failed',
					'Could not prune volumes — check server logs for details.'
				)
			);
		} finally {
			pruningTarget = null;
			if (recentReclaimedText) {
				setTimeout(() => {
					recentReclaimedText = null;
				}, 6000);
			}
		}
	}

	function handleModalPruned(reclaimedGB: number) {
		recentReclaimedText = reclaimedGB > 0 ? `${reclaimedGB} GB` : 'storage cache';
		loadDiskUsage();
		setTimeout(() => {
			recentReclaimedText = null;
		}, 6000);
	}
</script>

<div class="flex flex-col gap-5 text-left">
	<!-- Flash Feedback if recently pruned -->
	{#if recentReclaimedText}
		<div
			class="flex items-center justify-between rounded-[var(--radius-card)] border border-[var(--status-green)] bg-[var(--status-green-muted)] px-4 py-3 text-xs text-[var(--status-green)]"
		>
			<div class="flex items-center gap-2">
				<CheckCircle size={16} class="shrink-0" />
				<span>Successfully reclaimed <strong>{recentReclaimedText}</strong> of Podman storage.</span
				>
			</div>
			<button
				type="button"
				onclick={() => (recentReclaimedText = null)}
				class="cursor-pointer border-0 bg-transparent text-[11px] underline opacity-80 hover:opacity-100"
			>
				Dismiss
			</button>
		</div>
	{/if}

	<!-- 1. Master Storage Deck with View Toggle & 1-Click Prune Controls -->
	<div
		class="relative overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]"
	>
		<StorageHeaderDeck
			{totalPodmanDisplay}
			{totalReclaimableDisplay}
			{totalRawReclaimable}
			{chartMode}
			{isLoadingDf}
			{pruningTarget}
			ontoggleChartMode={(m) => (chartMode = m)}
			onrefresh={loadDiskUsage}
			onpruneAll={executePruneAll}
			onopenModal={() => (isPruneModalOpen = true)}
		/>

		<!-- Visualization Body -->
		<div class="p-6">
			{#if chartMode === 'donut'}
				<StorageDonutView
					{storageCategories}
					{totalPodmanDisplay}
					{totalReclaimableDisplay}
					{totalPodmanNum}
					{pruningTarget}
					onpruneCategory={executePruneCategory}
				/>
			{:else}
				<StorageBarView
					{storageCategories}
					{totalPodmanNum}
					{pruningTarget}
					onpruneCategory={executePruneCategory}
				/>
			{/if}
		</div>

		<!-- Actionable Subsystem Table -->
		<StorageSubsystemTable
			{storageCategories}
			{pruningTarget}
			onpruneCategory={executePruneCategory}
		/>
	</div>

	<!-- 2. Cleanup engine: scheduled nightly, label-safe, never removes volumes -->
	<div
		class="relative overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5"
	>
		<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<div class="flex flex-col gap-1.5">
				<div class="flex flex-wrap items-center gap-2.5">
					<h3 class="text-sm font-semibold text-[var(--text-primary)]">Cleanup engine</h3>
					<span
						class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-0.5 font-mono text-[11px] text-[var(--text-tertiary)]"
					>
						nightly 23:50
					</span>
					{#if gcPreview?.busy}
						<span
							class="rounded border border-[var(--status-amber)] bg-[var(--status-amber-muted)] px-2 py-0.5 text-[11px] text-[var(--status-amber)]"
						>
							deployment in progress — cleanup waits
						</span>
					{/if}
				</div>
				<p class="text-xs text-[var(--text-tertiary)]">
					Reclaims dangling image layers, stopped containers, build cache, and quadlet units left by
					deleted services. Named volumes are never removed automatically.
				</p>
				{#if gcPreview}
					<div class="flex flex-wrap items-center gap-2 text-[11px] text-[var(--text-secondary)]">
						<span class="rounded bg-[var(--bg-surface)] px-2 py-0.5 font-mono"
							>{gcPreview.danglingImages} dangling image{gcPreview.danglingImages === 1
								? ''
								: 's'}</span
						>
						<span class="rounded bg-[var(--bg-surface)] px-2 py-0.5 font-mono"
							>{gcPreview.stoppedContainers} stopped container{gcPreview.stoppedContainers === 1
								? ''
								: 's'}</span
						>
						<span class="rounded bg-[var(--bg-surface)] px-2 py-0.5 font-mono"
							>{gcPreview.staleUnitFiles?.length ?? 0} stale unit file{(gcPreview.staleUnitFiles
								?.length ?? 0) === 1
								? ''
								: 's'}</span
						>
						<span class="rounded bg-[var(--bg-surface)] px-2 py-0.5 font-mono"
							>{gcPreview.orphanVolumes?.length ?? 0} orphan volume{(gcPreview.orphanVolumes
								?.length ?? 0) === 1
								? ''
								: 's'}</span
						>
					</div>
				{/if}
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<Button
					variant="secondary"
					size="sm"
					onclick={loadGCPreview}
					disabled={isLoadingGcPreview || gcRunning}
					aria-label="Refresh cleanup preview"
				>
					<ArrowClockwise size={13} class={isLoadingGcPreview ? 'animate-spin' : ''} />
				</Button>
				<Button
					variant="primary"
					size="sm"
					onclick={runEngineCleanup}
					disabled={gcRunning || gcPreview?.busy === true}
					aria-label="Run cleanup engine now"
				>
					{#if gcRunning}
						<CircleNotch size={14} class="animate-spin" />
						<span>Cleaning...</span>
					{:else}
						<Broom size={14} />
						<span>Run cleanup now</span>
					{/if}
				</Button>
			</div>
		</div>
	</div>

	<!-- 3. Largest Disk Consumers -->
	<StorageConsumersTable {topConsumers} />
</div>

<!-- Modal: Custom Selective Prune -->
<PruneStorageModal
	bind:open={isPruneModalOpen}
	onclose={() => (isPruneModalOpen = false)}
	onpruned={handleModalPruned}
/>

<!-- Safety Confirmation for Pruning Volumes -->
<ConfirmDialog
	open={isVolumeConfirmOpen}
	title="Prune Unused Volumes"
	description="This will permanently delete all unreferenced local storage volumes. Any unmounted data will be lost. Proceed?"
	variant="warning"
	confirmText="Prune Volumes"
	onconfirm={confirmVolumePrune}
	oncancel={() => (isVolumeConfirmOpen = false)}
/>
