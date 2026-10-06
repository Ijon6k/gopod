<script lang="ts">
	import { volumes, services, dataStore } from '$lib/data';
	import { Button } from '$lib/components/primitives';
	import type { VolumeSnapshot, VolumeBackupSchedule } from '$lib/types';
	import {
		Database,
		ClockCounterClockwise,
		DownloadSimple,
		Trash,
		ArrowClockwise,
		CalendarCheck,
		HardDrives,
		CheckCircle,
		Warning,
		CircleNotch,
		ShieldCheck,
		FileArchive,
		X
	} from 'phosphor-svelte';

	interface Props {
		projectId: string;
	}

	let { projectId }: Props = $props();

	// Reactive data from dataStore
	let projectVolumes = $derived(volumes.filter((v) => v.projectId === projectId));
	let snapshots = $derived(dataStore.getProjectVolumeSnapshots(projectId));
	let schedules = $derived(dataStore.getProjectVolumeSchedules(projectId));

	// Local states for UX feedback
	let isCreatingSnapshot = $state(false);
	let snapshotTargetVolume = $state<string | null>(null);
	let restoringSnapshot = $state<VolumeSnapshot | null>(null);
	let isRestoring = $state(false);
	let restoreSuccessMessage = $state<string | null>(null);

	async function handleCreateSnapshot(volumeName: string, serviceId: string) {
		isCreatingSnapshot = true;
		snapshotTargetVolume = volumeName;

		// Simulate rootless podman volume tar.zst creation
		await new Promise((r) => setTimeout(r, 900));

		dataStore.createVolumeSnapshot(volumeName, projectId, serviceId);
		isCreatingSnapshot = false;
		snapshotTargetVolume = null;
	}

	function handleDeleteSnapshot(id: string) {
		if (confirm('Delete this volume snapshot? This action cannot be undone.')) {
			dataStore.deleteVolumeSnapshot(id);
		}
	}

	function openRestoreModal(snap: VolumeSnapshot) {
		restoringSnapshot = snap;
	}

	async function confirmRestore() {
		if (!restoringSnapshot) return;
		isRestoring = true;

		// Simulate non-destructive restore into ~/.local/share/containers/storage/volumes/<vol>/_data
		await new Promise((r) => setTimeout(r, 1200));

		isRestoring = false;
		const targetVol = restoringSnapshot.volumeName;
		restoringSnapshot = null;
		restoreSuccessMessage = `Successfully restored snapshot to volume "${targetVol}".`;
		setTimeout(() => (restoreSuccessMessage = null), 4000);
	}

	function handleToggleSchedule(id: string) {
		dataStore.toggleVolumeSchedule(id);
	}

	function handleDownload(filename: string) {
		alert(`Preparing download for ${filename} (compressed zstd archive)`);
	}
</script>

