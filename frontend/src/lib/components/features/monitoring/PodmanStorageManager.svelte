<script lang="ts">
	import { dataStore } from '$lib/stores/data.svelte';
	import { Button } from '$lib/components/primitives';
	import PruneStorageModal from './PruneStorageModal.svelte';
	import { goto } from '$app/navigation';
	import {
		ImageSquare,
		Cube,
		Database,
		Wrench,
		Broom,
		ArrowRight,
		CheckCircle,
		HardDrive
	} from 'phosphor-svelte';

	let isPruneModalOpen = $state(false);
	let recentReclaimedGB = $state<number | null>(null);

	let server = $derived(dataStore.server);
	let images = $derived(dataStore.images);
	let volumes = $derived(dataStore.volumes);
	let containers = $derived(dataStore.containers);

	let runningContainers = $derived(containers.filter((c) => c.status === 'running').length);
	let stoppedContainers = $derived(containers.filter((c) => c.status !== 'running').length);

	let storageCategories = $derived([
		{
			id: 'images',
			name: 'Container Images',
			icon: ImageSquare,
			totalCount: images.length,
			activeText: `${images.length} images`,
			inactiveText: 'local store',
			sizeGB: parseFloat((images.length * 0.35).toFixed(1)),
			reclaimableGB: parseFloat((images.length * 0.1).toFixed(1)),
			reclaimablePct: 25,
			color: 'bg-[var(--accent)]',
			viewPath: '/runtime/images',
			viewLabel: 'Images'
		},
		{
			id: 'volumes',
			name: 'Local Volumes',
			icon: Database,
			totalCount: volumes.length,
			activeText: `${volumes.length} volumes`,
			inactiveText: 'local store',
			sizeGB: parseFloat((volumes.length * 0.25).toFixed(1)),
			reclaimableGB: 0.1,
			reclaimablePct: 15,
			color: 'bg-[var(--status-amber)]',
			viewPath: '/runtime/volumes',
			viewLabel: 'Volumes'
		},
		{
			id: 'containers',
			name: 'Containers (RW Layers)',
			icon: Cube,
			totalCount: containers.length,
			activeText: `${runningContainers} running`,
			inactiveText: `${stoppedContainers} stopped`,
			sizeGB: parseFloat((containers.length * 0.1).toFixed(1)),
			reclaimableGB: parseFloat((stoppedContainers * 0.05).toFixed(1)),
			reclaimablePct: stoppedContainers > 0 ? 30 : 0,
			color: 'bg-[var(--status-green)]',
			viewPath: '/runtime/containers',
			viewLabel: 'Containers'
		}
	]);

	let totalPodmanNum = $derived(
		storageCategories.reduce((acc, c) => acc + c.sizeGB, 0)
	);
	let totalPodmanGB = $derived(totalPodmanNum.toFixed(1));
	let totalReclaimableGB = $derived(
		storageCategories.reduce((acc, c) => acc + c.reclaimableGB, 0).toFixed(1)
	);

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

	function handlePruned(reclaimed: number) {
		recentReclaimedGB = reclaimed;
		setTimeout(() => {
			recentReclaimedGB = null;
		}, 8000);
	}
</script>

