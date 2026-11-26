<script lang="ts">
	import { PageHeader, Tabs } from '$lib/components/ui';
	import { authStore } from '$lib/stores/auth.svelte';
	import UsersPermissionsTab from '$lib/components/features/access/UsersPermissionsTab.svelte';
	import ActiveSessionsTab from '$lib/components/features/access/ActiveSessionsTab.svelte';

	let activeTab = $state('users');

	const tabs = [
		{ id: 'users', label: 'Users & Permissions' },
		{ id: 'sessions', label: 'Active Sessions' }
	];
</script>

<svelte:head>
	<title>Access Control — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader
		title="Access Control & Sessions"
		subtitle="Manage fine-grained user permissions, project/service scoping, and active authenticated sessions."
	/>

	{#if authStore.isAdmin}
		<Tabs {tabs} bind:active={activeTab} />
		{#if activeTab === 'users'}
			<UsersPermissionsTab />
		{:else}
			<ActiveSessionsTab scope="admin" />
		{/if}
	{:else}
		<ActiveSessionsTab scope="self" />
	{/if}
</div>

