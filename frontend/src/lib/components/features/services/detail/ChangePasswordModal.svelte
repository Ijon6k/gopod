<script lang="ts">
	import { Modal, Input, CopyButton, DatabaseIcon } from '$lib/components/ui';
	import { Button } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import type { Service } from '$lib/types';
	import {
		extractDatabaseCredentials,
		updateServiceWithDatabaseConfig,
		generateDatabasePassword,
		validateDatabasePassword,
		DATABASE_ENGINES
	} from '../database/database-utils';
	import {
		Key,
		ArrowsClockwise,
		Eye,
		EyeSlash,
		WarningCircle,
		RocketLaunch,
		Check
	} from 'phosphor-svelte';

	interface Props {
		open: boolean;
		service: Service;
		onclose: () => void;
		onupdated?: (updated: Service) => void;
		onredeploy?: () => void;
	}

	let { open = $bindable(false), service, onclose, onupdated, onredeploy }: Props = $props();

	let newPassword = $state('');
	let showPassword = $state(false);
	let isSaving = $state(false);
	let saveError = $state<string | null>(null);

	const creds = $derived(extractDatabaseCredentials(service));
	const currentEngine = $derived(
		DATABASE_ENGINES.find((e) => e.id === creds.engine) ?? DATABASE_ENGINES[0]
	);

	$effect(() => {
		if (open) {
			newPassword = creds.password || generateDatabasePassword();
			saveError = null;
		}
	});

	let validation = $derived(validateDatabasePassword(newPassword));

	function handleRegenerate() {
		newPassword = generateDatabasePassword();
	}

	async function handleSave(triggerRedeploy = false) {
		if (!validation.valid || isSaving) return;
		isSaving = true;
		saveError = null;

		try {
			const updatedConfig = {
				engine: creds.engine,
				name: service.name,
				description: service.description || '',
				image: creds.image,
				databaseName: creds.databaseName,
				user: creds.user,
				password: newPassword,
				rootPassword: creds.rootPassword,
				internalPort: creds.port,
				hostPort: creds.hostPort,
				volumeName: creds.volumeName,
				volumeMountPath: creds.volumeMountPath,
				envVars: []
			};

			const updatedService = updateServiceWithDatabaseConfig(service, updatedConfig);
			await dataStore.updateService(updatedService);

			if (triggerRedeploy) {
				if (onredeploy) {
					onredeploy();
				} else {
					await dataStore.deployService(service.id, 'manual');
				}
			}

			onupdated?.(updatedService);
			open = false;
			onclose();
		} catch (err: any) {
			saveError = err?.message || 'Failed to update database password';
		} finally {
			isSaving = false;
		}
	}
</script>

<Modal
	{open}
	{onclose}
	title="Change Database Password"
	subtitle="Update the credentials for {currentEngine.name} instance ({service.name})"
	size="md"
>
	{#snippet headerActions()}
		<DatabaseIcon engine={creds.engine} size={20} />
	{/snippet}

	<div class="flex flex-col gap-4">
		{#if saveError}
			<div
				class="flex items-center gap-2 rounded-md border border-[var(--status-red)]/30 bg-[var(--status-red)]/10 p-3 text-xs text-[var(--status-red)]"
			>
				<WarningCircle size={16} />
				<span>{saveError}</span>
			</div>
		{/if}

		<div class="flex flex-col gap-1.5">
			<div class="flex items-center justify-between">
				<label for="change-db-pass" class="text-xs font-medium text-[var(--text-secondary)]">
					New Database Password <span class="text-[var(--status-red)]">*</span>
				</label>
				<button
					type="button"
					onclick={handleRegenerate}
					class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline"
				>
					<ArrowsClockwise size={12} /> Regenerate Strong Password
				</button>
			</div>

			<div class="relative flex items-center">
				<Input
					id="change-db-pass"
					type={showPassword ? 'text' : 'password'}
					bind:value={newPassword}
					placeholder="Enter secure password"
					class="pr-16 text-xs font-mono"
				/>
				<div class="absolute right-1.5 flex items-center gap-1">
					<button
						type="button"
						onclick={() => (showPassword = !showPassword)}
						class="cursor-pointer border-0 bg-transparent p-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
						title={showPassword ? 'Hide password' : 'Show password'}
					>
						{#if showPassword}
							<EyeSlash size={14} />
						{:else}
							<Eye size={14} />
						{/if}
					</button>
					<CopyButton text={newPassword} />
				</div>
			</div>

			{#if !validation.valid}
				<span class="flex items-center gap-1 text-[11px] text-[var(--status-red)]">
					<WarningCircle size={12} /> {validation.error}
				</span>
			{/if}
		</div>

		<!-- Dokploy Parity Warning Info Box -->
		<div
			class="flex flex-col gap-1 rounded-md border border-[var(--status-amber)]/30 bg-[var(--status-amber)]/10 p-3 text-xs text-[var(--text-secondary)]"
		>
			<span class="font-semibold text-[var(--status-amber)]">Redeployment Notice</span>
			<p class="m-0 text-[11px] leading-relaxed text-[var(--text-tertiary)]">
				Updating the password rewrites the container environment variables. For existing running containers,
				you must redeploy or restart the service to apply the new credentials to the database process.
			</p>
		</div>
	</div>

	{#snippet footer()}
		<Button variant="secondary" size="sm" onclick={onclose} disabled={isSaving}>
			Cancel
		</Button>
		<Button
			variant="secondary"
			size="sm"
			onclick={() => handleSave(false)}
			disabled={!validation.valid || isSaving}
		>
			<Check size={14} class="mr-1" />
			Save Changes
		</Button>
		<Button
			variant="primary"
			size="sm"
			onclick={() => handleSave(true)}
			disabled={!validation.valid || isSaving}
		>
			<RocketLaunch size={14} class="mr-1" />
			Save & Redeploy
		</Button>
	{/snippet}
</Modal>
