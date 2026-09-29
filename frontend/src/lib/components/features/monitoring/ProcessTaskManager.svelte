<script lang="ts">
	import { dataStore } from '$lib/data';
	import type { Container } from '$lib/types';
	import { StatusBadge } from '$lib/components/ui';
	import { Button, Chip } from '$lib/components/primitives';
	import {
		MagnifyingGlass,
		X,
		CaretDown,
		CaretRight,
		ArrowsDownUp,
		Folder,
		Gear,
		Cube
	} from 'phosphor-svelte';

	interface Props {
		onOpenTerminal?: (containerName: string) => void;
		onOpenLogs?: (serviceId: string, containerName?: string) => void;
	}

	let { onOpenTerminal, onOpenLogs }: Props = $props();

	// View modes
	let viewMode = $state<'tree' | 'flat' | 'system'>('tree');
	let searchQuery = $state('');
	let sortBy = $state<'cpu' | 'memory' | 'name' | 'pids'>('cpu');
	let sortDesc = $state(true);

	// Collapsed state for tree nodes
	let openProjects = $state<Record<string, boolean>>({
		aerochat: true,
		ngumpulhost: true,
		portfolio: true,
		minecraft: true
	});

	let openServices = $state<Record<string, boolean>>({
		'aerochat-web': true,
		'aerochat-realtime': true,
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

	function expandAll() {
		dataStore.projects.forEach((p) => (openProjects[p.id] = true));
		dataStore.services.forEach((s) => (openServices[s.id] = true));
	}

	function collapseAll() {
		dataStore.projects.forEach((p) => (openProjects[p.id] = false));
		dataStore.services.forEach((s) => (openServices[s.id] = false));
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

	let filteredContainers = $derived.by(() => {
		let list = [...allContainers];
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

		return projects
			.map((project) => {
				const services = dataStore.getProjectServices(project.id);

				const serviceNodes = services
					.map((service) => {
						const containers = dataStore.containers.filter((c) => c.serviceId === service.id);

						// Aggregate service metrics
						const totalCpu = containers.reduce((acc, c) => acc + (c.cpu || 0), 0);
						const totalMem = containers.reduce((acc, c) => acc + (c.memory || 0), 0);
						const totalPids = containers.reduce((acc, c) => acc + (c.pids || 0), 0);

						const matchesQuery =
							!query ||
							service.name.toLowerCase().includes(query) ||
							service.type.toLowerCase().includes(query) ||
							containers.some((c) => c.name.toLowerCase().includes(query) || c.image.toLowerCase().includes(query));

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
	});
</script>

<div class="w-full flex flex-col gap-4">
	<!-- ══════════════════════════════════════════════════════════════
	     1. TOOLBAR & CONTROLS (Clean & Minimalist, No Shadows)
	     ══════════════════════════════════════════════════════════════ -->
	<div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
		<!-- View Mode Switcher -->
		<div class="flex items-center gap-1 p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-xs">
			<button
				type="button"
				onclick={() => (viewMode = 'tree')}
				class="px-3 py-1.5 rounded transition-colors border-0 cursor-pointer flex items-center gap-1.5 {viewMode === 'tree'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium border border-[var(--border-subtle)]'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<span>Hierarchy Tree</span>
				<Chip variant="mono" size="sm" class="text-[10px]">Projects</Chip>
			</button>

			<button
				type="button"
				onclick={() => (viewMode = 'flat')}
				class="px-3 py-1.5 rounded transition-colors border-0 cursor-pointer flex items-center gap-1.5 {viewMode === 'flat'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium border border-[var(--border-subtle)]'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<span>Flat Containers</span>
				<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]">({dataStore.containers.length})</span>
			</button>

			<button
				type="button"
				onclick={() => (viewMode = 'system')}
				class="px-3 py-1.5 rounded transition-colors border-0 cursor-pointer flex items-center gap-1.5 {viewMode === 'system'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium border border-[var(--border-subtle)]'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<span>System Daemons</span>
				<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]">({systemContainers.length})</span>
			</button>
		</div>

		<!-- Right: Search, Sorting, Tree Expanders -->
		<div class="flex items-center flex-wrap gap-2 text-xs">
			<!-- Search filter -->
			<div class="flex items-center gap-2 px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] focus-within:border-[var(--accent)] min-w-[240px]">
				<MagnifyingGlass size={14} class="text-[var(--text-tertiary)]" />
				<input
					type="text"
					bind:value={searchQuery}
					placeholder="Search container, service, image…"
					class="w-full bg-transparent border-0 outline-none text-[13px] text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)]"
				/>
				{#if searchQuery}
					<button
						type="button"
						onclick={() => (searchQuery = '')}
						class="text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border-0 bg-transparent cursor-pointer p-0"
					>
						<X size={12} />
					</button>
				{/if}
			</div>

			<!-- Sort dropdown -->
			<div class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)]">
				<ArrowsDownUp size={13} class="text-[var(--text-tertiary)]" />
				<select
					bind:value={sortBy}
					class="bg-transparent border-0 outline-none text-[13px] text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
				>
					<option value="cpu">Sort: CPU %</option>
					<option value="memory">Sort: Memory</option>
					<option value="pids">Sort: PIDs</option>
					<option value="name">Sort: Name</option>
				</select>
			</div>

			{#if viewMode === 'tree'}
				<Button variant="ghost" size="sm" onclick={expandAll} class="h-8 text-[12px]">
					Expand All
				</Button>
				<Button variant="ghost" size="sm" onclick={collapseAll} class="h-8 text-[12px]">
					Collapse
				</Button>
			{/if}

			<!-- Live telemetry indicator -->
			<div class="flex items-center gap-1.5 px-2.5 py-1 rounded bg-[var(--bg-surface)] border border-[var(--border-subtle)] text-[11px] text-[var(--status-green)] font-medium select-none ml-1">
				<span class="w-1.5 h-1.5 rounded-full bg-[var(--status-green)] animate-pulse"></span>
				<span>Live</span>
			</div>
		</div>
	</div>

	<!-- ══════════════════════════════════════════════════════════════
	     2. TASK MANAGER TABLE (Reusable DataTable Style, 14px Font, No Actions, Pure Monitoring)
	     ══════════════════════════════════════════════════════════════ -->
	<div class="w-full overflow-x-auto rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]">
		<table class="w-full border-collapse min-w-[760px]">
			<thead>
				<tr class="border-b border-[var(--border)]">
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-left">
						Workload / Entity Tree
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-left w-28">
						Status
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-right w-36">
						CPU Utilization
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-right w-48">
						Memory (Used / Limit)
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-right w-32">
						Net I/O
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-center w-24">
						PIDs
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-right w-28">
						Uptime
					</th>
				</tr>
			</thead>
			<tbody>
				{#if viewMode === 'tree'}
					<!-- ─────────────────────────────────────────────────────────
					     MODE A: HIERARCHICAL TREE (Project -> Service -> Container)
					     ───────────────────────────────────────────────────────── -->
					{#if projectTree.length === 0}
						<tr>
							<td colspan="7" class="px-3.5 py-8 text-center text-[var(--text-tertiary)] text-[14px]">
								No workloads matched your search query.
							</td>
						</tr>
					{:else}
						{#each projectTree as pNode (pNode.project.id)}
							{@const isProjOpen = openProjects[pNode.project.id] ?? true}

							<!-- LEVEL 1: PROJECT ROW -->
							<tr class="transition-colors duration-100 hover:bg-[var(--bg-hover)] border-b border-[var(--border-subtle)]">
								<!-- Workload / Entity -->
								<td class="px-3.5 py-3 text-[14px] text-[var(--text-primary)] align-middle whitespace-nowrap">
									<div class="flex items-center gap-2 min-w-0">
										<button
											type="button"
											onclick={() => toggleProject(pNode.project.id)}
											class="p-1 -ml-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border-0 bg-transparent cursor-pointer transition-colors"
											title={isProjOpen ? 'Collapse project' : 'Expand project'}
										>
											{#if isProjOpen}
												<CaretDown size={14} />
											{:else}
												<CaretRight size={14} />
											{/if}
										</button>

										<Folder size={16} class="text-[var(--accent)] shrink-0" />

										<span class="font-semibold text-[14px] text-[var(--text-primary)] truncate">
											{pNode.project.name}
										</span>

										<span class="text-[11px] font-mono text-[var(--text-tertiary)] bg-[var(--bg-surface)] px-2 py-0.5 rounded border border-[var(--border)]">
											{pNode.totalContainers} containers
										</span>
									</div>
								</td>

								<!-- Status -->
								<td class="px-3.5 py-3 text-[14px] text-[var(--text-secondary)] align-middle whitespace-nowrap">
									<StatusBadge status="running" label="Active" />
								</td>

								<!-- CPU -->
								<td class="px-3.5 py-3 text-right text-[14px] font-mono font-medium text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
									{pNode.totalCpu.toFixed(1)}%
								</td>

								<!-- Memory -->
								<td class="px-3.5 py-3 text-right text-[14px] font-mono font-medium text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
									{pNode.totalMem} MB
								</td>

								<!-- Net I/O -->
								<td class="px-3.5 py-3 text-right text-[14px] font-mono text-[var(--text-tertiary)] align-middle whitespace-nowrap">
									Aggregate
								</td>

								<!-- PIDs -->
								<td class="px-3.5 py-3 text-center text-[14px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
									—
								</td>

								<!-- Uptime -->
								<td class="px-3.5 py-3 text-right text-[14px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
									—
								</td>
							</tr>

							<!-- LEVEL 2: SERVICE ROWS (WHEN PROJECT OPEN) -->
							{#if isProjOpen}
								{#each pNode.serviceNodes as sNode (sNode.service.id)}
									{@const isSvcOpen = openServices[sNode.service.id] ?? true}
									{@const hasMultiple = sNode.containers.length > 1}

									<tr class="transition-colors duration-100 hover:bg-[var(--bg-hover)] border-b border-[var(--border-subtle)]">
										<!-- Workload / Entity -->
										<td class="px-3.5 py-2.5 text-[14px] text-[var(--text-primary)] align-middle whitespace-nowrap">
											<div class="flex items-center gap-2 pl-7 min-w-0">
												{#if hasMultiple}
													<button
														type="button"
														onclick={() => toggleService(sNode.service.id)}
														class="p-1 -ml-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border-0 bg-transparent cursor-pointer transition-colors"
														title={isSvcOpen ? 'Collapse service' : 'Expand service'}
													>
														{#if isSvcOpen}
															<CaretDown size={13} />
														{:else}
															<CaretRight size={13} />
														{/if}
													</button>
												{:else}
													<span class="w-4 inline-block"></span>
												{/if}

												<Gear size={15} class="text-[var(--text-tertiary)] shrink-0" />

												<span class="font-medium text-[14px] text-[var(--text-primary)] truncate">
													{sNode.service.name}
												</span>

												<Chip variant="mono" size="sm" class="text-[11px]">
													{sNode.service.type}
												</Chip>
											</div>
										</td>

										<!-- Status -->
										<td class="px-3.5 py-2.5 text-[14px] align-middle whitespace-nowrap">
											<StatusBadge status={sNode.service.status} />
										</td>

										<!-- CPU -->
										<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
											{sNode.totalCpu.toFixed(1)}%
										</td>

										<!-- Memory -->
										<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
											{sNode.totalMem} MB
										</td>

										<!-- Net / Port -->
										<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											{sNode.service.port ? `:${sNode.service.port}` : '—'}
										</td>

										<!-- PIDs -->
										<td class="px-3.5 py-2.5 text-center text-[14px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											{sNode.totalPids || '—'}
										</td>

										<!-- Uptime -->
										<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											Active
										</td>
									</tr>

									<!-- LEVEL 3: CONTAINER ROWS (MICRO LEVEL) -->
									{#if isSvcOpen}
										{#each sNode.containers as c (c.id)}
											{@const limit = c.memoryLimit || 512}

											<tr class="transition-colors duration-100 hover:bg-[var(--bg-hover)] border-b border-[var(--border-subtle)]">
												<!-- Workload / Entity -->
												<td class="px-3.5 py-2 text-[14px] text-[var(--text-primary)] align-middle whitespace-nowrap">
													<div class="flex items-center gap-2 pl-14 min-w-0">
														<span class="text-[var(--text-muted)] font-mono select-none text-[13px]">└─</span>
														<Cube size={14} class="text-[var(--text-tertiary)] shrink-0" />
														<div class="flex items-baseline gap-2 min-w-0">
															<span class="font-mono text-[14px] text-[var(--text-primary)] font-medium truncate" title={c.name}>
																{c.name}
															</span>
															<span class="text-[12px] text-[var(--text-tertiary)] truncate font-mono" title={c.image}>
																{c.image}
															</span>
														</div>
													</div>
												</td>

												<!-- Status -->
												<td class="px-3.5 py-2 text-[14px] align-middle whitespace-nowrap">
													<StatusBadge status={c.status} />
												</td>

												<!-- CPU (Clean text, no colored bars) -->
												<td class="px-3.5 py-2 text-right text-[14px] font-mono font-medium text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
													{c.cpu.toFixed(1)}%
												</td>

												<!-- Memory (Clean text, e.g. 128 / 512 MB) -->
												<td class="px-3.5 py-2 text-right text-[14px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
													{c.memory} <span class="text-[12px] text-[var(--text-tertiary)] font-normal">/ {limit} MB</span>
												</td>

												<!-- Net I/O -->
												<td class="px-3.5 py-2 text-right text-[14px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
													{c.netTx || '—'}
												</td>

												<!-- PIDs -->
												<td class="px-3.5 py-2 text-center text-[14px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
													{c.pids || 1}
												</td>

												<!-- Uptime -->
												<td class="px-3.5 py-2 text-right text-[14px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
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
					<!-- ─────────────────────────────────────────────────────────
					     MODE B & C: FLAT CONTAINERS OR SYSTEM DAEMONS
					     ───────────────────────────────────────────────────────── -->
					{#if filteredContainers.length === 0}
						<tr>
							<td colspan="7" class="px-3.5 py-8 text-center text-[var(--text-tertiary)] text-[14px]">
								No containers matched your search query.
							</td>
						</tr>
					{:else}
						{#each filteredContainers as c (c.id)}
							{@const limit = c.memoryLimit || 512}

							<tr class="transition-colors duration-100 hover:bg-[var(--bg-hover)] border-b border-[var(--border-subtle)]">
								<!-- Container Identity -->
								<td class="px-3.5 py-2.5 text-[14px] text-[var(--text-primary)] align-middle whitespace-nowrap">
									<div class="flex items-center gap-2 min-w-0">
										<Cube size={15} class="text-[var(--accent)] shrink-0" />
										<div class="flex items-baseline gap-2 min-w-0">
											<span class="font-mono text-[14px] font-medium text-[var(--text-primary)] truncate">
												{c.name}
											</span>
											<span class="text-[12px] text-[var(--text-tertiary)] font-mono truncate">
												{c.projectName} / {c.serviceName} • {c.image}
											</span>
										</div>
									</div>
								</td>

								<!-- Status -->
								<td class="px-3.5 py-2.5 text-[14px] align-middle whitespace-nowrap">
									<StatusBadge status={c.status} />
								</td>

								<!-- CPU -->
								<td class="px-3.5 py-2.5 text-right text-[14px] font-mono font-medium text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
									{c.cpu.toFixed(1)}%
								</td>

								<!-- Memory -->
								<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
									{c.memory} <span class="text-[12px] text-[var(--text-tertiary)] font-normal">/ {limit} MB</span>
								</td>

								<!-- Net I/O -->
								<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
									{c.netTx || c.ports || '—'}
								</td>

								<!-- PIDs -->
								<td class="px-3.5 py-2.5 text-center text-[14px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
									{c.pids || 1}
								</td>

								<!-- Uptime -->
								<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
									{c.uptime || '—'}
								</td>
							</tr>
						{/each}
					{/if}
				{/if}
			</tbody>
		</table>
	</div>
</div>
