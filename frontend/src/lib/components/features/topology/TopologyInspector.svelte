<script lang="ts">
	import type { SelectedItem, TopologyNode } from './types';
	import { StatusBadge, CopyButton, Kbd } from '$lib/components/ui';
	import {
		projects,
		services,
		containers,
		pods,
		domains,
		volumes,
		networks,
		server
	} from '$lib/data';
	import {
		X,
		Globe,
		AppWindow,
		Cube,
		HardDrive,
		TreeStructure,
		Folder,
		Stack,
		Plugs
	} from 'phosphor-svelte';

	import ProjectInspector from './inspectors/ProjectInspector.svelte';
	import ServiceInspector from './inspectors/ServiceInspector.svelte';
	import ContainerInspector from './inspectors/ContainerInspector.svelte';
	import PortInspector from './inspectors/PortInspector.svelte';
	import DomainInspector from './inspectors/DomainInspector.svelte';
	import ProxyInspector from './inspectors/ProxyInspector.svelte';
	import InternetInspector from './inspectors/InternetInspector.svelte';

	interface Props {
		selected: SelectedItem;
		onclose: () => void;
		onFocusProject?: (projectId: string) => void;
	}

	let { selected, onclose, onFocusProject }: Props = $props();

	let node = $derived(selected?.kind === 'node' ? selected.item : null);
	let region = $derived(selected?.kind === 'region' ? selected.item : null);
	let raw = $derived((node?.raw || region?.raw) as any);

	// Factual data resolution from reactive stores
	let matchedProject = $derived.by(() => {
		if (node?.groupType === 'project' || region?.type === 'project') {
			const pid = node?.projectId || region?.projectId || node?.id?.replace('group-project-', '');
			return projects.find((p) => p.id === pid) || null;
		}
		if (node?.projectId) {
			return projects.find((p) => p.id === node.projectId) || null;
		}
		return null;
	});

	let matchedService = $derived.by(() => {
		if (node?.type === 'service' || (node?.type === 'group' && node.groupType !== 'project')) {
			const sid = node.serviceId || node.id.replace(/^group-/, '').replace(/^svc-/, '');
			return services.find((s) => s.id === sid) || null;
		}
		if (node?.serviceId) {
			return services.find((s) => s.id === node.serviceId) || null;
		}
		return null;
	});

	let matchedContainer = $derived.by(() => {
		if (node?.type === 'container') {
			const cid = node.id.replace(/^container-/, '');
			return (
				containers.find((c) => c.id === cid || c.name === node.title || c.name === cid) ||
				(node.raw as any)
			);
		}
		return null;
	});

	let projectServicesList = $derived.by(() => {
		if (!matchedProject) return [];
		return services.filter((s) => s.projectId === matchedProject.id);
	});

	let projectContainersList = $derived.by(() => {
		if (!matchedProject) return [];
		return containers.filter((c) => c.projectId === matchedProject.id);
	});

	let projectDomainsList = $derived.by(() => {
		if (!matchedProject) return [];
		return domains.filter((d) => d.projectId === matchedProject.id);
	});

	let projectVolumesList = $derived.by(() => {
		if (!matchedProject) return [];
		return volumes.filter((v) => v.projectId === matchedProject.id);
	});

	let serviceContainersList = $derived.by(() => {
		if (!matchedService) return [];
		const matched = containers.filter(
			(c) =>
				c.serviceId === matchedService.id ||
				c.name === matchedService.name ||
				c.name.startsWith(`${matchedService.id}-`) ||
				c.name.startsWith(`${matchedService.id}_`) ||
				(c.labels &&
					(c.labels['io.gopod.service'] === matchedService.id ||
						c.labels['com.docker.compose.service'] === matchedService.name))
		);
		if (matched.length > 0) return matched;
		if (matchedService.workloads && matchedService.workloads.length > 0) {
			return matchedService.workloads.map((w: any) => ({
				id: w.name,
				name: w.name,
				image: w.image || 'container:latest',
				status: w.status || matchedService.status,
				cpu: w.cpu || 0,
				memory: w.memory || 0,
				ports: w.ports || ''
			}));
		}
		return [];
	});

	let serviceDomainsList = $derived.by(() => {
		if (!matchedService) return [];
		return domains.filter((d) => d.serviceId === matchedService.id);
	});

	let projectPublishedPorts = $derived.by(() => {
		if (!matchedProject) return [];
		const portsList: {
			hostPort: number;
			containerPort: number;
			ip: string;
			serviceName: string;
		}[] = [];
		const seen = new Set<string>();

		projectContainersList.forEach((c) => {
			if (c.ports) {
				const pList = Array.isArray(c.ports) ? c.ports : [c.ports];
				pList.forEach((pStr: string) => {
					const parts = String(pStr).split(':');
					if (parts.length >= 2) {
						const hostPort = parseInt(parts[parts.length - 2], 10);
						const containerPort = parseInt(parts[parts.length - 1].split('/')[0], 10);
						const ip = parts.length > 2 ? parts[0] : '0.0.0.0';
						const key = `${ip}:${hostPort}`;
						if (!seen.has(key) && !isNaN(hostPort)) {
							seen.add(key);
							portsList.push({
								hostPort,
								containerPort,
								ip,
								serviceName: c.name
							});
						}
					}
				});
			}
		});
		return portsList;
	});

	function getCategoryMeta(targetNode: TopologyNode) {
		const isCompose =
			targetNode.subtitle?.toLowerCase().includes('compose') ||
			matchedService?.type === 'compose' ||
			targetNode.raw?.type === 'compose';

		if (targetNode.type === 'internet') {
			return { label: 'Public Gateway', icon: Globe, color: 'text-[var(--status-blue)]' };
		}
		if (targetNode.type === 'proxy') {
			return { label: 'Caddy Reverse Proxy', icon: Globe, color: 'text-[var(--accent)]' };
		}
		if (targetNode.type === 'port') {
			return { label: 'Host Port Binding', icon: Plugs, color: 'text-[var(--status-amber)]' };
		}
		if (targetNode.type === 'domain') {
			return { label: 'Ingress Domain', icon: Globe, color: 'text-[var(--accent)]' };
		}
		if (targetNode.type === 'group' && targetNode.groupType === 'project') {
			return { label: 'Project Space', icon: Folder, color: 'text-[var(--accent)]' };
		}
		if (targetNode.type === 'group') {
			return { label: 'Workload Stack', icon: Stack, color: 'text-[var(--accent)]' };
		}
		if (isCompose) {
			return { label: 'Compose Service', icon: Stack, color: 'text-[var(--accent)]' };
		}
		if (targetNode.type === 'container') {
			return { label: 'Container Instance', icon: Cube, color: 'text-[var(--status-green)]' };
		}
		if (targetNode.type === 'volume') {
			return { label: 'Storage Volume', icon: HardDrive, color: 'text-[var(--status-amber)]' };
		}
		if (targetNode.type === 'network') {
			return { label: 'Bridge Network', icon: TreeStructure, color: 'text-[var(--status-blue)]' };
		}
		return { label: 'Service', icon: AppWindow, color: 'text-[var(--text-secondary)]' };
	}
