<script lang="ts">
	import { dataStore } from '$lib/data';
	import type { Container } from '$lib/types';
	import { StatusBadge, SearchInput } from '$lib/components/ui';
	import {
		MagnifyingGlass,
		X,
		CaretDown,
		CaretRight,
		CaretUp,
		Folder,
		Gear,
		Cube,
		Terminal,
		FileText,
		Warning,
		TreeStructure,
		List,
		ArrowsOutSimple,
		ArrowsInSimple
	} from 'phosphor-svelte';

	interface Props {
		onOpenTerminal?: (containerName: string) => void;
		onOpenLogs?: (serviceId: string, containerName?: string) => void;
		initialViewMode?: 'tree' | 'flat' | 'system';
	}

	let { onOpenTerminal, onOpenLogs, initialViewMode = 'tree' }: Props = $props();

	// View modes: tree or flat for workloads; system fixed when initialViewMode is system
	let viewMode = $state<'tree' | 'flat' | 'system'>('tree');
	let searchQuery = $state('');
	let sortBy = $state<'cpu' | 'memory' | 'name' | 'pids'>('cpu');
	let sortDesc = $state(true);
	let filterAnomaliesOnly = $state(false);

	$effect(() => {
		if (initialViewMode) {
			viewMode = initialViewMode;
		}
	});

	// Collapsed state for tree nodes
	let openProjects = $state<Record<string, boolean>>({
		aerochat: true,
		ngumpulhost: true,
		portfolio: true,
		minecraft: true,
		homelab: true
	});

	let openServices = $state<Record<string, boolean>>({
		'aerochat-web': true,
		'aerochat-ws': true,
		'aerochat-redis': true,
		'homelab-metube': true,
		'homelab-stack': true,
		'ngumpul-frontend': true,
		'ngumpul-backend': true,
		'portfolio-web': true,
		'minecraft-server': true,
		'minecraft-rcon': true
	});

	function toggleProject(id: string) {
		openProjects[id] = !openProjects[id];
	}

	function toggleService(id: string) {
		openServices[id] = !openServices[id];
	}

	let isAllExpanded = $derived(
		dataStore.projects.every((p) => openProjects[p.id] !== false)
	);

	function toggleAllExpanded() {
		const nextState = !isAllExpanded;
		dataStore.projects.forEach((p) => (openProjects[p.id] = nextState));
		dataStore.services.forEach((s) => (openServices[s.id] = nextState));
	}

	function handleSort(column: 'cpu' | 'memory' | 'name' | 'pids') {
		if (sortBy === column) {
			sortDesc = !sortDesc;
		} else {
			sortBy = column;
			sortDesc = column === 'name' ? false : true;
		}
	}

	// System daemons data
	const systemContainers: Container[] = [
		{
			id: 'sys-traefik',
			name: 'gopod-traefik',
			projectId: 'system',
			projectName: 'System Daemons',
			serviceId: 'ingress',
			serviceName: 'Reverse Proxy',
			image: 'traefik:v3.1',
			status: 'running',
			cpu: 0.3,
			memory: 64,
			memoryLimit: 256,
			ports: '80:80, 443:443',
			startedAt: '14 days ago',
			netRx: '184 MB',
			netTx: '242 MB',
			blockRead: '1.2 MB',
			blockWrite: '2.4 MB',
			pids: 8,
			restarts: 0,
			uptime: '14d 7h'
		},
		{
			id: 'sys-podman',
			name: 'podman-engine',
			projectId: 'system',
			projectName: 'System Daemons',
			serviceId: 'daemon',
			serviceName: 'Container Engine',
			image: 'podman:v5.3.1 (host socket)',
			status: 'running',
			cpu: 0.2,
			memory: 42,
			memoryLimit: 256,
			ports: '/run/user/1000/podman.sock',
			startedAt: '14 days ago',
			netRx: '—',
			netTx: '—',
			blockRead: '0.4 MB',
			blockWrite: '0.8 MB',
			pids: 4,
			restarts: 0,
			uptime: '14d 7h'
		},
		{
			id: 'sys-quadlet',
			name: 'systemd-generator',
			projectId: 'system',
			projectName: 'System Daemons',
			serviceId: 'quadlet-mgr',
			serviceName: 'Quadlet Generator',
			image: 'systemd-quadlet:rootless',
			status: 'running',
			cpu: 0.1,
			memory: 24,
			memoryLimit: 128,
			ports: 'dbus:session',
			startedAt: '14 days ago',
			netRx: '—',
			netTx: '—',
			blockRead: '0.1 MB',
			blockWrite: '0.1 MB',
			pids: 2,
			restarts: 0,
			uptime: '14d 7h'
		}
	];

	// Filtered container lists
	let allContainers = $derived(viewMode === 'system' ? systemContainers : dataStore.containers);

	// Anomaly identification
	function isAnomaly(c: Container): boolean {
		return (c.cpu ?? 0) >= 2.0 || (c.memory ?? 0) >= 400 || (c.status !== 'running' && c.status !== 'healthy');
	}

	let anomalyContainers = $derived(allContainers.filter(isAnomaly));

	let filteredContainers = $derived.by(() => {
		let list = [...allContainers];
		if (filterAnomaliesOnly) {
			list = list.filter(isAnomaly);
		}
		if (searchQuery.trim()) {
			const q = searchQuery.toLowerCase().trim();
			list = list.filter(
				(c) =>
					c.name.toLowerCase().includes(q) ||
					c.image.toLowerCase().includes(q) ||
					c.projectName.toLowerCase().includes(q) ||
					c.serviceName.toLowerCase().includes(q)
			);
		}

		list.sort((a, b) => {
			let valA: any = a[sortBy] ?? 0;
			let valB: any = b[sortBy] ?? 0;
			if (typeof valA === 'string') {
				return sortDesc ? valB.localeCompare(valA) : valA.localeCompare(valB);
			}
			return sortDesc ? valB - valA : valA - valB;
		});

		return list;
	});

	// Build hierarchical structure: Project -> Service -> Containers
	let projectTree = $derived.by(() => {
		const projects = dataStore.projects;
		const query = searchQuery.toLowerCase().trim();

		let result = projects
			.map((project) => {
				const services = dataStore.getProjectServices(project.id);

				let serviceNodes = services
					.map((service) => {
						let containers = dataStore.containers.filter((c) => c.serviceId === service.id);
						if (filterAnomaliesOnly) {
							containers = containers.filter(isAnomaly);
						}

						// Sort containers within service
						containers.sort((a, b) => {
							let valA: any = a[sortBy] ?? 0;
							let valB: any = b[sortBy] ?? 0;
							if (typeof valA === 'string') {
								return sortDesc ? valB.localeCompare(valA) : valA.localeCompare(valB);
							}
							return sortDesc ? valB - valA : valA - valB;
						});

						const totalCpu = containers.reduce((acc, c) => acc + (c.cpu || 0), 0);
						const totalMem = containers.reduce((acc, c) => acc + (c.memory || 0), 0);
						const totalPids = containers.reduce((acc, c) => acc + (c.pids || 0), 0);

						const matchesQuery =
							containers.length > 0 &&
							(!query ||
							service.name.toLowerCase().includes(query) ||
							service.type.toLowerCase().includes(query) ||
							containers.some(
								(c) => c.name.toLowerCase().includes(query) || c.image.toLowerCase().includes(query)
							));

						return {
							service,
							containers,
							totalCpu,
							totalMem,
							totalPids,
							visible: matchesQuery
						};
					})
					.filter((s) => s.visible);

				// Sort service nodes within project
				serviceNodes.sort((a, b) => {
					let valA: any = sortBy === 'cpu' ? a.totalCpu : sortBy === 'memory' ? a.totalMem : sortBy === 'pids' ? a.totalPids : a.service.name;
					let valB: any = sortBy === 'cpu' ? b.totalCpu : sortBy === 'memory' ? b.totalMem : sortBy === 'pids' ? b.totalPids : b.service.name;
					if (typeof valA === 'string') {
						return sortDesc ? valB.localeCompare(valA) : valA.localeCompare(valB);
					}
					return sortDesc ? valB - valA : valA - valB;
				});

				const totalProjectCpu = serviceNodes.reduce((acc, s) => acc + s.totalCpu, 0);
				const totalProjectMem = serviceNodes.reduce((acc, s) => acc + s.totalMem, 0);
				const totalProjectContainers = serviceNodes.reduce((acc, s) => acc + s.containers.length, 0);

				const matchesProject =
					!query ||
					project.name.toLowerCase().includes(query) ||
					project.id.toLowerCase().includes(query) ||
					serviceNodes.length > 0;

				return {
					project,
					serviceNodes,
					totalCpu: totalProjectCpu,
					totalMem: totalProjectMem,
					totalContainers: totalProjectContainers,
					visible: matchesProject
				};
			})
			.filter((p) => p.visible);

		// Sort projects
		result.sort((a, b) => {
			let valA: any = sortBy === 'cpu' ? a.totalCpu : sortBy === 'memory' ? a.totalMem : a.project.name;
			let valB: any = sortBy === 'cpu' ? b.totalCpu : sortBy === 'memory' ? b.totalMem : b.project.name;
			if (typeof valA === 'string') {
				return sortDesc ? valB.localeCompare(valA) : valA.localeCompare(valB);
			}
			return sortDesc ? valB - valA : valA - valB;
		});

		return result;
	});

	function getCpuColor(cpu: number): string {
		if (cpu >= 5.0) return 'bg-[var(--status-red)]';
		if (cpu >= 2.0) return 'bg-[var(--status-amber)]';
		return 'bg-[var(--accent)]';
	}

	function getMemPercent(used: number, limit: number): number {
		return Math.min(Math.round((used / (limit || 512)) * 100), 100);
	}

	function getMemColor(percent: number): string {
		if (percent >= 85) return 'bg-[var(--status-red)]';
		if (percent >= 70) return 'bg-[var(--status-amber)]';
		return 'bg-[var(--status-green)]';
	}
