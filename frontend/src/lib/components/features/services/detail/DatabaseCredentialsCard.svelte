<script lang="ts">
	import type { Service } from '$lib/types';
	import { CopyButton, Input, DatabaseIcon } from '$lib/components/ui';
	import { LayerCard, Button } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import {
		extractDatabaseCredentials,
		buildConnectionString,
		buildCliCommand,
		revealMaskedEnvValues,
		DATABASE_ENGINES,
		type DatabaseEngine
	} from '../database/database-utils';
	import EditDatabaseModal from './modals/EditDatabaseModal.svelte';
	import ChangePasswordModal from './modals/ChangePasswordModal.svelte';
	import {
		Database,
		ShieldCheck,
		LinkSimple,
		HardDrive,
		TerminalWindow,
		Eye,
		EyeSlash,
		PencilSimple,
		Key,
		ArrowsClockwise,
		RocketLaunch
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
		onRedeploy?: () => void;
		onChangeEngine?: () => void;
	}

	let { service, onRedeploy, onChangeEngine }: Props = $props();

	let showPassword = $state(false);
	let showRootPassword = $state(false);
	let editModalOpen = $state(false);
	let passwordModalOpen = $state(false);
	let isRedeploying = $state(false);

	// Secret env values arrive masked; this card reveals them so display, copy
	// buttons and connection strings stay usable. The backend records each
	// reveal in the audit trail.
	let revealedEnv = $state<Record<string, string>>({});
	let revealedFor = $state<string | null>(null);

	async function refreshRevealed() {
		const id = service.id;
		revealedFor = id;
		const values = await revealMaskedEnvValues(service);
		if (revealedFor === id) revealedEnv = values;
	}

	$effect(() => {
		if (service.id !== revealedFor) {
			revealedEnv = {};
			void refreshRevealed();
		}
	});

	const creds = $derived(extractDatabaseCredentials(service, revealedEnv));
	const currentEngine = $derived(
		DATABASE_ENGINES.find((e) => e.id === creds.engine) ?? DATABASE_ENGINES[0]
	);

	const internalHost = $derived(service.id || service.name);

	// Internal connection string
	const internalConnUri = $derived(
		buildConnectionString({
			engine: creds.engine,
			user: currentEngine.hasUser ? creds.user : undefined,
			password: creds.password,
			host: internalHost,
			port: creds.port,
			databaseName: currentEngine.hasDbName ? creds.databaseName : undefined
		})
	);

	// Host connection string (if hostPort is exposed)
	const hostConnUri = $derived.by(() => {
		if (!creds.hostPort) return null;
		return buildConnectionString({
			engine: creds.engine,
			user: currentEngine.hasUser ? creds.user : undefined,
			password: creds.password,
			host: 'localhost',
			port: creds.hostPort,
			databaseName: currentEngine.hasDbName ? creds.databaseName : undefined
		});
	});

	// CLI quick access command
	const cliCommand = $derived(
		buildCliCommand(service, {
			engine: creds.engine,
			user: creds.user,
			password: creds.password,
			databaseName: creds.databaseName
		})
	);

	async function handleRedeploy() {
		if (onRedeploy) {
			onRedeploy();
			return;
		}
		isRedeploying = true;
		try {
			await dataStore.deployService(service.id, 'manual');
		} finally {
			isRedeploying = false;
		}
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- Elevated Header (LayerCard.Secondary) -->
	<LayerCard.Secondary
		class="flex flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline px-5 py-3.5"
	>
		<div class="flex items-center gap-3">
			<div
				class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-kumo-base ring-1 ring-kumo-line"
			>
				<DatabaseIcon engine={creds.engine} size={20} />
			</div>
			<div class="flex flex-col">
				<div class="flex items-center gap-2">
					<span class="text-sm font-semibold text-kumo-default">
						{currentEngine.name} Instance
					</span>
					<span
						class="rounded-sm bg-kumo-base px-1.5 py-0.5 font-mono text-[10px] font-medium text-kumo-subtle uppercase ring-1 ring-kumo-hairline"
					>
						Managed
					</span>
				</div>
				<span class="text-[11px] text-kumo-subtle">
					Image: {creds.image} • Internal Port: {creds.port}
				</span>
			</div>
		</div>

		<div class="flex items-center gap-2">
			{#if onChangeEngine}
				<Button
					variant="secondary"
					size="sm"
					onclick={onChangeEngine}
					class="flex items-center gap-1.5 text-xs"
				>
					<ArrowsClockwise size={13} />
					Switch Engine
				</Button>
			{/if}

			<Button
				variant="secondary"
				size="sm"
				onclick={() => (editModalOpen = true)}
				class="flex items-center gap-1.5 text-xs"
			>
				<PencilSimple size={13} />
				Edit Database
			</Button>

			<Button
				variant="secondary"
				size="sm"
				onclick={handleRedeploy}
				disabled={isRedeploying}
				class="flex items-center gap-1.5 text-xs"
			>
				<RocketLaunch size={13} />
				{isRedeploying ? 'Deploying...' : 'Redeploy'}
			</Button>
		</div>
	</LayerCard.Secondary>

	<!-- Body (LayerCard.Primary) -->
	<LayerCard.Primary class="flex flex-col gap-4 p-5">
		<!-- 1. Connection URLs -->
		<div class="flex flex-col gap-2">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-1.5">
					<LinkSimple size={14} class="text-[var(--accent)]" />
					<span class="text-xs font-semibold text-kumo-default">Internal Connection String</span>
				</div>
				<CopyButton text={internalConnUri} />
			</div>
			<div
				class="flex items-center justify-between rounded-md bg-kumo-elevated px-3 py-2 font-mono text-xs text-[var(--accent)] ring-1 ring-kumo-hairline"
			>
				<span class="truncate select-all">
					{showPassword ? internalConnUri : internalConnUri.replace(creds.password, '••••••••••••')}
				</span>
			</div>
			<span class="text-[10px] text-kumo-subtle">
				Use this connection string from any application running on the <code>gopod-net</code> network.
			</span>
		</div>

		{#if hostConnUri}
			<div class="flex flex-col gap-2 border-t border-kumo-hairline pt-3">
				<div class="flex items-center justify-between">
					<div class="flex items-center gap-1.5">
						<LinkSimple size={14} class="text-kumo-success" />
						<span class="text-xs font-semibold text-kumo-default"
							>External Host Connection String</span
						>
					</div>
					<CopyButton text={hostConnUri} />
				</div>
				<div
					class="flex items-center justify-between rounded-md bg-kumo-elevated px-3 py-2 font-mono text-xs text-kumo-success ring-1 ring-kumo-hairline"
				>
					<span class="truncate select-all">
						{showPassword ? hostConnUri : hostConnUri.replace(creds.password, '••••••••••••')}
					</span>
				</div>
				<span class="text-[10px] text-kumo-subtle">
					Exposed externally on host port <code>{creds.hostPort}</code> for local development tools and
					database GUIs.
				</span>
			</div>
		{/if}

		<!-- 2. Credentials Grid (Dokploy Parity with Quick Actions) -->
		<div class="flex flex-col gap-2.5 border-t border-kumo-hairline pt-3">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-1.5">
					<ShieldCheck size={14} class="text-[var(--accent)]" />
					<span class="text-xs font-semibold text-kumo-default">Database Credentials</span>
				</div>
				<button
					type="button"
					onclick={() => (editModalOpen = true)}
					class="cursor-pointer text-[11px] font-medium text-[var(--accent)] hover:underline"
				>
					Modify Credentials
				</button>
			</div>

			<div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
				<!-- Host -->
				<div class="flex flex-col gap-1 rounded bg-kumo-elevated p-2.5 ring-1 ring-kumo-hairline">
					<span class="text-[10px] font-medium text-kumo-subtle uppercase">Host</span>
					<div class="flex items-center justify-between">
						<span class="truncate font-mono text-xs font-medium text-kumo-default"
							>{internalHost}</span
						>
						<CopyButton text={internalHost} />
					</div>
				</div>

				<!-- Port -->
				<div class="flex flex-col gap-1 rounded bg-kumo-elevated p-2.5 ring-1 ring-kumo-hairline">
					<span class="text-[10px] font-medium text-kumo-subtle uppercase">Port</span>
					<div class="flex items-center justify-between">
						<span class="font-mono text-xs font-medium text-kumo-default">{creds.port}</span>
						<CopyButton text={String(creds.port)} />
					</div>
				</div>

				<!-- User (if applicable) -->
				{#if currentEngine.hasUser}
					<div class="flex flex-col gap-1 rounded bg-kumo-elevated p-2.5 ring-1 ring-kumo-hairline">
						<span class="text-[10px] font-medium text-kumo-subtle uppercase">Username</span>
						<div class="flex items-center justify-between">
							<span class="truncate font-mono text-xs font-medium text-kumo-default"
								>{creds.user}</span
							>
							<CopyButton text={creds.user} />
						</div>
					</div>
				{/if}

				<!-- Database Name (if applicable) -->
				{#if currentEngine.hasDbName}
					<div class="flex flex-col gap-1 rounded bg-kumo-elevated p-2.5 ring-1 ring-kumo-hairline">
						<span class="text-[10px] font-medium text-kumo-subtle uppercase">Database</span>
						<div class="flex items-center justify-between">
							<span class="truncate font-mono text-xs font-medium text-kumo-default"
								>{creds.databaseName}</span
							>
							<CopyButton text={creds.databaseName} />
						</div>
					</div>
				{/if}

				<!-- Password (with Change Password modal trigger) -->
				<div
					class="flex flex-col gap-1 rounded bg-kumo-elevated p-2.5 ring-1 ring-kumo-hairline sm:col-span-2"
				>
					<div class="flex items-center justify-between">
						<span class="text-[10px] font-medium text-kumo-subtle uppercase">Password</span>
						<div class="flex items-center gap-2">
							<button
								type="button"
								onclick={() => (passwordModalOpen = true)}
								class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[10px] font-medium text-[var(--accent)] hover:underline"
							>
								<Key size={11} /> Change Password
							</button>
							<button
								type="button"
								onclick={() => (showPassword = !showPassword)}
								class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[10px] text-kumo-subtle hover:text-kumo-default"
							>
								{#if showPassword}
									<EyeSlash size={12} /> Hide
								{:else}
									<Eye size={12} /> Show
								{/if}
							</button>
						</div>
					</div>
					<div class="flex items-center justify-between">
						<span class="truncate font-mono text-xs font-medium text-kumo-default">
							{showPassword ? creds.password : '••••••••••••••••'}
						</span>
						<CopyButton text={creds.password} />
					</div>
				</div>

				<!-- Root Password (if MySQL / MariaDB) -->
				{#if currentEngine.hasRootPassword && creds.rootPassword}
					<div
						class="flex flex-col gap-1 rounded bg-kumo-elevated p-2.5 ring-1 ring-kumo-hairline sm:col-span-2"
					>
						<div class="flex items-center justify-between">
							<span class="text-[10px] font-medium text-kumo-subtle uppercase">Root Password</span>
							<button
								type="button"
								onclick={() => (showRootPassword = !showRootPassword)}
								class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[10px] text-kumo-subtle hover:text-kumo-default"
							>
								{#if showRootPassword}
									<EyeSlash size={12} /> Hide
								{:else}
									<Eye size={12} /> Show
								{/if}
							</button>
						</div>
						<div class="flex items-center justify-between">
							<span class="truncate font-mono text-xs font-medium text-kumo-default">
								{showRootPassword ? creds.rootPassword : '••••••••••••••••'}
							</span>
							<CopyButton text={creds.rootPassword} />
						</div>
					</div>
				{/if}
			</div>
		</div>

		<!-- 3. Storage Persistence -->
		<div class="flex flex-col gap-1.5 border-t border-kumo-hairline pt-3">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-1.5">
					<HardDrive size={14} class="text-[var(--accent)]" />
					<span class="text-xs font-semibold text-kumo-default">Storage Persistence</span>
				</div>
				<button
					type="button"
					onclick={() => (editModalOpen = true)}
					class="cursor-pointer text-[11px] font-medium text-[var(--accent)] hover:underline"
				>
					Edit Volume
				</button>
			</div>
			<div
				class="flex items-center justify-between rounded-md bg-kumo-elevated px-3 py-2 text-xs ring-1 ring-kumo-hairline"
			>
				<div class="flex items-center gap-2">
					<span class="text-kumo-subtle">Named Volume:</span>
					<code class="font-mono font-medium text-kumo-default">
						{creds.volumeName}
					</code>
				</div>
				<span class="font-mono text-[11px] text-kumo-subtle">
					→ {creds.volumeMountPath}
				</span>
			</div>
		</div>

		<!-- 4. CLI Quick Access Command -->
		<div class="flex flex-col gap-1.5 border-t border-kumo-hairline pt-3">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-1.5">
					<TerminalWindow size={14} class="text-kumo-subtle" />
					<span class="text-xs font-semibold text-kumo-default">CLI Terminal Access</span>
				</div>
				<CopyButton text={cliCommand} />
			</div>
			<div
				class="rounded-md bg-kumo-elevated px-3 py-2 font-mono text-xs text-kumo-subtle ring-1 ring-kumo-hairline"
			>
				<code>{cliCommand}</code>
			</div>
		</div>
	</LayerCard.Primary>
</LayerCard>

<!-- Edit Database Full Configuration Modal -->
<EditDatabaseModal
	bind:open={editModalOpen}
	{service}
	onredeploy={handleRedeploy}
	onupdated={() => void refreshRevealed()}
	onclose={() => (editModalOpen = false)}
/>

<!-- Quick Change Password Modal -->
<ChangePasswordModal
	bind:open={passwordModalOpen}
	{service}
	onredeploy={handleRedeploy}
	onupdated={() => void refreshRevealed()}
	onclose={() => (passwordModalOpen = false)}
/>
