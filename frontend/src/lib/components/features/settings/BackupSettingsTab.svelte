<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { dataStore } from '$lib/data';
	import { formatTimeAgo } from '$lib/utils/format';
	import type { VolumeSnapshot } from '$lib/api/storage/types';
	import {
		Button,
		Field,
		Select,
		Switch,
		LayerCard,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		Badge,
		Empty,
		InputGroup,
		InputGroupAddon,
		InputGroupButton,
		InputGroupInput,
		ConfirmDialog,
		Modal
	} from '$lib/components/ui';
	import type { SystemBackupVerification } from '$lib/api/system/types';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import BackupDestinationsCard from '$lib/components/features/storage/BackupDestinationsCard.svelte';
	import {
		Archive,
		ArrowClockwise,
		CalendarCheck,
		DownloadSimple,
		Trash,
		CircleNotch,
		FileArchive,
		FileMagnifyingGlass,
		WarningCircle,
		CloudCheck,
		CloudWarning,
		ClockCounterClockwise,
		CheckCircle,
		Minus,
		Plus,
		Database,
		Key,
		Sliders
	} from 'phosphor-svelte';

	const toast = useKumoToastManager();

	let backups = $state<VolumeSnapshot[]>([]);
	let isLoading = $state(true);
	let loadFailed = $state(false);
	let isBackingUp = $state(false);
	let isSavingSchedule = $state(false);

	// Manual backup form
	let selectedDestination = $state('');

	// Schedule form
	let schedCron = $state('0 3 * * *');
	let schedRetention = $state(7);
	let schedDestination = $state('');
	let schedEnabled = $state(true);

	// Cron presets
	const cronPresets = [
		{ label: 'Daily at 03:00', expr: '0 3 * * *' },
		{ label: 'Daily at midnight', expr: '0 0 * * *' },
		{ label: 'Every 12 hours', expr: '0 */12 * * *' },
		{ label: 'Weekly on Sunday', expr: '0 2 * * 0' }
	];

	// Delete confirmation
	let snapToDelete = $state<string | null>(null);
	let isDeleting = $state(false);

	// Archive verification report
	let verifyTarget = $state<VolumeSnapshot | null>(null);
	let verifyResult = $state<SystemBackupVerification | null>(null);
	let isVerifying = $state(false);

	// Single source for both destination selects on this tab.
	const destinationItems = $derived([
		{ label: '— Local only —', value: '' },
		...dataStore.backupDestinations.map((d) => ({ label: d.name, value: d.id }))
	]);

	function clampRetention(next: number | string): number {
		const n = Math.floor(Number(next));
		if (!Number.isFinite(n)) return 7;
		return Math.min(365, Math.max(1, n));
	}

	async function loadBackupData() {
		try {
			const [historyList, schedRes] = await Promise.all([
				api.system.listBackups(),
				api.system.getBackupSchedule().catch(() => null),
				dataStore.fetchBackupDestinations().catch(() => null)
			]);
			backups = historyList;
			loadFailed = false;
			if (schedRes) {
				// Form fields are the single source of truth for the UI; schedule
				// metadata only feeds them (no duplicated read/write state).
				schedCron = schedRes.cron;
				schedRetention = clampRetention(schedRes.retentionCount ?? 7);
				schedDestination = schedRes.destinationId ?? '';
				schedEnabled = schedRes.enabled;
			}
		} catch (err: unknown) {
			console.error('Failed to load system backup data:', err);
			loadFailed = true;
			const msg = err instanceof Error ? err.message : 'Unknown backend error';
			toast.add(
				errorToastOptions('Load failed', `Disaster recovery backups could not be loaded: ${msg}`)
			);
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		void loadBackupData();
	});

	async function handleTriggerBackup() {
		isBackingUp = true;
		try {
			const created = await api.system.createBackup(selectedDestination || undefined);
			if (created.remoteStatus === 'synced') {
				toast.add({
					title: 'Instance backup completed',
					description: `Archive ${created.filename} created and mirrored to remote storage.`,
					variant: 'success'
				});
			} else {
				// HTTP 201 with a failed mirror is still a success condition for the
				// archive itself — but hiding the mirror failure would make the
				// off-host copy a silent assumption.
				toast.add({
					title: 'Local backup completed',
					description: `Archive ${created.filename} created, but the remote mirror copy failed: ${
						created.remoteError || 'unreachable destination'
					}`,
					variant: 'warning'
				});
			}
			await loadBackupData();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Instance backup failed';
			toast.add(errorToastOptions('Backup failed', msg));
		} finally {
			isBackingUp = false;
		}
	}

	async function handleSaveSchedule() {
		const parts = schedCron.trim().split(/\s+/);
		if (parts.length !== 5) {
			toast.add(
				errorToastOptions(
					'Invalid cron expression',
					'Standard cron requires 5 fields (minute hour day month weekday).'
				)
			);
			return;
		}
		const retention = clampRetention(schedRetention);
		if (Number(schedRetention) !== retention) {
			// A cleared or mistyped field must not fall back silently to the
			// server default; state the correction explicitly.
			toast.add(
				errorToastOptions(
					'Invalid retention',
					'Retention copies must be a number between 1 and 365.'
				)
			);
			return;
		}

		isSavingSchedule = true;
		try {
			const saved = await api.system.setBackupSchedule({
				cron: schedCron.trim(),
				retentionCount: retention,
				destinationId: schedDestination || undefined,
				enabled: schedEnabled
			});
			// Reflect the values the backend actually stored (it clamps), so the
			// form never shows stale or edited-out numbers after a save.
			schedCron = saved.cron;
			schedRetention = clampRetention(saved.retentionCount ?? retention);
			schedDestination = saved.destinationId ?? '';
			schedEnabled = saved.enabled;
			toast.add({
				title: 'Schedule updated',
				description: 'Automated disaster recovery backup policy saved.',
				variant: 'success'
			});
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to save schedule';
			toast.add(errorToastOptions('Save failed', msg));
		} finally {
			isSavingSchedule = false;
		}
	}

	function handleDownload(snap: VolumeSnapshot) {
		const url = api.volumes.getDownloadUrl(snap.id);
		const a = document.createElement('a');
		a.href = url;
		a.download = snap.filename;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
	}

	async function handleVerify(snap: VolumeSnapshot) {
		if (isVerifying) return;
		isVerifying = true;
		verifyTarget = snap;
		verifyResult = null;
		try {
			// A backend report with valid=false still renders: the modal is the
			// product of the request, not an exception condition.
			verifyResult = await api.system.verifyBackup(snap.id);
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Verification request failed';
			verifyTarget = null;
			toast.add(errorToastOptions('Verification failed', msg));
		} finally {
			isVerifying = false;
		}
	}

	async function confirmDelete() {
		if (!snapToDelete || isDeleting) return;
		isDeleting = true;
		try {
			await api.volumes.deleteSnapshot(snapToDelete);
			toast.add({
				title: 'Backup deleted',
				description: 'Disaster recovery archive removed.',
				variant: 'success'
			});
			await loadBackupData();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Delete failed';
			toast.add(errorToastOptions('Delete failed', msg));
		} finally {
			snapToDelete = null;
			isDeleting = false;
		}
	}
