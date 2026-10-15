<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		PageHeader,
		Tabs,
		SearchInput,
		TablePagination,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		LayerCard,
		Badge,
		Button,
		Select,
		Empty,
		Banner
	} from '$lib/components/ui';
	import { api, ApiError } from '$lib/api';
	import type { ActivityItem, EngineEvent, Project, Service } from '$lib/types';
	import { ArrowClockwise, Pause, Play, Trash, CaretDown, CaretRight } from 'phosphor-svelte';

	type MainView = 'activity' | 'console';
	type LevelFilter = 'all' | 'info' | 'warn' | 'error' | 'debug';

	let activeView = $state<MainView>('activity');

	// Filter option data
	let projects = $state<Project[]>([]);
	let services = $state<Service[]>([]);

	// Activity state
	let activityItems = $state<ActivityItem[]>([]);
	let activityTotal = $state(0);
	let activityLoading = $state(true);
	let activityRefreshing = $state(false);
	let activityError = $state<string | null>(null);
	let activityNotAvailable = $state(false);
	let selectedProjectId = $state('all');
	let selectedServiceId = $state('all');
	let selectedStatus = $state('all');
	let actionQuery = $state('');
	let debouncedAction = $state('');
	let activityPage = $state(1);
	let activityPageSize = $state(10);

	// Console state
	let engineItems = $state<EngineEvent[]>([]);
	let engineLoading = $state(true);
	let engineError = $state<string | null>(null);
	let engineNotAvailable = $state(false);
	let levelFilter = $state<LevelFilter>('all');
	let consoleServiceId = $state('all');
	let tailing = $state(true);
	let expanded = $state<Record<string, boolean>>({});
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	const mainTabs = [
		{ id: 'activity', label: 'Activity' },
		{ id: 'console', label: 'Console' }
	];

	const levelTabs = [
		{ id: 'all', label: 'All' },
		{ id: 'info', label: 'Info' },
		{ id: 'warn', label: 'Warn' },
		{ id: 'error', label: 'Error' },
		{ id: 'debug', label: 'Debug' }
	];

	let filteredServices = $derived(
		selectedProjectId === 'all'
			? services
			: services.filter((s) => s.projectId === selectedProjectId)
	);

	function isNotFound(err: unknown): boolean {
		if (err instanceof ApiError) return err.status === 404;
		return (err as { status?: number })?.status === 404;
	}

	function timeAgo(ts: string): string {
		const t = new Date(ts).getTime();
		if (Number.isNaN(t)) return ts || '-';
		const diffSec = Math.max(0, Math.floor((Date.now() - t) / 1000));
		if (diffSec < 10) return 'just now';
		if (diffSec < 60) return `${diffSec}s ago`;
		const diffMin = Math.floor(diffSec / 60);
		if (diffMin < 60) return `${diffMin}m ago`;
		const diffHrs = Math.floor(diffMin / 60);
		if (diffHrs < 24) return `${diffHrs}h ago`;
		const diffDays = Math.floor(diffHrs / 24);
		if (diffDays < 30) return `${diffDays}d ago`;
		return new Date(ts).toLocaleDateString();
	}

	function formatTime(ts: string): string {
		const d = new Date(ts);
		if (Number.isNaN(d.getTime())) return ts || '-';
		return d.toLocaleString();
	}

	function shortId(id?: string): string {
		if (!id) return '-';
		return id.length > 8 ? id.slice(0, 8) : id;
	}

	function resolveServiceName(id?: string): string {
		if (!id) return '-';
		return services.find((s) => s.id === id)?.name ?? shortId(id);
	}

	function resolveProjectName(id?: string): string {
		if (!id) return '-';
		return projects.find((p) => p.id === id)?.name ?? shortId(id);
	}

	function activityStatusVariant(status: string): 'success' | 'error' | 'info' | 'neutral' {
		const s = (status || '').toLowerCase();
		if (s === 'success') return 'success';
		if (s === 'failed' || s === 'error') return 'error';
		if (s === 'info') return 'info';
		return 'neutral';
	}

	function levelVariant(level: string): 'info' | 'warning' | 'error' | 'neutral' | 'success' {
		const l = (level || '').toLowerCase();
		if (l === 'info') return 'info';
		if (l === 'warn' || l === 'warning') return 'warning';
		if (l === 'error') return 'error';
		if (l === 'debug') return 'neutral';
		return 'neutral';
	}

	function openEntity(item: ActivityItem) {
		if (item.serviceId && item.projectId) {
			goto(`/projects/${item.projectId}/services/${item.serviceId}`);
			return;
		}
		if (item.serviceId) {
			const svc = services.find((s) => s.id === item.serviceId);
			if (svc?.projectId) {
				goto(`/projects/${svc.projectId}/services/${item.serviceId}`);
				return;
			}
		}
		if (item.projectId) {
			goto(`/projects/${item.projectId}`);
		}
	}

	function entityTarget(item: ActivityItem): string | null {
		if (item.serviceId) return `Service: ${resolveServiceName(item.serviceId)}`;
		if (item.projectId) return `Project: ${resolveProjectName(item.projectId)}`;
		if (item.entity) return item.entityId ? `${item.entity}: ${shortId(item.entityId)}` : item.entity;
		return null;
	}

	function hasEntityLink(item: ActivityItem): boolean {
		return Boolean(item.serviceId || item.projectId);
	}

	async function fetchFilterOptions() {
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
	}

	async function fetchActivity() {
		if (activityNotAvailable) {
			activityLoading = false;
			return;
		}
		activityLoading = true;
		activityError = null;
		try {
			const res = await api.activity.list({
				projectId: selectedProjectId !== 'all' ? selectedProjectId : undefined,
				serviceId: selectedServiceId !== 'all' ? selectedServiceId : undefined,
				action: debouncedAction.trim() ? debouncedAction.trim() : undefined,
				status: selectedStatus !== 'all' ? selectedStatus : undefined,
				limit: activityPageSize,
				offset: (activityPage - 1) * activityPageSize
			});
			activityItems = res.items ?? [];
			activityTotal = typeof res.total === 'number' ? res.total : activityItems.length;
		} catch (err) {
			if (isNotFound(err)) {
				activityNotAvailable = true;
				activityItems = [];
				activityTotal = 0;
			} else {
				activityError = err instanceof Error ? err.message : 'Failed to load activity.';
			}
		} finally {
			activityLoading = false;
			activityRefreshing = false;
		}
	}

	async function fetchEngine(poll = false) {
		if (engineNotAvailable) {
			engineLoading = false;
			return;
		}
		if (!poll) engineLoading = true;
		engineError = null;
		try {
			const latest = engineItems[0]?.ts;
			const res = await api.engineEvents.list({
				level: levelFilter !== 'all' ? levelFilter : undefined,
				serviceId: consoleServiceId !== 'all' ? consoleServiceId : undefined,
				since: poll && latest ? latest : undefined,
				limit: 100
			});
			const incoming = res.items ?? [];
			if (poll && latest) {
				const known = new Set(engineItems.map((e) => e.id));
				const fresh = incoming.filter((e) => !known.has(e.id));
				engineItems = [...fresh, ...engineItems].slice(0, 200);
			} else {
				engineItems = incoming.slice(0, 200);
			}
		} catch (err) {
			if (isNotFound(err)) {
				engineNotAvailable = true;
				engineItems = [];
			} else if (!poll) {
				engineError = err instanceof Error ? err.message : 'Failed to load engine events.';
			}
		} finally {
			engineLoading = false;
		}
	}

	function startPolling() {
		stopPolling();
		if (!tailing) return;
		pollTimer = setInterval(() => {
			if (activeView === 'console' && tailing && !engineNotAvailable) {
				void fetchEngine(true);
			}
		}, 3000);
	}

	function stopPolling() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	function toggleTail() {
		tailing = !tailing;
		if (tailing) {
			void fetchEngine(false);
			startPolling();
		} else {
			stopPolling();
		}
	}

	function clearConsoleView() {
		engineItems = [];
		expanded = {};
	}

	function toggleExpanded(id: string) {
		expanded = { ...expanded, [id]: !expanded[id] };
	}

	function handleActivityRefresh() {
		activityRefreshing = true;
		void fetchActivity();
	}

	function resetActivityPage() {
		activityPage = 1;
	}

	onMount(() => {
		void fetchFilterOptions();
		void fetchActivity();
		void fetchEngine(false);
		startPolling();
	});

	onDestroy(() => {
		stopPolling();
	});

	// Debounce action text filter
	$effect(() => {
		const q = actionQuery;
		const t = setTimeout(() => {
			debouncedAction = q;
		}, 350);
		return () => clearTimeout(t);
	});

	// Refetch activity when view, filters, or pagination change
	$effect(() => {
		if (activeView !== 'activity') return;
		// Track reactive deps
		void selectedProjectId;
		void selectedServiceId;
		void selectedStatus;
		void debouncedAction;
		void activityPage;
		void activityPageSize;
		void fetchActivity();
	});

	// Refetch console when level or service filter changes; manage polling lifecycle
	$effect(() => {
		if (activeView !== 'console') return;
		void levelFilter;
		void consoleServiceId;
		void fetchEngine(false);
		startPolling();
		return () => stopPolling();
	});

	// Keep polling aligned with tailing toggle and active view
	$effect(() => {
		void tailing;
		void activeView;
		if (activeView === 'console' && tailing && !engineNotAvailable) {
			startPolling();
		} else {
			stopPolling();
		}
		return () => stopPolling();
	});
