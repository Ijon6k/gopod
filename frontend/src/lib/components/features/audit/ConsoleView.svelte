<script lang="ts">
	import { Badge, Banner, Button, Empty, LayerCard, Select, Tabs } from '$lib/components/ui';
	import { api } from '$lib/api';
	import { createPoller } from '$lib/utils/poller';
	import { CaretDown, CaretRight, Pause, Play, Trash } from 'phosphor-svelte';
	import type { EngineEvent, Service } from '$lib/types';
	import { formatTime, isNotFound, levelVariant, resolveServiceName, type LevelFilter } from './audit-helpers';

	interface Props {
		services: Service[];
	}

	let { services }: Props = $props();

	// Engine tail poller: guarded against overlapping fetches and paused while
	// the browser tab is hidden.
	const tailPoller = createPoller(
		3000,
		() => {
			if (tailing && !notAvailable) void fetchEngine(true);
		},
		{ pauseWhenHidden: true }
	);

	const levelTabs = [
		{ id: 'all', label: 'All' },
		{ id: 'info', label: 'Info' },
		{ id: 'warn', label: 'Warn' },
		{ id: 'error', label: 'Error' },
		{ id: 'debug', label: 'Debug' }
	];

	let items = $state<EngineEvent[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let notAvailable = $state(false);
	let levelFilter = $state<LevelFilter>('all');
	let consoleServiceId = $state('all');
	let tailing = $state(true);
	let expanded = $state<Record<string, boolean>>({});

	async function fetchEngine(poll = false) {
		if (notAvailable) {
			loading = false;
			return;
		}
		if (!poll) loading = true;
		error = null;
		try {
			const latest = items[0]?.ts;
			const res = await api.engineEvents.list({
				level: levelFilter !== 'all' ? levelFilter : undefined,
				serviceId: consoleServiceId !== 'all' ? consoleServiceId : undefined,
				since: poll && latest ? latest : undefined,
				limit: 100
			});
			const incoming = res.items ?? [];
			if (poll && latest) {
				const known = new Set(items.map((e) => e.id));
				const fresh = incoming.filter((e) => !known.has(e.id));
				items = [...fresh, ...items].slice(0, 200);
			} else {
				items = incoming.slice(0, 200);
			}
		} catch (err) {
			if (isNotFound(err)) {
				notAvailable = true;
				items = [];
			} else if (!poll) {
				error = err instanceof Error ? err.message : 'Failed to load engine events.';
			}
		} finally {
			loading = false;
		}
	}

	function toggleTail() {
		tailing = !tailing;
		if (tailing) void fetchEngine(false);
	}

	function clearConsoleView() {
		items = [];
		expanded = {};
	}

	function toggleExpanded(id: string) {
		expanded = { ...expanded, [id]: !expanded[id] };
	}

	// Refetch when filters change; tail poller rides this effect's lifecycle.
	$effect(() => {
		void levelFilter;
		void consoleServiceId;
		void fetchEngine(false);
	});

	// Live-tail alignment: poll while tailing, pause otherwise; unmount stops it.
	$effect(() => {
		void tailing;
		void notAvailable;
		if (tailing && !notAvailable) {
			tailPoller.start();
		} else {
			tailPoller.stop();
		}
		return () => tailPoller.stop();
	});
</script>

<!-- Controls -->
<div class="flex flex-col items-stretch justify-between gap-3 sm:flex-row sm:items-center">
	<div class="flex flex-1 flex-wrap items-center gap-2.5">
		<Tabs variant="segmented" size="sm" tabs={levelTabs} bind:active={levelFilter} />
		<div class="w-40">
			<Select
				value={consoleServiceId}
				onValueChange={(val) => {
					if (typeof val === 'string') consoleServiceId = val;
				}}
				options={[{ label: 'All services', value: 'all' }, ...services.map((s) => ({ label: s.name, value: s.id }))]}
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

{#if notAvailable}
	<Empty
		size="base"
		title="Engine events endpoint unavailable"
		description="Backend has not deployed GET /api/engine/events yet (404). The engine console stream will appear here once the endpoint is available."
	/>
{:else if error}
	<Banner variant="error" size="sm">
		<div class="text-xs">
			<span class="font-medium">Failed to load engine events: </span>
			<span>{error}</span>
		</div>
	</Banner>
{:else}
	<LayerCard class="overflow-hidden p-0">
		<div
			class="flex items-center justify-between border-b border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2.5"
		>
			<span class="text-[13px] font-semibold text-[var(--text-secondary)]">Engine console</span>
			<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
				{items.length} events in view
			</span>
		</div>
		{#if loading && items.length === 0}
			<div class="p-8 text-center text-[13px] text-[var(--text-tertiary)]">Loading engine events...</div>
		{:else if items.length === 0}
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
					{#each items as evt (evt.id)}
						<div class="flex flex-col gap-1.5 px-4 py-2.5 text-[13px]">
							<div class="flex flex-wrap items-center gap-2">
								<span class="font-mono text-xs whitespace-nowrap text-[var(--text-tertiary)]">
									{formatTime(evt.ts)}
								</span>
								<Badge variant={levelVariant(evt.level)} size="sm" class="font-mono uppercase">
									{evt.level || 'info'}
								</Badge>
								{#if evt.scope}
									<span class="font-mono text-xs text-[var(--text-secondary)]">{evt.scope}</span>
								{/if}
								{#if evt.serviceId}
									<span class="font-mono text-xs text-[var(--text-tertiary)]">
										{resolveServiceName(evt.serviceId, services)}
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
								<pre
									class="m-0 overflow-x-auto rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap text-[var(--text-secondary)]">{JSON.stringify(evt.data, null, 2)}</pre>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</LayerCard>
{/if}
