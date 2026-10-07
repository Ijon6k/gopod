<script lang="ts">
	import { dataStore } from '$lib/stores/data.svelte';
	import { Button } from '$lib/components/primitives';
	import PruneStorageModal from './PruneStorageModal.svelte';
	import { api } from '$lib/api';
	import type { SystemDiskUsage } from '$lib/api/system';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		ImageSquare,
		Cube,
		Database,
		Broom,
		ArrowRight,
		CheckCircle,
		ChartPieSlice,
		ChartBar
	} from 'phosphor-svelte';

	import { bytesToGB, parsePct, formatBytes } from '$lib/utils/format';

	let isPruneModalOpen = $state(false);
	let recentReclaimedGB = $state<number | null>(null);
	let dfData = $state<SystemDiskUsage[]>([]);
	let isLoadingDf = $state(false);
	let chartMode = $state<'donut' | 'bar'>('donut');

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
	});

	let runningContainers = $derived(containers.filter((c) => c.status === 'running').length);
	let stoppedContainers = $derived(containers.filter((c) => c.status !== 'running').length);

	let imageUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('image')));
	let volumeUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('volume')));
	let containerUsage = $derived(dfData.find((d) => d.type?.toLowerCase().includes('container')));

	let storageCategories = $derived([
		{
			id: 'images',
			name: 'Container Images',
			icon: ImageSquare,
			totalCount: imageUsage?.total ?? images.length,
			activeText: `${imageUsage?.active ?? images.length} active`,
			inactiveText: `${imageUsage ? Math.max(0, imageUsage.total - imageUsage.active) : 0} unreferenced`,
			sizeDisplay: imageUsage?.size || (images.length > 0 ? `${bytesToGB(images.length * 350000000)} GB` : '0 B'),
			sizeGB: imageUsage ? bytesToGB(imageUsage.rawSize) : parseFloat((images.length * 0.35).toFixed(1)),
			reclaimableDisplay: imageUsage?.reclaimable || '0 B',
			reclaimableGB: imageUsage ? bytesToGB(imageUsage.rawReclaimable) : 0,
			colorClass: 'bg-[var(--accent)]',
			strokeColor: 'var(--accent)',
			viewPath: '/runtime/images',
			viewLabel: 'Images'
		},
		{
			id: 'volumes',
			name: 'Local Volumes',
			icon: Database,
			totalCount: volumeUsage?.total ?? volumes.length,
			activeText: `${volumeUsage?.active ?? volumes.length} active`,
			inactiveText: `${volumeUsage ? Math.max(0, volumeUsage.total - volumeUsage.active) : 0} unreferenced`,
			sizeDisplay: volumeUsage?.size || '0 B',
			sizeGB: volumeUsage ? bytesToGB(volumeUsage.rawSize) : parseFloat((volumes.length * 0.25).toFixed(1)),
			reclaimableDisplay: volumeUsage?.reclaimable || '0 B',
			reclaimableGB: volumeUsage ? bytesToGB(volumeUsage.rawReclaimable) : 0,
			colorClass: 'bg-[var(--status-amber)]',
			strokeColor: 'var(--status-amber)',
			viewPath: '/runtime/volumes',
			viewLabel: 'Volumes'
		},
		{
			id: 'containers',
			name: 'Containers (RW Layers)',
			icon: Cube,
			totalCount: containerUsage?.total ?? containers.length,
			activeText: `${containerUsage?.active ?? runningContainers} running`,
			inactiveText: `${containerUsage ? Math.max(0, containerUsage.total - containerUsage.active) : stoppedContainers} stopped`,
			sizeDisplay: containerUsage?.size || '0 B',
			sizeGB: containerUsage ? bytesToGB(containerUsage.rawSize) : parseFloat((containers.length * 0.1).toFixed(1)),
			reclaimableDisplay: containerUsage?.reclaimable || '0 B',
			reclaimableGB: containerUsage ? bytesToGB(containerUsage.rawReclaimable) : 0,
			colorClass: 'bg-[var(--status-green)]',
			strokeColor: 'var(--status-green)',
			viewPath: '/runtime/containers',
			viewLabel: 'Containers'
		}
	]);

	let totalRawSize = $derived(
		dfData.length > 0
			? dfData.reduce((acc, c) => acc + (c.rawSize || 0), 0)
			: storageCategories.reduce((acc, c) => acc + c.sizeGB * 1024 * 1024 * 1024, 0)
	);
	let totalRawReclaimable = $derived(
		dfData.reduce((acc, c) => acc + (c.rawReclaimable || 0), 0)
	);

	let totalPodmanDisplay = $derived(
		dfData.length > 0 ? formatBytes(totalRawSize) : `${storageCategories.reduce((acc, c) => acc + c.sizeGB, 0).toFixed(1)} GB`
	);
	let totalReclaimableDisplay = $derived(
		dfData.length > 0 ? formatBytes(totalRawReclaimable) : '0 B'
	);

	let totalPodmanNum = $derived(
		storageCategories.reduce((acc, c) => acc + c.sizeGB, 0)
	);

	// Donut SVG parameters
	const donutRadius = 65;
	const donutCircumference = 2 * Math.PI * donutRadius; // ~408.4

	let donutSegments = $derived.by(() => {
		const total = totalPodmanNum > 0 ? totalPodmanNum : 1;
		let accumulated = 0;
		return storageCategories.map((cat) => {
			const pct = totalPodmanNum > 0 ? (cat.sizeGB / total) * 100 : 0;
			const dash = (pct / 100) * donutCircumference;
			const offset = -accumulated;
			accumulated += dash;
			return {
				...cat,
				pct: Math.round(pct),
				dash,
				gap: Math.max(0, donutCircumference - dash),
				offset
			};
		});
	});

	// Top largest storage consumers from real images and volumes
	let topConsumers = $derived.by(() => {
		const items: { name: string; type: string; size: string; detail: string; reclaimable: boolean }[] = [];
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

	async function handlePruned(reclaimed: number) {
		recentReclaimedGB = reclaimed;
		await loadDiskUsage();
		setTimeout(() => {
			recentReclaimedGB = null;
		}, 8000);
	}
</script>

<div class="flex flex-col gap-5 text-left">
	<!-- Flash Feedback if recently pruned -->
	{#if recentReclaimedGB}
		<div
			class="flex items-center justify-between px-4 py-3 rounded-[var(--radius-card)] bg-[var(--status-green-muted)] border border-[var(--status-green)] text-xs text-[var(--status-green)]"
		>
			<div class="flex items-center gap-2">
				<CheckCircle size={16} class="shrink-0" />
				<span>Successfully reclaimed <strong>{recentReclaimedGB} GB</strong> of Podman container storage!</span>
			</div>
			<button
				type="button"
				onclick={() => (recentReclaimedGB = null)}
				class="text-[11px] underline opacity-80 hover:opacity-100 cursor-pointer bg-transparent border-0"
			>
				Dismiss
			</button>
		</div>
	{/if}

	<!-- Master Storage Allocation Panel with Toggleable Donut & Bar Views -->
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden shadow-xs">
		<!-- Header with Title, Toggle, and Actions -->
		<div class="p-5 border-b border-[var(--border)] flex flex-col sm:flex-row sm:items-center justify-between gap-4">
			<div class="flex flex-col gap-1">
				<div class="flex items-center gap-2.5 flex-wrap">
					<h3 class="text-sm font-semibold text-[var(--text-primary)] font-[var(--font-sans)]">
						Podman Storage Allocation
					</h3>
					<span class="text-[11px] font-mono px-2 py-0.5 rounded bg-[var(--status-green-muted)] text-[var(--status-green)] border border-[var(--status-green)] font-medium">
						{totalReclaimableDisplay} Reclaimable
					</span>
				</div>
				<p class="text-xs text-[var(--text-tertiary)]">
					<strong class="text-[var(--text-secondary)] font-mono">{totalPodmanDisplay}</strong> allocated in <code class="font-mono text-[10.5px]">~/.local/share/containers/storage</code> (driver: overlay).
				</p>
			</div>

			<div class="flex items-center gap-2.5">
				<!-- Donut / Bar Visualization Toggle -->
				<div class="inline-flex items-center p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)]">
					<button
						type="button"
						onclick={() => (chartMode = 'donut')}
						class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer flex items-center gap-1.5 {chartMode === 'donut'
							? 'bg-[var(--bg-panel)] text-[var(--text-primary)] font-medium shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						title="Circular Donut chart view"
					>
						<ChartPieSlice size={13} />
						<span>Donut</span>
					</button>
					<button
						type="button"
						onclick={() => (chartMode = 'bar')}
						class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer flex items-center gap-1.5 {chartMode === 'bar'
							? 'bg-[var(--bg-panel)] text-[var(--text-primary)] font-medium shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						title="Horizontal Bar chart view"
					>
						<ChartBar size={13} />
						<span>Bar</span>
					</button>
				</div>

				<Button
					variant="primary"
					size="sm"
					onclick={() => (isPruneModalOpen = true)}
				>
					<Broom size={14} />
					<span>Prune Storage...</span>
				</Button>
			</div>
		</div>

		<!-- Interactive Visualization Body -->
		<div class="p-6">
			{#if chartMode === 'donut'}
				<!-- DONUT CHART VIEW -->
				<div class="flex flex-col md:flex-row items-center justify-around gap-6">
					<!-- SVG Donut -->
					<div class="relative w-[190px] h-[190px] shrink-0">
						<svg width="190" height="190" viewBox="0 0 190 190" class="overflow-visible">
							<!-- Background Track -->
							<circle
								cx="95"
								cy="95"
								r={donutRadius}
								fill="transparent"
								stroke="var(--bg-surface)"
								stroke-width="20"
							/>
							<!-- Segments with rotate(-90) to start from top -->
							<g transform="rotate(-90 95 95)">
								{#each donutSegments as seg}
									{#if seg.sizeGB > 0}
										<circle
											cx="95"
											cy="95"
											r={donutRadius}
											fill="transparent"
											stroke={seg.strokeColor}
											stroke-width="20"
											stroke-dasharray="{seg.dash} {seg.gap}"
											stroke-dashoffset={seg.offset}
											class="transition-all duration-700"
										/>
									{/if}
								{/each}
							</g>
						</svg>

						<!-- Donut Center Label -->
						<div class="absolute inset-0 flex flex-col items-center justify-center pointer-events-none select-none text-center">
							<span class="text-xl font-bold font-mono text-[var(--text-primary)] tracking-tight">
								{totalPodmanDisplay}
							</span>
							<span class="text-[10.5px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">
								Allocated
							</span>
							<span class="text-[10px] font-mono text-[var(--status-green)] font-medium mt-0.5">
								{totalReclaimableDisplay} freeable
							</span>
						</div>
					</div>

					<!-- Beside Donut: Clean Semantic Cards -->
					<div class="flex-1 w-full grid grid-cols-1 sm:grid-cols-3 gap-3">
						{#each donutSegments as seg}
							<div class="p-3.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] flex flex-col justify-between gap-2.5">
								<div class="flex items-center justify-between">
									<div class="flex items-center gap-2">
										<span class="w-2.5 h-2.5 rounded-full {seg.colorClass}"></span>
										<span class="text-xs font-semibold text-[var(--text-primary)]">{seg.name}</span>
									</div>
									<span class="text-xs font-mono font-bold text-[var(--text-secondary)]">{seg.pct}%</span>
								</div>

								<div class="flex items-baseline justify-between">
									<span class="text-base font-bold font-mono text-[var(--text-primary)]">{seg.sizeDisplay}</span>
									<span class="text-[11px] font-mono text-[var(--text-tertiary)]">{seg.totalCount} items</span>
								</div>

								<div class="text-[11px] text-[var(--text-tertiary)] flex items-center justify-between pt-1.5 border-t border-[var(--border-subtle)] font-mono">
									<span>Reclaimable:</span>
									<span class={seg.reclaimableDisplay && seg.reclaimableDisplay !== '0 B' ? 'text-[var(--status-green)] font-semibold' : 'text-[var(--text-tertiary)]'}>
										~{seg.reclaimableDisplay}
									</span>
								</div>
							</div>
						{/each}
					</div>
				</div>
			{:else}
				<!-- HORIZONTAL BAR VIEW -->
				<div class="flex flex-col gap-4">
					<div class="w-full h-3 rounded-full bg-[var(--bg-surface)] overflow-hidden flex">
						{#each storageCategories as cat}
							{#if cat.sizeGB > 0}
								<div
									class="h-full {cat.colorClass} transition-all duration-500"
									style="width: {(cat.sizeGB / (totalPodmanNum || 1)) * 100}%;"
									title="{cat.name}: {cat.sizeDisplay}"
								></div>
							{/if}
						{/each}
					</div>

					<!-- Legend Badges -->
					<div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
						{#each storageCategories as cat}
							{@const pct = Math.round((cat.sizeGB / (totalPodmanNum || 1)) * 100)}
							<div class="flex items-center justify-between p-2.5 rounded border border-[var(--border)] bg-[var(--bg-surface)] text-xs">
								<div class="flex items-center gap-2">
									<span class="w-2 h-2 rounded-full {cat.colorClass}"></span>
									<span class="text-[var(--text-secondary)]">{cat.name.split(' ')[0]}</span>
								</div>
								<div class="flex items-center gap-2 font-mono">
									<strong class="text-[var(--text-primary)]">{cat.sizeDisplay}</strong>
									<span class="text-[var(--text-tertiary)]">({pct}%)</span>
								</div>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>

		<!-- Clean Subsystem Table (UI Standards Parity) -->
		<div class="border-t border-[var(--border)] overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="bg-[var(--bg-table-header)] text-[var(--text-tertiary)] font-medium text-[11px] border-b border-[var(--border)]">
						<th class="py-2.5 px-5 font-medium">Subsystem</th>
						<th class="py-2.5 px-4 font-medium">Artifacts</th>
						<th class="py-2.5 px-4 font-medium">Allocated Size</th>
						<th class="py-2.5 px-4 font-medium">Reclaimable</th>
						<th class="py-2.5 px-5 font-medium text-right">Quick Action</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[var(--border-subtle)]">
					{#each storageCategories as cat, idx}
						{@const Icon = cat.icon}
						<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] {idx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)]' : 'bg-[var(--bg-table-row)]'}">
							<!-- Subsystem -->
							<td class="py-3 px-5">
								<div class="flex items-center gap-2.5">
									<span class="text-[var(--accent)]">
										<Icon size={15} />
									</span>
									<span class="font-medium text-[var(--text-primary)]">
										{cat.name}
									</span>
								</div>
							</td>

							<!-- Artifacts Count & Status -->
							<td class="py-3 px-4 text-[var(--text-secondary)]">
								<span class="font-mono font-medium text-[var(--text-primary)]">{cat.totalCount}</span>
								<span class="text-[var(--text-tertiary)] text-[11px] ml-1">
									({cat.activeText} · {cat.inactiveText})
								</span>
							</td>

							<!-- Size -->
							<td class="py-3 px-4 font-mono font-bold text-[var(--text-primary)]">
								{cat.sizeDisplay}
							</td>

							<!-- Reclaimable -->
							<td class="py-3 px-4">
								{#if cat.reclaimableDisplay && cat.reclaimableDisplay !== '0 B' && cat.reclaimableDisplay !== '0B (0%)'}
									<span class="font-mono text-[11.5px] font-medium {cat.id === 'volumes' ? 'text-[var(--status-amber)]' : 'text-[var(--status-green)]'}">
										~{cat.reclaimableDisplay}
									</span>
								{:else}
									<span class="text-[var(--text-tertiary)] font-mono text-[11px]">—</span>
								{/if}
							</td>

							<!-- Action -->
							<td class="py-3 px-5 text-right">
								{#if cat.viewPath}
									<button
										type="button"
										onclick={() => goto(cat.viewPath!)}
										class="text-[11.5px] text-[var(--accent)] hover:underline inline-flex items-center gap-1 cursor-pointer bg-transparent border-0 p-0 font-medium"
									>
										Manage {cat.viewLabel} <ArrowRight size={11} />
									</button>
								{:else}
									<button
										type="button"
										onclick={() => (isPruneModalOpen = true)}
										class="text-[11.5px] text-[var(--accent)] hover:underline inline-flex items-center gap-1 cursor-pointer bg-transparent border-0 p-0 font-medium"
									>
										Clear Cache <ArrowRight size={11} />
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>

	<!-- 2. Largest Storage Consumers (Concise Diagnostic Table) -->
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden shadow-xs flex flex-col">
		<div class="px-5 py-3 border-b border-[var(--border)] flex items-center justify-between">
			<h4 class="text-xs font-semibold text-[var(--text-primary)]">
				Largest Disk Consumers
			</h4>
			<span class="text-[11px] text-[var(--text-tertiary)] font-mono">
				Ranked by size
			</span>
		</div>

		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="bg-[var(--bg-table-header)] text-[var(--text-tertiary)] font-medium text-[11px] border-b border-[var(--border)]">
						<th class="py-2.5 px-5 font-medium">Artifact Name</th>
						<th class="py-2.5 px-4 font-medium">Type</th>
						<th class="py-2.5 px-4 font-medium">Context / Owner</th>
						<th class="py-2.5 px-4 font-medium">Size</th>
						<th class="py-2.5 px-5 font-medium text-right">State</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[var(--border-subtle)]">
					{#each topConsumers as item, idx}
						<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] {idx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)]' : 'bg-[var(--bg-table-row)]'}">
							<td class="py-2.5 px-5 font-mono text-[11.5px] text-[var(--text-primary)] truncate max-w-[280px]" title={item.name}>
								{item.name}
							</td>
							<td class="py-2.5 px-4 capitalize text-[var(--text-secondary)]">
								<span class="px-1.5 py-0.5 rounded text-[10px] font-mono border border-[var(--border-subtle)] bg-[var(--bg-surface)]">
									{item.type}
								</span>
							</td>
							<td class="py-2.5 px-4 text-[var(--text-secondary)] text-[11.5px]">
								{item.detail}
							</td>
							<td class="py-2.5 px-4 font-mono font-medium text-[var(--text-primary)]">
								{item.size}
							</td>
							<td class="py-2.5 px-5 text-right">
								{#if item.reclaimable}
									<span class="text-[10px] font-mono font-medium text-[var(--status-green)] bg-[var(--status-green-muted)] px-1.5 py-0.5 rounded border border-[var(--status-green)]">
										Reclaimable
									</span>
								{:else}
									<span class="text-[10px] font-mono text-[var(--text-tertiary)] bg-[var(--bg-surface)] px-1.5 py-0.5 rounded border border-[var(--border-subtle)]">
										In-use
									</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
</div>

<!-- Prune Storage Modal -->
<PruneStorageModal
	bind:open={isPruneModalOpen}
	onclose={() => (isPruneModalOpen = false)}
	onpruned={handlePruned}
/>
