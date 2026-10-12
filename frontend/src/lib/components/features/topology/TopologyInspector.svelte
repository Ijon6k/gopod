<script lang="ts">
	import type { SelectedItem, TopologyNode } from './types';
	import { goto } from '$app/navigation';
	import { Button, Badge } from 'kumo-svelte';
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
		ArrowRight,
		ArrowSquareOut,
		Globe,
		AppWindow,
		Cube,
		HardDrive,
		TreeStructure,
		ShieldCheck,
		Folder,
		Cpu,
		Database,
		FileText,
		Stack,
		Plugs,
		Terminal,
		Lock,
		LockOpen
	} from 'phosphor-svelte';

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

	let serviceVolumesList = $derived.by(() => {
		if (!matchedService) return [];
		return volumes.filter((v) => v.serviceId === matchedService.id);
	});

	// Direct published ports in project
	let projectPublishedPorts = $derived.by(() => {
		if (!matchedProject) return [];
		const ports: { hostPort: number; containerPort: number; ip: string; serviceName: string }[] = [];
		const seen = new Set<string>();

		projectContainersList.forEach((c) => {
			if (c.ports) {
				const pList = Array.isArray(c.ports) ? c.ports : [c.ports];
				pList.forEach((pStr: string) => {
					const match = pStr.match(/(?:([0-9.]+):)?([0-9]+)(?:->|:)([0-9]+)/);
					if (match) {
						const ip = match[1] || '0.0.0.0';
						const hp = parseInt(match[2], 10);
						const cp = parseInt(match[3], 10);
						const key = `${ip}:${hp}`;
						if (!seen.has(key) && hp > 0) {
							seen.add(key);
							ports.push({
								hostPort: hp,
								containerPort: cp,
								ip,
								serviceName: c.serviceName || c.name
							});
						}
					}
				});
			}
		});

		projectServicesList.forEach((s) => {
			if (s.hostPort && s.hostPort > 0) {
				const key = `0.0.0.0:${s.hostPort}`;
				if (!seen.has(key)) {
					seen.add(key);
					ports.push({
						hostPort: s.hostPort,
						containerPort: s.port || s.hostPort,
						ip: '0.0.0.0',
						serviceName: s.name
					});
				}
			}
		});

		return ports;
	});

	function getCategoryMeta(targetNode: TopologyNode) {
		const rawNode = targetNode.raw as any;
		const isQuadlet =
			targetNode.groupType === 'quadlet' ||
			rawNode?.type === 'quadlet' ||
			rawNode?.runtimeTarget === 'quadlet' ||
			(targetNode.badge || '').toLowerCase() === 'quadlet' ||
			targetNode.title.endsWith('.container') ||
			targetNode.title.endsWith('.service');

		const isCompose =
			targetNode.groupType === 'compose' ||
			rawNode?.type === 'compose' ||
			(targetNode.badge || '').toLowerCase() === 'compose';

		const isPod =
			targetNode.groupType === 'pod' ||
			rawNode?.type === 'pod' ||
			(targetNode.badge || '').toLowerCase().includes('pod');

		const isDatabase =
			targetNode.type === 'service' || targetNode.type === 'container'
				? rawNode?.type === 'database' ||
					['redis', 'postgres', 'mysql', 'mongo', 'db'].some((k) =>
						targetNode.title.toLowerCase().includes(k)
					)
				: false;

		if (targetNode.type === 'domain') {
			return { label: 'Web Domain', icon: Globe, color: 'text-[var(--accent)]' };
		}
		if (targetNode.type === 'port') {
			const isPublic = targetNode.title.startsWith('0.0.0.0') || !targetNode.title.startsWith('127.');
			return {
				label: isPublic ? 'Public WAN Port' : 'Host Loopback Port',
				icon: Plugs,
				color: isPublic ? 'text-[var(--status-amber)]' : 'text-[var(--status-blue)]'
			};
		}
		if (targetNode.type === 'proxy') {
			return { label: 'Caddy Gateway', icon: ShieldCheck, color: 'text-[var(--status-green)]' };
		}
		if (targetNode.type === 'internet') {
			return { label: 'Public Internet', icon: Globe, color: 'text-[var(--accent)]' };
		}
		if (targetNode.type === 'group') {
			if (isPod) return { label: 'Podman Pod', icon: TreeStructure, color: 'text-[var(--status-blue)]' };
			if (isCompose) return { label: 'Compose Stack', icon: Stack, color: 'text-[var(--accent)]' };
			if (isQuadlet) return { label: 'Quadlet Unit', icon: FileText, color: 'text-[var(--accent)]' };
			if (targetNode.groupType === 'project')
				return { label: 'Project Space', icon: Folder, color: 'text-[var(--accent)]' };
			return { label: 'Workload Group', icon: Stack, color: 'text-[var(--text-secondary)]' };
		}
		if (isDatabase) {
			return { label: 'Database Service', icon: Database, color: 'text-[var(--status-amber)]' };
		}
		if (isQuadlet) {
			return { label: 'Quadlet Service', icon: FileText, color: 'text-[var(--accent)]' };
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
			class="flex items-center justify-between border-b border-[var(--border)] bg-[var(--bg-panel)] px-4 py-3 shrink-0"
		>
			<div class="flex items-center gap-2 min-w-0">
				<div
					class="flex h-7 w-7 shrink-0 items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] {meta.color}"
				>
					<CategoryIcon size={15} weight="bold" />
				</div>
				<div class="min-w-0">
					<span class="block text-xs font-semibold text-[var(--text-tertiary)] uppercase tracking-wider truncate">
						{meta.label}
					</span>
				</div>
			</div>

			<!-- Close Action with Keyboard Esc Hint -->
			<div class="flex items-center gap-1.5 shrink-0">
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
					<h3 class="text-base font-semibold leading-snug text-[var(--text-primary)] break-words">
						{title}
					</h3>
					{#if status}
						<div class="shrink-0 pt-0.5">
							<StatusBadge {status} size="sm" />
						</div>
					{/if}
				</div>

				{#if node?.subtitle || region?.sublabel}
					<p class="text-xs text-[var(--text-secondary)] leading-relaxed">
						{node?.subtitle || region?.sublabel}
					</p>
				{/if}
			</div>

			<!-- ────────────────────────────────────────── -->
			<!-- A. PROJECT SPACE FACTUAL DETAILS -->
			<!-- ────────────────────────────────────────── -->
			{#if (node && node.groupType === 'project') || (region && region.type === 'project')}
				{@const proj = matchedProject || raw}
				{@const activeContainersCount = projectContainersList.filter((c) => c.status === 'running').length}

				<!-- Aggregated Telemetry Deck -->
				<div class="grid grid-cols-2 gap-2">
					<div class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs">
						<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
							<Cpu size={13} />
							<span>CPU Aggregated</span>
						</div>
						<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
							{(proj?.cpu || 0).toFixed(1)}%
						</div>
					</div>

					<div class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs">
						<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
							<Database size={13} />
							<span>Memory Total</span>
						</div>
						<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
							{proj?.memory || 0} MB
						</div>
					</div>
				</div>

				<!-- Workloads & Container Status Summary -->
				<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
					<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
						<span class="flex items-center gap-1.5">
							<Stack size={14} class="text-[var(--accent)]" />
							<span>Member Services ({projectServicesList.length})</span>
						</span>
						<Badge variant="secondary" size="sm">
							{activeContainersCount}/{projectContainersList.length} Containers Active
						</Badge>
					</div>

					<div class="flex flex-col gap-1.5 mt-1">
						{#each projectServicesList as svc}
							<button
								type="button"
								onclick={() => goto(`/projects/${proj.id}/services/${svc.id}`)}
								class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs hover:border-[var(--accent)] hover:bg-[var(--bg-hover)] transition-colors text-left"
							>
								<div class="flex items-center gap-2 min-w-0">
									<StatusBadge status={svc.status} size="sm" />
									<span class="font-medium text-[var(--text-primary)] truncate">{svc.name}</span>
								</div>
								<div class="flex items-center gap-1.5 shrink-0">
									{#if svc.port || svc.hostPort}
										<span class="font-mono text-[10px] text-[var(--text-secondary)]">
											:{svc.port || svc.hostPort}
										</span>
									{/if}
									<Badge variant="outline" size="sm" class="capitalize text-[10px]">
										{svc.type}
									</Badge>
								</div>
							</button>
						{/each}
					</div>
				</div>

				<!-- Connected Domains & Ingress -->
				{#if projectDomainsList.length > 0}
					<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
						<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
							<span class="flex items-center gap-1.5">
								<Globe size={14} class="text-[var(--accent)]" />
								<span>Ingress Domains ({projectDomainsList.length})</span>
							</span>
						</div>

						<div class="flex flex-col gap-1.5 mt-1">
							{#each projectDomainsList as dom}
								<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs">
									<a
										href={`https://${dom.hostname}`}
										target="_blank"
										rel="noreferrer"
										class="font-mono text-xs text-[var(--accent)] hover:underline truncate"
									>
										{dom.hostname}
									</a>
									<Badge variant={dom.tls ? 'success' : 'secondary'} size="sm">
										{#if dom.tls}
											<span class="inline-flex items-center gap-1"><Lock size={10} /> TLS</span>
										{:else}
											HTTP Only
										{/if}
									</Badge>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				<!-- Published Host Ports -->
				{#if projectPublishedPorts.length > 0}
					<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
						<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
							<span class="flex items-center gap-1.5">
								<Plugs size={14} class="text-[var(--status-amber)]" />
								<span>Published Host Ports</span>
							</span>
						</div>

						<div class="grid grid-cols-2 gap-1.5 mt-1">
							{#each projectPublishedPorts as pp}
								<div class="flex flex-col rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 text-xs">
									<span class="font-mono text-[var(--text-primary)] font-medium">
										{pp.ip}:{pp.hostPort}
									</span>
									<span class="text-[10px] text-[var(--text-tertiary)] truncate mt-0.5">
										→ {pp.serviceName} (:{pp.containerPort})
									</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				<!-- Persistent Storage Volumes -->
				{#if projectVolumesList.length > 0}
					<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
						<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
							<span class="flex items-center gap-1.5">
								<HardDrive size={14} class="text-[var(--status-amber)]" />
								<span>Storage Volumes ({projectVolumesList.length})</span>
							</span>
						</div>

						<div class="flex flex-col gap-1.5 mt-1">
							{#each projectVolumesList as vol}
								<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs">
									<span class="font-medium text-[var(--text-primary)] truncate">{vol.name}</span>
									<span class="font-mono text-[11px] text-[var(--text-secondary)]">{vol.size || 'Dynamic'}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			{/if}

			<!-- ────────────────────────────────────────── -->
			<!-- B. SERVICE / GROUP FACTUAL DETAILS -->
			<!-- ────────────────────────────────────────── -->
			{#if (node && (node.type === 'service' || (node.type === 'group' && node.groupType !== 'project'))) || (region && region.type === 'pod')}
				{@const svc = matchedService || raw}

				<!-- Real Telemetry Cards -->
				<div class="grid grid-cols-2 gap-2">
					<div class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs">
						<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
							<Cpu size={13} />
							<span>CPU Usage</span>
						</div>
						<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
							{(svc?.cpu || raw?.cpu || 0).toFixed(1)}%
						</div>
					</div>

					<div class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs">
						<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
							<Database size={13} />
							<span>Memory</span>
						</div>
						<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
							{svc?.memory || raw?.memory || 0} MB
						</div>
					</div>
				</div>

				<!-- Member Containers List -->
				{#if serviceContainersList.length > 0}
					<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
						<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
							<span class="flex items-center gap-1.5">
								<Cube size={14} class="text-[var(--status-green)]" />
								<span>Containers ({serviceContainersList.length})</span>
							</span>
						</div>

						<div class="flex flex-col gap-1.5 mt-1">
							{#each serviceContainersList as c}
								<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-2 text-xs">
									<div class="flex items-center gap-2 min-w-0">
										<StatusBadge status={c.status} size="sm" />
										<div class="min-w-0">
											<span class="font-medium text-[var(--text-primary)] block truncate">{c.name}</span>
											<span class="text-[10px] font-mono text-[var(--text-tertiary)] block truncate">
												{c.image}
											</span>
										</div>
									</div>
									<div class="font-mono text-[10px] text-[var(--text-secondary)] shrink-0 text-right">
										<div>{(c.cpu || 0).toFixed(1)}%</div>
										<div>{c.memory || 0}MB</div>
									</div>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				<!-- Ports & Ingress Bindings -->
				{#if svc?.port || svc?.hostPort || serviceDomainsList.length > 0}
					<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
						<span class="text-xs font-semibold text-[var(--text-primary)]">Networking & Ingress</span>
						<div class="flex flex-col gap-1.5">
							{#if svc?.hostPort}
								<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs font-mono">
									<span class="text-[var(--text-tertiary)]">Host Port</span>
									<span class="text-[var(--text-primary)] font-medium">0.0.0.0:{svc.hostPort} → :{svc.port || svc.hostPort}</span>
								</div>
							{:else if svc?.port}
								<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs font-mono">
									<span class="text-[var(--text-tertiary)]">Container Port</span>
									<span class="text-[var(--text-primary)] font-medium">:{svc.port}</span>
								</div>
							{/if}

							{#each serviceDomainsList as d}
								<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs">
									<a
										href={`https://${d.hostname}`}
										target="_blank"
										rel="noreferrer"
										class="font-mono text-xs text-[var(--accent)] hover:underline truncate"
									>
										{d.hostname}
									</a>
									<Badge variant={d.tls ? 'success' : 'secondary'} size="sm">
										{d.tls ? 'TLS' : 'HTTP'}
									</Badge>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				<!-- Quadlet Systemd Unit Definition -->
				{#if svc?.quadletConfig}
					<div class="flex flex-col gap-1.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
						<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
							<span class="flex items-center gap-1.5">
								<FileText size={14} class="text-[var(--accent)]" />
								<span>Systemd Quadlet Unit</span>
							</span>
							<CopyButton text={svc.quadletConfig} size={12} label="Copy Unit" variant="inline" />
						</div>
						<pre class="max-h-40 overflow-auto rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 font-mono text-[10.5px] leading-relaxed whitespace-pre text-[var(--text-secondary)] mt-1">{svc.quadletConfig}</pre>
					</div>
				{/if}
			{/if}

			<!-- ────────────────────────────────────────── -->
			<!-- C. CONTAINER INSTANCE FACTUAL DETAILS -->
			<!-- ────────────────────────────────────────── -->
			{#if node && node.type === 'container'}
				{@const cnt = matchedContainer}

				<!-- Telemetry Deck -->
				<div class="grid grid-cols-2 gap-2">
					<div class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs">
						<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
							<Cpu size={13} />
							<span>CPU Usage</span>
						</div>
						<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
							{(cnt?.cpu || raw?.cpu || 0).toFixed(1)}%
						</div>
					</div>

					<div class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs">
						<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
							<Database size={13} />
							<span>Memory</span>
						</div>
						<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
							{cnt?.memory || raw?.memory || 0} MB
						</div>
					</div>
				</div>

				<!-- Container Image Specification -->
				<div class="flex flex-col gap-1.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
					<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
						<span>Container Image</span>
						<CopyButton text={cnt?.image || node.subtitle || ''} size={12} label="Copy" variant="inline" />
					</div>
					<div class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 font-mono text-xs text-[var(--text-primary)] break-all mt-1">
						{cnt?.image || node.subtitle}
					</div>
				</div>

				<!-- Runtime Diagnostics -->
				<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
					<span class="text-xs font-semibold text-[var(--text-primary)]">Runtime State</span>
					<div class="grid grid-cols-2 gap-2 text-xs">
						<div class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2">
							<span class="text-[10px] text-[var(--text-tertiary)] block">Uptime</span>
							<span class="font-mono text-[var(--text-primary)]">{cnt?.uptime || 'Active'}</span>
						</div>
						<div class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2">
							<span class="text-[10px] text-[var(--text-tertiary)] block">Restart Policy</span>
							<span class="font-mono text-[var(--text-primary)]">always</span>
						</div>
					</div>

					{#if cnt?.ports}
						<div class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 text-xs">
							<span class="text-[10px] text-[var(--text-tertiary)] block">Port Forwarding</span>
							<span class="font-mono text-[var(--text-primary)] break-all">
								{Array.isArray(cnt.ports) ? cnt.ports.join(', ') : cnt.ports}
							</span>
						</div>
					{/if}
				</div>
			{/if}

			<!-- ────────────────────────────────────────── -->
			<!-- D. DIRECT HOST PORT DETAILS -->
			<!-- ────────────────────────────────────────── -->
			{#if node && node.type === 'port'}
				{@const isPublic = node.title.startsWith('0.0.0.0') || !node.title.startsWith('127.')}
				<div class="flex flex-col gap-2.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
					<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
						<span class="flex items-center gap-1.5">
							<Plugs size={14} class={isPublic ? 'text-[var(--status-amber)]' : 'text-[var(--status-blue)]'} />
							<span>Host Port Mapping</span>
						</span>
						<Badge variant={isPublic ? 'warning' : 'info'} size="sm">
							{isPublic ? '0.0.0.0 WAN Exposed' : '127.0.0.1 Loopback'}
						</Badge>
					</div>

					<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-2">
						<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">{node.title}</span>
						<CopyButton text={node.title} size={13} />
					</div>

					<div class="text-xs text-[var(--text-secondary)] leading-relaxed">
						{#if isPublic}
							Directly published to host interface. Traffic connects straight to container, bypassing Caddy reverse proxy.
						{:else}
							Bound to host loopback interface only. Secure from WAN; accessible via localhost or SSH port forwarding.
						{/if}
					</div>
				</div>
			{/if}

			<!-- ────────────────────────────────────────── -->
			<!-- E. DOMAIN ENDPOINT DETAILS -->
			<!-- ────────────────────────────────────────── -->
			{#if node && node.type === 'domain'}
				<div class="flex flex-col gap-2.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
					<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
						<span class="flex items-center gap-1.5">
							<Globe size={14} class="text-[var(--accent)]" />
							<span>Ingress Endpoint</span>
						</span>
						<Badge variant={raw?.tls ? 'success' : 'secondary'} size="sm">
							{#if raw?.tls}
								<span class="inline-flex items-center gap-1"><Lock size={10} /> TLS Active</span>
							{:else}
								<span class="inline-flex items-center gap-1"><LockOpen size={10} /> HTTP Only</span>
							{/if}
						</Badge>
					</div>

					<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5">
						<a
							href={`https://${node.title}`}
							target="_blank"
							rel="noreferrer"
							class="flex items-center gap-1 font-mono text-xs text-[var(--accent)] hover:underline truncate"
						>
							<span>https://{node.title}</span>
							<ArrowSquareOut size={12} class="shrink-0" />
						</a>
						<CopyButton text={`https://${node.title}`} size={13} />
					</div>

					<div class="grid grid-cols-2 gap-2 text-xs text-[var(--text-secondary)]">
						<div class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2">
							<span class="text-[10px] text-[var(--text-tertiary)] block">Upstream Port</span>
							<span class="font-mono text-[var(--text-primary)]">:{raw?.containerPort || raw?.proxyPort || 80}</span>
						</div>
						<div class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2">
							<span class="text-[10px] text-[var(--text-tertiary)] block">Caddy TLS Gateway</span>
							<span class="font-mono text-[var(--text-primary)]">ports 80, 443</span>
						</div>
					</div>
				</div>
			{/if}

			<!-- ────────────────────────────────────────── -->
			<!-- F. CADDY REVERSE PROXY GATEWAY DETAILS -->
			<!-- ────────────────────────────────────────── -->
			{#if node && node.type === 'proxy'}
				<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
					<span class="text-xs font-semibold text-[var(--text-primary)]">Caddy Ingress Capabilities</span>
					<div class="space-y-1.5 text-xs text-[var(--text-secondary)]">
						<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5">
							<span>Automatic HTTPS</span>
							<Badge variant="success" size="sm">Let's Encrypt / ACME</Badge>
						</div>
						<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5">
							<span>Reverse Proxy Ports</span>
							<span class="font-mono text-[var(--text-primary)]">80 (HTTP) · 443 (HTTPS)</span>
						</div>
						<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5">
							<span>Server Platform</span>
							<span class="font-mono text-[var(--text-primary)]">{server.os}</span>
						</div>
					</div>
				</div>

				<!-- Active Managed Routes -->
				<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
					<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
						<span class="flex items-center gap-1.5">
							<Globe size={14} class="text-[var(--accent)]" />
							<span>Active Ingress Routes ({domains.length})</span>
						</span>
					</div>

					<div class="flex flex-col gap-1.5 mt-1 max-h-48 overflow-y-auto">
						{#each domains as d}
							<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs">
								<span class="font-mono text-xs text-[var(--text-primary)] truncate">{d.hostname}</span>
								<Badge variant={d.tls ? 'success' : 'secondary'} size="sm">
									{d.tls ? 'TLS' : 'HTTP'}
								</Badge>
							</div>
						{/each}
					</div>
				</div>
			{/if}

			<!-- ────────────────────────────────────────── -->
			<!-- G. PUBLIC INTERNET WAN DETAILS -->
			<!-- ────────────────────────────────────────── -->
			{#if node && node.type === 'internet'}
				<div class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs">
					<span class="text-xs font-semibold text-[var(--text-primary)]">Public Internet Traffic</span>
					<div class="space-y-1.5 text-xs text-[var(--text-secondary)]">
						<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5">
							<span>Host Public IP</span>
							<span class="font-mono text-[var(--text-primary)]">{server.ip}</span>
						</div>
						<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5">
							<span>Podman Engine</span>
							<span class="font-mono text-[var(--text-primary)]">Podman {server.podmanVersion}</span>
						</div>
						<div class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5">
							<span>Host Operating System</span>
							<span class="text-[var(--text-primary)]">{server.os}</span>
						</div>
					</div>
				</div>
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

		<!-- 3. Primary Action Footer -->
		<div
			class="flex flex-col gap-2 border-t border-[var(--border)] bg-[var(--bg-panel)] p-4 shrink-0"
		>
			<!-- Service Context Actions -->
			{#if node && node.type === 'service' && node.projectId && node.serviceId}
				<Button
					variant="primary"
					size="sm"
					class="w-full justify-center"
					onclick={() => goto(`/projects/${node.projectId}/services/${node.serviceId}`)}
				>
					<span>Open Service Details</span>
					<ArrowRight size={13} />
				</Button>
				<Button
					variant="secondary"
					size="sm"
					class="w-full justify-center"
					onclick={() => goto(`/projects/${node.projectId}/services/${node.serviceId}?tab=logs`)}
				>
					<Terminal size={13} />
					<span>Service Logs & Terminal</span>
				</Button>
			{/if}

			<!-- Project Level Group Focus -->
			{#if (node && node.groupType === 'project' && node.projectId) || (region && region.type === 'project' && region.projectId)}
				{@const targetProjId = node?.projectId || region?.projectId}
				{#if onFocusProject && targetProjId}
					<Button
						variant="primary"
						size="sm"
						class="w-full justify-center"
						onclick={() => onFocusProject(targetProjId)}
					>
						<span>Focus Project Topology</span>
						<ArrowRight size={13} />
					</Button>
				{/if}
				{#if targetProjId}
					<Button
						variant="secondary"
						size="sm"
						class="w-full justify-center"
						onclick={() => goto(`/projects/${targetProjId}`)}
					>
						<span>Open Project Dashboard</span>
					</Button>
				{/if}
			{/if}

			<!-- Container Instance Actions -->
			{#if node && node.type === 'container'}
				<Button
					variant="primary"
					size="sm"
					class="w-full justify-center"
					onclick={() => goto(`/runtime/containers?search=${node.title}`)}
				>
					<Terminal size={13} />
					<span>Inspect in Runtime Table</span>
				</Button>
				{#if node.projectId && node.serviceId}
					<Button
						variant="secondary"
						size="sm"
						class="w-full justify-center"
						onclick={() => goto(`/projects/${node.projectId}/services/${node.serviceId}`)}
					>
						<span>Parent Service Details</span>
					</Button>
				{/if}
			{/if}

			<!-- Domain Actions -->
			{#if node && node.type === 'domain'}
				<Button
					variant="primary"
					size="sm"
					class="w-full justify-center"
					href={`https://${node.title}`}
					external={true}
				>
					<span>Visit Live Site</span>
					<ArrowSquareOut size={13} />
				</Button>
				<Button
					variant="secondary"
					size="sm"
					class="w-full justify-center"
					onclick={() => goto('/networking/domains')}
				>
					<span>Manage Domain Settings</span>
				</Button>
			{/if}

			<!-- Direct Port Actions -->
			{#if node && node.type === 'port'}
				{#if node.projectId && node.serviceId}
					<Button
						variant="primary"
						size="sm"
						class="w-full justify-center"
						onclick={() => goto(`/projects/${node.projectId}/services/${node.serviceId}`)}
					>
						<span>Inspect Target Service</span>
						<ArrowRight size={13} />
					</Button>
				{/if}
			{/if}

			<!-- Caddy Actions -->
			{#if node && node.type === 'proxy'}
				<Button
					variant="primary"
					size="sm"
					class="w-full justify-center"
					onclick={() => goto('/networking/domains')}
				>
					<span>Manage Ingress & Caddyfile</span>
					<ArrowRight size={13} />
				</Button>
			{/if}

			<!-- Server Actions -->
			{#if node && node.type === 'internet'}
				<Button
					variant="primary"
					size="sm"
					class="w-full justify-center"
					onclick={() => goto('/settings')}
				>
					<span>Host System Overview</span>
					<ArrowRight size={13} />
				</Button>
			{/if}
		</div>
	</aside>
{/if}
