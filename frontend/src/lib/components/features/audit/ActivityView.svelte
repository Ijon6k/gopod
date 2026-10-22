<script lang="ts">
	import {
		Banner,
		Badge,
		Button,
		Empty,
		LayerCard,
		SearchInput,
		Select,
		Table,
		TableBody,
		TableCell,
		TableHead,
		TableHeader,
		TablePagination,
		TableRow
	} from '$lib/components/ui';
	import { api, ApiError } from '$lib/api';
	import { ArrowClockwise } from 'phosphor-svelte';
	import type { ActivityItem, Project, Service } from '$lib/types';
	import { activityStatusVariant, entityTarget, hasEntityLink, isNotFound, openEntity, timeAgo } from './audit-helpers';

	interface Props {
		projects: Project[];
		services: Service[];
	}

	let { projects, services }: Props = $props();

	let items = $state<ActivityItem[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<string | null>(null);
	let notAvailable = $state(false);

	let selectedProjectId = $state('all');
	let selectedServiceId = $state('all');
	let selectedStatus = $state('all');
	let actionQuery = $state('');
	let debouncedAction = $state('');
	let page = $state(1);
	let pageSize = $state(10);

	let filteredServices = $derived(
		selectedProjectId === 'all' ? services : services.filter((s) => s.projectId === selectedProjectId)
	);

	async function fetchActivity() {
		if (notAvailable) {
			loading = false;
			return;
		}
		loading = true;
		error = null;
		try {
			const res = await api.activity.list({
				projectId: selectedProjectId !== 'all' ? selectedProjectId : undefined,
				serviceId: selectedServiceId !== 'all' ? selectedServiceId : undefined,
				action: debouncedAction.trim() ? debouncedAction.trim() : undefined,
				status: selectedStatus !== 'all' ? selectedStatus : undefined,
				limit: pageSize,
				offset: (page - 1) * pageSize
			});
			items = res.items ?? [];
			total = typeof res.total === 'number' ? res.total : items.length;
		} catch (err) {
			if (isNotFound(err)) {
				notAvailable = true;
				items = [];
				total = 0;
			} else {
				error = err instanceof Error ? err.message : 'Failed to load activity.';
			}
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	function handleRefresh() {
		refreshing = true;
		void fetchActivity();
	}

	function resetPage() {
		page = 1;
	}

	// Debounce action text filter
	$effect(() => {
		const q = actionQuery;
		const t = setTimeout(() => {
			debouncedAction = q;
		}, 350);
		return () => clearTimeout(t);
	});

	// Refetch when filters or pagination change
	$effect(() => {
		void selectedProjectId;
		void selectedServiceId;
		void selectedStatus;
		void debouncedAction;
		void page;
		void pageSize;
		void fetchActivity();
	});
</script>

<!-- Filters -->
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
						resetPage();
					}
				}}
				options={[{ label: 'All projects', value: 'all' }, ...projects.map((p) => ({ label: p.name, value: p.id }))]}
				size="sm"
			/>
		</div>
		<div class="w-36">
			<Select
				value={selectedServiceId}
				onValueChange={(val) => {
					if (typeof val === 'string') {
						selectedServiceId = val;
						resetPage();
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
						resetPage();
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
		<Button variant="secondary" size="sm" onclick={handleRefresh} disabled={refreshing}>
			<ArrowClockwise size={13} class={refreshing ? 'animate-spin' : ''} />
			<span>{refreshing ? 'Refreshing...' : 'Refresh'}</span>
		</Button>
	</div>
</div>

{#if notAvailable}
	<Empty
		size="base"
		title="Activity endpoint unavailable"
		description="Backend has not deployed GET /api/activity yet (404). The operator activity timeline will appear here once the endpoint is available."
	/>
{:else if error}
	<Banner variant="error" size="sm">
		<div class="text-xs">
			<span class="font-medium">Failed to load activity: </span>
			<span>{error}</span>
		</div>
	</Banner>
{:else}
	<LayerCard class="overflow-hidden p-0">
		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<Table class="w-full text-left">
				<TableHeader class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]">
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
					{#if loading && items.length === 0}
						<TableRow>
							<TableCell colspan={6} class="py-12 text-center text-[13px] text-[var(--text-tertiary)]">
								Loading operator activity...
							</TableCell>
						</TableRow>
					{:else if items.length === 0}
						<TableRow>
							<TableCell colspan={6} class="py-12 text-center">
								<div class="mx-auto flex max-w-md flex-col items-center gap-2 text-center">
									<p class="m-0 text-[13px] font-medium text-[var(--text-primary)]">No activity found</p>
									<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
										No operator actions match the current filters. Try clearing the action search or
										selecting a different project, service, or status.
									</p>
								</div>
							</TableCell>
						</TableRow>
					{:else}
						{#each items as item, idx (item.id)}
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
											onclick={() => openEntity(item, services)}
											class="cursor-pointer border-0 bg-transparent p-0 text-left text-[13px] font-medium text-[var(--accent)] hover:underline"
										>
											{entityTarget(item, services, projects)}
										</button>
									{:else}
										<span class="text-[var(--text-secondary)]">
											{entityTarget(item, services, projects) ?? '-'}
										</span>
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
								<TableCell
									class="px-3.5 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
								>
									{item.ip || '-'}
								</TableCell>
							</TableRow>
						{/each}
					{/if}
				</TableBody>
			</Table>
		</div>
		<TablePagination
			bind:page
			bind:pageSize
			totalCount={total}
			pageSizeOptions={[10, 25, 50]}
			itemName="events"
		/>
	</LayerCard>
{/if}
