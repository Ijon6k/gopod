<script lang="ts">
	import type { VolumeSnapshot } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { Button, Field, Input } from '$lib/components/ui';
	import { Warning, CircleNotch, Database, CloudCheck } from 'phosphor-svelte';

	interface Props {
		snapshot: VolumeSnapshot | null;
		onclose: () => void;
		onrestored?: (msg: string) => void;
	}

	let { snapshot, onclose, onrestored }: Props = $props();

	const toast = useKumoToastManager();

	let isRestoring = $state(false);
	let dbDatabase = $state('');
	let dbUsername = $state('');
	let dbPassword = $state('');
	let confirmText = $state('');

	let isDbSnapshot = $derived(Boolean(snapshot?.volumeName?.startsWith('db:')));
	let isRemoteOnlyHint = $derived(snapshot?.remoteStatus === 'synced');
	let canConfirm = $derived(!isDbSnapshot || confirmText.trim().toUpperCase() === 'RESTORE');

	$effect(() => {
		// Reset form whenever modal opens for a different snapshot
		void snapshot?.id;
		dbDatabase = '';
		dbUsername = '';
		dbPassword = '';
		confirmText = '';
	});

	async function handleConfirmRestore() {
		if (!snapshot || isRestoring || !canConfirm) return;
		isRestoring = true;
		try {
			const opts = isDbSnapshot
				? {
						...(snapshot.serviceId ? { serviceId: snapshot.serviceId } : {}),
						...(dbDatabase.trim() ? { database: dbDatabase.trim() } : {}),
						...(dbUsername.trim() ? { username: dbUsername.trim() } : {}),
						...(dbPassword ? { password: dbPassword } : {})
					}
				: undefined;
			await dataStore.restoreVolumeSnapshot(snapshot.id, opts);
			const msg = isDbSnapshot
				? `Database restored from "${snapshot.filename}" (${snapshot.volumeName}).`
				: `Snapshot restored successfully for "${snapshot.volumeName}".`;
			toast.add({
				title: 'Restore completed',
				description: msg,
				variant: 'default'
			});
			onrestored?.(msg);
			onclose();
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Restore failed'));
		} finally {
			isRestoring = false;
		}
	}
</script>

{#if snapshot}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(5,6,7,0.82)] p-4 backdrop-blur-[3px]"
		role="dialog"
		aria-modal="true"
		aria-labelledby="restore-modal-title"
	>
		<div
			class="animate-in fade-in zoom-in-95 flex w-full max-w-md flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-5 text-left shadow-2xl duration-150"
		>
			<div class="flex items-start gap-3">
				<div
					class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--status-amber-muted)] text-[var(--status-amber)]"
				>
					{#if isDbSnapshot}
						<Database size={18} />
					{:else}
						<Warning size={18} />
					{/if}
				</div>
				<div class="flex flex-col gap-1">
					<h3 id="restore-modal-title" class="m-0 text-sm font-semibold text-[var(--text-primary)]">
						{isDbSnapshot ? 'Restore Database Backup' : 'Restore Volume Snapshot'}
					</h3>
					<span class="text-xs leading-relaxed text-[var(--text-tertiary)]">
						Restoring <code class="font-mono text-[var(--text-primary)]">{snapshot.filename}</code>
						will overwrite existing data in
						<strong class="text-[var(--text-primary)]">{snapshot.volumeName}</strong>.
					</span>
				</div>
			</div>

			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 text-[11px] leading-relaxed text-[var(--text-secondary)]"
			>
				{#if isDbSnapshot}
					The dump will stream directly into the running database container using configured service
					environment credentials (or the overrides below).
				{:else}
					Target containers mounting this volume will be briefly quiesced to ensure file integrity
					during import.
				{/if}
				{#if isRemoteOnlyHint}
					<div
						class="mt-2 flex items-center gap-1.5 font-mono text-[10.5px] text-[var(--status-green)]"
					>
						<CloudCheck size={13} />
						<span
							>Mirrored in S3 — if missing locally, GOPOD fetches &amp; verifies SHA-256
							automatically.</span
						>
					</div>
				{/if}
			</div>

			{#if isDbSnapshot}
				<div class="flex flex-col gap-2.5">
					<div class="grid grid-cols-2 gap-2.5">
						<Field label="Target database (optional)">
							<Input
								bind:value={dbDatabase}
								placeholder="From service env"
								class="font-mono text-xs"
							/>
						</Field>
						<Field label="Username (optional)">
							<Input
								bind:value={dbUsername}
								placeholder="From service env"
								class="font-mono text-xs"
							/>
						</Field>
					</div>
					<Field label="Password override (optional)">
						<Input
							type="password"
							bind:value={dbPassword}
							placeholder="Leave blank to use service env"
							class="font-mono text-xs"
						/>
					</Field>
					<Field label="Type RESTORE to confirm destructive overwrite">
						<Input bind:value={confirmText} placeholder="RESTORE" class="font-mono text-xs" />
					</Field>
				</div>
			{/if}

			<div class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-2">
				<Button variant="secondary" size="sm" onclick={onclose} disabled={isRestoring}>
					Cancel
				</Button>
				<Button
					variant="primary"
					size="sm"
					disabled={isRestoring || !canConfirm}
					onclick={handleConfirmRestore}
				>
					{#if isRestoring}
						<CircleNotch size={13} class="animate-spin" /> Restoring...
					{:else}
						Confirm &amp; Restore
					{/if}
				</Button>
			</div>
		</div>
	</div>
{/if}
