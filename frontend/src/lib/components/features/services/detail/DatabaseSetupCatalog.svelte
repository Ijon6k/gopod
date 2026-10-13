<script lang="ts">
	import type { Service } from '$lib/types';
	import { Input, Button, CopyButton, DatabaseIcon } from '$lib/components/ui';
	import { LayerCard } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import {
		DATABASE_ENGINES,
		type DatabaseConfig,
		type DatabaseEngine,
		generateDatabasePassword,
		validateDatabasePassword,
		buildDatabaseEnvVars,
		updateServiceWithDatabaseConfig,
		extractDatabaseCredentials
	} from '../database/database-utils';
	import {
		RocketLaunch,
		FloppyDisk,
		ArrowsClockwise,
		Eye,
		EyeSlash,
		HardDrive,
		LinkSimple,
		ShieldCheck,
		CheckCircle,
		X,
		Tag,
		WarningCircle
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
		onConfigured?: () => void;
		onCancel?: () => void;
	}

	let { service, onConfigured, onCancel }: Props = $props();

	// Extract existing credentials if already configured (e.g. when changing engine)
	const existingCreds = $derived(extractDatabaseCredentials(service));
	const initialEngine = $derived(
		service.image ? existingCreds.engine : 'postgres'
	);

	let selectedEngineId = $state<DatabaseEngine>('postgres');

	$effect(() => {
		if (service.image && existingCreds.engine) {
			selectedEngineId = existingCreds.engine;
		}
	});

	const currentEngine = $derived(
		DATABASE_ENGINES.find((e) => e.id === selectedEngineId) ?? DATABASE_ENGINES[0]
	);

	const safeSlug = $derived(
		(service.name || 'db')
			.toLowerCase()
			.trim()
			.replace(/[^a-z0-9_-]/g, '-')
			.replace(/-+/g, '-')
	);

	// Custom Image vs Preset Tag
	let customImageInput = $state('');
	let selectedTag = $state('postgres:17-alpine');
	let isCustomImageMode = $state(false);

	const activeImage = $derived.by(() => {
		if (selectedEngineId === 'custom' || isCustomImageMode) {
			return customImageInput.trim() || currentEngine.defaultImage;
		}
		return selectedTag || currentEngine.defaultImage;
	});

	// Credentials
	let dbName = $state('gopod');
	let dbUser = $state('postgres');
	let dbPassword = $state(generateDatabasePassword());
	let dbRootPassword = $state(generateDatabasePassword());
	let showPassword = $state(false);
	let showRootPassword = $state(false);

	// Networking
	let internalPortInput = $state('5432');
	let hostPortInput = $state('');

	// Storage
	let volumeNameInput = $state('');
	let storagePathInput = $state('/var/lib/postgresql/data');

	// Status & Error
	let isDeploying = $state(false);
	let isSaving = $state(false);
	let errorMessage = $state<string | null>(null);

	// Validation
	let passwordValidation = $derived(validateDatabasePassword(dbPassword));

	function handleSelectEngine(engineId: DatabaseEngine) {
		selectedEngineId = engineId;
		const def = DATABASE_ENGINES.find((e) => e.id === engineId) ?? DATABASE_ENGINES[0];

		dbUser = def.defaultUser;
		dbName = def.defaultDbName;
		internalPortInput = String(def.defaultPort);
		selectedTag = def.popularTags[0] || def.defaultImage;
		storagePathInput = def.defaultStoragePath;
		volumeNameInput = `${safeSlug}-data`;
		isCustomImageMode = engineId === 'custom';

		if (engineId === 'custom') {
			customImageInput = '';
		}

		errorMessage = null;
	}

	function regeneratePassword() {
		dbPassword = generateDatabasePassword();
	}

	function regenerateRootPassword() {
		dbRootPassword = generateDatabasePassword();
	}

	async function handleSubmit(autoDeploy: boolean) {
		if (isDeploying || isSaving) return;
		errorMessage = null;

		// Password validation for engines that require one
		if (selectedEngineId !== 'libsql') {
			const val = validateDatabasePassword(dbPassword);
			if (!val.valid) {
				errorMessage = val.error || 'Invalid password';
				return;
			}
		}

		if (currentEngine.hasRootPassword && dbRootPassword) {
			const rootVal = validateDatabasePassword(dbRootPassword);
			if (!rootVal.valid) {
				errorMessage = `Root password error: ${rootVal.error}`;
				return;
			}
		}

		const parsedInternalPort = parseInt(internalPortInput.trim(), 10) || currentEngine.defaultPort;
		const parsedHostPort = hostPortInput.trim() ? parseInt(hostPortInput.trim(), 10) : undefined;

		const config: DatabaseConfig = {
			engine: selectedEngineId,
			name: service.name,
			description: service.description || `${currentEngine.name} managed database instance`,
			image: activeImage,
			databaseName: currentEngine.hasDbName ? dbName.trim() : '',
			user: currentEngine.hasUser ? dbUser.trim() : '',
			password: dbPassword,
			rootPassword: currentEngine.hasRootPassword ? dbRootPassword : undefined,
			internalPort: parsedInternalPort,
			hostPort: parsedHostPort,
			volumeName: volumeNameInput.trim() || `${safeSlug}-data`,
			volumeMountPath: storagePathInput.trim() || currentEngine.defaultStoragePath,
			envVars: []
		};
		config.envVars = buildDatabaseEnvVars(config);

		const updatedService = updateServiceWithDatabaseConfig(service, config);

		if (autoDeploy) {
			isDeploying = true;
		} else {
			isSaving = true;
		}

		try {
			await dataStore.updateService(updatedService);
			if (autoDeploy) {
				await dataStore.deployService(service.id, 'manual');
			}
			onConfigured?.();
		} catch (err: any) {
			errorMessage = err?.message || 'Failed to configure database';
		} finally {
			isDeploying = false;
			isSaving = false;
		}
	}
