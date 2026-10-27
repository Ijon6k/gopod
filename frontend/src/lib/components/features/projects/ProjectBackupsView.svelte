<script lang="ts">
	import { volumes, services, dataStore } from '$lib/data';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import BackupDestinationsCard from '$lib/components/features/storage/BackupDestinationsCard.svelte';
	import {
		Button,
		Field,
		Input,
		Select,
		LayerCard,
		MountPath,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell
	} from '$lib/components/ui';
	import type { VolumeSnapshot, VolumeBackupSchedule } from '$lib/types';
	import {
		Database,
		ClockCounterClockwise,
		DownloadSimple,
		Trash,
		ArrowClockwise,
		CalendarCheck,
		CalendarPlus,
		CloudCheck,
		CloudWarning,
		HardDrives,
		CheckCircle,
		Warning,
		CircleNotch,
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

	// Schedule form state. The backend is the source of truth for cron syntax
	// (robfig cron.ParseStandard); the client-side check only catches the
	// obvious paste mistakes (fewer than 5 fields) before the round-trip.
	let showScheduleForm = $state(false);
	let isCreatingSchedule = $state(false);
	let scheduleVolume = $state('');
	let scheduleCron = $state('0 2 * * *');
	let scheduleRetention = $state(7);
	let scheduleLabel = $state('');
	let scheduleDestination = $state('');
	let scheduleError = $state('');

	const toast = useKumoToastManager();

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
			await dataStore.restoreVolumeSnapshot(restoringSnapshot.id);
			const targetVol = restoringSnapshot.volumeName;
			restoreSuccessMessage = `Snapshot restored successfully for volume "${targetVol}".`;
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

	// ── Remote destinations ──

	function destinationName(id: string): string {
		return dataStore.backupDestinations.find((d) => d.id === id)?.name ?? id;
	}

	function handleDownload(snap: VolumeSnapshot) {
		dataStore.downloadVolumeSnapshot(snap.id, snap.filename);
	}

	function openScheduleForm() {
		scheduleVolume = projectVolumes[0]?.name ?? '';
		scheduleCron = '0 2 * * *';
		scheduleRetention = 7;
		scheduleLabel = '';
		scheduleDestination = '';
		scheduleError = '';
		showScheduleForm = true;
	}

	function closeScheduleForm() {
		showScheduleForm = false;
		scheduleError = '';
	}

	async function handleCreateSchedule() {
		// Server-side validation is authoritative (robfig ParseStandard); this
		// only rejects the obvious mistakes locally so the user gets instant
		// feedback instead of a failed round-trip.
		const cronFields = scheduleCron.trim().split(/\s+/);
		if (!scheduleVolume) {
			scheduleError = 'Select a volume to back up.';
			return;
		}
		if (cronFields.length !== 5) {
			scheduleError = 'Cron needs exactly 5 fields (minute hour day month weekday).';
			return;
		}
		const retention = Number(scheduleRetention);
		if (!Number.isFinite(retention) || retention < 1 || retention > 365) {
			scheduleError = 'Retention must be between 1 and 365.';
			return;
		}

		isCreatingSchedule = true;
		scheduleError = '';
		try {
			await dataStore.createVolumeSchedule({
				projectId,
				volumeName: scheduleVolume,
				cron: scheduleCron.trim(),
				label: scheduleLabel.trim() || undefined,
				retentionCount: retention,
				destinationId: scheduleDestination || undefined
			});
			toast.add({
				title: 'Backup schedule created',
				description:
					scheduleDestination !== ''
						? 'Snapshots will mirror to the selected destination.'
						: 'The scheduler will pick it up on the next tick.',
				variant: 'success'
			});
			closeScheduleForm();
		} catch (err: unknown) {
			console.error('Failed to create schedule:', err);
			const message = err instanceof Error ? err.message : 'Unknown error';
			toast.add(
				errorToastOptions(
					'Backup schedule failed',
					message.includes('invalid cron')
						? 'The cron expression was rejected by the server — check the fields.'
						: 'Could not create the schedule — check server logs for details.'
				)
			);
		} finally {
			isCreatingSchedule = false;
		}
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
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Active project volumes ({projectVolumes.length})
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
								<span
									class="flex flex-wrap items-center gap-1 text-[11px] text-[var(--text-tertiary)]"
								>
									<span>Mount:</span>
									<MountPath value={vol.mount} />
									<span
										>· Service: <strong class="text-[var(--text-secondary)]"
											>{vol.serviceName}</strong
										></span
									>
								</span>
							</div>

							<div class="flex flex-col items-end">
								<span class="font-mono text-sm font-semibold text-[var(--text-primary)]"
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

	<!-- 2. Automated Backup Schedules -->
	<section class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<CalendarCheck size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Automated backup policies (cron)
				</h3>
			</div>
			<Button
				variant="secondary"
				size="sm"
				disabled={projectVolumes.length === 0}
				onclick={openScheduleForm}
			>
				<CalendarPlus size={12} />
				<span>New schedule</span>
			</Button>
		</div>

		<LayerCard class="overflow-hidden p-0">
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<Table class="w-full text-left text-xs">
					<TableHeader class="sticky top-0 z-20 border-b border-kumo-line bg-kumo-base">
						<TableRow>
							<TableHead
								class="px-3.5 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Target volume</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Schedule (cron)</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Retention</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Last run</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Next run</TableHead
							>
							<TableHead
								class="px-3.5 py-2.5 text-right text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Policy status</TableHead
							>
						</TableRow>
					</TableHeader>
					<TableBody class="divide-y divide-[var(--border-subtle)]">
						{#if schedules.length === 0}
							<TableRow>
								<TableCell colspan={6} class="py-6 text-center text-xs text-[var(--text-tertiary)]">
									No automated backup schedules defined for this project. Use "New schedule" to
									create one.
								</TableCell>
							</TableRow>
						{:else}
							{#each schedules as sched (sched.id)}
								<TableRow class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
									<TableCell
										class="px-3.5 py-2.5 font-mono font-medium whitespace-nowrap text-[var(--text-primary)]"
									>
										{sched.volumeName}
									</TableCell>
									<TableCell class="px-3 py-2.5 whitespace-nowrap">
										<div class="flex items-center gap-1.5">
											<span class="font-mono text-xs text-[var(--text-secondary)]"
												>{sched.cron}</span
											>
											<span class="text-[10.5px] text-[var(--text-tertiary)]">({sched.label})</span>
										</div>
									</TableCell>
									<TableCell class="px-3 py-2.5 whitespace-nowrap text-[var(--text-secondary)]">
										Keep last {sched.retentionCount} snapshots
									</TableCell>
									<TableCell
										class="px-3 py-2.5 font-mono text-[11.5px] whitespace-nowrap text-[var(--text-tertiary)]"
									>
										{sched.lastRun || '—'}
									</TableCell>
									<TableCell
										class="px-3 py-2.5 font-mono text-[11.5px] whitespace-nowrap text-[var(--text-tertiary)]"
									>
										{sched.nextRun || '—'}
									</TableCell>
									<TableCell class="px-3.5 py-2.5 text-right whitespace-nowrap">
										<button
											type="button"
											onclick={() => handleToggleSchedule(sched.id)}
											class="cursor-pointer border-0 bg-transparent text-xs font-medium {sched.enabled
												? 'text-[var(--status-green)]'
												: 'text-[var(--text-tertiary)]'}"
										>
											{sched.enabled ? '● Active' : '○ Paused'}
										</button>
									</TableCell>
								</TableRow>
							{/each}
						{/if}
					</TableBody>
				</Table>
			</div>
		</LayerCard>
	</section>

	<!-- 3. Remote backup destinations (shared card) -->
	<BackupDestinationsCard />

	<!-- 4. Snapshot History & Restore (Clean Table Standards) -->
	<section class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<ClockCounterClockwise size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Volume snapshot history ({snapshots.length})
				</h3>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Standard format: <code class="font-mono text-[var(--text-secondary)]">tar.zst</code> (lossless
				ultra-fast Zstandard)
			</span>
		</div>

		<LayerCard class="overflow-hidden p-0">
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<Table class="w-full border-collapse text-left text-xs">
					<TableHeader class="sticky top-0 z-20 border-b border-kumo-line bg-kumo-base">
						<TableRow>
							<TableHead
								class="px-3.5 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Archive filename</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Volume source</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Size</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Status</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Mirror</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Created</TableHead
							>
							<TableHead
								class="px-3.5 py-2.5 text-right text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Actions</TableHead
							>
						</TableRow>
					</TableHeader>
					<TableBody class="divide-y divide-[var(--border-subtle)]">
						{#if snapshots.length === 0}
							<TableRow>
								<TableCell
									colspan={7}
									class="py-12 text-center text-xs text-[var(--text-tertiary)]"
								>
									No volume snapshots taken yet. Click "Snapshot Now" on an active volume above.
								</TableCell>
							</TableRow>
						{:else}
							{#each snapshots as snap (snap.id)}
								<TableRow class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
									<!-- Filename -->
									<TableCell class="px-3.5 py-3 whitespace-nowrap">
										<div class="flex items-center gap-2">
											<FileArchive size={15} class="text-[var(--accent)]" />
											<span class="font-mono text-xs font-medium text-[var(--text-primary)]">
												{snap.filename}
											</span>
										</div>
									</TableCell>

									<!-- Volume -->
									<TableCell
										class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
									>
										{snap.volumeName}
									</TableCell>

									<!-- Size -->
									<TableCell
										class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-primary)]"
									>
										{snap.size}
									</TableCell>

									<!-- Status -->
									<TableCell class="px-3 py-3 whitespace-nowrap">
										<span
											class="inline-flex items-center gap-1 rounded border border-[var(--status-green)]/30 bg-[var(--status-green-muted)] px-1.5 py-0.5 text-[11px] font-medium text-[var(--status-green)]"
										>
											<CheckCircle size={12} /> Ready
										</span>
									</TableCell>

									<!-- Mirror -->
									<TableCell class="px-3 py-3 whitespace-nowrap">
										{#if snap.remoteStatus === 'synced'}
											<span
												class="inline-flex items-center gap-1 rounded border border-[var(--status-green)]/30 bg-[var(--status-green-muted)] px-1.5 py-0.5 text-[11px] font-medium text-[var(--status-green)]"
												title="Mirrored to {destinationName(snap.remoteDestinationId ?? '')}"
											>
												<CloudCheck size={12} /> Remote
											</span>
										{:else if snap.remoteStatus === 'local-only'}
											<span
												class="inline-flex items-center gap-1 rounded border border-[var(--status-amber)]/30 bg-[var(--status-amber-muted)] px-1.5 py-0.5 text-[11px] font-medium text-[var(--status-amber)]"
												title="Off-host copy missing: {snap.remoteError}"
											>
												<CloudWarning size={12} /> Local only
											</span>
										{:else if snap.remoteStatus === 'failed'}
											<span
												class="inline-flex items-center gap-1 rounded border border-[var(--status-red)]/30 bg-[var(--status-red-muted)] px-1.5 py-0.5 text-[11px] font-medium text-[var(--status-red)]"
												title="Mirror error: {snap.remoteError}"
											>
												<Warning size={12} /> Failed
											</span>
										{:else}
											<span
												class="text-[11px] text-[var(--text-tertiary)]"
												title="Never pushed off-host"
											>
												Local
											</span>
										{/if}
									</TableCell>

									<!-- Created -->
									<TableCell class="px-3 py-3 whitespace-nowrap text-[var(--text-tertiary)]">
										{snap.timeAgo}
									</TableCell>

									<!-- Actions: Restore, Download, Delete -->
									<TableCell class="px-3.5 py-3 text-right whitespace-nowrap">
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
												onclick={() => handleDownload(snap)}
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
									</TableCell>
								</TableRow>
							{/each}
						{/if}
					</TableBody>
				</Table>
			</div>
		</LayerCard>
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

<!-- Modal: Create Backup Schedule -->
{#if showScheduleForm}
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
					class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--accent-muted)] text-[var(--accent)]"
				>
					<CalendarPlus size={18} />
				</div>
				<div class="flex flex-col gap-1">
					<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
						New Automated Backup Schedule
					</h3>
					<span class="text-xs leading-relaxed text-[var(--text-tertiary)]">
						Backs up a volume on a cron schedule and keeps the newest archives (retention).
					</span>
				</div>
			</div>

			<Field label="Target volume">
				<Select
					bind:value={scheduleVolume}
					items={projectVolumes.map((vol) => ({ label: vol.name, value: vol.name }))}
					placeholder="Choose a volume..."
					disabled={isCreatingSchedule}
				/>
			</Field>

			<Field
				label="Cron schedule"
				description="Server timezone — 5 fields: minute hour day month weekday."
			>
				<Input
					id="schedule-cron"
					bind:value={scheduleCron}
					placeholder="0 2 * * *"
					class="font-mono"
					disabled={isCreatingSchedule}
				/>
				<div class="flex flex-wrap items-center gap-1.5 pt-1.5">
					{#each [{ label: 'Daily 02:00', cron: '0 2 * * *' }, { label: 'Every 6 h', cron: '0 */6 * * *' }, { label: 'Weekly Sun 03:00', cron: '0 3 * * 0' }] as preset (preset.cron)}
						<button
							type="button"
							onclick={() => (scheduleCron = preset.cron)}
							class="cursor-pointer rounded border-0 bg-[var(--accent-muted)] px-2 py-1 text-[10.5px] font-medium text-[var(--accent)] transition-colors hover:bg-[var(--accent)] hover:text-[var(--bg-shell)]"
						>
							{preset.label}
						</button>
					{/each}
				</div>
			</Field>

			<Field
				label="Remote destination (optional)"
				description="Mirror every scheduled snapshot to an object store in addition to this host."
			>
				<Select
					bind:value={scheduleDestination}
					items={[
						{ label: '— Local only (this host) —', value: '' },
						...dataStore.backupDestinations.map((d) => ({ label: d.name, value: d.id }))
					]}
					placeholder="Choose a destination..."
					disabled={isCreatingSchedule}
				/>
				{#if dataStore.backupDestinations.length === 0}
					<span class="pt-1.5 text-[11px] text-[var(--text-tertiary)]">
						No destinations configured yet — create one under "Remote backup destinations".
					</span>
				{/if}
			</Field>

			<div class="grid grid-cols-2 gap-3">
				<Field label="Retention" description="Newest archives kept (1–365).">
					<Input
						id="schedule-retention"
						type="number"
						bind:value={scheduleRetention}
						min="1"
						max="365"
						disabled={isCreatingSchedule}
					/>
				</Field>
				<Field label="Label (optional)">
					<Input
						id="schedule-label"
						bind:value={scheduleLabel}
						placeholder="Daily at 02:00"
						disabled={isCreatingSchedule}
					/>
				</Field>
			</div>

			{#if scheduleError}
				<div
					class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--status-red)]/30 bg-[var(--status-red-muted)] p-2.5 text-[11px] font-medium text-[var(--status-red)]"
				>
					<Warning size={13} />
					{scheduleError}
				</div>
			{/if}

			<div class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-2">
				<Button variant="secondary" size="sm" onclick={closeScheduleForm}>Cancel</Button>
				<Button
					variant="primary"
					size="sm"
					disabled={isCreatingSchedule || !scheduleVolume}
					onclick={handleCreateSchedule}
				>
					{#if isCreatingSchedule}
						<CircleNotch size={13} class="animate-spin" /> Creating...
					{:else}
						Create schedule
					{/if}
				</Button>
			</div>
		</div>
	</div>
{/if}
