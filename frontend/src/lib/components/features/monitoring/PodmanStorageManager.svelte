<script lang="ts">
	import { server, images, volumes, containers } from '$lib/data';
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

	// Real-world Podman rootless storage data (podman system df)
	const storageCategories = [
		{
			id: 'images',
			name: 'Container Images',
			icon: ImageSquare,
			totalCount: 12,
			activeText: '7 in-use',
			inactiveText: '5 unused',
			sizeGB: 7.2,
			reclaimableGB: 2.9,
			reclaimablePct: 40,
			color: 'bg-[var(--accent)]',
			viewPath: '/runtime/images',
			viewLabel: 'Images'
		},
		{
			id: 'volumes',
			name: 'Local Volumes',
			icon: Database,
			totalCount: 5,
			activeText: '4 mounted',
			inactiveText: '1 orphan',
			sizeGB: 2.1,
			reclaimableGB: 0.68,
			reclaimablePct: 32,
			color: 'bg-[var(--status-amber)]',
			viewPath: '/runtime/volumes',
			viewLabel: 'Volumes'
		},
		{
			id: 'buildCache',
			name: 'Buildah / Cache',
			icon: Wrench,
			totalCount: 14,
			activeText: '14 layers',
			inactiveText: 'intermediate',
			sizeGB: 1.4,
			reclaimableGB: 1.4,
			reclaimablePct: 100,
			color: 'bg-[var(--text-secondary)]',
			viewPath: null,
			viewLabel: 'Cache'
		},
		{
			id: 'containers',
			name: 'Containers (RW Layers)',
			icon: Cube,
			totalCount: 9,
			activeText: '6 running',
			inactiveText: '3 stopped',
			sizeGB: 1.1,
			reclaimableGB: 0.42,
			reclaimablePct: 38,
			color: 'bg-[var(--status-green)]',
			viewPath: '/runtime/containers',
			viewLabel: 'Containers'
		}
	];

	const totalPodmanGB = 11.8;
	const totalReclaimableGB = 5.4;

	// Top largest storage consumers
	const topConsumers = [
		{ name: 'itzg/minecraft-server:latest', type: 'image', size: '1.2 GB', detail: 'Minecraft / server', reclaimable: false },
		{ name: 'ngumpul-uploads', type: 'volume', size: '8.4 GB', detail: 'NgumpulHost / backend', reclaimable: false },
		{ name: 'minecraft-world', type: 'volume', size: '3.6 GB', detail: 'Minecraft / server', reclaimable: false },
		{ name: 'ghcr.io/ngumpul/backend:v3.1.0', type: 'image', size: '441 MB', detail: 'NgumpulHost / backend', reclaimable: false },
		{ name: 'ghcr.io/ngumpul/frontend:v3.1.0', type: 'image', size: '312 MB', detail: 'NgumpulHost / frontend', reclaimable: false },
		{ name: 'old-build-cache-layer:c18fa2', type: 'cache', size: '280 MB', detail: 'Dangling build layer', reclaimable: true },
		{ name: 'ghcr.io/joko/aerochat:v1.7.9 (old)', type: 'image', size: '276 MB', detail: 'Unused tagged image', reclaimable: true }
	];

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
				<div
					class="h-full bg-[var(--accent)]"
					style="width: {(7.2 / totalPodmanGB) * 100}%;"
					title="Images: 7.2 GB"
				></div>
				<div
					class="h-full bg-[var(--status-amber)]"
					style="width: {(2.1 / totalPodmanGB) * 100}%;"
					title="Volumes: 2.1 GB"
				></div>
				<div
					class="h-full bg-[var(--text-secondary)]"
					style="width: {(1.4 / totalPodmanGB) * 100}%;"
					title="Build Cache: 1.4 GB"
				></div>
				<div
					class="h-full bg-[var(--status-green)]"
					style="width: {(1.1 / totalPodmanGB) * 100}%;"
					title="Containers: 1.1 GB"
				></div>
			</div>

			<!-- Sleek Inline Legend -->
			<div class="flex flex-wrap items-center gap-x-5 gap-y-1.5 text-xs text-[var(--text-tertiary)] font-mono">
				<div class="flex items-center gap-1.5">
					<span class="w-2 h-2 rounded-full bg-[var(--accent)]"></span>
					<span>Images: <strong class="text-[var(--text-primary)]">7.2 GB</strong></span>
				</div>
				<div class="flex items-center gap-1.5">
					<span class="w-2 h-2 rounded-full bg-[var(--status-amber)]"></span>
					<span>Volumes: <strong class="text-[var(--text-primary)]">2.1 GB</strong></span>
				</div>
				<div class="flex items-center gap-1.5">
					<span class="w-2 h-2 rounded-full bg-[var(--text-secondary)]"></span>
					<span>Build Cache: <strong class="text-[var(--text-primary)]">1.4 GB</strong></span>
				</div>
				<div class="flex items-center gap-1.5">
					<span class="w-2 h-2 rounded-full bg-[var(--status-green)]"></span>
					<span>Containers: <strong class="text-[var(--text-primary)]">1.1 GB</strong></span>
				</div>
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
