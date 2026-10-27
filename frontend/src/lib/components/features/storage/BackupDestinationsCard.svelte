<script lang="ts">
	// Self-contained management card for remote S3-compatible backup destinations.
	// Shared by Settings → Backups and every project's Volumes & Backups view;
	// both render the same global destination pool (one shared API + store).
	import {
		Button,
		Field,
		Input,
		Checkbox,
		SensitiveInput,
		Modal,
		LayerCard,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell
	} from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { api } from '$lib/api';
	import { dataStore } from '$lib/data';
	import { onMount } from 'svelte';
	import {
		CloudArrowUp,
		CloudCheck,
		ArrowClockwise,
		Trash,
		CircleNotch,
		ShieldCheck,
		Warning
	} from 'phosphor-svelte';

	const toast = useKumoToastManager();

	// Destination form state. The "test before save" flow probes the bucket
	// first so a typed credential is verified before it ever gets stored.
	let showDestinationForm = $state(false);
	let isCreatingDestination = $state(false);
	let isTestingDestination = $state(false);
	let destinationNameInput = $state('');
	let destinationEndpoint = $state('');
	let destinationRegion = $state('');
	let destinationAccessKey = $state('');
	let destinationSecretKey = $state('');
	let destinationBucket = $state('');
	let destinationPrefix = $state('');
	let destinationPathStyle = $state(true);
	let destinationError = $state('');

	onMount(() => {
		dataStore.fetchBackupDestinations().catch(() => null);
	});

	function destinationName(id: string): string {
		return dataStore.backupDestinations.find((d) => d.id === id)?.name ?? id;
	}

	function openDestinationForm() {
		destinationNameInput = '';
		destinationEndpoint = '';
		destinationRegion = '';
		destinationAccessKey = '';
		destinationSecretKey = '';
		destinationBucket = '';
		destinationPrefix = '';
		destinationPathStyle = true;
		destinationError = '';
		showDestinationForm = true;
	}

	function closeDestinationForm() {
		showDestinationForm = false;
		destinationError = '';
	}

	async function handleTestDestination() {
		destinationError = '';
		if (!destinationNameInput || !destinationEndpoint || !destinationBucket) {
			destinationError = 'Name, endpoint and bucket are required to test.';
			return;
		}
		isTestingDestination = true;
		try {
			await api.volumes.testDestination({
				name: destinationNameInput.trim(),
				endpoint: destinationEndpoint.trim(),
				region: destinationRegion.trim(),
				accessKey: destinationAccessKey.trim(),
				secretKey: destinationSecretKey,
				bucket: destinationBucket.trim(),
				prefix: destinationPrefix.trim(),
				pathStyle: destinationPathStyle
			});
			toast.add({
				title: 'Destination reachable',
				description: `${destinationBucket} responded correctly to the probe.`,
				variant: 'success'
			});
		} catch (err: unknown) {
			console.error('Destination test failed:', err);
			const message = err instanceof Error ? err.message : 'Unknown error';
			toast.add(errorToastOptions('Destination unreachable', message));
		} finally {
			isTestingDestination = false;
		}
	}

	async function handleCreateDestination() {
		if (!destinationNameInput || !destinationEndpoint || !destinationBucket) {
			destinationError = 'Name, endpoint and bucket are required.';
			return;
		}
		if (!destinationAccessKey || !destinationSecretKey) {
			destinationError = 'Access key and secret key are required.';
			return;
		}
		isCreatingDestination = true;
		destinationError = '';
		try {
			await dataStore.createBackupDestination({
				name: destinationNameInput.trim(),
				endpoint: destinationEndpoint.trim(),
				region: destinationRegion.trim(),
				accessKey: destinationAccessKey.trim(),
				secretKey: destinationSecretKey,
				bucket: destinationBucket.trim(),
				prefix: destinationPrefix.trim(),
				pathStyle: destinationPathStyle
			});
			toast.add({
				title: 'Destination created',
				description: 'Snapshots can now mirror to this object store.',
				variant: 'success'
			});
			closeDestinationForm();
		} catch (err: unknown) {
			console.error('Failed to create destination:', err);
			const message = err instanceof Error ? err.message : 'Unknown error';
			destinationError = message;
			toast.add(errorToastOptions('Destination creation failed', message));
		} finally {
			isCreatingDestination = false;
		}
	}

	async function handleDeleteDestination(id: string) {
		if (
			!confirm(
				`Delete destination "${destinationName(id)}"? Schedules using it keep producing local-only snapshots.`
			)
		) {
			return;
		}
		try {
			await dataStore.deleteBackupDestination(id);
			toast.add({
				title: 'Destination deleted',
				description: 'Schedules referencing it now stay local-only.',
				variant: 'success'
			});
		} catch (err: unknown) {
			console.error('Failed to delete destination:', err);
			toast.add(
				errorToastOptions(
					'Destination deletion failed',
					'Could not delete the destination — check server logs.'
				)
			);
		}
	}

	async function handleTestSavedDestination(id: string) {
		try {
			await api.volumes.testSavedDestination(id);
			toast.add({
				title: 'Destination reachable',
				description: 'The bucket probe succeeded.',
				variant: 'success'
			});
		} catch (err: unknown) {
			console.error('Destination test failed:', err);
			toast.add(
				errorToastOptions(
					'Destination unreachable',
					err instanceof Error ? err.message : 'Check credentials and endpoint.'
				)
			);
		}
	}
