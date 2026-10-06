<script lang="ts">
	import type { Service, Workload } from '$lib/types';
	import { SearchInput } from '$lib/components/ui';
	import { Button } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { ArrowClockwise, ArrowDown, Trash } from 'phosphor-svelte';
	import { onMount } from 'svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let isPod = $derived(service.type === 'pod' || (service.workloads && service.workloads.length > 1));
	let workloads = $derived<Workload[]>(
		service.workloads ?? [{ name: service.name, image: service.image ?? '—', status: service.status }]
	);

	let selectedContainer = $state('all');
	let searchQuery = $state('');
	let follow = $state(true);
	let isLoading = $state(false);
	let rawLogsText = $state<string>('');
	let logsContainer = $state<HTMLDivElement | null>(null);

	// Find active container(s) for this service
	let matchingContainers = $derived.by(() => {
		return dataStore.containers.filter(
			(c) =>
				c.serviceId === service.id ||
				c.name === service.name ||
				c.name.startsWith(`${service.projectId}-${service.name}`) ||
				c.name.includes(service.name)
		);
	});

	interface LogEntry {
		time: string;
		container: string;
		level: string;
		text: string;
	}

	let parsedLogs = $derived.by(() => {
		if (!rawLogsText.trim()) return [] as LogEntry[];
		const lines = rawLogsText.split('\n').filter((l) => l.trim().length > 0);
		return lines.map((line, idx) => {
			let level = 'INFO';
			if (/err|error|fail|fatal/i.test(line)) level = 'ERROR';
			else if (/warn|warning/i.test(line)) level = 'WARN';

			// Check for ISO timestamp at start
			const isoMatch = line.match(/^(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(\.\d+)?Z?)\s*(.*)/);
			let time = '';
			let text = line;
			if (isoMatch) {
				time = isoMatch[1].substring(11, 23);
				text = isoMatch[3] || line;
			} else {
				time = new Date().toISOString().substring(11, 23);
			}

			const targetContainer = matchingContainers[0]?.name || service.name;

			return {
				time,
				container: targetContainer,
				level,
				text
			};
		});
	});

	let filteredLogs = $derived.by(() => {
		if (!searchQuery.trim()) return parsedLogs;
		const q = searchQuery.toLowerCase().trim();
		return parsedLogs.filter((l) => l.text.toLowerCase().includes(q) || l.container.toLowerCase().includes(q));
	});

	async function fetchLogs() {
		isLoading = true;
		try {
			// Determine which container to query
			const target = matchingContainers[0]?.id || matchingContainers[0]?.name || service.name;
			const res = await api.runtime.containers.logs(target, 200);
			if (res && res.logs != null) {
				rawLogsText = res.logs;
			}
		} catch (err) {
			// Container might not be currently running
			if (!rawLogsText) {
				rawLogsText = `[system] Service '${service.name}' status: ${service.status}\n[system] No active container running or container has not emitted stdout/stderr yet.`;
			}
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		fetchLogs();
		const interval = setInterval(() => {
			if (follow) {
				fetchLogs();
			}
		}, 3000);
		return () => clearInterval(interval);
	});

	$effect(() => {
		if (follow && logsContainer && parsedLogs.length) {
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
			<Button variant="ghost" size="sm" onclick={fetchLogs} disabled={isLoading}>
				<ArrowClockwise class="w-3.5 h-3.5 mr-1.5 {isLoading ? 'animate-spin' : ''}" />
				Refresh
			</Button>

			<label class="flex items-center gap-1.5 text-xs text-[var(--text-secondary)] cursor-pointer select-none">
				<input type="checkbox" bind:checked={follow} class="accent-[var(--accent)]" />
				<span>Live follow</span>
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
				{#if isLoading}
					Fetching container logs from Podman engine…
				{:else if searchQuery}
					No log lines matching "{searchQuery}".
				{:else}
					No logs emitted by container yet.
				{/if}
			</div>
		{/if}
	</div>
</div>
