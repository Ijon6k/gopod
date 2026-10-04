<script lang="ts">
	import { Button } from '$lib/components/primitives';
	import {
		X,
		Broom,
		CircleNotch,
		CheckCircle,
		Warning,
		ImageSquare,
		Cube,
		Database,
		Wrench
	} from 'phosphor-svelte';

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

	// Estimated size per category
	const SIZES = {
		images: 2.9,
		containers: 0.42,
		volumes: 0.68,
		buildCache: 1.4
	};

	let totalReclaimable = $derived.by(() => {
		let total = 0;
		if (pruneImages) total += SIZES.images;
		if (pruneContainers) total += SIZES.containers;
		if (pruneVolumes) total += SIZES.volumes;
		if (pruneBuildCache) total += SIZES.buildCache;
		return total;
	});

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && !isPruning) {
			onclose();
		}
	}

	async function executePrune() {
		if (totalReclaimable <= 0) return;
		isPruning = true;
		pruneSuccess = false;

		const steps: string[] = [];
		if (pruneContainers) steps.push('Pruning exited containers (podman container prune)...');
		if (pruneImages) steps.push('Pruning untagged & unused images (podman image prune -a)...');
		if (pruneBuildCache) steps.push('Clearing Buildah build cache layers...');
		if (pruneVolumes) steps.push('Pruning unreferenced local storage volumes...');

		for (const step of steps) {
			currentStep = step;
			await new Promise((r) => setTimeout(r, 600));
		}

		currentStep = 'Optimizing rootless storage catalog...';
		await new Promise((r) => setTimeout(r, 500));

		isPruning = false;
		pruneSuccess = true;
		onpruned?.(Number(totalReclaimable.toFixed(2)));

		setTimeout(() => {
			pruneSuccess = false;
			onclose();
		}, 1400);
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-5 bg-[rgba(5,6,7,0.82)] backdrop-blur-[3px]"
		role="dialog"
		aria-modal="true"
	>
		<div
			class="w-full max-w-[560px] flex flex-col rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden shadow-2xl animate-in fade-in zoom-in-95 duration-150"
		>
			<!-- Modal Header -->
			<div class="flex items-center justify-between px-5 py-4 border-b border-[var(--border)] bg-[var(--bg-panel)]">
				<div class="flex items-center gap-2.5">
					<div class="w-8 h-8 rounded-lg bg-[var(--accent-muted)] text-[var(--accent)] flex items-center justify-center shrink-0">
						<Broom size={18} />
					</div>
					<div class="flex flex-col">
						<h3 class="text-sm font-semibold text-[var(--text-primary)] font-[var(--font-sans)]">
							Prune Podman Storage
						</h3>
						<span class="text-xs text-[var(--text-tertiary)]">
							Reclaim unreferenced storage layers and disk cache safely.
						</span>
					</div>
				</div>

				{#if !isPruning}
					<button
						type="button"
						onclick={onclose}
						class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
						title="Close dialog"
					>
						<X size={15} />
					</button>
				{/if}
			</div>

			<!-- Modal Body -->
			<div class="p-5 flex flex-col gap-4 text-left">
				{#if pruneSuccess}
					<div class="flex flex-col items-center justify-center py-8 gap-3 text-center">
						<div class="w-12 h-12 rounded-full bg-[var(--status-green-muted)] text-[var(--status-green)] flex items-center justify-center">
							<CheckCircle size={28} />
						</div>
						<div class="flex flex-col gap-1">
							<h4 class="text-base font-semibold text-[var(--text-primary)]">Storage Cleaned Successfully</h4>
							<p class="text-xs text-[var(--text-secondary)]">
								Recovered <strong class="text-[var(--status-green)] font-mono">{totalReclaimable.toFixed(2)} GB</strong> of disk space.
							</p>
						</div>
					</div>
				{:else if isPruning}
					<div class="flex flex-col items-center justify-center py-8 gap-3 text-center">
						<CircleNotch size={32} class="text-[var(--accent)] animate-spin" />
						<div class="flex flex-col gap-1">
							<span class="text-sm font-medium text-[var(--text-primary)]">Executing Podman Prune</span>
							<span class="text-xs font-mono text-[var(--text-tertiary)] animate-pulse">{currentStep}</span>
						</div>
					</div>
				{:else}
					<p class="text-xs text-[var(--text-secondary)] leading-relaxed">
						Select the categories of unused container artifacts you want to remove. Active workloads and mounted volumes will not be affected.
					</p>

					<!-- Options Checklist -->
					<div class="flex flex-col gap-2.5">
						<!-- Option 1: Unused Images -->
						<label
							class="flex items-start gap-3 p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] transition-colors cursor-pointer"
						>
							<input
								type="checkbox"
								bind:checked={pruneImages}
								class="mt-1 accent-[var(--accent)] rounded cursor-pointer"
							/>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between gap-2">
									<div class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
										<ImageSquare size={14} class="text-[var(--accent)]" />
										<span>Unused & Dangling Images</span>
									</div>
									<span class="font-mono text-xs font-semibold text-[var(--status-green)]">
										~{SIZES.images} GB
									</span>
								</div>
								<p class="text-[11px] text-[var(--text-tertiary)] mt-0.5">
									Deletes untagged <code class="text-[10px]">&lt;none&gt;</code> layers and images not used by running containers (<code class="text-[10px]">podman image prune -a</code>).
								</p>
							</div>
						</label>

						<!-- Option 2: Stopped Containers -->
						<label
							class="flex items-start gap-3 p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] transition-colors cursor-pointer"
						>
							<input
								type="checkbox"
								bind:checked={pruneContainers}
								class="mt-1 accent-[var(--accent)] rounded cursor-pointer"
							/>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between gap-2">
									<div class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
										<Cube size={14} class="text-[var(--text-secondary)]" />
										<span>Stopped / Exited Containers</span>
									</div>
									<span class="font-mono text-xs font-semibold text-[var(--status-green)]">
										~{(SIZES.containers * 1024).toFixed(0)} MB
									</span>
								</div>
								<p class="text-[11px] text-[var(--text-tertiary)] mt-0.5">
									Removes ephemeral writable layers from 3 containers in stopped/exited states.
								</p>
							</div>
						</label>

						<!-- Option 3: Build Cache -->
						<label
							class="flex items-start gap-3 p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] transition-colors cursor-pointer"
						>
							<input
								type="checkbox"
								bind:checked={pruneBuildCache}
								class="mt-1 accent-[var(--accent)] rounded cursor-pointer"
							/>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between gap-2">
									<div class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
										<Wrench size={14} class="text-[var(--accent)]" />
										<span>Buildah / Build Cache</span>
									</div>
									<span class="font-mono text-xs font-semibold text-[var(--status-green)]">
										~{SIZES.buildCache} GB
									</span>
								</div>
								<p class="text-[11px] text-[var(--text-tertiary)] mt-0.5">
									Removes intermediate image build caches created during git push or Dockerfile builds.
								</p>
							</div>
						</label>

						<!-- Option 4: Dangling Volumes (Caution) -->
						<label
							class="flex items-start gap-3 p-3 rounded-[var(--radius-sm)] border border-[var(--status-amber)]/30 bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] transition-colors cursor-pointer"
						>
							<input
								type="checkbox"
								bind:checked={pruneVolumes}
								class="mt-1 accent-[var(--status-amber)] rounded cursor-pointer"
							/>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between gap-2">
									<div class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
										<Database size={14} class="text-[var(--status-amber)]" />
										<span>Unused Volumes (Caution)</span>
									</div>
									<span class="font-mono text-xs font-semibold text-[var(--status-amber)]">
										~{(SIZES.volumes * 1024).toFixed(0)} MB
									</span>
								</div>
								<p class="text-[11px] text-[var(--status-amber)]/90 mt-0.5 flex items-center gap-1">
									<Warning size={12} class="shrink-0" />
									<span>Destructive: Removes local storage volumes not actively mounted by any container.</span>
								</p>
							</div>
						</label>
					</div>

					<!-- Calculation Summary Bar -->
					<div class="flex items-center justify-between p-3 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] text-xs">
						<span class="text-[var(--text-secondary)] font-medium">Estimated Recoverable Space:</span>
						<span class="font-mono text-sm font-bold {totalReclaimable > 0 ? 'text-[var(--status-green)]' : 'text-[var(--text-tertiary)]'}">
							{totalReclaimable > 0 ? `~${totalReclaimable.toFixed(2)} GB` : '0 GB'}
						</span>
					</div>
				{/if}
			</div>

			<!-- Modal Footer -->
			{#if !isPruning && !pruneSuccess}
				<div class="flex items-center justify-between px-5 py-3.5 border-t border-[var(--border)] bg-[var(--bg-panel)]">
					<Button variant="secondary" size="sm" onclick={onclose}>
						Cancel
					</Button>

					<Button
						variant="primary"
						size="sm"
						disabled={totalReclaimable <= 0}
						onclick={executePrune}
					>
						<Broom size={14} />
						<span>Prune Selected ({totalReclaimable.toFixed(1)} GB)</span>
					</Button>
				</div>
			{/if}
		</div>
	</div>
{/if}
