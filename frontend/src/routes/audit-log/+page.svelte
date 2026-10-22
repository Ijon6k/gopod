<script lang="ts">
	import { onMount } from 'svelte';
	import { PageHeader, Tabs } from '$lib/components/ui';
	import { api } from '$lib/api';
	import type { Project, Service } from '$lib/types';
	import ActivityView from '$lib/components/features/audit/ActivityView.svelte';
	import ConsoleView from '$lib/components/features/audit/ConsoleView.svelte';

	let activeView = $state<'activity' | 'console'>('activity');

	// Filter option data, fetched once and shared by both views.
	let projects = $state<Project[]>([]);
	let services = $state<Service[]>([]);

	const mainTabs = [
		{ id: 'activity', label: 'Activity' },
		{ id: 'console', label: 'Console' }
	];

	onMount(async () => {
		try {
			projects = await api.projects.list();
		} catch {
			projects = [];
		}
		try {
			services = await api.services.list();
		} catch {
			services = [];
		}
	});
</script>

<svelte:head>
	<title>Audit Log - GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader title="Audit Log" subtitle="Operator activity timeline and engine console stream." />

	<Tabs tabs={mainTabs} bind:active={activeView} />

	{#if activeView === 'activity'}
		<ActivityView {projects} {services} />
	{:else}
		<ConsoleView {services} />
	{/if}
</div>