<div class="flex flex-col gap-6 w-full text-left">
	<!-- Success Notification -->
	{#if restoreSuccessMessage}
		<div class="p-3.5 rounded-[var(--radius-card)] border border-[var(--status-green)]/30 bg-[var(--status-green-muted)] flex items-center justify-between animate-in fade-in duration-150">
			<div class="flex items-center gap-2.5 text-xs text-[var(--status-green)] font-medium">
				<CheckCircle size={16} />
				<span>{restoreSuccessMessage}</span>
			</div>
			<button
				type="button"
				onclick={() => (restoreSuccessMessage = null)}
				class="text-[var(--status-green)] hover:opacity-75 bg-transparent border-0 cursor-pointer p-1"
			>
				<X size={14} />
			</button>
		</div>
	{/if}

	<!-- 1. Volume Overview Cards -->
	<section class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<Database size={16} class="text-[var(--accent)]" />
				<h3 class="text-xs font-semibold text-[var(--text-primary)] uppercase tracking-wider m-0">
					Active Project Volumes ({projectVolumes.length})
				</h3>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)] font-mono">
				Podman rootless storage: ~/.local/share/containers/storage/volumes/
			</span>
		</div>

		<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
			{#if projectVolumes.length === 0}
				<div class="col-span-2 p-8 text-center text-xs text-[var(--text-tertiary)] rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)]">
					No persistent volumes configured for this project.
				</div>
			{:else}
				{#each projectVolumes as vol (vol.id)}
					{@const volSnapshots = snapshots.filter((s) => s.volumeName === vol.name)}
					{@const latestSnap = volSnapshots[0]}
					<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] flex flex-col justify-between gap-3">
						<div class="flex items-start justify-between gap-2">
							<div class="flex flex-col gap-1">
								<div class="flex items-center gap-2">
									<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">
										{vol.name}
									</span>
									<span class="px-1.5 py-0.2 rounded text-[10px] font-mono text-[var(--status-green)] bg-[var(--status-green-muted)] font-medium">
										mounted
									</span>
								</div>
								<span class="text-[11px] text-[var(--text-tertiary)]">
									Mount: <code class="font-mono text-[var(--text-secondary)]">{vol.mount}</code> · Service: <strong class="text-[var(--text-secondary)]">{vol.serviceName}</strong>
								</span>
							</div>

							<div class="flex flex-col items-end">
								<span class="text-sm font-bold font-mono text-[var(--text-primary)]">{vol.size}</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]">on disk</span>
							</div>
						</div>

						<div class="flex items-center justify-between pt-2 border-t border-[var(--border-subtle)] text-xs">
							<span class="text-[11px] text-[var(--text-tertiary)]">
								{#if latestSnap}
									Last snapshot: <strong class="text-[var(--text-secondary)]">{latestSnap.timeAgo}</strong> ({latestSnap.size})
								{:else}
									No snapshots taken yet
								{/if}
							</span>

							<Button
								variant="secondary"
								size="sm"
								disabled={isCreatingSnapshot && snapshotTargetVolume === vol.name}
								onclick={() => handleCreateSnapshot(vol.name, vol.serviceId)}
							>
								{#if isCreatingSnapshot && snapshotTargetVolume === vol.name}
									<CircleNotch size={12} class="animate-spin" />
									<span>Archiving...</span>
								{:else}
									<ArrowClockwise size={12} />
									<span>Snapshot Now</span>
								{/if}
							</Button>
						</div>
					</div>
				{/each}
			{/if}
		</div>
	</section>

	<!-- 2. Automated Backup Schedules (Dokploy Parity) -->
	<section class="flex flex-col gap-3">
		<div class="flex items-center gap-2">
			<CalendarCheck size={16} class="text-[var(--accent)]" />
			<h3 class="text-xs font-semibold text-[var(--text-primary)] uppercase tracking-wider m-0">
				Automated Backup Policies (Cron)
			</h3>
		</div>

		<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden">
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<table class="w-full border-collapse text-left text-xs">
					<thead class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)]">
						<tr>
							<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Target Volume</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Schedule (Cron)</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Retention</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Last Run</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Next Run</th>
							<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)] text-right">Policy Status</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border-subtle)]">
						{#if schedules.length === 0}
							<tr>
								<td colspan="6" class="py-6 text-center text-[var(--text-tertiary)] text-xs">
									No automated backup schedules defined for this project.
								</td>
							</tr>
						{:else}
							{#each schedules as sched (sched.id)}
								<tr class="hover:bg-[var(--bg-table-row-alt)] transition-colors">
									<td class="py-2.5 px-3.5 whitespace-nowrap font-mono font-medium text-[var(--text-primary)]">
										{sched.volumeName}
									</td>
									<td class="py-2.5 px-3 whitespace-nowrap">
										<div class="flex items-center gap-1.5">
											<span class="font-mono text-xs text-[var(--text-secondary)]">{sched.cron}</span>
											<span class="text-[10.5px] text-[var(--text-tertiary)]">({sched.label})</span>
										</div>
									</td>
									<td class="py-2.5 px-3 whitespace-nowrap text-[var(--text-secondary)]">
										Keep last {sched.retentionCount} snapshots
									</td>
									<td class="py-2.5 px-3 whitespace-nowrap text-[var(--text-tertiary)] font-mono text-[11.5px]">
										{sched.lastRun || '—'}
									</td>
									<td class="py-2.5 px-3 whitespace-nowrap text-[var(--text-tertiary)] font-mono text-[11.5px]">
										{sched.nextRun || '—'}
									</td>
									<td class="py-2.5 px-3.5 whitespace-nowrap text-right">
										<button
											type="button"
											onclick={() => handleToggleSchedule(sched.id)}
											class="cursor-pointer border-0 bg-transparent text-xs font-medium {sched.enabled ? 'text-[var(--status-green)]' : 'text-[var(--text-tertiary)]'}"
										>
											{sched.enabled ? '● Active' : '○ Paused'}
										</button>
									</td>
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>
		</div>
	</section>

	<!-- 3. Snapshot History & Restore (Clean Table Standards) -->
	<section class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<ClockCounterClockwise size={16} class="text-[var(--accent)]" />
				<h3 class="text-xs font-semibold text-[var(--text-primary)] uppercase tracking-wider m-0">
					Volume Snapshot History ({snapshots.length})
				</h3>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Standard format: <code class="font-mono text-[var(--text-secondary)]">tar.zst</code> (lossless ultra-fast Zstandard)
			</span>
		</div>

		<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden">
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<table class="w-full border-collapse text-left text-xs">
					<thead class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)]">
						<tr>
							<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Archive Filename</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Volume Source</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Size</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Status</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Created</th>
							<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)] text-right">Actions</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border-subtle)]">
						{#if snapshots.length === 0}
							<tr>
								<td colspan="6" class="py-12 text-center text-[var(--text-tertiary)] text-xs">
									No volume snapshots taken yet. Click "Snapshot Now" on an active volume above.
								</td>
							</tr>
						{:else}
							{#each snapshots as snap (snap.id)}
								<tr class="hover:bg-[var(--bg-table-row-alt)] transition-colors">
									<!-- Filename -->
									<td class="py-3 px-3.5 whitespace-nowrap">
										<div class="flex items-center gap-2">
											<FileArchive size={15} class="text-[var(--accent)]" />
											<span class="font-mono text-xs font-medium text-[var(--text-primary)]">
												{snap.filename}
											</span>
										</div>
									</td>

									<!-- Volume -->
									<td class="py-3 px-3 whitespace-nowrap font-mono text-xs text-[var(--text-secondary)]">
										{snap.volumeName}
									</td>

									<!-- Size -->
									<td class="py-3 px-3 whitespace-nowrap font-mono text-xs text-[var(--text-primary)]">
										{snap.size}
									</td>

									<!-- Status -->
									<td class="py-3 px-3 whitespace-nowrap">
										<span class="inline-flex items-center gap-1 text-[11px] font-medium text-[var(--status-green)] bg-[var(--status-green-muted)] px-1.5 py-0.5 rounded border border-[var(--status-green)]/30">
											<CheckCircle size={12} /> Ready
										</span>
									</td>

									<!-- Created -->
									<td class="py-3 px-3 whitespace-nowrap text-[var(--text-tertiary)]">
										{snap.timeAgo}
									</td>

									<!-- Actions: Restore, Download, Delete -->
									<td class="py-3 px-3.5 whitespace-nowrap text-right">
										<div class="inline-flex items-center gap-1.5">
											<button
												type="button"
												onclick={() => openRestoreModal(snap)}
												class="px-2 py-1 rounded text-[11px] font-medium bg-[var(--accent-muted)] text-[var(--accent)] hover:bg-[var(--accent)] hover:text-[var(--bg-shell)] border-0 cursor-pointer transition-colors"
												title="Restore this snapshot to volume"
											>
												Restore
											</button>
											<button
												type="button"
												onclick={() => handleDownload(snap.filename)}
												class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
												title="Download Archive"
											>
												<DownloadSimple size={14} />
											</button>
											<button
												type="button"
												onclick={() => handleDeleteSnapshot(snap.id)}
												class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--status-red)] hover:bg-[var(--status-red-muted)] border-0 bg-transparent cursor-pointer transition-colors"
												title="Delete Snapshot"
											>
												<Trash size={14} />
											</button>
										</div>
									</td>
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>
		</div>
	</section>
