<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, DatabaseIcon } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import {
		DATABASE_ENGINES,
		type DatabaseConfig,
		type DatabaseEngine,
		generateDatabasePassword,
		validateDatabasePassword,
		updateServiceWithDatabaseConfig,
		extractDatabaseCredentials
	} from '../database/database-utils';
	import { X } from 'phosphor-svelte';
	import DatabaseEngineGrid from './database/DatabaseEngineGrid.svelte';
	import DatabaseConfigCard from './database/DatabaseConfigCard.svelte';

	interface Props {
		service: Service;
		onConfigured?: () => void;
		onCancel?: () => void;
	}

	let { service, onConfigured, onCancel }: Props = $props();

	// Extract existing credentials if already configured (e.g. when changing engine)
	const existingCreds = $derived(extractDatabaseCredentials(service));

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
			description: service.description || '',
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

			if (onConfigured) {
				onConfigured();
			}
		} catch (err: any) {
			errorMessage = err?.message || 'Failed to save database configuration.';
		} finally {
			isDeploying = false;
			isSaving = false;
		}
	}
</script>

<div class="flex flex-col gap-6">
	<!-- Catalog Header -->
	<div
		class="flex flex-col justify-between gap-3 border-b border-[var(--border)] pb-4 sm:flex-row sm:items-center"
	>
		<div class="flex items-center gap-3">
			<div
				class="flex size-10 shrink-0 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-2 text-[var(--accent)]"
			>
				<DatabaseIcon engine={selectedEngineId} size={28} useInternetImage={true} />
			</div>
			<div>
				<h2 class="m-0 text-base font-semibold text-[var(--text-primary)]">
					{service.image ? 'Change Database Engine' : 'Setup Database Engine'}
				</h2>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">
					One-click managed persistent database instances with volume mounts & credentials.
				</p>
			</div>
		</div>

		{#if onCancel}
			<Button variant="ghost" size="sm" onclick={onCancel} class="self-start sm:self-auto">
				<X size={14} /> Cancel
			</Button>
		{/if}
	</div>

	<!-- Database Selection Grid -->
	<DatabaseEngineGrid {selectedEngineId} onSelectEngine={handleSelectEngine} />

	<!-- Configuration & Launch Card -->
	<DatabaseConfigCard
		{service}
		{currentEngine}
		{selectedEngineId}
		bind:selectedTag
		bind:customImageInput
		bind:isCustomImageMode
		bind:dbName
		bind:dbUser
		bind:dbPassword
		bind:dbRootPassword
		bind:internalPortInput
		bind:hostPortInput
		bind:volumeNameInput
		bind:storagePathInput
		{safeSlug}
		{passwordValidation}
		{errorMessage}
		{isDeploying}
		{isSaving}
		{onCancel}
		onSubmit={handleSubmit}
		onRegeneratePassword={regeneratePassword}
		onRegenerateRootPassword={regenerateRootPassword}
	/>
</div>
