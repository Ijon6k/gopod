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
		DATABASE_ENGINES,
		type DatabaseConfig,
		type DatabaseEngine,
		buildConnectionString
	} from '../database/database-utils';
	import {
		PencilSimple,
		ShieldCheck,
		ArrowsClockwise,
		Eye,
		EyeSlash,
		Tag,
		HardDrive,
		LinkSimple,
		WarningCircle,
		RocketLaunch,
		Check,
		SlidersHorizontal
	} from 'phosphor-svelte';

	interface Props {
		open: boolean;
		service: Service;
		onclose: () => void;
		onupdated?: (updated: Service) => void;
		onredeploy?: () => void;
	}

	let { open = $bindable(false), service, onclose, onupdated, onredeploy }: Props = $props();

	const creds = $derived(extractDatabaseCredentials(service));
	const currentEngine = $derived(
		DATABASE_ENGINES.find((e) => e.id === creds.engine) ?? DATABASE_ENGINES[0]
	);

	// Editable state
	let serviceName = $state('');
	let serviceDescription = $state('');
	let activeImage = $state('');
	let isCustomImage = $state(false);

	let dbName = $state('');
	let dbUser = $state('');
	let dbPassword = $state('');
	let dbRootPassword = $state('');
	let showPassword = $state(false);
	let showRootPassword = $state(false);

	let internalPortInput = $state('5432');
	let hostPortInput = $state('');

	let volumeNameInput = $state('');
	let storagePathInput = $state('');
	let showAdvancedStorage = $state(false);

	let isSaving = $state(false);
	let saveError = $state<string | null>(null);

	$effect(() => {
		if (open) {
			const c = extractDatabaseCredentials(service);
			serviceName = service.name;
			serviceDescription = service.description || '';
			activeImage = c.image;
			isCustomImage = !currentEngine.popularTags.includes(c.image);

			dbName = c.databaseName;
			dbUser = c.user;
			dbPassword = c.password;
			dbRootPassword = c.rootPassword || '';

			internalPortInput = String(c.port);
			hostPortInput = c.hostPort ? String(c.hostPort) : '';

			volumeNameInput = c.volumeName;
			storagePathInput = c.volumeMountPath;
			saveError = null;
		}
	});

	let passwordValidation = $derived(validateDatabasePassword(dbPassword));
	let isFormValid = $derived(
		serviceName.trim().length > 0 &&
		activeImage.trim().length > 0 &&
		passwordValidation.valid
	);

	let parsedInternalPort = $derived.by(() => {
		const p = parseInt(internalPortInput.trim(), 10);
		return !isNaN(p) && p > 0 && p <= 65535 ? p : creds.port;
	});

	let parsedHostPort = $derived.by(() => {
		const p = parseInt(hostPortInput.trim(), 10);
		return !isNaN(p) && p > 0 && p <= 65535 ? p : undefined;
	});

	// Live Connection String Preview
	let liveConnUri = $derived(
		buildConnectionString({
			engine: creds.engine,
			user: currentEngine.hasUser ? dbUser : undefined,
			password: dbPassword,
			host: service.id || service.name,
			port: parsedInternalPort,
			databaseName: currentEngine.hasDbName ? dbName : undefined
		})
	);

	let maskedConnUri = $derived.by(() => {
		if (showPassword || !dbPassword) return liveConnUri;
		return liveConnUri.replace(dbPassword, '••••••••••••');
	});

	function handleRegeneratePassword() {
		dbPassword = generateDatabasePassword();
	}

	function handleRegenerateRootPassword() {
		dbRootPassword = generateDatabasePassword();
	}

	async function handleSave(triggerRedeploy = false) {
		if (!isFormValid || isSaving) return;
		isSaving = true;
		saveError = null;

		try {
			const updatedConfig: DatabaseConfig = {
				engine: creds.engine,
				name: serviceName.trim(),
				description: serviceDescription.trim(),
				image: activeImage.trim(),
				databaseName: currentEngine.hasDbName ? dbName.trim() : '',
				user: currentEngine.hasUser ? dbUser.trim() : '',
				password: dbPassword,
				rootPassword: currentEngine.hasRootPassword ? dbRootPassword : undefined,
				internalPort: parsedInternalPort,
				hostPort: parsedHostPort,
				volumeName: volumeNameInput.trim() || creds.volumeName,
				volumeMountPath: storagePathInput.trim() || creds.volumeMountPath,
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
			saveError = err?.message || 'Failed to update database service';
		} finally {
			isSaving = false;
		}
	}
</script>

<Modal
	{open}
	{onclose}
	title="Edit Database Configuration"
	subtitle="Customize service identity, credentials, Docker image, and networking ports"
	size="xl"
>
	{#snippet headerActions()}
		<div class="flex items-center gap-2">
			<DatabaseIcon engine={creds.engine} size={22} />
			<span class="rounded bg-kumo-elevated px-2 py-0.5 font-mono text-xs text-kumo-default ring-1 ring-kumo-hairline">
				{currentEngine.name}
			</span>
		</div>
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

		<!-- 1. Service Name & Description -->
		<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
			<div class="flex flex-col gap-1.5">
				<label for="edit-db-name" class="text-xs font-medium text-[var(--text-secondary)]">
					Service Name <span class="text-[var(--status-red)]">*</span>
				</label>
				<Input
					id="edit-db-name"
					bind:value={serviceName}
					placeholder="e.g. postgres-db"
					class="text-xs font-mono"
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="edit-db-desc" class="text-xs font-medium text-[var(--text-secondary)]">
					Description (Optional)
				</label>
				<Input
					id="edit-db-desc"
					bind:value={serviceDescription}
					placeholder="{currentEngine.name} instance"
					class="text-xs"
				/>
			</div>
		</div>

		<!-- 2. Docker Image & Version Upgrades -->
		<div class="flex flex-col gap-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)]/60 p-3.5">
			<div class="flex items-center justify-between border-b border-[var(--border)] pb-2">
				<div class="flex items-center gap-2">
					<Tag size={15} class="text-[var(--accent)]" />
					<span class="text-xs font-semibold text-[var(--text-primary)]">Docker Image & Version</span>
				</div>
				<button
					type="button"
					onclick={() => (isCustomImage = !isCustomImage)}
					class="cursor-pointer text-[11px] font-medium text-[var(--accent)] hover:underline"
				>
					{isCustomImage ? 'Select Popular Tag' : 'Edit Custom Image'}
				</button>
			</div>

			{#if !isCustomImage && currentEngine.popularTags.length > 0}
				<div class="flex flex-wrap gap-2 pt-1">
					{#each currentEngine.popularTags as tag}
						<button
							type="button"
							onclick={() => (activeImage = tag)}
							class="cursor-pointer rounded border px-2 py-0.5 font-mono text-xs transition-colors {activeImage === tag
								? 'border-[var(--accent)] bg-[var(--accent)]/15 font-semibold text-[var(--accent)] ring-1 ring-[var(--accent)]/30'
								: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:border-[var(--border-hover)]'}"
						>
							{tag}
						</button>
					{/each}
				</div>
			{/if}

			<div class="flex flex-col gap-1 pt-1">
				<label for="edit-image-val" class="text-xs font-medium text-[var(--text-secondary)]">
					Active Image Tag
				</label>
				<Input
					id="edit-image-val"
					bind:value={activeImage}
					placeholder={currentEngine.defaultImage}
					class="text-xs font-mono"
				/>
			</div>
		</div>

		<!-- 3. Engine Credentials -->
		<div class="flex flex-col gap-3 rounded-md border border-[var(--border)] bg-[var(--bg-surface)]/60 p-3.5">
			<div class="flex items-center gap-2 border-b border-[var(--border)] pb-2">
				<ShieldCheck size={15} class="text-[var(--accent)]" />
				<span class="text-xs font-semibold text-[var(--text-primary)]">Database Credentials</span>
			</div>

			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				{#if currentEngine.hasDbName}
					<div class="flex flex-col gap-1.5">
						<label for="edit-db-database-name" class="text-xs font-medium text-[var(--text-secondary)]">
							Database Name
						</label>
						<Input
							id="edit-db-database-name"
							bind:value={dbName}
							placeholder="e.g. gopod"
							class="text-xs font-mono"
						/>
					</div>
				{/if}

				{#if currentEngine.hasUser}
					<div class="flex flex-col gap-1.5">
						<label for="edit-db-user" class="text-xs font-medium text-[var(--text-secondary)]">
							Username
						</label>
						<Input
							id="edit-db-user"
							bind:value={dbUser}
							placeholder={currentEngine.defaultUser}
							class="text-xs font-mono"
						/>
					</div>
				{/if}

				<!-- Password Field -->
				<div class="flex flex-col gap-1.5 sm:col-span-2">
					<div class="flex items-center justify-between">
						<label for="edit-db-pass" class="text-xs font-medium text-[var(--text-secondary)]">
							Password <span class="text-[var(--status-red)]">*</span>
						</label>
						<button
							type="button"
							onclick={handleRegeneratePassword}
							class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline"
						>
							<ArrowsClockwise size={12} /> Regenerate Password
						</button>
					</div>

					<div class="relative flex items-center">
						<Input
							id="edit-db-pass"
							type={showPassword ? 'text' : 'password'}
							bind:value={dbPassword}
							class="pr-16 text-xs font-mono"
						/>
						<div class="absolute right-1.5 flex items-center gap-1">
							<button
								type="button"
								onclick={() => (showPassword = !showPassword)}
								class="cursor-pointer border-0 bg-transparent p-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
							>
								{#if showPassword}
									<EyeSlash size={14} />
								{:else}
									<Eye size={14} />
								{/if}
							</button>
							<CopyButton text={dbPassword} />
						</div>
					</div>

					{#if !passwordValidation.valid}
						<span class="flex items-center gap-1 text-[11px] text-[var(--status-red)]">
							<WarningCircle size={12} /> {passwordValidation.error}
						</span>
					{/if}
				</div>

				<!-- Root Password for MySQL & MariaDB -->
				{#if currentEngine.hasRootPassword}
					<div class="flex flex-col gap-1.5 sm:col-span-2 border-t border-[var(--border)] pt-2.5">
						<div class="flex items-center justify-between">
							<label for="edit-db-root-pass" class="text-xs font-medium text-[var(--text-secondary)]">
								Root Password (DB Administrator)
							</label>
							<button
								type="button"
								onclick={handleRegenerateRootPassword}
								class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline"
							>
								<ArrowsClockwise size={12} /> Regenerate Root
							</button>
						</div>

						<div class="relative flex items-center">
							<Input
								id="edit-db-root-pass"
								type={showRootPassword ? 'text' : 'password'}
								bind:value={dbRootPassword}
								class="pr-16 text-xs font-mono"
							/>
							<div class="absolute right-1.5 flex items-center gap-1">
								<button
									type="button"
									onclick={() => (showRootPassword = !showRootPassword)}
									class="cursor-pointer border-0 bg-transparent p-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
								>
									{#if showRootPassword}
										<EyeSlash size={14} />
									{:else}
										<Eye size={14} />
									{/if}
								</button>
								<CopyButton text={dbRootPassword} />
							</div>
						</div>
					</div>
				{/if}
			</div>
		</div>

		<!-- 4. Ports & Networking -->
		<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)]/60 p-3.5">
			<div class="flex flex-col gap-1">
				<label for="edit-internal-port" class="text-xs font-medium text-[var(--text-secondary)]">
					Container Port <span class="text-[var(--status-red)]">*</span>
				</label>
				<Input
					id="edit-internal-port"
					bind:value={internalPortInput}
					placeholder={String(currentEngine.defaultPort)}
					class="text-xs font-mono"
				/>
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Internal port listening inside container
				</span>
			</div>

			<div class="flex flex-col gap-1">
				<label for="edit-host-port" class="text-xs font-medium text-[var(--text-secondary)]">
					Host Port Exposure <span class="font-normal text-[var(--text-tertiary)]">(Optional)</span>
				</label>
				<Input
					id="edit-host-port"
					bind:value={hostPortInput}
					placeholder="e.g. {currentEngine.defaultPort}"
					class="text-xs font-mono"
				/>
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Leave empty for private internal-only access
				</span>
			</div>
		</div>

		<!-- 5. Storage Persistence Settings -->
		<div class="flex flex-col gap-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)]/60 p-3.5">
			<button
				type="button"
				onclick={() => (showAdvancedStorage = !showAdvancedStorage)}
				class="flex cursor-pointer items-center justify-between text-xs font-medium text-[var(--accent)] hover:underline"
			>
				<div class="flex items-center gap-1.5">
					<HardDrive size={15} />
					<span>Storage Volume Configuration</span>
				</div>
				<span class="text-[11px]">{showAdvancedStorage ? 'Hide Details' : 'Customize Paths'}</span>
			</button>

			{#if showAdvancedStorage}
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 pt-2">
					<div class="flex flex-col gap-1">
						<label for="edit-vol-name" class="text-xs font-medium text-[var(--text-secondary)]">
							Named Volume
						</label>
						<Input
							id="edit-vol-name"
							bind:value={volumeNameInput}
							placeholder="{service.id}-data"
							class="text-xs font-mono"
						/>
					</div>
					<div class="flex flex-col gap-1">
						<label for="edit-mount-path" class="text-xs font-medium text-[var(--text-secondary)]">
							Container Mount Target
						</label>
						<Input
							id="edit-mount-path"
							bind:value={storagePathInput}
							placeholder={currentEngine.defaultStoragePath}
							class="text-xs font-mono"
						/>
					</div>
				</div>
			{:else}
				<div class="flex items-center justify-between text-xs text-[var(--text-secondary)]">
					<span>Volume: <code>{volumeNameInput || creds.volumeName}</code></span>
					<span class="font-mono text-[11px] text-[var(--text-tertiary)]">→ {storagePathInput || creds.volumeMountPath}</span>
				</div>
			{/if}
		</div>

		<!-- 6. Live Connection String Preview -->
		<div class="flex flex-col gap-1.5 rounded-md border border-[var(--border)] bg-[var(--bg-panel)] p-3">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-1.5">
					<LinkSimple size={14} class="text-[var(--accent)]" />
					<span class="text-xs font-semibold text-[var(--text-primary)]">
						Updated Connection String (Live Preview)
					</span>
				</div>
				<CopyButton text={liveConnUri} />
			</div>
			<div class="overflow-x-auto rounded bg-[var(--bg-surface)] px-2.5 py-1.5 font-mono text-[11px] text-[var(--accent)] select-all">
				{maskedConnUri}
			</div>
		</div>

		<!-- Warning Notice -->
		<div
			class="flex flex-col gap-1 rounded-md border border-[var(--status-amber)]/30 bg-[var(--status-amber)]/10 p-3 text-xs text-[var(--text-secondary)]"
		>
			<span class="font-semibold text-[var(--status-amber)]">Redeploy to apply changes</span>
			<p class="m-0 text-[11px] leading-relaxed text-[var(--text-tertiary)]">
				Saving modifies the database service specification and environment variables. To take effect, the container
				must be restarted or redeployed. Choose <strong>Save & Redeploy</strong> to apply immediately.
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
			disabled={!isFormValid || isSaving}
		>
			<Check size={14} class="mr-1" />
			Save Changes
		</Button>
		<Button
			variant="primary"
			size="sm"
			onclick={() => handleSave(true)}
			disabled={!isFormValid || isSaving}
		>
			<RocketLaunch size={14} class="mr-1" />
			Save & Redeploy
		</Button>
	{/snippet}
</Modal>