</script>

<svelte:head>
	<title>Audit Log - GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader
		title="Audit Log"
		subtitle="Operator activity timeline and engine console stream."
	/>

	<Tabs tabs={mainTabs} bind:active={activeView} />

	{#if activeView === 'activity'}
		<!-- Activity filters -->
		<div class="flex flex-col items-stretch justify-between gap-3 sm:flex-row sm:items-center">
			<div class="flex flex-1 flex-wrap items-center gap-2.5">
				<div class="w-full sm:w-56">
					<SearchInput bind:value={actionQuery} placeholder="Filter by action, actor..." />
				</div>
				<div class="w-36">
					<Select
						value={selectedProjectId}
						onValueChange={(val) => {
							if (typeof val === 'string') {
								selectedProjectId = val;
								selectedServiceId = 'all';
								resetActivityPage();
							}
						}}
						options={[
							{ label: 'All projects', value: 'all' },
							...projects.map((p) => ({ label: p.name, value: p.id }))
						]}
						size="sm"
					/>
				</div>
				<div class="w-36">
					<Select
						value={selectedServiceId}
						onValueChange={(val) => {
							if (typeof val === 'string') {
								selectedServiceId = val;
								resetActivityPage();
							}
						}}
						options={[
							{ label: 'All services', value: 'all' },
							...filteredServices.map((s) => ({ label: s.name, value: s.id }))
						]}
						size="sm"
					/>
				</div>
				<div class="w-32">
					<Select
						value={selectedStatus}
						onValueChange={(val) => {
							if (typeof val === 'string') {
								selectedStatus = val;
								resetActivityPage();
							}
						}}
						options={[
							{ label: 'All statuses', value: 'all' },
							{ label: 'Success', value: 'success' },
							{ label: 'Failed', value: 'failed' },
							{ label: 'Info', value: 'info' }
						]}
						size="sm"
					/>
				</div>
			</div>
			<div class="flex shrink-0 items-center gap-2">
				<Button variant="secondary" size="sm" onclick={handleActivityRefresh} disabled={activityRefreshing}>
					<ArrowClockwise size={13} class={activityRefreshing ? 'animate-spin' : ''} />
					<span>{activityRefreshing ? 'Refreshing...' : 'Refresh'}</span>
				</Button>
			</div>
		</div>

		{#if activityNotAvailable}
			<Empty
				size="base"
				title="Activity endpoint unavailable"
				description="Backend has not deployed GET /api/activity yet (404). The operator activity timeline will appear here once the endpoint is available."
			/>
		{:else if activityError}
			<Banner variant="error" size="sm">
				<div class="text-xs">
					<span class="font-medium">Failed to load activity: </span>
					<span>{activityError}</span>
				</div>
			</Banner>
		{:else}
			<LayerCard class="overflow-hidden p-0">
				<div class="w-full overflow-x-auto md:overflow-x-visible">
					<Table class="w-full text-left">
						<TableHeader
							class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
						>
							<TableRow>
								<TableHead
									class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
								>
									Actor
								</TableHead>
								<TableHead
									class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
								>
									Action
								</TableHead>
								<TableHead
									class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
								>
									Entity
								</TableHead>
								<TableHead
									class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
								>
									Status
								</TableHead>
								<TableHead
									class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
								>
									When
								</TableHead>
								<TableHead
									class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
								>
									Source IP
								</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody class="divide-y divide-[var(--border-subtle)]">
							{#if activityLoading && activityItems.length === 0}
								<TableRow>
									<TableCell colspan={6} class="py-12 text-center text-[13px] text-[var(--text-tertiary)]">
										Loading operator activity...
									</TableCell>
								</TableRow>
							{:else if activityItems.length === 0}
								<TableRow>
									<TableCell colspan={6} class="py-12 text-center">
										<div class="mx-auto flex max-w-md flex-col items-center gap-2 text-center">
											<p class="m-0 text-[13px] font-medium text-[var(--text-primary)]">
												No activity found
											</p>
											<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
												No operator actions match the current filters. Try clearing the action
												search or selecting a different project, service, or status.
											</p>
										</div>
									</TableCell>
								</TableRow>
							{:else}
								{#each activityItems as item, idx (item.id)}
									<TableRow
										class={idx % 2 === 1
											? 'bg-[var(--bg-table-row-alt)] hover:bg-[var(--bg-table-row-hover)]'
											: 'bg-[var(--bg-table-row)] hover:bg-[var(--bg-table-row-hover)]'}
									>
										<TableCell class="px-3.5 py-3 text-[13px] whitespace-nowrap text-[var(--text-primary)]">
											{item.actor || '-'}
										</TableCell>
										<TableCell class="min-w-[220px] px-3 py-3">
											<div class="flex flex-col gap-0.5">
												<span class="font-mono text-[13px] font-medium text-[var(--text-primary)]">
													{item.action || '-'}
												</span>
												{#if item.detail}
													<span class="max-w-[320px] truncate text-xs text-[var(--text-tertiary)]">
														{item.detail}
													</span>
												{/if}
											</div>
										</TableCell>
										<TableCell class="px-3 py-3 text-[13px] whitespace-nowrap">
											{#if hasEntityLink(item)}
												<button
													type="button"
													onclick={() => openEntity(item)}
													class="cursor-pointer border-0 bg-transparent p-0 text-left text-[13px] font-medium text-[var(--accent)] hover:underline"
												>
													{entityTarget(item)}
												</button>
											{:else}
												<span class="text-[var(--text-secondary)]">{entityTarget(item) ?? '-'}</span>
											{/if}
										</TableCell>
										<TableCell class="px-3 py-3 whitespace-nowrap">
											<Badge variant={activityStatusVariant(item.status)} size="sm">
												{item.status || 'info'}
											</Badge>
										</TableCell>
										<TableCell class="px-3 py-3 text-[13px] whitespace-nowrap text-[var(--text-tertiary)]">
											{timeAgo(item.ts)}
										</TableCell>
										<TableCell class="px-3.5 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]">
											{item.ip || '-'}
										</TableCell>
									</TableRow>
								{/each}
							{/if}
						</TableBody>
					</Table>
				</div>
				<TablePagination
					bind:page={activityPage}
					bind:pageSize={activityPageSize}
					totalCount={activityTotal}
					pageSizeOptions={[10, 25, 50]}
					itemName="events"
				/>
			</LayerCard>
		{/if}
	{:else}
		<!-- Console controls -->
		<div class="flex flex-col items-stretch justify-between gap-3 sm:flex-row sm:items-center">
			<div class="flex flex-1 flex-wrap items-center gap-2.5">
				<Tabs
					variant="segmented"
					size="sm"
					tabs={levelTabs}
					bind:active={levelFilter}
				/>
				<div class="w-40">
					<Select
						value={consoleServiceId}
						onValueChange={(val) => {
							if (typeof val === 'string') consoleServiceId = val;
						}}
						options={[
							{ label: 'All services', value: 'all' },
							...services.map((s) => ({ label: s.name, value: s.id }))
						]}
						size="sm"
					/>
				</div>
			</div>
			<div class="flex shrink-0 items-center gap-2">
				<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
					{tailing ? 'Live tail (3s)' : 'Paused'}
				</span>
				<Button variant="secondary" size="sm" onclick={toggleTail}>
					{#if tailing}
						<Pause size={13} />
						<span>Pause</span>
					{:else}
						<Play size={13} />
						<span>Resume</span>
					{/if}
				</Button>
				<Button variant="ghost" size="sm" onclick={clearConsoleView}>
					<Trash size={13} />
					<span>Clear view</span>
				</Button>
			</div>
		</div>

		{#if engineNotAvailable}
			<Empty
				size="base"
				title="Engine events endpoint unavailable"
				description="Backend has not deployed GET /api/engine/events yet (404). The engine console stream will appear here once the endpoint is available."
			/>
		{:else if engineError}
			<Banner variant="error" size="sm">
				<div class="text-xs">
					<span class="font-medium">Failed to load engine events: </span>
					<span>{engineError}</span>
				</div>
			</Banner>
		{:else}
			<LayerCard class="overflow-hidden p-0">
				<div class="flex items-center justify-between border-b border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2.5">
					<span class="text-[13px] font-semibold text-[var(--text-secondary)]">
						Engine console
					</span>
					<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
						{engineItems.length} events in view
					</span>
				</div>
				{#if engineLoading && engineItems.length === 0}
					<div class="p-8 text-center text-[13px] text-[var(--text-tertiary)]">
						Loading engine events...
					</div>
				{:else if engineItems.length === 0}
					<div class="p-4">
						<Empty
							size="sm"
							title="No engine events"
							description="No engine internals match the current level and service filters. Try selecting a different level or service, or resume live tail."
						/>
					</div>
				{:else}
					<div class="max-h-[480px] overflow-y-auto">
						<div class="flex flex-col divide-y divide-[var(--border-subtle)]">
							{#each engineItems as evt (evt.id)}
								<div class="flex flex-col gap-1.5 px-4 py-2.5 text-[13px]">
									<div class="flex flex-wrap items-center gap-2">
										<span class="font-mono text-xs whitespace-nowrap text-[var(--text-tertiary)]">
											{formatTime(evt.ts)}
										</span>
										<Badge variant={levelVariant(evt.level)} size="sm" class="font-mono uppercase">
											{evt.level || 'info'}
										</Badge>
										{#if evt.scope}
											<span class="font-mono text-xs text-[var(--text-secondary)]">
												{evt.scope}
											</span>
										{/if}
										{#if evt.serviceId}
											<span class="font-mono text-xs text-[var(--text-tertiary)]">
												{resolveServiceName(evt.serviceId)}
											</span>
										{/if}
										{#if evt.data}
											<button
												type="button"
												onclick={() => toggleExpanded(evt.id)}
												class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 font-mono text-[11px] text-[var(--accent)] hover:underline"
												aria-label={expanded[evt.id] ? 'Collapse event data' : 'Expand event data'}
											>
												{#if expanded[evt.id]}
													<CaretDown size={12} />
													<span>Hide data</span>
												{:else}
													<CaretRight size={12} />
													<span>Show data</span>
												{/if}
											</button>
										{/if}
									</div>
									<p class="m-0 font-mono text-[13px] leading-relaxed break-words text-[var(--text-primary)]">
										{evt.message || evt.action || '-'}
									</p>
									{#if evt.data && expanded[evt.id]}
										<pre class="m-0 overflow-x-auto rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap text-[var(--text-secondary)]">{JSON.stringify(evt.data, null, 2)}</pre>
									{/if}
								</div>
							{/each}
						</div>
					</div>
				{/if}
			</LayerCard>
		{/if}
	{/if}
</div>
