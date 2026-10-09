<script lang="ts">
	import type { Service } from '$lib/types';
	import DeploySettingsCard from './DeploySettingsCard.svelte';
	import QuadletEditorCard from './QuadletEditorCard.svelte';
	import ComposeEditorCard from './ComposeEditorCard.svelte';
	import SourceCard from './SourceCard.svelte';
	import PortMappingCard from './PortMappingCard.svelte';
	import BuildTypeCard from './BuildTypeCard.svelte';
	import RuntimeTargetCard from './RuntimeTargetCard.svelte';
	import DeployTriggerCard from './DeployTriggerCard.svelte';
	import AddCredentialModal from './AddCredentialModal.svelte';

	interface Props {
		service: Service;
		onNavigateTab?: (tabId: string) => void;
		onTerminalClick?: () => void;
		onRedeploy?: () => void;
	}

	let { service, onNavigateTab: _onNavigateTab, onTerminalClick, onRedeploy }: Props = $props();

	let credentialModalOpen = $state(false);
	let credentialInitialTab = $state<'ssh' | 'registry'>('ssh');

	function handleOpenCredentialModal(type: 'ssh' | 'registry') {
		credentialInitialTab = type;
		credentialModalOpen = true;
	}
</script>

<div class="flex w-full flex-col gap-6">
	<!-- 1. Dokploy-style Operational Action Bar at top of General tab -->
	<DeploySettingsCard {service} {onTerminalClick} {onRedeploy} />

	<!-- 2. Main Workload Body depending on Type -->
	{#if service.type === 'quadlet'}
		<!-- First-Class Quadlet Systemd Editor with Dokploy layout -->
		<QuadletEditorCard {service} />
	{:else if service.type === 'compose' || service.type === 'kubernetes'}
		<!-- First-Class Docker / Podman Compose Manifest with Dokploy layout -->
		<ComposeEditorCard {service} />
	{:else}
		<!-- Standard Application / Image / Container Workloads -->
		<SourceCard {service} onOpenCredentialModal={handleOpenCredentialModal} />

		<!-- Dedicated Port Configuration Card -->
		<PortMappingCard {service} />

		{#if (service.sourceType || 'git') !== 'image'}
			<BuildTypeCard {service} />
		{/if}

		<RuntimeTargetCard {service} />

		<DeployTriggerCard {service} />
	{/if}
</div>

<!-- Modal to add SSH key or Registry in-context without leaving page -->
<AddCredentialModal bind:open={credentialModalOpen} initialTab={credentialInitialTab} />