</script>

<div class="flex w-full flex-col gap-6">
	<!-- Catalog Header -->
	<div class="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h2 class="m-0 text-base font-semibold text-[var(--text-primary)]">
				{service.image ? 'Change Database Engine' : 'Select Database Engine'}
			</h2>
			<p class="m-0 text-xs text-[var(--text-tertiary)]">
				Choose an engine, configure credentials and storage, then deploy.
			</p>
		</div>

		{#if onCancel}
			<Button variant="ghost" size="sm" onclick={onCancel} class="self-start sm:self-auto">
				<X size={14} /> Cancel
			</Button>
		{/if}
	</div>

	<!-- Database Selection Grid with Internet Photos / Logos -->
	<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
		{#each DATABASE_ENGINES as eng (eng.id)}
			{@const isSelected = selectedEngineId === eng.id}
			<button
				type="button"
				onclick={() => handleSelectEngine(eng.id)}
				class="group relative flex flex-col items-start rounded-[var(--radius-card)] border p-4 text-left transition-all cursor-pointer {isSelected
					? 'border-[var(--accent)] bg-[var(--bg-surface)] shadow-sm ring-1 ring-[var(--accent)]'
					: 'border-[var(--border)] bg-[var(--bg-panel)] hover:border-[var(--border-strong)] hover:bg-[var(--bg-surface)]'}"
			>
				<!-- Top Bar: Internet Photo/Logo & Badges -->
				<div class="flex w-full items-start justify-between gap-2">
					<div
						class="flex size-14 shrink-0 items-center justify-center rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-panel)] p-2 shadow-2xs transition-transform group-hover:scale-105"
					>
						<DatabaseIcon engine={eng.id} size={36} useInternetImage={true} />
					</div>

					<div class="flex flex-col items-end gap-1">
						{#if isSelected}
							<span
								class="flex items-center gap-1 rounded-full bg-[var(--accent)]/15 px-2 py-0.5 text-[10px] font-semibold text-[var(--accent)]"
							>
								<CheckCircle size={12} weight="fill" /> Selected
							</span>
						{/if}
						<span
							class="rounded bg-[var(--bg-elevated)] px-1.5 py-0.5 font-mono text-[10px] font-medium text-[var(--text-tertiary)]"
						>
							Port {eng.defaultPort}
						</span>
					</div>
				</div>

				<!-- Database Info -->
				<div class="mt-3 flex w-full flex-col">
					<div class="flex items-center justify-between">
						<span class="text-sm font-semibold text-[var(--text-primary)]">
							{eng.name}
						</span>
						<span class="text-[10px] text-[var(--text-tertiary)] uppercase font-mono">
							{eng.id === 'redis'
								? 'Cache / KV'
								: eng.id === 'mongodb'
									? 'NoSQL'
									: eng.id === 'clickhouse'
										? 'OLAP'
										: eng.id === 'libsql'
											? 'SQLite'
											: eng.id === 'custom'
												? 'Docker'
												: 'SQL'}
						</span>
					</div>
					<p class="m-0 mt-1 line-clamp-2 text-xs leading-relaxed text-[var(--text-secondary)]">
						{eng.desc}
					</p>
				</div>
			</button>
		{/each}
	</div>

	<!-- Configuration & Launch Card -->
	<div
		class="flex flex-col gap-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
	>
		<!-- Section Header -->
		<div class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-subtle)] pb-4">
			<div class="flex items-center gap-3">
				<div
					class="flex size-10 shrink-0 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-1.5"
				>
					<DatabaseIcon engine={selectedEngineId} size={26} useInternetImage={true} />
				</div>
				<div>
					<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
						Configure {currentEngine.name} Instance
					</h3>
					<span class="text-xs text-[var(--text-tertiary)]">
						Configure credentials, ports, and persistent storage.
					</span>
				</div>
			</div>

			<div class="flex items-center gap-2">
				{#if onCancel && service.image}
					<Button variant="ghost" size="sm" onclick={onCancel} disabled={isDeploying || isSaving}>
						Cancel
					</Button>
				{/if}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => handleSubmit(false)}
					disabled={isDeploying || isSaving}
					class="text-xs"
				>
					<FloppyDisk size={13} />
					{isSaving ? 'Saving...' : 'Save Configuration'}
				</Button>
				<Button
					variant="primary"
					size="sm"
					onclick={() => handleSubmit(true)}
					disabled={isDeploying || isSaving}
					class="text-xs"
				>
					<RocketLaunch size={13} />
					{isDeploying ? 'Deploying...' : 'Deploy Database'}
				</Button>
			</div>
		</div>

		<!-- Error Message Alert -->
		{#if errorMessage}
			<div
				class="flex items-center gap-2 rounded-md border border-[var(--status-red)]/30 bg-[var(--status-red)]/10 px-3.5 py-2.5 text-xs text-[var(--status-red)]"
			>
				<WarningCircle size={15} class="shrink-0" />
				<span>{errorMessage}</span>
			</div>
		{/if}

		<!-- 1. Image & Tag Selection -->
		<div class="flex flex-col gap-2.5">
			<div class="flex items-center justify-between">
				<label for="db-image-input" class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
					<Tag size={13} class="text-[var(--accent)]" />
					Container Image & Version
				</label>
				<button
					type="button"
					onclick={() => (isCustomImageMode = !isCustomImageMode)}
					class="cursor-pointer text-[11px] font-medium text-[var(--accent)] hover:underline"
				>
					{isCustomImageMode ? 'Use Popular Presets' : 'Custom Image / Digest'}
				</button>
			</div>

			{#if isCustomImageMode || selectedEngineId === 'custom'}
				<Input
					id="db-image-input"
					bind:value={customImageInput}
					placeholder={currentEngine.defaultImage || 'docker.io/library/postgres:17-alpine'}
					class="font-mono text-xs"
				/>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Enter any container image tag from Docker Hub, Quay, or GHCR.
				</span>
			{:else}
				<div class="flex flex-wrap gap-2">
					{#each currentEngine.popularTags as tag}
						<button
							type="button"
							onclick={() => (selectedTag = tag)}
							class="cursor-pointer rounded-md border px-2.5 py-1 font-mono text-xs transition-colors {selectedTag ===
							tag
								? 'border-[var(--accent)] bg-[var(--accent)]/10 font-semibold text-[var(--accent)]'
								: 'border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:border-[var(--border-strong)] hover:text-[var(--text-primary)]'}"
						>
							{tag}
						</button>
					{/each}
				</div>
			{/if}
		</div>

		<!-- 2. Database Credentials (Dokploy Parity) -->
		<div class="flex flex-col gap-3 border-t border-[var(--border-subtle)] pt-4">
			<div class="flex items-center gap-1.5">
				<ShieldCheck size={14} class="text-[var(--accent)]" />
				<span class="text-xs font-semibold text-[var(--text-primary)]">Credentials & Authentication</span>
			</div>

			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				<!-- Username (if applicable) -->
				{#if currentEngine.hasUser}
					<div class="flex flex-col gap-1.5">
						<label for="catalog-db-user" class="text-xs font-medium text-[var(--text-secondary)]">
							Database User
						</label>
						<Input
							id="catalog-db-user"
							bind:value={dbUser}
							placeholder={currentEngine.defaultUser}
							class="font-mono text-xs"
						/>
					</div>
				{/if}

				<!-- Database Name (if applicable) -->
				{#if currentEngine.hasDbName}
					<div class="flex flex-col gap-1.5">
						<label for="catalog-db-name" class="text-xs font-medium text-[var(--text-secondary)]">
							Database Name
						</label>
						<Input
							id="catalog-db-name"
							bind:value={dbName}
							placeholder={currentEngine.defaultDbName}
							class="font-mono text-xs"
						/>
					</div>
				{/if}

				<!-- Password (with secure generator) -->
				{#if selectedEngineId !== 'libsql'}
					<div class="flex flex-col gap-1.5 {currentEngine.hasUser && currentEngine.hasDbName ? 'sm:col-span-2' : ''}">
						<div class="flex items-center justify-between">
							<label for="catalog-db-password" class="text-xs font-medium text-[var(--text-secondary)]">
								Password <span class="text-[var(--status-red)]">*</span>
							</label>
							<div class="flex items-center gap-2">
								<button
									type="button"
									onclick={regeneratePassword}
									class="flex cursor-pointer items-center gap-1 text-[11px] font-medium text-[var(--accent)] hover:underline"
								>
									<ArrowsClockwise size={12} /> Regenerate
								</button>
								<button
									type="button"
									onclick={() => (showPassword = !showPassword)}
									class="flex cursor-pointer items-center gap-1 text-[11px] text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
								>
									{#if showPassword}
										<EyeSlash size={12} /> Hide
									{:else}
										<Eye size={12} /> Show
									{/if}
								</button>
							</div>
						</div>
						<div class="relative flex items-center">
							<Input
								id="catalog-db-password"
								type={showPassword ? 'text' : 'password'}
								bind:value={dbPassword}
								placeholder="Generated secure password"
								class="pr-16 font-mono text-xs"
							/>
							<div class="absolute right-1.5">
								<CopyButton text={dbPassword} />
							</div>
						</div>
						{#if !passwordValidation.valid}
							<span class="text-[11px] text-[var(--status-red)]">{passwordValidation.error}</span>
						{:else}
							<span class="text-[10px] text-[var(--text-tertiary)]">
								Auto-generated 20-character secure alphanumeric password safe for shell environments.
							</span>
						{/if}
					</div>
				{/if}

				<!-- Root Password (for MySQL / MariaDB) -->
				{#if currentEngine.hasRootPassword}
					<div class="flex flex-col gap-1.5 sm:col-span-2">
						<div class="flex items-center justify-between">
							<label for="catalog-db-root-pwd" class="text-xs font-medium text-[var(--text-secondary)]">
								Root Password
							</label>
							<div class="flex items-center gap-2">
								<button
									type="button"
									onclick={regenerateRootPassword}
									class="flex cursor-pointer items-center gap-1 text-[11px] font-medium text-[var(--accent)] hover:underline"
								>
									<ArrowsClockwise size={12} /> Regenerate
								</button>
								<button
									type="button"
									onclick={() => (showRootPassword = !showRootPassword)}
									class="flex cursor-pointer items-center gap-1 text-[11px] text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
								>
									{#if showRootPassword}
										<EyeSlash size={12} /> Hide
									{:else}
										<Eye size={12} /> Show
									{/if}
								</button>
							</div>
						</div>
						<div class="relative flex items-center">
							<Input
								id="catalog-db-root-pwd"
								type={showRootPassword ? 'text' : 'password'}
								bind:value={dbRootPassword}
								placeholder="Root user password"
								class="pr-16 font-mono text-xs"
							/>
							<div class="absolute right-1.5">
								<CopyButton text={dbRootPassword} />
							</div>
						</div>
					</div>
				{/if}
			</div>
		</div>

		<!-- 3. Port & Storage Networking -->
		<div class="flex flex-col gap-3 border-t border-[var(--border-subtle)] pt-4">
			<div class="flex items-center gap-1.5">
				<LinkSimple size={14} class="text-[var(--accent)]" />
				<span class="text-xs font-semibold text-[var(--text-primary)]">Ports & Storage Volumes</span>
			</div>

			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				<!-- Internal Port -->
				<div class="flex flex-col gap-1.5">
					<label for="catalog-internal-port" class="text-xs font-medium text-[var(--text-secondary)]">
						Container Port
					</label>
					<Input
						id="catalog-internal-port"
						bind:value={internalPortInput}
						placeholder={String(currentEngine.defaultPort)}
						class="font-mono text-xs"
					/>
					<span class="text-[10px] text-[var(--text-tertiary)]">
						Internal port inside the container listening on the Podman bridge.
					</span>
				</div>

				<!-- Host Port -->
				<div class="flex flex-col gap-1.5">
					<label for="catalog-host-port" class="text-xs font-medium text-[var(--text-secondary)]">
						Host Port (Optional)
					</label>
					<Input
						id="catalog-host-port"
						bind:value={hostPortInput}
						placeholder="e.g. 5432 (leave blank to keep internal only)"
						class="font-mono text-xs"
					/>
					<span class="text-[10px] text-[var(--text-tertiary)]">
						Expose directly on localhost for external GUI tools (DBeaver, TablePlus).
					</span>
				</div>

				<!-- Storage Volume Name -->
				<div class="flex flex-col gap-1.5">
					<label for="catalog-volume-name" class="text-xs font-medium text-[var(--text-secondary)]">
						Volume Name
					</label>
					<Input
						id="catalog-volume-name"
						bind:value={volumeNameInput}
						placeholder="{safeSlug}-data"
						class="font-mono text-xs"
					/>
				</div>

				<!-- Storage Mount Path -->
				<div class="flex flex-col gap-1.5">
					<label for="catalog-volume-path" class="text-xs font-medium text-[var(--text-secondary)]">
						Mount Path
					</label>
					<Input
						id="catalog-volume-path"
						bind:value={storagePathInput}
						placeholder={currentEngine.defaultStoragePath}
						class="font-mono text-xs"
					/>
				</div>
			</div>
		</div>

		<!-- Action Footer -->
		<div
			class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-4"
		>
			{#if onCancel && service.image}
				<Button variant="ghost" size="sm" onclick={onCancel} disabled={isDeploying || isSaving}>
					Cancel
				</Button>
			{/if}
			<Button
				variant="secondary"
				size="sm"
				onclick={() => handleSubmit(false)}
				disabled={isDeploying || isSaving}
			>
				<FloppyDisk size={13} />
				{isSaving ? 'Saving...' : 'Save Configuration'}
			</Button>
			<Button
				variant="primary"
				size="sm"
				onclick={() => handleSubmit(true)}
				disabled={isDeploying || isSaving}
			>
				<RocketLaunch size={13} />
				{isDeploying ? 'Deploying...' : 'Deploy Database'}
			</Button>
		</div>
	</div>
</div>
