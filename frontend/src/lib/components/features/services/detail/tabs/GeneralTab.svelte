<script lang="ts">
	import type { Service } from '$lib/types';
	import DeploySettingsCard from '../DeploySettingsCard.svelte';
	import QuadletEditorCard from '../QuadletEditorCard.svelte';
	import ComposeEditorCard from '../ComposeEditorCard.svelte';
	import PodWorkspaceCard from '../PodWorkspaceCard.svelte';
	import ApplicationConfigCard from '../ApplicationConfigCard.svelte';
	import DatabaseCredentialsCard from '../DatabaseCredentialsCard.svelte';
	import DatabaseSetupCatalog from '../DatabaseSetupCatalog.svelte';
	import AddCredentialModal from '../modals/AddCredentialModal.svelte';

	interface Props {
		service: Service;
		onNavigateTab?: (tabId: string) => void;
		onTerminalClick?: () => void;
		onRedeploy?: () => void;
	}

	let { service, onNavigateTab: _onNavigateTab, onTerminalClick, onRedeploy }: Props = $props();

	let credentialModalOpen = $state(false);
	let credentialInitialTab = $state<'ssh' | 'registry'>('ssh');
	let isChangingEngine = $state(false);

	function handleOpenCredentialModal(type: 'ssh' | 'registry') {
		credentialInitialTab = type;
		credentialModalOpen = true;
	}
</script>

<div class="flex w-full flex-col gap-4">
	<!-- 1. Operational Action Bar at top of General tab (hidden if engine setup is active) -->
	{#if service.type !== 'database' || (service.image && !isChangingEngine)}
		<DeploySettingsCard {service} {onTerminalClick} {onRedeploy} />
	{/if}

	<!-- 2. Main Workload Body depending on Type -->
	{#if service.type === 'pod'}
		<!-- First-Class Pod Multi-Container Workspace -->
		<PodWorkspaceCard {service} />
	{:else if service.type === 'quadlet'}
		<!-- First-Class Quadlet Systemd Editor -->
		<QuadletEditorCard {service} />
	{:else if service.type === 'compose' || service.type === 'kubernetes'}
		<!-- First-Class Docker / Podman Compose Manifest -->
		<ComposeEditorCard {service} />
	{:else if service.type === 'database'}
		<!-- First-Class Dedicated Database Instance (Dokploy Parity) -->
		{#if !service.image || isChangingEngine}
			<DatabaseSetupCatalog
				{service}
				onConfigured={() => (isChangingEngine = false)}
				onCancel={service.image ? () => (isChangingEngine = false) : undefined}
			/>
		{:else}
			<DatabaseCredentialsCard
				{service}
				{onRedeploy}
				onChangeEngine={() => (isChangingEngine = true)}
			/>
			<ApplicationConfigCard {service} onOpenCredentialModal={handleOpenCredentialModal} />
		{/if}
	{:else}
		<!-- Standard Application / Image / Container Workloads (Unified Kumo UI) -->
		<ApplicationConfigCard {service} onOpenCredentialModal={handleOpenCredentialModal} />
	{/if}
</div>

<!-- Modal to add SSH key or Registry in-context without leaving page -->
<AddCredentialModal bind:open={credentialModalOpen} initialTab={credentialInitialTab} />