</div>

<!-- Confirmation Modal: Restore Snapshot -->
{#if restoringSnapshot}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-[rgba(5,6,7,0.82)] backdrop-blur-[3px]"
		role="dialog"
		aria-modal="true"
	>
		<div class="w-full max-w-md rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-5 flex flex-col gap-4 shadow-2xl animate-in fade-in zoom-in-95 duration-150 text-left">
			<div class="flex items-start gap-3">
				<div class="w-8 h-8 rounded-lg bg-[var(--status-amber-muted)] text-[var(--status-amber)] flex items-center justify-center shrink-0 mt-0.5">
					<Warning size={18} />
				</div>
				<div class="flex flex-col gap-1">
					<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">
						Restore Volume Snapshot
					</h3>
					<span class="text-xs text-[var(--text-tertiary)] leading-relaxed">
						Restoring <code class="font-mono text-[var(--text-primary)]">{restoringSnapshot.filename}</code> will replace the current contents of volume <strong class="text-[var(--text-primary)]">{restoringSnapshot.volumeName}</strong>.
					</span>
				</div>
			</div>

			<div class="p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-[11px] text-[var(--text-secondary)] leading-relaxed">
				The target container will be briefly quiesced to ensure consistent file write integrity before resuming execution.
			</div>

			<div class="flex items-center justify-end gap-2 pt-2 border-t border-[var(--border-subtle)]">
				<Button variant="secondary" size="sm" onclick={() => (restoringSnapshot = null)}>
					Cancel
				</Button>
				<Button variant="primary" size="sm" disabled={isRestoring} onclick={confirmRestore}>
					{#if isRestoring}
						<CircleNotch size={13} class="animate-spin" /> Restoring...
					{:else}
						Confirm & Restore
					{/if}
				</Button>
			</div>
		</div>
	</div>
{/if}
