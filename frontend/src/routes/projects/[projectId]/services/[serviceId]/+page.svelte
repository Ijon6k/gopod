<script lang="ts">
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { Tabs } from '$lib/components/ui';
	import { getServiceById, getProjectById, dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import type { Service, Project } from '$lib/types';
	import ServiceHeader from '$lib/components/features/services/detail/ServiceHeader.svelte';
	import GeneralTab from '$lib/components/features/services/detail/tabs/GeneralTab.svelte';
	import EnvironmentTab from '$lib/components/features/services/detail/tabs/EnvironmentTab.svelte';
	import QuadletTab from '$lib/components/features/services/detail/tabs/QuadletTab.svelte';
	import DomainsTab from '$lib/components/features/services/detail/tabs/DomainsTab.svelte';
	import DeploymentsTab from '$lib/components/features/services/detail/tabs/DeploymentsTab.svelte';
	import MonitoringTab from '$lib/components/features/services/detail/tabs/MonitoringTab.svelte';
	import LogsTab from '$lib/components/features/services/detail/tabs/LogsTab.svelte';
	import TerminalTab from '$lib/components/features/services/detail/tabs/TerminalTab.svelte';
	import AdvancedTab from '$lib/components/features/services/detail/tabs/AdvancedTab.svelte';
	import BackupsTab from '$lib/components/features/services/detail/tabs/BackupsTab.svelte';
	import { SpinnerGap } from 'phosphor-svelte';
	import { createPoller } from '$lib/utils/poller';

	let serviceId = $derived(page.params.serviceId ?? '');
	let projectId = $derived(page.params.projectId ?? '');

	let directService = $state<Service | null>(null);
	let directProject = $state<Project | null>(null);
	let isLoading = $state(true);
	let lastResolvedServiceId = '';

	let service = $derived((serviceId ? getServiceById(serviceId) : undefined) ?? directService);
	let project = $derived(
		(projectId ? getProjectById(projectId) : undefined) ??
			(service ? getProjectById(service.projectId) : undefined) ??
			directProject
	);

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
				const storeProj = pId
					? getProjectById(pId)
					: storeSvc
						? getProjectById(storeSvc.projectId)
						: undefined;

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
						storeProj
							? Promise.resolve(storeProj)
							: pId
								? api.projects.get(pId).catch(() => null)
								: null
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
						} catch {}
					}
				} catch {
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

	// Periodic status polling to guarantee zero desynchronization with the
	// Podman daemon. Runs through createPoller so ticks stay latched (a slow
	// response never overlaps the next one); errored ticks are console-warned
	// by the poller and the loop keeps running. The effect keys on the
	// transitional-state flag too: when it flips, the poller restarts at the
	// matching cadence instead of keeping a frozen interval forever.
	let serviceTransitional = $derived(
		service?.status === 'deploying' ||
			service?.status === 'building' ||
			service?.status === 'stopping' ||
			service?.status === 'starting'
	);

	$effect(() => {
		const sId = serviceId;
		// Read the transition flag so a cadence change re-arms the poller.
		const transitional = serviceTransitional;
		if (!sId) return;

		let active = true;
		// No local try/catch: createPoller owns failed ticks (console-warn +
		// loop keeps running), which replaces the previous silent `catch {}`.
		const poll = async () => {
			const freshSvc = await api.services.get(sId);
			if (!active || !freshSvc) return;
			if (directService && directService.status !== freshSvc.status) {
				if (directService.status === 'stopping' && freshSvc.status === 'running') {
					// Keep local transitional state until stopped
				} else {
					directService.status = freshSvc.status;
				}
			}
			const idx = dataStore.services.findIndex((s) => s.id === freshSvc.id);
			if (idx >= 0 && dataStore.services[idx].status !== freshSvc.status) {
				if (dataStore.services[idx].status === 'stopping' && freshSvc.status === 'running') {
					// Keep local transitional state until stopped
				} else {
					dataStore.services[idx].status = freshSvc.status;
				}
			}

			// Full manifest sync (not just status): another tab/device may
			// have saved ports or manifests. Skipped while transitional or
			// while the user is typing, so local edits and in-progress
			// editors are never clobbered.
			const cur = dataStore.services.find((s) => s.id === freshSvc.id);
			const transitionalNow =
				cur?.status === 'deploying' ||
				cur?.status === 'building' ||
				cur?.status === 'stopping' ||
				cur?.status === 'starting';
			const ae = typeof document !== 'undefined' ? document.activeElement : null;
			const typing =
				!!ae &&
				(ae.tagName === 'INPUT' ||
					ae.tagName === 'TEXTAREA' ||
					ae.tagName === 'SELECT' ||
					ae.getAttribute?.('contenteditable') === 'true' ||
					!!ae.closest?.('.cm-editor'));
			if (!transitionalNow && !typing) {
				if (directService && directService.id === freshSvc.id) {
					directService = {
						...freshSvc,
						deployments: directService.deployments ?? freshSvc.deployments ?? []
					};
				}
				if (idx >= 0) {
					dataStore.services[idx] = {
						...freshSvc,
						deployments: dataStore.services[idx].deployments ?? freshSvc.deployments ?? []
					};
				}
			}
		};

		const poller = createPoller(transitional ? 1500 : 4000, poll);
		poller.start();

		return () => {
			active = false;
			poller.stop();
		};
	});

	let activeTab = $state('general');

	const tabs = [
		{ id: 'general', label: 'General' },
		{ id: 'environment', label: 'Environment' },
		{ id: 'quadlet', label: 'Quadlet' },
		{ id: 'domains', label: 'Domains' },
		{ id: 'deployments', label: 'Deployments' },
		{ id: 'monitoring', label: 'Monitoring' },
		{ id: 'logs', label: 'Logs' },
		{ id: 'backups', label: 'Backups' },
		{ id: 'advanced', label: 'Advanced' }
	];

	async function handleRedeploy() {
		if (!service) return;
		try {
			await dataStore.deployService(service.id, 'manual');
		} catch (err: unknown) {
			console.error('[GOPOD] Deployment trigger error:', err);
		}
	}
</script>

<svelte:head>
	<title>{service?.name ?? 'Service'} — {project?.name ?? 'Project'} — GOPOD</title>
</svelte:head>

{#if isLoading && !service}
	<div class="flex flex-col items-center justify-center gap-3 py-24 text-[var(--text-tertiary)]">
		<SpinnerGap size={24} class="animate-spin text-[var(--accent)]" />
		<span class="text-xs">Loading service…</span>
	</div>
{:else if service && project}
	<div class="flex w-full flex-col gap-6">
		<!-- Compact Operational Header -->
		<ServiceHeader
			{service}
			{project}
			onTerminalClick={() => (activeTab = 'terminal')}
			onRedeploy={handleRedeploy}
		/>

		<!-- Tabs Bar -->
		<div class="relative flex items-center justify-between">
			<Tabs {tabs} bind:active={activeTab} />

			{#if activeTab === 'terminal'}
				<span
					class="absolute right-0 bottom-2.5 text-xs font-[var(--font-mono)] font-medium text-[var(--accent)]"
				>
					[Terminal Session]
				</span>
			{/if}
		</div>

		<!-- Tab Contents -->
		<div class="pt-1">
			{#if activeTab === 'general'}
				<GeneralTab
					{service}
					onNavigateTab={(tab: string) => (activeTab = tab)}
					onTerminalClick={() => (activeTab = 'terminal')}
					onRedeploy={handleRedeploy}
				/>
			{:else if activeTab === 'environment'}
				<EnvironmentTab {service} />
			{:else if activeTab === 'quadlet'}
				<QuadletTab {service} />
			{:else if activeTab === 'domains'}
				<DomainsTab {service} />
			{:else if activeTab === 'deployments'}
				<DeploymentsTab {service} onRedeploy={handleRedeploy} />
			{:else if activeTab === 'monitoring'}
				<MonitoringTab {service} onNavigateTab={(tab: string) => (activeTab = tab)} />
			{:else if activeTab === 'logs'}
				<LogsTab {service} />
			{:else if activeTab === 'backups'}
				<BackupsTab {service} />
			{:else if activeTab === 'terminal'}
				<TerminalTab {service} />
			{:else if activeTab === 'advanced'}
				<AdvancedTab {service} onNavigateTab={(tab: string) => (activeTab = tab)} />
			{/if}
		</div>
	</div>
{:else}
	<div
		class="flex flex-col items-center justify-center gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-12 text-center"
	>
		<span class="text-sm font-medium text-[var(--text-primary)]">Service not found</span>
		<p class="text-xs text-[var(--text-secondary)]">
			The requested service could not be loaded or does not exist.
		</p>
	</div>
{/if}
