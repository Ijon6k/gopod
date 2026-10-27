<script lang="ts">
	import type { Service, VolumeSnapshot, VolumeBackupSchedule } from '$lib/types';
	import { volumes, dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import {
		Button,
		Field,
		Input,
		Select,
		LayerCard,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell
	} from '$lib/components/ui';
	import {
		detectDatabaseEngine,
		extractDatabaseCredentials,
		revealMaskedEnvValues,
		type DatabaseEngine
	} from '../../database/database-utils';
	import {
		Database,
		HardDrives,
		ClockCounterClockwise,
		CalendarCheck,
		CalendarPlus,
		DownloadSimple,
		Trash,
		CheckCircle,
		Warning,
		CircleNotch,
		FileArchive,
		CloudCheck,
		CloudWarning,
		X
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	// Detect if this service is or behaves like a database
	let detectedEngine = $derived(detectDatabaseEngine(service));
	let isDbService = $derived(detectedEngine !== 'custom' || service.type === 'database');

	// Credential extraction for DB dump suggestions
	let revealedEnv = $state<Record<string, string>>({});
	let creds = $derived(extractDatabaseCredentials(service, revealedEnv));

	// Auto reveal env on mount/change to extract credentials accurately
	$effect(() => {
		void (async () => {
			revealedEnv = await revealMaskedEnvValues(service);
		})();
	});

	// Service-specific data
	let serviceSnapshots = $derived(
		dataStore.volumeSnapshots.filter(
			(s) =>
				s.serviceId === service.id ||
				(service.projectId &&
					s.projectId === service.projectId &&
					s.volumeName.includes(service.name))
		)
	);
	let serviceSchedules = $derived(
		dataStore.volumeSchedules.filter(
			(s) =>
				s.serviceId === service.id ||
				(service.projectId &&
					s.projectId === service.projectId &&
					s.volumeName.includes(service.name))
		)
	);
	let serviceVolumes = $derived(
		volumes.filter(
			(v) =>
				v.serviceId === service.id ||
				(service.projectId && v.projectId === service.projectId && v.name.includes(service.name))
		)
	);

	// DB Dump form state
	let isDumpingDb = $state(false);
	let dumpDbName = $state('');
	let dumpUser = $state('');
	let dumpPassword = $state('');
	let dumpDestination = $state('');

	// Keep dump defaults aligned with extracted credentials
	$effect(() => {
		if (creds.databaseName && !dumpDbName) dumpDbName = creds.databaseName;
		if (creds.user && !dumpUser) dumpUser = creds.user;
		if (creds.password && !dumpPassword) dumpPassword = creds.password;
	});

	// Schedule dialog state
	let showScheduleModal = $state(false);
	let isCreatingSchedule = $state(false);
	let schedTarget = $state('');
	let schedCron = $state('0 2 * * *');
	let schedRetention = $state(7);
	let schedLabel = $state('');
	let schedDestination = $state('');
	let schedError = $state('');

	// Snapshot restore state
	let restoringSnapshot = $state<VolumeSnapshot | null>(null);
	let isRestoring = $state(false);
	let restoreSuccessMessage = $state<string | null>(null);

	// Snapshot manual state
	let isSnapshottingVol = $state<string | null>(null);

	async function handleRunDbDump() {
		isDumpingDb = true;
		try {
			await dataStore.createDatabaseDump({
				projectId: service.projectId,
				serviceId: service.id,
				engine: detectedEngine,
				database: dumpDbName.trim() || undefined,
				username: dumpUser.trim() || undefined,
				password: dumpPassword || undefined,
				destinationId: dumpDestination || undefined
			});
			toast.add({
				title: 'Database dump completed',
				description: `Dump created successfully for ${service.name}.`,
				variant: 'success'
			});
		} catch (err: unknown) {
			console.error('DB dump failed:', err);
			const message = err instanceof Error ? err.message : 'Dump failed';
			toast.add(errorToastOptions('Database dump failed', message));
		} finally {
			isDumpingDb = false;
		}
	}

	async function handleSnapshotVolume(volName: string) {
		isSnapshottingVol = volName;
		try {
			await dataStore.createVolumeSnapshot(volName, service.projectId, service.id);
			toast.add({
				title: 'Volume snapshot created',
				description: `Snapshot archive generated for volume ${volName}.`,
				variant: 'success'
			});
		} catch (err: unknown) {
			console.error('Volume snapshot failed:', err);
			const message = err instanceof Error ? err.message : 'Snapshot failed';
			toast.add(errorToastOptions('Snapshot failed', message));
		} finally {
			isSnapshottingVol = null;
		}
	}

	function openScheduleModal() {
		schedTarget = isDbService ? `db:${detectedEngine}` : (serviceVolumes[0]?.name ?? '');
		schedCron = '0 2 * * *';
		schedRetention = 7;
		schedLabel = `${service.name} backup`;
		schedDestination = '';
		schedError = '';
		showScheduleModal = true;
	}

	async function handleCreateSchedule() {
		const cronFields = schedCron.trim().split(/\s+/);
		if (!schedTarget) {
			schedError = 'Target volume or database is required.';
			return;
		}
		if (cronFields.length !== 5) {
			schedError = 'Cron needs exactly 5 fields (minute hour day month weekday).';
			return;
		}
		isCreatingSchedule = true;
		schedError = '';
		try {
			await dataStore.createVolumeSchedule({
				projectId: service.projectId,
				serviceId: service.id,
				volumeName: schedTarget,
				cron: schedCron.trim(),
				label: schedLabel.trim() || undefined,
				retentionCount: schedRetention,
				destinationId: schedDestination || undefined
			});
			toast.add({
				title: 'Backup schedule created',
				description: 'Automated backups configured for this service.',
				variant: 'success'
			});
			showScheduleModal = false;
		} catch (err: unknown) {
			console.error('Failed to create schedule:', err);
			schedError = err instanceof Error ? err.message : 'Failed to create schedule';
		} finally {
			isCreatingSchedule = false;
		}
	}

	async function confirmRestore() {
		if (!restoringSnapshot) return;
		isRestoring = true;
		try {
			await dataStore.restoreVolumeSnapshot(restoringSnapshot.id);
			restoreSuccessMessage = `Restored ${restoringSnapshot.filename} successfully.`;
			setTimeout(() => (restoreSuccessMessage = null), 4000);
		} catch (err) {
			console.error('Failed to restore snapshot:', err);
		} finally {
			isRestoring = false;
			restoringSnapshot = null;
		}
	}

	function handleDownload(snap: VolumeSnapshot) {
		dataStore.downloadVolumeSnapshot(snap.id, snap.filename);
	}

	async function handleDeleteSnapshot(id: string) {
		if (confirm('Delete this backup archive?')) {
			try {
				await dataStore.deleteVolumeSnapshot(id);
			} catch (err) {
				console.error('Failed to delete snapshot:', err);
			}
		}
	}
</script>

<div class="flex w-full flex-col gap-6">
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

	<!-- 1. Database Dump Action Card (if DB detected) -->
	{#if isDbService}
		<section class="flex flex-col gap-3">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<Database size={16} class="text-[var(--accent)]" />
					<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
						Database Dump ({detectedEngine.toUpperCase()})
					</h3>
				</div>
				<span
					class="rounded bg-[var(--accent-muted)] px-2 py-0.5 text-[11px] font-medium text-[var(--accent)]"
				>
					Native logical dump streaming
				</span>
			</div>

			<LayerCard class="flex flex-col gap-4 p-4">
				<div class="text-xs leading-relaxed text-[var(--text-secondary)]">
					Triggers an engine-native logical dump (e.g. <code
						class="font-mono text-[var(--text-primary)]">pg_dump</code
					>, <code class="font-mono text-[var(--text-primary)]">mysqldump</code>) streamed and
					compressed into an off-host-ready archive.
				</div>

				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<Field label="Database name">
						<Input
							bind:value={dumpDbName}
							placeholder="gopod"
							class="font-mono"
							disabled={isDumpingDb}
						/>
					</Field>
					<Field label="User">
						<Input
							bind:value={dumpUser}
							placeholder="postgres"
							class="font-mono"
							disabled={isDumpingDb}
						/>
					</Field>
					<Field label="Remote mirror">
						<Select
							bind:value={dumpDestination}
							items={[
								{ label: '— Local only —', value: '' },
								...dataStore.backupDestinations.map((d) => ({ label: d.name, value: d.id }))
							]}
							placeholder="Select destination..."
							disabled={isDumpingDb}
						/>
					</Field>
				</div>

				<div class="flex items-center justify-end border-t border-[var(--border-subtle)] pt-3">
					<Button variant="primary" size="sm" disabled={isDumpingDb} onclick={handleRunDbDump}>
						{#if isDumpingDb}
							<CircleNotch size={13} class="animate-spin" /> Dumping database...
						{:else}
							<Database size={13} /> Dump Database Now
						{/if}
					</Button>
				</div>
			</LayerCard>
		</section>
	{/if}

	<!-- 2. Mounted Volumes -->
	<section class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<HardDrives size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Service Persistent Volumes ({serviceVolumes.length})
				</h3>
			</div>
		</div>

		{#if serviceVolumes.length === 0}
			<LayerCard class="p-6 text-center text-xs text-[var(--text-tertiary)]">
				No persistent volumes mounted directly to this service.
			</LayerCard>
		{:else}
			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				{#each serviceVolumes as vol (vol.name)}
					<LayerCard class="flex items-center justify-between p-3.5">
						<div class="flex flex-col gap-1">
							<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">
								{vol.name}
							</span>
							<span class="text-[11px] text-[var(--text-tertiary)]">
								Driver: {vol.driver || 'local'} • Size: {vol.size || 'N/A'}
							</span>
						</div>
						<Button
							variant="secondary"
							size="sm"
							disabled={isSnapshottingVol === vol.name}
							onclick={() => handleSnapshotVolume(vol.name)}
						>
							{#if isSnapshottingVol === vol.name}
								<CircleNotch size={12} class="animate-spin" />
							{:else}
								Snapshot Volume
							{/if}
						</Button>
					</LayerCard>
				{/each}
			</div>
		{/if}
	</section>

	<!-- 3. Automated Schedules for this Service -->
	<section class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<CalendarCheck size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Service Backup Schedules ({serviceSchedules.length})
				</h3>
			</div>
			<Button variant="secondary" size="sm" onclick={openScheduleModal}>
				<CalendarPlus size={12} />
				<span>New Schedule</span>
			</Button>
		</div>

		<LayerCard class="overflow-hidden p-0">
			{#if serviceSchedules.length === 0}
				<div class="p-6 text-center text-xs text-[var(--text-tertiary)]">
					No automated backup schedules configured for this service yet.
				</div>
			{:else}
				<div class="w-full overflow-x-auto md:overflow-x-visible">
					<Table class="w-full border-collapse text-left text-xs">
						<TableHeader class="border-b border-[var(--border)] bg-[var(--bg-table-header)]">
							<TableRow>
								<TableHead
									class="px-3.5 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Target</TableHead
								>
								<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Cron</TableHead
								>
								<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Retention</TableHead
								>
								<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Status</TableHead
								>
								<TableHead
									class="px-3.5 py-2.5 text-right text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Action</TableHead
								>
							</TableRow>
						</TableHeader>
						<TableBody class="divide-y divide-[var(--border-subtle)]">
							{#each serviceSchedules as sched (sched.id)}
								<TableRow class="hover:bg-[var(--bg-table-row-alt)]">
									<TableCell class="px-3.5 py-3 font-mono font-medium text-[var(--text-primary)]">
										{sched.volumeName}
									</TableCell>
									<TableCell class="px-3 py-3 font-mono text-[var(--text-secondary)]">
										{sched.cron}
									</TableCell>
									<TableCell class="px-3 py-3 text-[var(--text-tertiary)]">
										{sched.retentionCount ?? 7} copies
									</TableCell>
									<TableCell class="px-3 py-3">
										<span
											class="inline-flex items-center gap-1 text-[11px] {sched.enabled
												? 'text-[var(--status-green)]'
												: 'text-[var(--text-tertiary)]'}"
										>
											<span
												class="h-1.5 w-1.5 rounded-full {sched.enabled
													? 'bg-[var(--status-green)]'
													: 'bg-[var(--text-tertiary)]'}"
											></span>
											{sched.enabled ? 'Enabled' : 'Disabled'}
										</span>
									</TableCell>
									<TableCell class="px-3.5 py-3 text-right">
										<button
											type="button"
											onclick={() => dataStore.toggleVolumeSchedule(sched.id)}
											class="cursor-pointer border-0 bg-transparent text-[11px] text-[var(--accent)] hover:underline"
										>
											{sched.enabled ? 'Disable' : 'Enable'}
										</button>
									</TableCell>
								</TableRow>
							{/each}
						</TableBody>
					</Table>
				</div>
			{/if}
		</LayerCard>
	</section>

	<!-- 4. Snapshot History & Restore for this Service -->
	<section class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<ClockCounterClockwise size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Service Backups & Dumps History ({serviceSnapshots.length})
				</h3>
			</div>
		</div>

		<LayerCard class="overflow-hidden p-0">
			{#if serviceSnapshots.length === 0}
				<div class="p-8 text-center text-xs text-[var(--text-tertiary)]">
					No backup snapshots taken for this service yet.
				</div>
			{:else}
				<div class="w-full overflow-x-auto md:overflow-x-visible">
					<Table class="w-full border-collapse text-left text-xs">
						<TableHeader class="border-b border-[var(--border)] bg-[var(--bg-table-header)]">
							<TableRow>
								<TableHead
									class="px-3.5 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Archive filename</TableHead
								>
								<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Target</TableHead
								>
								<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Size</TableHead
								>
								<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Mirror</TableHead
								>
								<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Created</TableHead
								>
								<TableHead
									class="px-3.5 py-2.5 text-right text-[12px] font-semibold text-[var(--text-tertiary)]"
									>Actions</TableHead
								>
							</TableRow>
						</TableHeader>
						<TableBody class="divide-y divide-[var(--border-subtle)]">
							{#each serviceSnapshots as snap (snap.id)}
								<TableRow class="hover:bg-[var(--bg-table-row-alt)]">
									<TableCell class="px-3.5 py-3 font-mono font-medium text-[var(--text-primary)]">
										<div class="flex items-center gap-1.5">
											<FileArchive size={14} class="text-[var(--accent)]" />
											<span>{snap.filename}</span>
										</div>
									</TableCell>
									<TableCell class="px-3 py-3 font-mono text-[var(--text-secondary)]">
										{snap.volumeName}
									</TableCell>
									<TableCell class="px-3 py-3 font-mono text-[var(--text-primary)]">
										{snap.size}
									</TableCell>
									<TableCell class="px-3 py-3">
										{#if snap.remoteStatus === 'synced'}
											<span
												class="inline-flex items-center gap-1 rounded border border-[var(--status-green)]/30 bg-[var(--status-green-muted)] px-1.5 py-0.5 text-[10.5px] font-medium text-[var(--status-green)]"
											>
												<CloudCheck size={12} /> Remote
											</span>
										{:else if snap.remoteStatus === 'local-only'}
											<span
												class="inline-flex items-center gap-1 rounded border border-[var(--status-amber)]/30 bg-[var(--status-amber-muted)] px-1.5 py-0.5 text-[10.5px] font-medium text-[var(--status-amber)]"
											>
												<CloudWarning size={12} /> Local only
											</span>
										{:else}
											<span class="text-[11px] text-[var(--text-tertiary)]">Local</span>
										{/if}
									</TableCell>
									<TableCell class="px-3 py-3 text-[var(--text-tertiary)]">
										{snap.timeAgo || 'Recently'}
									</TableCell>
									<TableCell class="px-3.5 py-3 text-right">
										<div class="inline-flex items-center gap-1.5">
											{#if !snap.volumeName.startsWith('db:')}
												<button
													type="button"
													onclick={() => (restoringSnapshot = snap)}
													class="cursor-pointer rounded border-0 bg-[var(--accent-muted)] px-2 py-1 text-[11px] font-medium text-[var(--accent)] hover:bg-[var(--accent)] hover:text-white"
												>
													Restore
												</button>
											{/if}
											<button
												type="button"
												onclick={() => handleDownload(snap)}
												class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
												title="Download archive"
											>
												<DownloadSimple size={14} />
											</button>
											<button
												type="button"
												onclick={() => handleDeleteSnapshot(snap.id)}
												class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] hover:text-[var(--status-red)]"
												title="Delete"
											>
												<Trash size={14} />
											</button>
										</div>
									</TableCell>
								</TableRow>
							{/each}
						</TableBody>
					</Table>
				</div>
			{/if}
		</LayerCard>
	</section>
</div>

<!-- Modal: New Schedule for this Service -->
{#if showScheduleModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(5,6,7,0.82)] p-4 backdrop-blur-[3px]"
		role="dialog"
	>
		<div
			class="animate-in fade-in flex w-full max-w-md flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-5"
		>
			<div class="flex items-center gap-2">
				<CalendarPlus size={18} class="text-[var(--accent)]" />
				<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
					Schedule Backup for {service.name}
				</h3>
			</div>

			<Field label="Backup Target">
				<Select
					bind:value={schedTarget}
					items={[
						...(isDbService
							? [{ label: `Database Dump (${detectedEngine})`, value: `db:${detectedEngine}` }]
							: []),
						...serviceVolumes.map((v) => ({ label: `Volume: ${v.name}`, value: v.name }))
					]}
					disabled={isCreatingSchedule}
				/>
			</Field>

			<Field label="Cron expression" description="5 fields (e.g. 0 2 * * *)">
				<Input bind:value={schedCron} class="font-mono" disabled={isCreatingSchedule} />
			</Field>

			<div class="grid grid-cols-2 gap-3">
				<Field label="Retention (copies)">
					<Input
						type="number"
						bind:value={schedRetention}
						min="1"
						max="365"
						disabled={isCreatingSchedule}
					/>
				</Field>
				<Field label="Remote mirror">
					<Select
						bind:value={schedDestination}
						items={[
							{ label: '— Local only —', value: '' },
							...dataStore.backupDestinations.map((d) => ({ label: d.name, value: d.id }))
						]}
						disabled={isCreatingSchedule}
					/>
				</Field>
			</div>

			{#if schedError}
				<div
					class="rounded border border-[var(--status-red)]/30 bg-[var(--status-red-muted)] p-2 text-xs text-[var(--status-red)]"
				>
					{schedError}
				</div>
			{/if}

			<div class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-2">
				<Button variant="secondary" size="sm" onclick={() => (showScheduleModal = false)}
					>Cancel</Button
				>
				<Button
					variant="primary"
					size="sm"
					disabled={isCreatingSchedule}
					onclick={handleCreateSchedule}
				>
					{#if isCreatingSchedule}
						<CircleNotch size={13} class="animate-spin" /> Saving...
					{:else}
						Create Schedule
					{/if}
				</Button>
			</div>
		</div>
	</div>
{/if}

<!-- Modal: Restore Volume Snapshot -->
{#if restoringSnapshot}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(5,6,7,0.82)] p-4 backdrop-blur-[3px]"
		role="dialog"
	>
		<div
			class="animate-in fade-in flex w-full max-w-md flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-5"
		>
			<div class="flex items-center gap-2 text-[var(--status-amber)]">
				<Warning size={18} />
				<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Restore Snapshot</h3>
			</div>
			<div class="text-xs leading-relaxed text-[var(--text-secondary)]">
				Restoring <code class="font-mono text-[var(--text-primary)]"
					>{restoringSnapshot.filename}</code
				>
				will replace the current contents of volume <strong>{restoringSnapshot.volumeName}</strong>.
			</div>
			<div class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-2">
				<Button variant="secondary" size="sm" onclick={() => (restoringSnapshot = null)}
					>Cancel</Button
				>
				<Button variant="primary" size="sm" disabled={isRestoring} onclick={confirmRestore}>
					{#if isRestoring}
						<CircleNotch size={13} class="animate-spin" /> Restoring...
					{:else}
						Confirm Restore
					{/if}
				</Button>
			</div>
		</div>
	</div>
{/if}
