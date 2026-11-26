<script lang="ts">
	import { page } from '$app/state';
	import { PageHeader, Tabs } from '$lib/components/ui';
	import { authStore } from '$lib/stores/auth.svelte';
	import GeneralSettingsTab from '$lib/components/features/settings/GeneralSettingsTab.svelte';
	import EngineSettingsTab from '$lib/components/features/settings/EngineSettingsTab.svelte';
	import BackupSettingsTab from '$lib/components/features/settings/BackupSettingsTab.svelte';
	import SecuritySettingsTab from '$lib/components/features/settings/SecuritySettingsTab.svelte';
	import UpdatesSettingsTab from '$lib/components/features/settings/UpdatesSettingsTab.svelte';
	import NotificationsSettingsTab from '$lib/components/features/settings/NotificationsSettingsTab.svelte';

	let isAdmin = $derived(authStore.user?.role === 'admin');
	let activeTab = $state('general');
	let tabs = $derived(
		isAdmin
			? [
					{ id: 'general', label: 'General' },
					{ id: 'updates', label: 'Updates' },
					{ id: 'engine', label: 'Podman Engine' },
					{ id: 'backups', label: 'Backups' },
					{ id: 'notifications', label: 'Notifications' },
					{ id: 'security', label: 'Security & Sessions' }
				]
			: [{ id: 'security', label: 'Security & Sessions' }]
	);

	$effect(() => {
		const requested = page.url.searchParams.get('tab');
		if (requested && tabs.some((t) => t.id === requested)) {
			activeTab = requested;
			return;
		}
		if (!isAdmin && activeTab !== 'security') {
			activeTab = 'security';
		}
	});
</script>

<svelte:head>
	<title>Settings — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader
		title="System & Host Settings"
		subtitle="Manage instance updates, container engine telemetry, disaster recovery backups, and account security."
	/>
	<Tabs {tabs} bind:active={activeTab} />

	{#if activeTab === 'general' && isAdmin}
		<GeneralSettingsTab />
	{:else if activeTab === 'updates' && isAdmin}
		<UpdatesSettingsTab />
	{:else if activeTab === 'engine' && isAdmin}
		<EngineSettingsTab />
	{:else if activeTab === 'backups' && isAdmin}
		<BackupSettingsTab />
	{:else if activeTab === 'notifications' && isAdmin}
		<NotificationsSettingsTab />
	{:else if activeTab === 'security'}
		<SecuritySettingsTab />
	{/if}
</div>
