<script lang="ts">
	import type { Service, Workload } from '$lib/types';
	import { SearchInput } from '$lib/components/ui';
	import { Button } from '$lib/components/primitives';
	import { ArrowDown, Trash } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let isPod = $derived(service.type === 'pod' || (service.workloads && service.workloads.length > 1));
	let workloads = $derived<Workload[]>(service.workloads ?? [{ name: service.name, image: service.image ?? '—', status: service.status }]);

	let selectedContainer = $state('all');
	let searchQuery = $state('');
	let follow = $state(true);

	// Generate realistic logs
	function generateLogs(containerName: string) {
		const baseTimes = [
			'10:14:02.104',
			'10:14:03.250',
			'10:14:04.890',
			'10:15:10.012',
			'10:15:42.511',
			'10:16:01.300',
			'10:16:22.784',
			'10:17:05.120',
			'10:17:34.901'
		];

		const entries = [
			{ level: 'INFO', text: `[${containerName}] Container initialized with Podman cgroups v2` },
			{ level: 'INFO', text: `[${containerName}] Starting runtime daemon on 0.0.0.0:${service.port || 3000}` },
			{ level: 'INFO', text: `[${containerName}] Environment loaded: NODE_ENV=production, PORT=${service.port || 3000}` },
			{ level: 'INFO', text: `[${containerName}] Connected to database cluster successfully` },
			{ level: 'INFO', text: `[${containerName}] GET /health 200 OK (1.2ms)` },
			{ level: 'WARN', text: `[${containerName}] Cache latency slightly elevated: 12ms` },
			{ level: 'INFO', text: `[${containerName}] Inbound WebSocket connection established (client_id=c_910)` },
			{ level: 'INFO', text: `[${containerName}] Health probe passed: status=UP` },
			{ level: 'INFO', text: `[${containerName}] HTTP request completed: 200 OK (0.8ms)` }
		];

		return entries.map((e, idx) => ({
			time: baseTimes[idx] ?? '10:18:00.000',
			container: containerName,
			level: e.level,
			text: e.text
		}));
	}

	let allLogs = $derived.by(() => {
		if (!isPod || selectedContainer === 'all') {
			return workloads.flatMap((w) => generateLogs(w.name)).sort((a, b) => a.time.localeCompare(b.time));
		}
		return generateLogs(selectedContainer);
	});

	let filteredLogs = $derived.by(() => {
		if (!searchQuery.trim()) return allLogs;
		const q = searchQuery.toLowerCase().trim();
		return allLogs.filter((l) => l.text.toLowerCase().includes(q) || l.container.toLowerCase().includes(q));
	});

	let logsContainer = $state<HTMLDivElement | null>(null);

	$effect(() => {
		if (follow && logsContainer) {
			logsContainer.scrollTop = logsContainer.scrollHeight;
		}
	});
</script>

<div class="w-full flex flex-col gap-4">
	<!-- Filter bar -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-[var(--border-subtle)]">
		<div class="flex flex-wrap items-center gap-3">
			{#if isPod}
				<div class="flex items-center gap-2">
					<span class="text-xs text-[var(--text-tertiary)] font-medium">Container:</span>
					<div class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
						<select
							bind:value={selectedContainer}
							class="bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
						>
							<option value="all">All containers ({workloads.length})</option>
							{#each workloads as w}
								<option value={w.name}>{w.name}</option>
							{/each}
						</select>
					</div>
				</div>
			{/if}

			<div class="w-56">
				<SearchInput bind:value={searchQuery} placeholder="Filter logs…" />
			</div>
		</div>

		<!-- Right actions -->
		<div class="flex items-center gap-3">
			<label class="flex items-center gap-1.5 text-xs text-[var(--text-secondary)] cursor-pointer select-none">
				<input type="checkbox" bind:checked={follow} class="accent-[var(--accent)]" />
				<span>Follow logs</span>
			</label>

			<Button variant="ghost" size="sm" onclick={() => (searchQuery = '')}>
				Clear filter
			</Button>
		</div>
	</div>

	<!-- Log Stream Box -->
	<div
		bind:this={logsContainer}
		class="relative p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-terminal)] text-[var(--code-text)] font-[var(--font-mono)] text-[12px] leading-[1.65] h-[480px] overflow-auto select-text transition-colors"
	>
		{#each filteredLogs as log}
			<div class="flex items-baseline gap-3 hover:bg-[rgba(255,255,255,0.03)] px-1 rounded">
				<span class="text-[var(--text-tertiary)] select-none whitespace-nowrap text-[11px]">{log.time}</span>
				{#if isPod}
					<span class="text-[var(--accent)] font-medium text-[11px] select-none min-w-[70px]">[{log.container}]</span>
				{/if}
				<span
					class="select-none font-semibold text-[10.5px] min-w-[42px] {log.level === 'WARN'
						? 'text-[var(--status-amber)]'
						: log.level === 'ERROR'
							? 'text-[var(--status-red)]'
							: 'text-[var(--text-tertiary)]'}"
				>
					{log.level}
				</span>
				<span class="text-[var(--text-secondary)] whitespace-pre-wrap break-all flex-1">{log.text}</span>
			</div>
		{/each}

		{#if filteredLogs.length === 0}
			<div class="p-8 text-center text-xs text-[var(--text-tertiary)]">
				No log lines matching "{searchQuery}".
			</div>
		{/if}
	</div>
</div>
