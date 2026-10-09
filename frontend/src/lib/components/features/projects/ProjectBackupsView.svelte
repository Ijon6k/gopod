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

		try {
			await dataStore.createVolumeSnapshot(volumeName, projectId, serviceId);
		} catch (err) {
			console.error('Failed to create snapshot:', err);
		} finally {
			isCreatingSnapshot = false;
			snapshotTargetVolume = null;
		}
	}

	async function handleDeleteSnapshot(id: string) {
		if (confirm('Delete this volume snapshot? This action cannot be undone.')) {
			try {
				await dataStore.deleteVolumeSnapshot(id);
			} catch (err) {
				console.error('Failed to delete snapshot:', err);
			}
		}
	}

	function openRestoreModal(snap: VolumeSnapshot) {
		restoringSnapshot = snap;
	}

	async function confirmRestore() {
		if (!restoringSnapshot) return;
		isRestoring = true;

		try {
			const targetVol = restoringSnapshot.volumeName;
			restoreSuccessMessage = `Snapshot restore initialized for volume "${targetVol}".`;
			setTimeout(() => (restoreSuccessMessage = null), 4000);
		} catch (err) {
			console.error('Failed to restore snapshot:', err);
		} finally {
			isRestoring = false;
			restoringSnapshot = null;
		}
	}

	function handleToggleSchedule(id: string) {
		dataStore.toggleVolumeSchedule(id);
	}

	function handleDownload(filename: string) {
		alert(`Preparing download for ${filename} (compressed zstd archive)`);
	}
</script>