</script>

<div class="w-full flex flex-col gap-3">
	<!-- ══════════════════════════════════════════════════════════════
	     UNIFIED COMPACT TOOLBAR
	     ══════════════════════════════════════════════════════════════ -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
		<!-- Left: View Mode Toggle & Tree Expander -->
		<div class="flex items-center gap-2">
			{#if initialViewMode === 'system'}
				<span class="text-xs font-semibold text-[var(--text-secondary)]">
					System Daemons ({systemContainers.length})
				</span>
			{:else}
				<div class="inline-flex items-center p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
					<button
						type="button"
						onclick={() => (viewMode = 'tree')}
						class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer flex items-center gap-1.5 {viewMode === 'tree'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						title="Hierarchical Project / Service / Container tree"
					>
						<TreeStructure size={13} />
						<span>Tree</span>
					</button>

					<button
						type="button"
						onclick={() => (viewMode = 'flat')}
						class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer flex items-center gap-1.5 {viewMode === 'flat'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						title="Flat container process list"
					>
						<List size={13} />
						<span>Flat ({dataStore.containers.length})</span>
					</button>
				</div>

				{#if viewMode === 'tree'}
					<button
						type="button"
						onclick={toggleAllExpanded}
						class="px-2.5 py-1 rounded-[var(--radius-sm)] text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-panel)] border border-[var(--border)] transition-colors cursor-pointer flex items-center gap-1.5"
						title={isAllExpanded ? 'Collapse all projects & services' : 'Expand all projects & services'}
					>
						{#if isAllExpanded}
							<ArrowsInSimple size={13} />
							<span>Collapse</span>
						{:else}
							<ArrowsOutSimple size={13} />
							<span>Expand</span>
						{/if}
					</button>
				{/if}
			{/if}
		</div>

		<!-- Right: Anomalies filter + Search -->
		<div class="flex items-center gap-2">
			{#if anomalyContainers.length > 0}
				<button
					type="button"
					onclick={() => (filterAnomaliesOnly = !filterAnomaliesOnly)}
					class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border text-xs font-medium cursor-pointer transition-colors flex items-center gap-1.5 {filterAnomaliesOnly
						? 'bg-[var(--status-amber-muted)] border-[var(--status-amber)] text-[var(--status-amber)] shadow-xs'
						: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
					title="Show only workloads with elevated resource usage or warnings"
				>
					<Warning size={13} class={filterAnomaliesOnly ? 'text-[var(--status-amber)]' : 'text-[var(--text-tertiary)]'} />
					<span>Anomalies</span>
					<span class="px-1.5 py-0.2 rounded-full text-[10px] font-mono font-bold {filterAnomaliesOnly ? 'bg-[var(--status-amber)] text-black' : 'bg-[var(--bg-surface)] text-[var(--text-tertiary)]'}">
						{anomalyContainers.length}
					</span>
				</button>
			{/if}

			<SearchInput
				bind:value={searchQuery}
				placeholder="Search workloads..."
				class="w-56 sm:w-64"
			/>
		</div>
	</div>

	<!-- ══════════════════════════════════════════════════════════════
	     NATURAL TABLE WITH DIRECT HEADER SORTING
	     ══════════════════════════════════════════════════════════════ -->
	<div class="w-full overflow-x-auto md:overflow-x-visible rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs">
		<table class="w-full border-collapse min-w-[700px]">
			<thead class="sticky top-0 z-20">
				<tr class="border-b border-[var(--border)] bg-[var(--bg-table-header)]">
					<th
						class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-left cursor-pointer hover:text-[var(--text-primary)] transition-colors select-none"
						onclick={() => handleSort('name')}
						title="Click to sort by name"
					>
						<div class="inline-flex items-center gap-1.5">
							<span>{viewMode === 'tree' ? 'Workload / Entity Tree' : 'Workload Name'}</span>
							{#if sortBy === 'name'}
								{#if sortDesc}
									<CaretDown size={12} class="text-[var(--accent)]" />
								{:else}
									<CaretUp size={12} class="text-[var(--accent)]" />
								{/if}
							{/if}
						</div>
					</th>

					<th class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-left w-24">
						Status
					</th>

					<th
						class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-36 cursor-pointer hover:text-[var(--text-primary)] transition-colors select-none"
						onclick={() => handleSort('cpu')}
						title="Click to sort by CPU usage"
					>
						<div class="inline-flex items-center justify-end gap-1.5 w-full">
							<span>CPU Utilization</span>
							{#if sortBy === 'cpu'}
								{#if sortDesc}
									<CaretDown size={12} class="text-[var(--accent)]" />
								{:else}
									<CaretUp size={12} class="text-[var(--accent)]" />
								{/if}
							{/if}
						</div>
					</th>

					<th
						class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-44 cursor-pointer hover:text-[var(--text-primary)] transition-colors select-none"
						onclick={() => handleSort('memory')}
						title="Click to sort by Memory usage"
					>
						<div class="inline-flex items-center justify-end gap-1.5 w-full">
							<span>Memory</span>
							{#if sortBy === 'memory'}
								{#if sortDesc}
									<CaretDown size={12} class="text-[var(--accent)]" />
								{:else}
									<CaretUp size={12} class="text-[var(--accent)]" />
								{/if}
							{/if}
						</div>
					</th>

					<th class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-24">
						Net I/O
					</th>

					<th
						class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-center w-16 cursor-pointer hover:text-[var(--text-primary)] transition-colors select-none"
						onclick={() => handleSort('pids')}
						title="Click to sort by PIDs"
					>
						<div class="inline-flex items-center justify-center gap-1 w-full">
							<span>PIDs</span>
							{#if sortBy === 'pids'}
								{#if sortDesc}
									<CaretDown size={12} class="text-[var(--accent)]" />
								{:else}
									<CaretUp size={12} class="text-[var(--accent)]" />
								{/if}
							{/if}
						</div>
					</th>

					<th class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-20">
						Uptime
					</th>
				</tr>
			</thead>

			<tbody>
				{#if viewMode === 'tree'}
					{#if projectTree.length === 0}
						<tr>
							<td colspan="7" class="px-3.5 py-8 text-center text-[var(--text-tertiary)] text-[13px]">
								No workloads matched your search query.
							</td>
						</tr>
					{:else}
						{#each projectTree as pNode (pNode.project.id)}
							{@const isProjOpen = openProjects[pNode.project.id] ?? true}

							<!-- LEVEL 1: PROJECT ROW -->
							<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] border-b border-[var(--border)] bg-[var(--bg-table-group)]">
								<td class="px-3.5 py-2 text-[13.5px] text-[var(--text-primary)] align-middle whitespace-nowrap">
									<div class="flex items-center gap-2 min-w-0">
										<button
											type="button"
											onclick={() => toggleProject(pNode.project.id)}
											class="p-1 -ml-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border-0 bg-transparent cursor-pointer transition-colors"
											title={isProjOpen ? 'Collapse project' : 'Expand project'}
										>
											{#if isProjOpen}
												<CaretDown size={15} />
											{:else}
												<CaretRight size={15} />
											{/if}
										</button>

										<Folder size={16} class="text-[var(--accent)] shrink-0" />

										<span class="font-bold text-[13.5px] text-[var(--text-primary)] truncate">
											{pNode.project.name}
										</span>

										<span class="text-[11px] font-mono text-[var(--text-tertiary)] bg-[var(--bg-surface)] px-1.5 py-0.5 rounded border border-[var(--border)]">
											{pNode.totalContainers} {pNode.totalContainers > 1 ? 'containers' : 'container'}
										</span>
									</div>
								</td>

								<td class="px-3.5 py-2 text-xs text-[var(--text-secondary)] align-middle whitespace-nowrap">
									<StatusBadge status="running" label="Active" size="sm" />
								</td>

								<!-- CPU Total -->
								<td class="px-3.5 py-2 text-right text-[13.5px] font-mono font-bold text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
									{pNode.totalCpu.toFixed(1)}%
								</td>

								<!-- Memory Total -->
								<td class="px-3.5 py-2 text-right text-[13.5px] font-mono font-bold text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
									{pNode.totalMem} MB
								</td>

								<td class="px-3.5 py-2 text-right text-xs font-mono text-[var(--text-tertiary)] align-middle whitespace-nowrap">
									Aggregate
								</td>

								<td class="px-3.5 py-2 text-center text-xs font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
									—
								</td>

								<td class="px-3.5 py-2 text-right text-xs font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
									—
								</td>
							</tr>

							<!-- LEVEL 2: SERVICE ROWS -->
							{#if isProjOpen}
								{#each pNode.serviceNodes as sNode (sNode.service.id)}
									{@const isSvcOpen = openServices[sNode.service.id] ?? true}
									{@const hasMultiple = sNode.containers.length > 1}

									<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] border-b border-[var(--border-subtle)] bg-[var(--bg-table-subgroup)]">
										<td class="px-3.5 py-2 text-[13px] text-[var(--text-primary)] align-middle whitespace-nowrap">
											<div class="flex items-center gap-2 pl-7 min-w-0">
												{#if hasMultiple}
													<button
														type="button"
														onclick={() => toggleService(sNode.service.id)}
														class="p-1 -ml-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border-0 bg-transparent cursor-pointer transition-colors"
														title={isSvcOpen ? 'Collapse service' : 'Expand service'}
													>
														{#if isSvcOpen}
															<CaretDown size={14} />
														{:else}
															<CaretRight size={14} />
														{/if}
													</button>
												{:else}
													<span class="w-4 inline-block"></span>
												{/if}

												<Gear size={15} class="text-[var(--text-tertiary)] shrink-0" />

												<span class="font-semibold text-[13px] text-[var(--text-primary)] truncate">
													{sNode.service.name}
												</span>

												<span class="text-[10px] font-mono text-[var(--text-tertiary)] bg-[var(--bg-surface)] px-1.5 py-0.5 rounded border border-[var(--border)]">
													{sNode.service.type}
												</span>
											</div>
										</td>

										<td class="px-3.5 py-2 align-middle whitespace-nowrap">
											<StatusBadge status={sNode.service.status} size="sm" />
										</td>

										<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
											{sNode.totalCpu.toFixed(1)}%
										</td>

										<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
											{sNode.totalMem} MB
										</td>

										<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											{sNode.service.port ? `:${sNode.service.port}` : '—'}
										</td>

										<td class="px-3.5 py-2 text-center text-[13px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											{sNode.totalPids || '—'}
										</td>

										<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											Active
										</td>
									</tr>

									<!-- LEVEL 3: CONTAINER ROWS -->
									{#if isSvcOpen}
										{#each sNode.containers as c, cIdx (c.id)}
											{@const limit = c.memoryLimit || 512}
											{@const memPct = getMemPercent(c.memory, limit)}

											<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] border-b border-[var(--border-subtle)] group {cIdx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)]' : 'bg-[var(--bg-table-row)]'}">
												<td class="px-3.5 py-2 text-[13px] text-[var(--text-primary)] align-middle whitespace-nowrap">
													<div class="flex items-center justify-between min-w-0 pr-2">
														<div class="flex items-center gap-2 pl-14 min-w-0">
															<span class="text-[var(--text-tertiary)] font-mono select-none text-[11px]">└─</span>
															<Cube size={15} class="text-[var(--accent)] shrink-0" />
															<div class="flex items-baseline gap-2 min-w-0">
																<span class="font-mono text-[13px] text-[var(--text-primary)] font-medium truncate" title={c.name}>
																	{c.name}
																</span>
																<span class="text-[11px] text-[var(--text-tertiary)] truncate font-mono" title={c.image}>
																	{c.image}
																</span>
															</div>
														</div>

														<!-- Hover Quick Actions (28px hit target, 14px icon) -->
														<div class="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1.5 shrink-0 ml-2">
															<button
																type="button"
																onclick={() => onOpenTerminal?.(c.name)}
																class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs"
																title="Open terminal console"
																aria-label="Open terminal console"
															>
																<Terminal size={14} />
															</button>
															<button
																type="button"
																onclick={() => onOpenLogs?.(c.serviceId, c.name)}
																class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs"
																title="View container logs"
																aria-label="View container logs"
															>
																<FileText size={14} />
															</button>
														</div>
													</div>
												</td>

												<td class="px-3.5 py-2 align-middle whitespace-nowrap">
													<StatusBadge status={c.status} size="sm" />
												</td>

												<!-- CPU with Micro Sparkbar -->
												<td class="px-3.5 py-2 text-right font-mono align-middle whitespace-nowrap">
													<div class="flex items-center justify-end gap-2">
														<div class="w-14 h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block shrink-0">
															<div
																class="h-full rounded-full transition-all duration-300 {getCpuColor(c.cpu)}"
																style="width: {Math.min(c.cpu * 12, 100)}%;"
															></div>
														</div>
														<span class="text-[13px] font-semibold tabular-nums text-[var(--text-primary)]">
															{c.cpu.toFixed(1)}%
														</span>
													</div>
												</td>

												<!-- Memory with Micro Sparkbar -->
												<td class="px-3.5 py-2 text-right font-mono align-middle whitespace-nowrap">
													<div class="flex items-center justify-end gap-2">
														<div class="w-14 h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block shrink-0">
															<div
																class="h-full rounded-full transition-all duration-300 {getMemColor(memPct)}"
																style="width: {memPct}%;"
															></div>
														</div>
														<span class="text-[13px] tabular-nums text-[var(--text-primary)]">
															{c.memory} <span class="text-[10px] text-[var(--text-tertiary)]">/ {limit} MB</span>
														</span>
													</div>
												</td>

												<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
													{c.netTx || '—'}
												</td>

												<td class="px-3.5 py-2 text-center text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
													{c.pids || 1}
												</td>

												<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
													{c.uptime || '—'}
												</td>
											</tr>
										{/each}
									{/if}
								{/each}
							{/if}
						{/each}
					{/if}
				{:else}
					<!-- FLAT & SYSTEM VIEWS -->
					{#if filteredContainers.length === 0}
						<tr>
							<td colspan="7" class="px-3.5 py-8 text-center text-[var(--text-tertiary)] text-[13px]">
								No containers matched your search query.
							</td>
						</tr>
					{:else}
						{#each filteredContainers as c, idx (c.id)}
							{@const limit = c.memoryLimit || 512}
							{@const memPct = getMemPercent(c.memory, limit)}

							<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] border-b border-[var(--border-subtle)] group {idx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)]' : 'bg-[var(--bg-table-row)]'}">
								<td class="px-3.5 py-2 text-[13px] text-[var(--text-primary)] align-middle whitespace-nowrap">
									<div class="flex items-center justify-between min-w-0 pr-2">
										<div class="flex items-center gap-2 min-w-0">
											<Cube size={15} class="text-[var(--accent)] shrink-0" />
											<div class="flex items-baseline gap-2 min-w-0">
												<span class="font-mono text-[13px] font-semibold text-[var(--text-primary)] truncate">
													{c.name}
												</span>
												<span class="text-[11px] text-[var(--text-tertiary)] font-mono truncate">
													{c.projectName} / {c.serviceName} • {c.image}
												</span>
											</div>
										</div>

										<!-- Hover Quick Actions (28px hit target, 14px icon) -->
										<div class="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1.5 shrink-0 ml-2">
											<button
												type="button"
												onclick={() => onOpenTerminal?.(c.name)}
												class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs"
												title="Open terminal console"
												aria-label="Open terminal console"
											>
												<Terminal size={14} />
											</button>
											<button
												type="button"
												onclick={() => onOpenLogs?.(c.serviceId, c.name)}
												class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs"
												title="View container logs"
												aria-label="View container logs"
											>
												<FileText size={14} />
											</button>
										</div>
									</div>
								</td>

								<td class="px-3.5 py-2 align-middle whitespace-nowrap">
									<StatusBadge status={c.status} size="sm" />
								</td>

								<!-- CPU -->
								<td class="px-3.5 py-2 text-right font-mono align-middle whitespace-nowrap">
									<div class="flex items-center justify-end gap-2">
										<div class="w-14 h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block shrink-0">
											<div
												class="h-full rounded-full transition-all duration-300 {getCpuColor(c.cpu)}"
												style="width: {Math.min(c.cpu * 12, 100)}%;"
											></div>
										</div>
										<span class="text-[13px] font-semibold tabular-nums text-[var(--text-primary)]">
											{c.cpu.toFixed(1)}%
										</span>
									</div>
								</td>

								<!-- Memory -->
								<td class="px-3.5 py-2 text-right font-mono align-middle whitespace-nowrap">
									<div class="flex items-center justify-end gap-2">
										<div class="w-14 h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block shrink-0">
											<div
												class="h-full rounded-full transition-all duration-300 {getMemColor(memPct)}"
												style="width: {memPct}%;"
											></div>
										</div>
										<span class="text-[13px] tabular-nums text-[var(--text-primary)]">
											{c.memory} <span class="text-[10px] text-[var(--text-tertiary)]">/ {limit} MB</span>
										</span>
									</div>
								</td>

								<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
									{c.netTx || c.ports || '—'}
								</td>

								<td class="px-3.5 py-2 text-center text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
									{c.pids || 1}
								</td>

								<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
									{c.uptime || 'Active'}
								</td>
							</tr>
						{/each}
					{/if}
				{/if}
			</tbody>
		</table>
	</div>
</div>