</script>

<div class="flex flex-col gap-6">
	<!-- Top Section: 2-Column Responsive Layout -->
	<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
		<!-- Manual Backup Trigger Card -->
		<LayerCard class="flex flex-col justify-between gap-5 p-5">
			<div class="flex flex-col gap-4">
				<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
					<div class="flex items-center gap-2">
						<Archive size={16} class="text-[var(--accent)]" />
						<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
							Instance disaster recovery
						</h3>
					</div>
					<Badge variant="success">Full state</Badge>
				</div>

				<p class="m-0 text-xs leading-relaxed text-[var(--text-secondary)]">
					Creates an atomic, consistent archive of GoPod's SQLite database, encryption master key,
					and system configurations.
				</p>

				<!-- Archive Manifest Breakdown -->
				<div
					class="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 text-xs"
				>
					<span class="text-[11px] font-medium text-[var(--text-tertiary)]"
						>Bundled archive assets</span
					>
					<div class="grid grid-cols-2 gap-2 text-[11px] text-[var(--text-secondary)]">
						<div class="flex items-center gap-1.5">
							<Database size={13} class="text-[var(--accent)]" />
							<span
								>SQLite database (<code class="font-mono text-[var(--text-primary)]">gopod.db</code
								>)</span
							>
						</div>
						<div class="flex items-center gap-1.5">
							<Key size={13} class="text-[var(--accent)]" />
							<span
								>Master key (<code class="font-mono text-[var(--text-primary)]">master.key</code
								>)</span
							>
						</div>
						<div class="flex items-center gap-1.5">
							<Sliders size={13} class="text-[var(--accent)]" />
							<span>Caddy & Quadlet configs</span>
						</div>
						<div class="flex items-center gap-1.5">
							<CheckCircle size={13} class="text-[var(--status-green)]" />
							<span
								>Disaster manifest (<code class="font-mono text-[var(--text-primary)]">json</code
								>)</span
							>
						</div>
					</div>
				</div>

				<Field
					label="Remote mirror destination"
					description="Optionally push this backup archive to S3"
				>
					<Select
						bind:value={selectedDestination}
						items={destinationItems}
						placeholder="Select remote destination..."
						disabled={isBackingUp}
					/>
				</Field>
			</div>

			<div class="pt-2">
				<Button
					variant="primary"
					size="base"
					class="w-full"
					disabled={isBackingUp}
					onclick={handleTriggerBackup}
				>
					{#if isBackingUp}
						<CircleNotch size={14} class="animate-spin" /> Packaging instance backup...
					{:else}
						<Archive size={14} /> Backup instance now
					{/if}
				</Button>
			</div>
		</LayerCard>

		<!-- Automated Backup Schedule Card -->
		<LayerCard class="flex flex-col justify-between gap-5 p-5">
			<div class="flex flex-col gap-4">
				<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
					<div class="flex items-center gap-2">
						<CalendarCheck size={16} class="text-[var(--accent)]" />
						<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
							Automated backup schedule
						</h3>
					</div>
					<div class="flex items-center gap-2">
						<span class="text-xs text-[var(--text-secondary)]">Schedule active:</span>
						<Switch
							bind:checked={schedEnabled}
							disabled={isSavingSchedule}
							aria-label="Schedule active"
						/>
					</div>
				</div>

				<p class="m-0 text-xs leading-relaxed text-[var(--text-secondary)]">
					Configure cron-based periodic backups with retention limits to prevent uncontrolled disk
					growth.
				</p>

				<div class="flex flex-col gap-3">
					<Field
						label="Cron expression"
						description="Standard 5-part cron syntax (min hr dom mon dow)"
					>
						<InputGroup class="w-full">
							<InputGroupInput
								bind:value={schedCron}
								class="font-mono text-xs"
								disabled={isSavingSchedule}
								placeholder="0 3 * * *"
							/>
						</InputGroup>
					</Field>

					<!-- Quick Presets -->
					<div class="flex flex-wrap items-center gap-1.5">
						{#each cronPresets as preset (preset.expr)}
							<button
								type="button"
								class="cursor-pointer rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2 py-1 text-[11px] text-[var(--text-secondary)] hover:border-[var(--accent)] hover:text-[var(--text-primary)]"
								onclick={() => (schedCron = preset.expr)}
								disabled={isSavingSchedule}
							>
								{preset.label}
							</button>
						{/each}
					</div>

					<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
						<!-- Native Kumo UI Primitive Increment -->
						<Field label="Retention copies" description="Max archives retained">
							<InputGroup class="w-full">
								<InputGroupAddon>
									<InputGroupButton
										icon={Minus}
										aria-label="Decrease retention"
										disabled={schedRetention <= 1 || isSavingSchedule}
										onclick={() => (schedRetention = Math.max(1, Number(schedRetention) - 1))}
									/>
								</InputGroupAddon>
								<InputGroupInput
									type="number"
									min="1"
									max="365"
									bind:value={schedRetention}
									onchange={() => (schedRetention = clampRetention(schedRetention))}
									class="text-center font-mono text-xs"
									disabled={isSavingSchedule}
								/>
								<InputGroupAddon align="end">
									<InputGroupButton
										icon={Plus}
										aria-label="Increase retention"
										disabled={schedRetention >= 365 || isSavingSchedule}
										onclick={() => (schedRetention = Math.min(365, Number(schedRetention) + 1))}
									/>
								</InputGroupAddon>
							</InputGroup>
						</Field>

						<Field label="Remote mirror" description="S3 bucket destination">
							<Select
								bind:value={schedDestination}
								items={destinationItems}
								disabled={isSavingSchedule}
							/>
						</Field>
					</div>
				</div>
			</div>

			<div class="pt-2">
				<Button
					variant="secondary"
					size="base"
					class="w-full"
					disabled={isSavingSchedule}
					onclick={handleSaveSchedule}
				>
					{#if isSavingSchedule}
						<CircleNotch size={14} class="animate-spin" /> Saving policy...
					{:else}
						Save schedule policy
					{/if}
				</Button>
			</div>
		</LayerCard>
	</div>

	<!-- Bottom Section: Backup History Table -->
	<LayerCard class="flex flex-col gap-0 overflow-hidden p-0">
		<div class="flex items-center justify-between border-b border-[var(--border-subtle)] px-5 py-4">
			<div class="flex items-center gap-2">
				<ClockCounterClockwise size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Disaster recovery archives
				</h3>
			</div>
			<Badge variant="neutral">{backups.length} snapshots</Badge>
		</div>

		{#if isLoading}
			<!-- Skeleton rows keep the card height stable while the list loads -->
			<div class="flex flex-col gap-2 p-5">
				<div class="h-9 animate-pulse rounded-[var(--radius-sm)] bg-[var(--bg-surface)]"></div>
				<div class="h-9 animate-pulse rounded-[var(--radius-sm)] bg-[var(--bg-surface)]"></div>
				<div class="h-9 animate-pulse rounded-[var(--radius-sm)] bg-[var(--bg-surface)]"></div>
			</div>
		{:else if loadFailed}
			<div class="p-8">
				<Empty
					size="sm"
					title="Backups could not be loaded"
					description="The backend did not respond while loading disaster recovery archives."
				>
					<Button variant="secondary" size="sm" onclick={loadBackupData}>
						<ArrowClockwise size={13} /> Retry
					</Button>
				</Empty>
			</div>
		{:else if backups.length === 0}
			<div class="p-8">
				<Empty
					size="sm"
					title="No disaster recovery archives"
					description="Generate a manual backup snapshot or configure the automated schedule to protect your instance state."
				>
					<Button
						variant="secondary"
						size="sm"
						onclick={handleTriggerBackup}
						disabled={isBackingUp}
					>
						{#if isBackingUp}
							<CircleNotch size={13} class="animate-spin" /> Packaging backup...
						{:else}
							<Archive size={13} /> Create first backup
						{/if}
					</Button>
				</Empty>
			</div>
		{:else}
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<Table class="w-full border-collapse text-left text-xs">
					<TableHeader
						class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
					>
						<TableRow>
							<TableHead class="px-4 py-3 text-[12px] font-semibold text-[var(--text-tertiary)]">
								Archive filename
							</TableHead>
							<TableHead class="px-4 py-3 text-[12px] font-semibold text-[var(--text-tertiary)]">
								Size
							</TableHead>
							<TableHead class="px-4 py-3 text-[12px] font-semibold text-[var(--text-tertiary)]">
								Storage target
							</TableHead>
							<TableHead class="px-4 py-3 text-[12px] font-semibold text-[var(--text-tertiary)]">
								Created
							</TableHead>
							<TableHead
								class="px-4 py-3 text-right text-[12px] font-semibold text-[var(--text-tertiary)]"
							>
								Actions
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody class="divide-y divide-[var(--border-subtle)]">
						{#each backups as snap (snap.id)}
							<TableRow class="hover:bg-[var(--bg-table-row-alt)]">
								<TableCell class="px-4 py-3 font-mono font-medium text-[var(--text-primary)]">
									<div class="flex items-center gap-2">
										<FileArchive size={15} class="shrink-0 text-[var(--accent)]" />
										<span class="truncate">{snap.filename}</span>
									</div>
								</TableCell>
								<TableCell class="px-4 py-3 font-mono text-[var(--text-secondary)]">
									{snap.size}
								</TableCell>
								<TableCell class="px-4 py-3">
									<span
										title={snap.remoteStatus === 'local-only'
											? `Mirror to remote failed: ${snap.remoteError || 'unreachable destination'}`
											: undefined}
									>
										{#if snap.remoteStatus === 'synced'}
											<Badge variant="success">
												<CloudCheck size={13} class="mr-1" /> Remote mirror
											</Badge>
										{:else if snap.remoteStatus === 'local-only'}
											<Badge variant="warning">
												<CloudWarning size={13} class="mr-1" /> Local only
											</Badge>
										{:else}
											<Badge variant="neutral">Local</Badge>
										{/if}
									</span>
								</TableCell>
								<TableCell class="px-4 py-3 text-[var(--text-tertiary)]">
									{formatTimeAgo(snap.createdAt)}
								</TableCell>
								<TableCell class="px-4 py-3 text-right">
									<div class="inline-flex items-center gap-1">
										<button
											type="button"
											onclick={() => handleVerify(snap)}
											disabled={isVerifying}
											class="flex h-7 w-7 cursor-pointer items-center justify-center rounded p-1.5 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] disabled:cursor-default disabled:opacity-50"
											title="Verify archive integrity"
											aria-label="Verify archive integrity"
										>
											{#if isVerifying && verifyTarget?.id === snap.id}
												<CircleNotch size={14} class="animate-spin" />
											{:else}
												<FileMagnifyingGlass size={14} />
											{/if}
										</button>
										<button
											type="button"
											onclick={() => handleDownload(snap)}
											class="flex h-7 w-7 cursor-pointer items-center justify-center rounded p-1.5 text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
											title="Download archive"
											aria-label="Download archive"
										>
											<DownloadSimple size={14} />
										</button>
										<button
											type="button"
											onclick={() => (snapToDelete = snap.id)}
											class="flex h-7 w-7 cursor-pointer items-center justify-center rounded p-1.5 text-[var(--status-red)] hover:text-[var(--status-red)]"
											title="Delete archive"
											aria-label="Delete archive"
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
</div>

<!-- Remote destination pool (shared with projects; Dokploy parity: managed at the settings level) -->
<div class="mt-5">
	<BackupDestinationsCard />
</div>

<!-- Confirm Dialog for Snapshot Deletion -->
<ConfirmDialog
	open={!!snapToDelete}
	title="Delete backup archive"
	description="Are you sure you want to permanently delete this instance disaster recovery archive?"
	confirmText="Delete archive"
	variant="danger"
	isConfirming={isDeleting}
	onconfirm={confirmDelete}
	oncancel={() => (snapToDelete = null)}
/>

<!-- Archive Integrity Verification Report -->
<Modal
	open={verifyTarget !== null || verifyResult !== null}
	onclose={() => {
		verifyTarget = null;
		verifyResult = null;
	}}
	title="Archive integrity report"
	subtitle={verifyTarget?.filename ?? ''}
	icon={FileMagnifyingGlass}
	size="md"
>
	{#if verifyResult}
		<div class="flex flex-col gap-4">
			<!-- Verdict banner -->
			<div
				class="flex items-center gap-2 rounded-[var(--radius-sm)] border p-3 text-xs font-medium {verifyResult.valid
					? 'border-[var(--status-green)] bg-[var(--status-green-muted)] text-[var(--status-green)]'
					: 'border-[var(--status-red)] bg-[var(--status-red-muted)] text-[var(--status-red)]'}"
			>
				{#if verifyResult.valid}
					<CheckCircle size={16} class="shrink-0 text-[var(--status-green)]" />
					<span>Archive passes all integrity checks and is safe to keep as a recovery copy.</span>
				{:else}
					<WarningCircle size={16} class="shrink-0 text-[var(--status-red)]" />
					<span>Archive failed verification — re-run the backup before relying on it.</span>
				{/if}
			</div>

			<!-- Detail board -->
			<div
				class="grid grid-cols-1 gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 text-xs sm:grid-cols-2"
			>
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Checked at</span>
					<span class="font-mono text-[var(--text-primary)]">
						{new Date(verifyResult.checkedAt).toLocaleString()}
					</span>
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Archive structure</span>
					<Badge variant={verifyResult.archiveReadable ? 'success' : 'error'}>
						{verifyResult.archiveReadable ? 'Valid gzip/tar stream' : 'Corrupt or unreadable'}
					</Badge>
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Packaged database</span>
					{#if verifyResult.databaseFound}
						<Badge variant={verifyResult.databaseIntegrity === 'ok' ? 'success' : 'error'}>
							SQLite {verifyResult.databaseIntegrity === 'ok' ? 'integrity: ok' : 'failed'}
						</Badge>
					{:else}
						<Badge variant="error">Not found in archive</Badge>
					{/if}
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Checksum vs mirror</span>
					<Badge
						variant={verifyResult.checksumMatch === 'match'
							? 'success'
							: verifyResult.checksumMatch === 'mismatch'
								? 'error'
								: 'neutral'}
					>
						{verifyResult.checksumMatch === 'match'
							? 'Match'
							: verifyResult.checksumMatch === 'mismatch'
								? 'Mismatch'
								: 'No reference stored'}
					</Badge>
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Manifest</span>
					{#if verifyResult.manifestFound}
						<span class="text-[var(--text-primary)]">
							Version {verifyResult.manifestVersion ?? 1}
							{#if verifyResult.manifestCreatedAt}
								· {new Date(verifyResult.manifestCreatedAt).toLocaleDateString()}
							{/if}
						</span>
					{:else}
						<Badge variant="warning">Missing</Badge>
					{/if}
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Encryption key</span>
					{#if verifyResult.keyBundleFound}
						<Badge variant="success">Bundled in archive</Badge>
					{:else if verifyResult.manifestKeyFromEnv}
						<Badge variant="neutral">Env key (GOPOD_ENCRYPTION_KEY)</Badge>
					{:else}
						<Badge variant="error">Missing</Badge>
					{/if}
				</div>
				<div class="flex flex-col gap-0.5 sm:col-span-2">
					<span class="text-[var(--text-tertiary)]">Archive contents</span>
					<span class="text-[var(--text-primary)]">
						{verifyResult.entries?.length ?? 0} entries
						{#if verifyResult.entries?.length}
							<span class="text-[var(--text-tertiary)]">
								— {verifyResult.entries.slice(0, 12).join(', ')}{verifyResult.entries.length > 12
									? `, +${verifyResult.entries.length - 12} more`
									: ''}
							</span>
						{/if}
					</span>
				</div>
			</div>

			{#if verifyResult.errors?.length}
				<div
					class="flex flex-col gap-1 rounded-[var(--radius-sm)] border border-[var(--status-red)] bg-[var(--status-red-muted)] p-3 text-xs text-[var(--status-red)]"
				>
					<span class="font-semibold">Problems</span>
					{#each verifyResult.errors as problem (problem)}
						<span>{problem}</span>
					{/each}
				</div>
			{/if}

			{#if verifyResult.warnings?.length}
				<div
					class="flex flex-col gap-1 rounded-[var(--radius-sm)] border border-[var(--status-amber)] bg-[var(--status-amber-muted)] p-3 text-xs text-[var(--status-amber)]"
				>
					<span class="font-semibold">Notes</span>
					{#each verifyResult.warnings as note (note)}
						<span>{note}</span>
					{/each}
				</div>
			{/if}

			<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
				Verification is a read-only inspection. Restoring this archive happens offline on the host:
				stop GoPod, extract the tar.gz over the data directory, then start the service again.
			</p>
		</div>
	{:else}
		<div class="flex items-center justify-center gap-2 p-8 text-xs text-[var(--text-tertiary)]">
			<CircleNotch size={14} class="animate-spin" /> Inspecting archive...
		</div>
	{/if}
	{#snippet footer()}
		<Button
			variant="secondary"
			size="sm"
			onclick={() => {
				verifyTarget = null;
				verifyResult = null;
			}}
		>
			Close
		</Button>
	{/snippet}
</Modal>
