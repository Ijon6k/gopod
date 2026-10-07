<script lang="ts">
	import type { Service, Container } from '$lib/types';
	import { SearchInput } from '$lib/components/ui';
	import { Button, Chip } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { ArrowClockwise, Copy, Trash, Funnel, Terminal } from 'phosphor-svelte';
	import { onMount } from 'svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let selectedContainer = $state<string>('all');
	let searchQuery = $state('');
	let selectedLevel = $state<'ALL' | 'INFO' | 'WARN' | 'ERROR'>('ALL');
	let follow = $state(true);
	let isLoading = $state(false);
	let rawLogsText = $state<string>('');
	let logsContainer = $state<HTMLDivElement | null>(null);
	let copyFeedback = $state(false);

	// Find active container(s) for this service from reactive dataStore
	let matchingContainers = $derived.by<Container[]>(() => {
		return dataStore.containers.filter(
			(c) =>
				c.serviceId === service.id ||
				c.name === service.name ||
				c.name.startsWith(`${service.projectId}-${service.name}`) ||
				(c.serviceName && c.serviceName === service.name) ||
				(c.labels &&
					(c.labels['com.docker.compose.project'] === service.name ||
						c.labels['io.podman.compose.project'] === service.name ||
						c.labels['io.gopod.service'] === service.id))
		);
	});

	interface LogEntry {
		time: string;
		container: string;
		level: 'INFO' | 'WARN' | 'ERROR';
		text: string;
	}

	let parsedLogs = $state<LogEntry[]>([]);

	function parseLogLines(raw: string, defaultContainer: string): LogEntry[] {
		if (!raw.trim()) return [];
		const lines = raw.split('\n').filter((l) => l.trim().length > 0);
		return lines.map((line) => {
			let level: 'INFO' | 'WARN' | 'ERROR' = 'INFO';
			if (/err|error|fail|fatal|panic/i.test(line)) level = 'ERROR';
			else if (/warn|warning/i.test(line)) level = 'WARN';

			let targetContainer = defaultContainer;
			let cleanLine = line;

			// Extract [container_name] prefix if present in merged log stream
			const prefixMatch = cleanLine.match(/^\[([^\]]+)\]\s*(.*)/);
			if (prefixMatch) {
				targetContainer = prefixMatch[1];
				cleanLine = prefixMatch[2];
			}

			// Extract ISO timestamp if present
			const isoMatch = cleanLine.match(/^(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(\.\d+)?Z?)\s*(.*)/);
			let time = '';
			let text = cleanLine;
			if (isoMatch) {
				time = isoMatch[1].substring(11, 23);
				text = isoMatch[3] || cleanLine;
			} else {
				time = new Date().toISOString().substring(11, 23);
			}

			return {
				time,
				container: targetContainer,
				level,
				text
			};
		});
	}

	let filteredLogs = $derived.by(() => {
		let logs = parsedLogs;
		if (selectedLevel !== 'ALL') {
			logs = logs.filter((l) => l.level === selectedLevel);
		}
		if (searchQuery.trim()) {
			const q = searchQuery.toLowerCase().trim();
			logs = logs.filter(
				(l) => l.text.toLowerCase().includes(q) || l.container.toLowerCase().includes(q)
			);
		}
		return logs;
	});

	async function fetchLogs() {
		isLoading = true;
		try {
			if (selectedContainer !== 'all') {
				// Query specific target container (Dokploy parity)
				const target = selectedContainer;
				const res = await api.runtime.containers.logs(target, 250);
				const containerObj = matchingContainers.find((c) => c.id === target || c.name === target);
				const contName = containerObj?.name || target;
				rawLogsText = res?.logs || '';
				parsedLogs = parseLogLines(rawLogsText, contName);
			} else if (matchingContainers.length > 1) {
				// Aggregate logs across all containers in the stack/pod
				const responses = await Promise.all(
					matchingContainers.map(async (c) => {
						try {
							const res = await api.runtime.containers.logs(c.id || c.name, 100);
							return { name: c.name, logs: res?.logs || '' };
						} catch {
							return { name: c.name, logs: '' };
						}
					})
				);

				const combined: LogEntry[] = [];
				for (const item of responses) {
					if (item.logs) {
						combined.push(...parseLogLines(item.logs, item.name));
					}
				}
				combined.sort((a, b) => a.time.localeCompare(b.time));
				parsedLogs = combined;
				rawLogsText = combined.map((e) => `[${e.container}] ${e.time} ${e.level} ${e.text}`).join('\n');
			} else {
				// Single container service
				const target = matchingContainers[0]?.id || matchingContainers[0]?.name || service.name;
				const res = await api.runtime.containers.logs(target, 250);
				rawLogsText = res?.logs || '';
				parsedLogs = parseLogLines(rawLogsText, matchingContainers[0]?.name || service.name);
			}
		} catch (err) {
			if (!rawLogsText) {
				const fallback = `[system] Service '${service.name}' status: ${service.status}\n[system] No active output received yet from container runtime.`;
				rawLogsText = fallback;
				parsedLogs = parseLogLines(fallback, service.name);
			}
		} finally {
			isLoading = false;
		}
	}

	function handleCopy() {
		if (!rawLogsText) return;
		navigator.clipboard.writeText(rawLogsText);
		copyFeedback = true;
		setTimeout(() => (copyFeedback = false), 2000);
	}

	function handleClear() {
		rawLogsText = '';
		parsedLogs = [];
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
		if (follow && logsContainer && filteredLogs.length) {
			logsContainer.scrollTop = logsContainer.scrollHeight;
		}
	});