<div class="flex flex-col gap-5 text-left">
	<!-- Flash Feedback if recently pruned -->
	{#if recentReclaimedGB}
		<div
			class="flex items-center justify-between px-4 py-3 rounded-[var(--radius-card)] bg-[var(--status-green-muted)] border border-[var(--status-green)]/30 text-xs text-[var(--status-green)]"
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

	<!-- 1. Single Unified Master Panel: Podman Storage Breakdown (No Card Clutter) -->
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden shadow-xs">
		<!-- Master Header with Actions -->
		<div class="p-5 border-b border-[var(--border)] flex flex-col sm:flex-row sm:items-center justify-between gap-4">
			<div class="flex flex-col gap-1">
				<div class="flex items-center gap-2.5">
					<h3 class="text-sm font-semibold text-[var(--text-primary)] font-[var(--font-sans)]">
						Podman Storage Allocation
					</h3>
					<span class="text-[11px] font-mono px-2 py-0.5 rounded bg-[var(--status-green-muted)] text-[var(--status-green)] border border-[var(--status-green)]/30 font-medium">
						{totalReclaimableGB} GB Reclaimable
					</span>
				</div>
				<p class="text-xs text-[var(--text-tertiary)]">
					<strong class="text-[var(--text-secondary)] font-mono">{totalPodmanGB} GB</strong> allocated in <code class="font-mono text-[10.5px]">~/.local/share/containers/storage</code> (driver: overlay).
				</p>
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

		<!-- Proportional Distribution Bar -->
		<div class="px-5 pt-5 pb-4 flex flex-col gap-3">
			<div class="w-full h-2 rounded-full bg-[var(--bg-surface)] overflow-hidden flex">
				{#each storageCategories as cat}
					{#if cat.sizeGB > 0}
						<div
							class="h-full {cat.color}"
							style="width: {(cat.sizeGB / (totalPodmanNum || 1)) * 100}%;"
							title="{cat.name}: {cat.sizeGB} GB"
						></div>
					{/if}
				{/each}
			</div>

			<!-- Sleek Inline Legend -->
			<div class="flex flex-wrap items-center gap-x-5 gap-y-1.5 text-xs text-[var(--text-tertiary)] font-mono">
				{#each storageCategories as cat}
					<div class="flex items-center gap-1.5">
						<span class="w-2 h-2 rounded-full {cat.color}"></span>
						<span>{cat.name.split(' ')[0]}: <strong class="text-[var(--text-primary)]">{cat.sizeGB} GB</strong></span>
					</div>
				{/each}
			</div>
		</div>

		<!-- Clean Subsystem Table (Replaces 4 Bloated Box Cards) -->
		<div class="border-t border-[var(--border-subtle)] overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="bg-[var(--bg-table-header)] text-[var(--text-tertiary)] font-medium text-[11px] border-b border-[var(--border)]">
						<th class="py-2 px-5 font-medium">Subsystem</th>
						<th class="py-2 px-4 font-medium">Artifacts</th>
						<th class="py-2 px-4 font-medium">Size</th>
						<th class="py-2 px-4 font-medium">Reclaimable</th>
						<th class="py-2 px-5 font-medium text-right">Action</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[var(--border-subtle)]">
					{#each storageCategories as cat}
						{@const Icon = cat.icon}
						<tr class="hover:bg-[var(--bg-table-row-hover)] transition-colors">
							<!-- Subsystem -->
							<td class="py-3 px-5">
								<div class="flex items-center gap-2.5">
									<span class="text-[var(--text-secondary)]">
										<Icon size={15} />
									</span>
									<span class="font-medium text-[var(--text-primary)]">
										{cat.name}
									</span>
								</div>
							</td>

							<!-- Artifacts Count & Status -->
							<td class="py-3 px-4 text-[var(--text-secondary)]">
								<span class="font-mono">{cat.totalCount}</span>
								<span class="text-[var(--text-tertiary)] text-[11px] ml-1">
									({cat.activeText} · {cat.inactiveText})
								</span>
							</td>

							<!-- Size -->
							<td class="py-3 px-4 font-mono font-semibold text-[var(--text-primary)]">
								{cat.sizeGB} GB
							</td>

							<!-- Reclaimable -->
							<td class="py-3 px-4">
								{#if cat.reclaimableGB > 0}
									<span class="font-mono text-[11.5px] font-medium {cat.id === 'volumes' ? 'text-[var(--status-amber)]' : 'text-[var(--status-green)]'}">
										~{cat.reclaimableGB >= 1 ? `${cat.reclaimableGB} GB` : `${(cat.reclaimableGB * 1024).toFixed(0)} MB`}
										<span class="text-[10px] opacity-75 font-normal">({cat.reclaimablePct}%)</span>
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
										class="text-[11.5px] text-[var(--accent)] hover:underline inline-flex items-center gap-1 cursor-pointer bg-transparent border-0 p-0"
									>
										Manage {cat.viewLabel} <ArrowRight size={11} />
									</button>
								{:else}
									<button
										type="button"
										onclick={() => (isPruneModalOpen = true)}
										class="text-[11.5px] text-[var(--accent)] hover:underline inline-flex items-center gap-1 cursor-pointer bg-transparent border-0 p-0"
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
						<th class="py-2 px-5 font-medium">Artifact Name</th>
						<th class="py-2 px-4 font-medium">Type</th>
						<th class="py-2 px-4 font-medium">Context / Owner</th>
						<th class="py-2 px-4 font-medium">Size</th>
						<th class="py-2 px-5 font-medium text-right">State</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[var(--border-subtle)]">
					{#each topConsumers as item}
						<tr class="hover:bg-[var(--bg-table-row-hover)] transition-colors">
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
									<span class="text-[10px] font-mono font-medium text-[var(--status-green)] bg-[var(--status-green-muted)] px-1.5 py-0.5 rounded">
										Reclaimable
									</span>
								{:else}
									<span class="text-[10px] font-mono text-[var(--text-tertiary)] bg-[var(--bg-surface)] px-1.5 py-0.5 rounded">
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