<div class="flex w-full flex-col gap-6 text-left">
	<!-- Success Notification -->
	{#if restoreSuccessMessage}
		<div
			class="animate-in fade-in flex items-center justify-between rounded-[var(--radius-card)] border border-[var(--status-green)]/30 bg-[var(--status-green-muted)] p-3.5 duration-150"
		>
			<div class="flex items-center gap-2.5 text-xs font-medium text-[var(--status-green)]">
				<CheckCircle size={16} />
				<span>{restoreSuccessMessage}</span>
			</div>
			<button
				type="button"
				onclick={() => (restoreSuccessMessage = null)}
				class="cursor-pointer border-0 bg-transparent p-1 text-[var(--status-green)] hover:opacity-75"
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
				<h3 class="m-0 text-xs font-semibold tracking-wider text-[var(--text-primary)] uppercase">
					Active Project Volumes ({projectVolumes.length})
				</h3>
			</div>
			<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
				Podman rootless storage: ~/.local/share/containers/storage/volumes/
			</span>
		</div>

		<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
			{#if projectVolumes.length === 0}
				<div
					class="col-span-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-8 text-center text-xs text-[var(--text-tertiary)]"
				>
					No persistent volumes configured for this project.
				</div>
			{:else}
				{#each projectVolumes as vol (vol.id)}
					{@const volSnapshots = snapshots.filter((s) => s.volumeName === vol.name)}
					{@const latestSnap = volSnapshots[0]}
					<div
						class="flex flex-col justify-between gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-4"
					>
						<div class="flex items-start justify-between gap-2">
							<div class="flex flex-col gap-1">
								<div class="flex items-center gap-2">
									<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">
										{vol.name}
									</span>
									<span
										class="py-0.2 rounded bg-[var(--status-green-muted)] px-1.5 font-mono text-[10px] font-medium text-[var(--status-green)]"
									>
										mounted
									</span>
								</div>
								<span class="text-[11px] text-[var(--text-tertiary)]">
									Mount: <code class="font-mono text-[var(--text-secondary)]">{vol.mount}</code> ·
									Service: <strong class="text-[var(--text-secondary)]">{vol.serviceName}</strong>
								</span>
							</div>

							<div class="flex flex-col items-end">
								<span class="font-mono text-sm font-bold text-[var(--text-primary)]"
									>{vol.size}</span
								>
								<span class="text-[10.5px] text-[var(--text-tertiary)]">on disk</span>
							</div>
						</div>

						<div
							class="flex items-center justify-between border-t border-[var(--border-subtle)] pt-2 text-xs"
						>
							<span class="text-[11px] text-[var(--text-tertiary)]">
								{#if latestSnap}
									Last snapshot: <strong class="text-[var(--text-secondary)]"
										>{latestSnap.timeAgo}</strong
									>
									({latestSnap.size})
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
			<h3 class="m-0 text-xs font-semibold tracking-wider text-[var(--text-primary)] uppercase">
				Automated Backup Policies (Cron)
			</h3>
		</div>

		<div
			class="overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)]"
		>
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<table class="w-full border-collapse text-left text-xs">
					<thead
						class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
					>
						<tr>
							<th
								class="px-3.5 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Target Volume</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Schedule (Cron)</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Retention</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Last Run</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Next Run</th
							>
							<th
								class="px-3.5 py-2.5 text-right text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Policy Status</th
							>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border-subtle)]">
						{#if schedules.length === 0}
							<tr>
								<td colspan="6" class="py-6 text-center text-xs text-[var(--text-tertiary)]">
									No automated backup schedules defined for this project.
								</td>
							</tr>
						{:else}
							{#each schedules as sched (sched.id)}
								<tr class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
									<td
										class="px-3.5 py-2.5 font-mono font-medium whitespace-nowrap text-[var(--text-primary)]"
									>
										{sched.volumeName}
									</td>
									<td class="px-3 py-2.5 whitespace-nowrap">
										<div class="flex items-center gap-1.5">
											<span class="font-mono text-xs text-[var(--text-secondary)]"
												>{sched.cron}</span
											>
											<span class="text-[10.5px] text-[var(--text-tertiary)]">({sched.label})</span>
										</div>
									</td>
									<td class="px-3 py-2.5 whitespace-nowrap text-[var(--text-secondary)]">
										Keep last {sched.retentionCount} snapshots
									</td>
									<td
										class="px-3 py-2.5 font-mono text-[11.5px] whitespace-nowrap text-[var(--text-tertiary)]"
									>
										{sched.lastRun || '—'}
									</td>
									<td
										class="px-3 py-2.5 font-mono text-[11.5px] whitespace-nowrap text-[var(--text-tertiary)]"
									>
										{sched.nextRun || '—'}
									</td>
									<td class="px-3.5 py-2.5 text-right whitespace-nowrap">
										<button
											type="button"
											onclick={() => handleToggleSchedule(sched.id)}
											class="cursor-pointer border-0 bg-transparent text-xs font-medium {sched.enabled
												? 'text-[var(--status-green)]'
												: 'text-[var(--text-tertiary)]'}"
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
				<h3 class="m-0 text-xs font-semibold tracking-wider text-[var(--text-primary)] uppercase">
					Volume Snapshot History ({snapshots.length})
				</h3>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Standard format: <code class="font-mono text-[var(--text-secondary)]">tar.zst</code> (lossless
				ultra-fast Zstandard)
			</span>
		</div>

		<div
			class="overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)]"
		>
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<table class="w-full border-collapse text-left text-xs">
					<thead
						class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
					>
						<tr>
							<th
								class="px-3.5 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Archive Filename</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Volume Source</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Size</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Status</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Created</th
							>
							<th
								class="px-3.5 py-2.5 text-right text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Actions</th
							>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border-subtle)]">
						{#if snapshots.length === 0}
							<tr>
								<td colspan="6" class="py-12 text-center text-xs text-[var(--text-tertiary)]">
									No volume snapshots taken yet. Click "Snapshot Now" on an active volume above.
								</td>
							</tr>
						{:else}
							{#each snapshots as snap (snap.id)}
								<tr class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
									<!-- Filename -->
									<td class="px-3.5 py-3 whitespace-nowrap">
										<div class="flex items-center gap-2">
											<FileArchive size={15} class="text-[var(--accent)]" />
											<span class="font-mono text-xs font-medium text-[var(--text-primary)]">
												{snap.filename}
											</span>
										</div>
									</td>

									<!-- Volume -->
									<td
										class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
									>
										{snap.volumeName}
									</td>

									<!-- Size -->
									<td
										class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-primary)]"
									>
										{snap.size}
									</td>

									<!-- Status -->
									<td class="px-3 py-3 whitespace-nowrap">
										<span
											class="inline-flex items-center gap-1 rounded border border-[var(--status-green)]/30 bg-[var(--status-green-muted)] px-1.5 py-0.5 text-[11px] font-medium text-[var(--status-green)]"
										>
											<CheckCircle size={12} /> Ready
										</span>
									</td>

									<!-- Created -->
									<td class="px-3 py-3 whitespace-nowrap text-[var(--text-tertiary)]">
										{snap.timeAgo}
									</td>

									<!-- Actions: Restore, Download, Delete -->
									<td class="px-3.5 py-3 text-right whitespace-nowrap">
										<div class="inline-flex items-center gap-1.5">
											<button
												type="button"
												onclick={() => openRestoreModal(snap)}
												class="cursor-pointer rounded border-0 bg-[var(--accent-muted)] px-2 py-1 text-[11px] font-medium text-[var(--accent)] transition-colors hover:bg-[var(--accent)] hover:text-[var(--bg-shell)]"
												title="Restore this snapshot to volume"
											>
												Restore
											</button>
											<button
												type="button"
												onclick={() => handleDownload(snap.filename)}
												class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
												title="Download Archive"
											>
												<DownloadSimple size={14} />
											</button>
											<button
												type="button"
												onclick={() => handleDeleteSnapshot(snap.id)}
												class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--status-red-muted)] hover:text-[var(--status-red)]"
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
		class="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(5,6,7,0.82)] p-4 backdrop-blur-[3px]"
		role="dialog"
		aria-modal="true"
	>
		<div
			class="animate-in fade-in zoom-in-95 flex w-full max-w-md flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-5 text-left shadow-2xl duration-150"
		>
			<div class="flex items-start gap-3">
				<div
					class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--status-amber-muted)] text-[var(--status-amber)]"
				>
					<Warning size={18} />
				</div>
				<div class="flex flex-col gap-1">
					<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
						Restore Volume Snapshot
					</h3>
					<span class="text-xs leading-relaxed text-[var(--text-tertiary)]">
						Restoring <code class="font-mono text-[var(--text-primary)]"
							>{restoringSnapshot.filename}</code
						>
						will replace the current contents of volume
						<strong class="text-[var(--text-primary)]">{restoringSnapshot.volumeName}</strong>.
					</span>
				</div>
			</div>

			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 text-[11px] leading-relaxed text-[var(--text-secondary)]"
			>
				The target container will be briefly quiesced to ensure consistent file write integrity
				before resuming execution.
			</div>

			<div class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-2">
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