</script>

{#snippet destinationFooter()}
	{#if isTestingDestination || isCreatingDestination}
		<span class="flex items-center gap-2 text-xs text-[var(--text-tertiary)]">
			<CircleNotch size={13} class="animate-spin" />
			{isTestingDestination ? 'Testing connection...' : 'Saving destination...'}
		</span>
	{:else}
		<span></span>
	{/if}
	<div class="flex items-center gap-2">
		<Button
			type="button"
			variant="secondary"
			size="sm"
			disabled={isTestingDestination || isCreatingDestination}
			onclick={handleTestDestination}
		>
			<ShieldCheck size={13} /> Test connection
		</Button>
		<Button variant="secondary" size="sm" onclick={closeDestinationForm}>Cancel</Button>
		<Button
			variant="primary"
			size="sm"
			disabled={isCreatingDestination || isTestingDestination}
			onclick={handleCreateDestination}
		>
			Create destination
		</Button>
	</div>
{/snippet}

<section class="flex flex-col gap-3">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-2">
			<CloudArrowUp size={16} class="text-[var(--accent)]" />
			<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
				Remote backup destinations ({dataStore.backupDestinations.length})
			</h3>
		</div>
		<Button variant="secondary" size="sm" onclick={openDestinationForm}>
			<CloudArrowUp size={12} />
			<span>New destination</span>
		</Button>
	</div>

	<LayerCard class="p-0">
		{#if dataStore.backupDestinations.length === 0}
			<div class="flex flex-col items-center gap-1.5 px-4 py-8 text-center">
				<CloudArrowUp size={22} class="text-[var(--text-tertiary)]" />
				<span class="text-xs font-medium text-[var(--text-secondary)]">
					No remote destinations configured
				</span>
				<span class="max-w-md text-[11px] leading-relaxed text-[var(--text-tertiary)]">
					Add any S3-compatible object store (AWS S3, Cloudflare R2, MinIO, Wasabi, B2...) to mirror
					snapshots off this host. Local snapshots always stay even if the remote fails.
				</span>
			</div>
		{:else}
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<Table class="w-full border-collapse text-left text-xs">
					<TableHeader
						class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
					>
						<TableRow>
							<TableHead
								class="px-3.5 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Name</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Endpoint</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Bucket</TableHead
							>
							<TableHead
								class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Prefix</TableHead
							>
							<TableHead
								class="px-3.5 py-2.5 text-right text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
								>Actions</TableHead
							>
						</TableRow>
					</TableHeader>
					<TableBody class="divide-y divide-[var(--border-subtle)]">
						{#each dataStore.backupDestinations as dest (dest.id)}
							<TableRow class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
								<TableCell class="px-3.5 py-3 whitespace-nowrap">
									<div class="flex items-center gap-2">
										<CloudCheck size={15} class="text-[var(--accent)]" />
										<span class="text-xs font-medium text-[var(--text-primary)]">
											{dest.name}
										</span>
									</div>
								</TableCell>
								<TableCell
									class="px-3 py-3 font-mono text-[11px] whitespace-nowrap text-[var(--text-secondary)]"
								>
									{dest.endpoint}
								</TableCell>
								<TableCell
									class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
								>
									{dest.bucket}
									{#if !dest.pathStyle}
										<span class="ml-1 text-[10px] text-[var(--text-tertiary)]">virtual-host</span>
									{/if}
								</TableCell>
								<TableCell
									class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-tertiary)]"
								>
									{dest.prefix || '—'}
								</TableCell>
								<TableCell class="px-3.5 py-3 text-right whitespace-nowrap">
									<div class="inline-flex items-center gap-1.5">
										<button
											type="button"
											onclick={() => handleTestSavedDestination(dest.id)}
											class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
											title="Test connection"
										>
											<ArrowClockwise size={14} />
										</button>
										<button
											type="button"
											onclick={() => handleDeleteDestination(dest.id)}
											class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--status-red-muted)] hover:text-[var(--status-red)]"
											title="Delete destination"
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

<!-- Modal: Create Backup Destination (S3-compatible) -->
<Modal
	open={showDestinationForm}
	onclose={closeDestinationForm}
	title="New backup destination"
	subtitle="Any S3-compatible object store: AWS S3, Cloudflare R2, MinIO, Wasabi, Backblaze B2..."
	icon={CloudArrowUp}
	size="sm"
	footer={destinationFooter}
>
	<div class="flex flex-col gap-4">
		<Field label="Name">
			<Input
				id="dest-name"
				bind:value={destinationNameInput}
				placeholder="e.g. R2-main, MinIO-lab"
				disabled={isCreatingDestination}
			/>
		</Field>

		<Field label="Endpoint URL">
			<Input
				id="dest-endpoint"
				bind:value={destinationEndpoint}
				placeholder="https://s3.eu.example.com"
				class="w-full font-mono text-xs"
				disabled={isCreatingDestination}
			/>
		</Field>

		<div class="grid grid-cols-2 gap-3">
			<Field label="Region" description="Defaults to us-east-1.">
				<Input
					id="dest-region"
					bind:value={destinationRegion}
					placeholder="us-east-1"
					class="font-mono text-xs"
					disabled={isCreatingDestination}
				/>
			</Field>
			<Field label="Bucket">
				<Input
					id="dest-bucket"
					bind:value={destinationBucket}
					placeholder="gopod-backups"
					class="font-mono text-xs"
					disabled={isCreatingDestination}
				/>
			</Field>
		</div>

		<div class="grid grid-cols-2 gap-3">
			<Field label="Access key">
				<Input
					id="dest-access-key"
					bind:value={destinationAccessKey}
					placeholder="AKIA..."
					class="font-mono text-xs"
					disabled={isCreatingDestination}
				/>
			</Field>
			<Field label="Prefix (optional)" description="Folder inside the bucket.">
				<Input
					id="dest-prefix"
					bind:value={destinationPrefix}
					placeholder="prod/project-x"
					class="font-mono text-xs"
					disabled={isCreatingDestination}
				/>
			</Field>
		</div>

		<Field label="Secret key">
			<SensitiveInput
				id="dest-secret-key"
				bind:value={destinationSecretKey}
				size="sm"
				disabled={isCreatingDestination}
			/>
		</Field>

		<div class="flex flex-col gap-1">
			<Checkbox
				label="Path-style addressing"
				bind:checked={destinationPathStyle}
				disabled={isCreatingDestination}
			/>
			<span class="pl-6 text-[11px] leading-relaxed text-[var(--text-tertiary)]">
				Keep enabled for R2, MinIO, Wasabi and most non-AWS providers (requests go to
				<span class="font-mono">host/bucket/object</span> instead of virtual-host DNS).
			</span>
		</div>

		{#if destinationError}
			<div
				class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--status-red)]/30 bg-[var(--status-red-muted)] p-2.5 text-[11px] font-medium text-[var(--status-red)]"
			>
				<Warning size={13} />
				{destinationError}
			</div>
		{/if}
	</div>
</Modal>
