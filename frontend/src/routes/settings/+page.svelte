<script lang="ts">
	import { PageHeader, Tabs } from '$lib/components/ui';
	import GeneralSettingsTab from '$lib/components/features/settings/GeneralSettingsTab.svelte';
	import EngineSettingsTab from '$lib/components/features/settings/EngineSettingsTab.svelte';
	import BackupSettingsTab from '$lib/components/features/settings/BackupSettingsTab.svelte';
	import SecuritySettingsTab from '$lib/components/features/settings/SecuritySettingsTab.svelte';

	let activeTab = $state('general');
	const tabs = [
		{ id: 'general', label: 'General' },
		{ id: 'engine', label: 'Podman Engine' },
		{ id: 'backups', label: 'Backups' },
		{ id: 'security', label: 'Security' }
	];
</script>

<svelte:head>
	<title>Settings — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader
		title="System & Host Settings"
		subtitle="Manage instance parameters, container engine telemetry, disaster recovery backups, and host security."
	/>
	<Tabs {tabs} bind:active={activeTab} />

	{#if activeTab === 'general'}
		<GeneralSettingsTab />
	{:else if activeTab === 'engine'}
		<EngineSettingsTab />
	{:else if activeTab === 'backups'}
		<BackupSettingsTab />
	{:else if activeTab === 'security'}
		<SecuritySettingsTab />
	{/if}
</div>