</script>

<div class="w-full flex flex-col gap-3">
	<!-- Unified Compact Toolbar (Dokploy Parity) -->
	<div class="flex flex-col md:flex-row md:items-center justify-between gap-3 pb-3 border-b border-[var(--border)]">
		<!-- Left: Container Selector & Filter Controls -->
		<div class="flex flex-wrap items-center gap-2.5">
			<!-- Container Selector Dropdown (Always visible for multi-container compose/pods, and single containers) -->
			<div class="flex items-center gap-2">
				<span class="text-xs text-[var(--text-tertiary)] font-medium">Container:</span>
				<div class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] flex items-center gap-1.5 shadow-xs">
					<Terminal size={13} class="text-[var(--text-tertiary)] shrink-0" />
					<select
						bind:value={selectedContainer}
						onchange={() => fetchLogs()}
						class="bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-mono)] cursor-pointer"
					>
						<option value="all">
							All containers ({matchingContainers.length || 1})
						</option>
						{#each matchingContainers as c}
							<option value={c.id || c.name}>
								{c.name} {c.serviceName && c.serviceName !== c.name ? `(${c.serviceName})` : ''}
							</option>
						{/each}
					</select>
				</div>
			</div>

			<!-- Log Level Filter Pills -->
			<div class="inline-flex items-center p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
				{#each ['ALL', 'INFO', 'WARN', 'ERROR'] as lvl}
					<button
						type="button"
						onclick={() => (selectedLevel = lvl as any)}
						class="px-2 py-1 rounded text-[11px] font-mono transition-colors border-0 cursor-pointer {selectedLevel === lvl
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-semibold shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						{lvl}
					</button>
				{/each}
			</div>

			<!-- Search Query Input -->
			<div class="w-48 sm:w-60">
				<SearchInput bind:value={searchQuery} placeholder="Filter output…" />
			</div>
		</div>

		<!-- Right: Action Buttons -->
		<div class="flex items-center gap-2 shrink-0 self-end md:self-auto">
			<label class="flex items-center gap-1.5 text-xs text-[var(--text-secondary)] cursor-pointer select-none px-2 py-1 rounded hover:bg-[var(--bg-surface)] transition-colors">
				<input type="checkbox" bind:checked={follow} class="accent-[var(--accent)] cursor-pointer" />
				<span class="font-mono text-[11px]">Follow</span>
			</label>

			<Button variant="ghost" size="sm" onclick={fetchLogs} disabled={isLoading} title="Reload container logs">
				<ArrowClockwise class="w-3.5 h-3.5 mr-1.5 {isLoading ? 'animate-spin' : ''}" />
				<span>Refresh</span>
			</Button>

			<Button variant="ghost" size="sm" onclick={handleCopy} title="Copy logs to clipboard">
				<Copy class="w-3.5 h-3.5 mr-1" />
				<span>{copyFeedback ? 'Copied!' : 'Copy'}</span>
			</Button>

			<Button variant="ghost" size="sm" onclick={handleClear} title="Clear terminal screen">
				<Trash class="w-3.5 h-3.5 text-[var(--text-tertiary)]" />
			</Button>
		</div>
	</div>

	<!-- Monospace Terminal Log Screen -->
	<div
		bind:this={logsContainer}
		class="relative p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-table-header)] text-[var(--text-primary)] font-[var(--font-mono)] text-[12px] leading-[1.65] h-[520px] overflow-auto select-text transition-colors shadow-inner"
	>
		{#each filteredLogs as log, idx (idx)}
			<div class="flex items-baseline gap-2.5 hover:bg-[var(--bg-table-row-hover)] px-1.5 py-0.5 rounded transition-colors group">
				<span class="text-[var(--text-tertiary)] select-none whitespace-nowrap text-[11px] font-mono shrink-0">
					{log.time}
				</span>

				{#if matchingContainers.length > 1 || selectedContainer === 'all'}
					<span class="text-[var(--accent)] font-medium text-[11px] select-none shrink-0 truncate max-w-[120px]" title={log.container}>
						[{log.container}]
					</span>
				{/if}

				<span
					class="select-none font-semibold text-[10px] uppercase shrink-0 min-w-[36px] {log.level === 'WARN'
						? 'text-[var(--status-amber)]'
						: log.level === 'ERROR'
							? 'text-[var(--status-red)]'
							: 'text-[var(--text-tertiary)]'}"
				>
					{log.level}
				</span>

				<span class="text-[var(--text-secondary)] whitespace-pre-wrap break-all flex-1 font-mono selection:bg-[var(--accent)] selection:text-black">
					{log.text}
				</span>
			</div>
		{/each}

		{#if filteredLogs.length === 0}
			<div class="h-full flex flex-col items-center justify-center gap-2 text-center text-xs text-[var(--text-tertiary)]">
				{#if isLoading}
					<div class="w-4 h-4 rounded-full border-2 border-[var(--accent)] border-t-transparent animate-spin"></div>
					<span class="font-mono text-[11px]">Streaming logs from container runtime…</span>
				{:else if searchQuery}
					<span class="font-mono text-[11px]">No lines matching "{searchQuery}"</span>
				{:else}
					<span class="font-mono text-[11px]">Container has not emitted any stdout/stderr logs yet.</span>
				{/if}
			</div>
		{/if}
	</div>
</div>
