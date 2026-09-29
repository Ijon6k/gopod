<script lang="ts">
	import { page } from '$app/state';
	import { Tabs } from '$lib/components/ui';
	import { getServiceById, getProjectById, dataStore } from '$lib/data';
	import ServiceHeader from '$lib/components/features/services/detail/ServiceHeader.svelte';
	import GeneralTab from '$lib/components/features/services/detail/GeneralTab.svelte';
	import EnvironmentTab from '$lib/components/features/services/detail/EnvironmentTab.svelte';
	import DomainsTab from '$lib/components/features/services/detail/DomainsTab.svelte';
	import DeploymentsTab from '$lib/components/features/services/detail/DeploymentsTab.svelte';
	import MonitoringTab from '$lib/components/features/services/detail/MonitoringTab.svelte';
	import LogsTab from '$lib/components/features/services/detail/LogsTab.svelte';
	import TerminalTab from '$lib/components/features/services/detail/TerminalTab.svelte';
	import AdvancedTab from '$lib/components/features/services/detail/AdvancedTab.svelte';

	let serviceId = $derived(page.params.serviceId ?? '');
	let service = $derived(serviceId ? getServiceById(serviceId) : undefined);
	let project = $derived(service ? getProjectById(service.projectId) : undefined);

	let activeTab = $state('general');

	const tabs = [
		{ id: 'general', label: 'General' },
		{ id: 'environment', label: 'Environment' },
		{ id: 'domains', label: 'Domains' },
		{ id: 'deployments', label: 'Deployments' },
		{ id: 'monitoring', label: 'Monitoring' },
		{ id: 'logs', label: 'Logs' },
		{ id: 'advanced', label: 'Advanced' }
	];

	function handleRedeploy() {
		if (!service) return;
		service.status = 'deploying';

		const newDep: import('$lib/types').Deployment = {
			id: `dep-${service.id}-${Date.now()}`,
			projectId: service.projectId,
			projectName: project?.name ?? service.projectId,
			serviceId: service.id,
			serviceName: service.name,
			number: (service.deployments?.length ?? 0) + 1,
			version: `v1.${(service.deployments?.length ?? 0) + 1}.0`,
			commit: Math.random().toString(16).substring(2, 9),
			commitMessage: 'Triggered manual redeployment',
			branch: service.branch ?? 'main',
			status: 'deploying',
			duration: 'Running…',
			timeAgo: 'Just now',
			startedAt: new Date().toISOString(),
			finishedAt: ''
		};

		dataStore.deployments.unshift(newDep);

		setTimeout(() => {
			if (service) {
				service.status = 'running';
				newDep.status = 'running';
				newDep.duration = '34s';
			}
		}, 1800);
	}
</script>

<svelte:head>
	<title>{service?.name ?? 'Service'} — {project?.name ?? 'Project'} — GOPOD</title>
</svelte:head>

{#if service && project}
	<div class="w-full flex flex-col gap-6">
		<!-- Compact Operational Header -->
		<ServiceHeader
			{service}
			{project}
			onTerminalClick={() => (activeTab = 'terminal')}
			onRedeploy={handleRedeploy}
		/>

		<!-- Tabs Bar -->
		<div class="flex items-center justify-between border-b border-[var(--border)] pb-0">
			<Tabs {tabs} bind:active={activeTab} />

			{#if activeTab === 'terminal'}
				<span class="text-xs text-[var(--accent)] font-medium font-[var(--font-mono)] pb-2 pr-1">
					[Terminal Session]
				</span>
			{/if}
		</div>

		<!-- Tab Contents -->
		<div class="pt-1">
			{#if activeTab === 'general'}
				<GeneralTab {service} onNavigateTab={(tab) => (activeTab = tab)} />
			{:else if activeTab === 'environment'}
				<EnvironmentTab {service} />
			{:else if activeTab === 'domains'}
				<DomainsTab {service} />
			{:else if activeTab === 'deployments'}
				<DeploymentsTab {service} onRedeploy={handleRedeploy} />
			{:else if activeTab === 'monitoring'}
				<MonitoringTab {service} />
			{:else if activeTab === 'logs'}
				<LogsTab {service} />
			{:else if activeTab === 'terminal'}
				<TerminalTab {service} />
			{:else if activeTab === 'advanced'}
				<AdvancedTab {service} />
			{/if}
		</div>
	</div>
{:else}
	<div class="text-[var(--text-tertiary)] py-12 text-center text-xs">
		Service not found.
	</div>
{/if}