</script>

{#if selected}
	{@const meta = node
		? getCategoryMeta(node)
		: {
				label: region?.type === 'project' ? 'Project Space' : 'Podman Pod',
				icon: region?.type === 'project' ? Folder : TreeStructure,
				color: 'text-[var(--accent)]'
			}}
	{@const CategoryIcon = meta.icon}
	{@const title = node ? node.title : region?.label || 'Resource'}
	{@const status = node ? node.status : region?.status}

	<aside
		class="z-20 flex h-full w-full flex-col overflow-hidden border-l border-[var(--border)] bg-[var(--bg-shell)] select-text"
	>
		<!-- 1. Drawer Header -->
		<div
			class="flex shrink-0 items-center justify-between border-b border-[var(--border)] bg-[var(--bg-panel)] px-4 py-3"
		>
			<div class="flex min-w-0 items-center gap-2">
				<div
					class="flex h-7 w-7 shrink-0 items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] {meta.color}"
				>
					<CategoryIcon size={15} weight="bold" />
				</div>
				<div class="min-w-0">
					<span class="block truncate text-xs font-semibold text-[var(--text-tertiary)]">
						{meta.label}
					</span>
				</div>
			</div>

			<!-- Close Action with Keyboard Esc Hint -->
			<div class="flex shrink-0 items-center gap-1.5">
				<Kbd class="hidden sm:inline-block">Esc</Kbd>
				<button
					type="button"
					onclick={(e) => {
						e.stopPropagation();
						onclose();
					}}
					class="flex h-7 w-7 cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
					aria-label="Close drawer"
				>
					<X size={14} />
				</button>
			</div>
		</div>

		<!-- 2. Scrollable Body Content -->
		<div class="flex flex-1 flex-col gap-4 overflow-y-auto p-4 text-left">
			<!-- Title & High-Level Identity -->
			<div class="flex flex-col gap-1.5 border-b border-[var(--border-subtle)] pb-3">
				<div class="flex items-start justify-between gap-2">
					<h3 class="text-base leading-snug font-semibold break-words text-[var(--text-primary)]">
						{title}
					</h3>
					{#if status}
						<div class="shrink-0 pt-0.5">
							<StatusBadge {status} size="sm" />
						</div>
					{/if}
				</div>

				{#if node?.subtitle || region?.sublabel}
					<p class="text-xs leading-relaxed text-[var(--text-secondary)]">
						{node?.subtitle || region?.sublabel}
					</p>
				{/if}
			</div>

			<!-- Entity Inspector Dispatcher -->
			{#if (node && node.groupType === 'project') || (region && region.type === 'project')}
				<ProjectInspector
					proj={matchedProject || raw}
					services={projectServicesList}
					containers={projectContainersList}
					domains={projectDomainsList}
					volumes={projectVolumesList}
					publishedPorts={projectPublishedPorts}
					{onFocusProject}
				/>
			{:else if (node && (node.type === 'service' || (node.type === 'group' && node.groupType !== 'project'))) || (region && region.type === 'pod')}
				<ServiceInspector
					service={matchedService || raw}
					containers={serviceContainersList}
					domains={serviceDomainsList}
					{raw}
				/>
			{:else if node && node.type === 'container'}
				<ContainerInspector {node} cnt={matchedContainer} {raw} />
			{:else if node && node.type === 'port'}
				<PortInspector {node} />
			{:else if node && node.type === 'domain'}
				<DomainInspector {node} {raw} />
			{:else if node && node.type === 'proxy'}
				<ProxyInspector {server} {domains} />
			{:else if node && node.type === 'internet'}
				<InternetInspector {server} />
			{/if}

			<!-- Metadata Identification Footer -->
			{#if node?.id}
				<div
					class="flex items-center justify-between border-t border-[var(--border-subtle)] pt-2 text-[10.5px] text-[var(--text-tertiary)]"
				>
					<span>ID: <code class="font-mono text-[var(--text-secondary)]">{node.id}</code></span>
					<CopyButton text={node.id} size={11} label="Copy ID" variant="inline" />
				</div>
			{/if}
		</div>
	</aside>
{/if}
