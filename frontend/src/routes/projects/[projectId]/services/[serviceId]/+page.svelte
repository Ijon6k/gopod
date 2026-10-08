<script lang="ts">
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { Tabs } from '$lib/components/ui';
	import { getServiceById, getProjectById, dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import type { Service, Project } from '$lib/types';
	import ServiceHeader from '$lib/components/features/services/detail/ServiceHeader.svelte';
	import GeneralTab from '$lib/components/features/services/detail/GeneralTab.svelte';
	import EnvironmentTab from '$lib/components/features/services/detail/EnvironmentTab.svelte';
	import DomainsTab from '$lib/components/features/services/detail/DomainsTab.svelte';
	import DeploymentsTab from '$lib/components/features/services/detail/DeploymentsTab.svelte';
	import MonitoringTab from '$lib/components/features/services/detail/MonitoringTab.svelte';
	import LogsTab from '$lib/components/features/services/detail/LogsTab.svelte';
	import TerminalTab from '$lib/components/features/services/detail/TerminalTab.svelte';
	import AdvancedTab from '$lib/components/features/services/detail/AdvancedTab.svelte';
	import { SpinnerGap } from 'phosphor-svelte';

	let serviceId = $derived(page.params.serviceId ?? '');
	let projectId = $derived(page.params.projectId ?? '');

	let directService = $state<Service | null>(null);
	let directProject = $state<Project | null>(null);
	let isLoading = $state(true);
	let lastResolvedServiceId = '';

	let service = $derived(directService ?? (serviceId ? getServiceById(serviceId) : undefined));
	let project = $derived(directProject ?? (projectId ? getProjectById(projectId) : (service ? getProjectById(service.projectId) : undefined)));

	$effect(() => {
		const sId = serviceId;
		const pId = projectId;
		if (!sId) return;

		let active = true;

		untrack(() => {
			if (lastResolvedServiceId === sId) return;
			lastResolvedServiceId = sId;

			async function resolveData() {
				const storeSvc = getServiceById(sId);
				const storeProj = pId ? getProjectById(pId) : (storeSvc ? getProjectById(storeSvc.projectId) : undefined);

				if (storeSvc && storeProj) {
					directService = storeSvc;
					directProject = storeProj;
					isLoading = false;
				} else {
					isLoading = true;
				}

				try {
					const [fetchedSvc, fetchedProj] = await Promise.all([
						api.services.get(sId).catch(() => null),
						storeProj ? Promise.resolve(storeProj) : (pId ? api.projects.get(pId).catch(() => null) : null)
					]);

					if (!active) return;

					if (fetchedSvc) {
						if (!fetchedSvc.deployments) fetchedSvc.deployments = [];
						directService = fetchedSvc;

						const idx = dataStore.services.findIndex((s) => s.id === fetchedSvc.id);
						if (idx >= 0) {
							dataStore.services[idx] = fetchedSvc;
						} else {
							dataStore.services.push(fetchedSvc);
						}
					}

					if (fetchedProj) {
						const fullProj: Project = {
							id: fetchedProj.id,
							name: fetchedProj.name,
							description: fetchedProj.description || '',
							status: 'healthy',
							services: [],
							domains: [],
							cpu: 0,
							memory: 0,
							memoryTotal: 0,
							createdAt: fetchedProj.createdAt || new Date().toISOString()
						};
						directProject = fullProj;

						const pIdx = dataStore.projects.findIndex((p) => p.id === fullProj.id);
						if (pIdx >= 0) {
							dataStore.projects[pIdx] = fullProj;
						} else {
							dataStore.projects.push(fullProj);
						}
					} else if (fetchedSvc && !directProject) {
						try {
							const parentProj = await api.projects.get(fetchedSvc.projectId);
							if (active && parentProj) {
								const fullProj: Project = {
									id: parentProj.id,
									name: parentProj.name,
									description: parentProj.description || '',
									status: 'healthy',
									services: [],
									domains: [],
									cpu: 0,
									memory: 0,
									memoryTotal: 0,
									createdAt: parentProj.createdAt || new Date().toISOString()
								};
								directProject = fullProj;
							}
						} catch (_) {}
					}
				} catch (_) {
					// Silently handle error
				} finally {
					if (active) isLoading = false;
				}
			}

			resolveData();
		});

		return () => {
			active = false;
		};
	});

	// Periodic status polling to guarantee zero desynchronization with Podman daemon (Dokploy parity)
	$effect(() => {
		const sId = serviceId;
		if (!sId) return;

		let active = true;
		const poll = async () => {
			try {
				const freshSvc = await api.services.get(sId);
				if (!active || !freshSvc) return;
				if (directService && directService.status !== freshSvc.status) {
					directService.status = freshSvc.status;
				}
				const idx = dataStore.services.findIndex((s) => s.id === freshSvc.id);
				if (idx >= 0 && dataStore.services[idx].status !== freshSvc.status) {
					dataStore.services[idx].status = freshSvc.status;
				}
			} catch (_) {}
		};

		const isTransitional = service?.status === 'deploying' || service?.status === 'building';
		const interval = setInterval(poll, isTransitional ? 1500 : 4000);

		return () => {
			active = false;
			clearInterval(interval);
		};
	});

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

	async function handleRedeploy() {
		if (!service) return;
		try {
			await dataStore.deployService(service.id, 'manual');
		} catch (err: any) {
			console.error('[GOPOD] Deployment trigger error:', err);
		}
	}
</script>

<svelte:head>
	<title>{service?.name ?? 'Service'} — {project?.name ?? 'Project'} — GOPOD</title>
</svelte:head>

{#if isLoading && !service}
	<div class="flex flex-col items-center justify-center py-24 gap-3 text-[var(--text-tertiary)]">
		<SpinnerGap size={24} class="animate-spin text-[var(--accent)]" />
		<span class="text-xs">Loading service…</span>
	</div>
{:else if service && project}
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
				<GeneralTab
					{service}
					onNavigateTab={(tab) => (activeTab = tab)}
					onTerminalClick={() => (activeTab = 'terminal')}
					onRedeploy={handleRedeploy}
				/>
			{:else if activeTab === 'environment'}
				<EnvironmentTab {service} />
			{:else if activeTab === 'domains'}
				<DomainsTab {service} />
			{:else if activeTab === 'deployments'}
				<DeploymentsTab {service} onRedeploy={handleRedeploy} />
			{:else if activeTab === 'monitoring'}
				<MonitoringTab {service} onNavigateTab={(tab) => (activeTab = tab)} />
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
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-12 text-center flex flex-col items-center justify-center gap-2">
		<span class="text-sm font-medium text-[var(--text-primary)]">Service not found</span>
		<p class="text-xs text-[var(--text-secondary)]">The requested service could not be loaded or does not exist.</p>
	</div>
{/if}
