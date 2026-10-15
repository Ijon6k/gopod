<script lang="ts">
	import type { Service, Container } from '$lib/types';
	import { SearchInput, Select, Checkbox, Button } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import {
		ArrowClockwise,
		Copy,
		Trash,
		Funnel,
		Terminal,
		ArrowsOutSimple,
		ArrowsInSimple
	} from 'phosphor-svelte';
	import { onMount } from 'svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let selectedContainer = $state<string>('all');
	let logSource = $state<'container' | 'journal'>('container');
	let searchQuery = $state('');
	let selectedLevel = $state<'ALL' | 'INFO' | 'WARN' | 'ERROR'>('ALL');
	let follow = $state(true);
	let isLoading = $state(false);
	let rawLogsText = $state<string>('');
	let logsContainer = $state<HTMLDivElement | null>(null);
	let copyFeedback = $state(false);
	let isFullscreen = $state(false);

	// Find active container(s) for this service from reactive dataStore
	let matchingContainers = $derived.by<Container[]>(() => {
		if (service.type === 'pod') {
			const podKey = service.id;
			const podNameKey = service.name;
			const podPrefixedKey = `pod_${service.name}`;

			const memberSvcs = dataStore.services.filter(
				(s) =>
					s.projectId === service.projectId &&
					s.id !== service.id &&
					(s.podId === podKey ||
						s.podId === podNameKey ||
						s.podId === podPrefixedKey ||
						(s.inPod && s.type !== 'pod'))
			);
			const memberServiceIds = new Set(memberSvcs.map((m) => m.id));

			return dataStore.containers.filter(
				(c) =>
					!c.name.endsWith('-infra') &&
					(c.pod === podPrefixedKey ||
						c.pod === podNameKey ||
						c.pod === podKey ||
						memberServiceIds.has(c.serviceId) ||
						(c.labels && c.labels['io.gopod.service'] && memberServiceIds.has(c.labels['io.gopod.service'])) ||
						(c.name.includes(service.name) && !c.name.endsWith('-infra')))
			);
		}

		return dataStore.containers.filter(
			(c) =>
				!c.name.endsWith('-infra') &&
				(c.serviceId === service.id ||
					c.name === service.name ||
					c.name.startsWith(`${service.projectId}-${service.name}`) ||
					(c.serviceName && c.serviceName === service.name) ||
					(c.labels &&
						(c.labels['com.docker.compose.project'] === service.name ||
							c.labels['io.podman.compose.project'] === service.name ||
							c.labels['io.gopod.service'] === service.id)))
		);
	});

	let containerOptions = $derived.by(() => {
		const opts = [{ label: `All containers (${matchingContainers.length || 1})`, value: 'all' }];
		for (const c of matchingContainers) {
			opts.push({
				label: `${c.name}${c.serviceName && c.serviceName !== c.name ? ` (${c.serviceName})` : ''}`,
				value: c.id || c.name
			});
		}
		return opts;
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
			const isoMatch = cleanLine.match(
				/^(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(\.\d+)?Z?)\s*(.*)/
			);
			// Extract syslog timestamp (e.g. Oct 12 18:32:00 host unit[pid]: msg)
			const syslogMatch = cleanLine.match(
				/^([A-Z][a-z]{2}\s+\d+\s+\d{2}:\d{2}:\d{2})\s+([^\s]+)\s+([^:]+):\s*(.*)/
			);

			let time: string;
			let text = cleanLine;

			if (isoMatch) {
				time = isoMatch[1].substring(11, 23);
				text = isoMatch[3] || cleanLine;
			} else if (syslogMatch) {
				time = syslogMatch[1];
				targetContainer = syslogMatch[3];
				text = syslogMatch[4] || cleanLine;
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
			if (logSource === 'journal') {
				const res = await api.services.getJournal(service.id, 250);
				rawLogsText = res?.journal || '';
				if (!rawLogsText.trim()) {
					rawLogsText = `[systemd] No journal entries recorded yet for ${service.name}.service`;
				}
				parsedLogs = parseLogLines(rawLogsText, `${service.name}.service`);
			} else if (selectedContainer !== 'all') {
				// Query specific target container
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
				rawLogsText = combined
					.map((e) => `[${e.container}] ${e.time} ${e.level} ${e.text}`)
					.join('\n');
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

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape' && isFullscreen) isFullscreen = false;
	}}
/>

<div class="flex w-full flex-col gap-3">
	<!-- Unified Compact Toolbar -->
	<div
		class="flex flex-col justify-between gap-3 border-b border-[var(--border)] pb-3 md:flex-row md:items-center"
	>
		<!-- Left: Source Selector, Container Selector & Filter Controls -->
		<div class="flex flex-wrap items-center gap-2.5">
			<!-- Dual Log Source Switcher -->
			<div
				class="inline-flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
			>
				<button
					type="button"
					onclick={() => {
						logSource = 'container';
						fetchLogs();
					}}
					class="cursor-pointer rounded border-0 px-2.5 py-1 font-mono text-[11px] transition-colors {logSource ===
					'container'
						? 'bg-[var(--bg-surface)] font-semibold text-[var(--text-primary)] shadow-xs'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Container Output
				</button>
				<button
					type="button"
					onclick={() => {
						logSource = 'journal';
						fetchLogs();
					}}
					class="cursor-pointer rounded border-0 px-2.5 py-1 font-mono text-[11px] transition-colors {logSource ===
					'journal'
						? 'bg-[var(--bg-surface)] font-semibold text-[var(--text-primary)] shadow-xs'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Systemd Journal
				</button>
			</div>

			<!-- Container Selector Dropdown (visible for Container Output) -->
			{#if logSource === 'container'}
				<div class="flex items-center gap-2">
					<span class="text-xs font-medium text-[var(--text-tertiary)]">Container:</span>
					<div class="min-w-[180px]">
						<Select
							value={selectedContainer}
							onValueChange={(val: any) => {
								if (typeof val === 'string') {
									selectedContainer = val;
									fetchLogs();
								}
							}}
							options={containerOptions}
							size="sm"
						/>
					</div>
				</div>
			{/if}

			<!-- Log Level Filter Pills -->
			<div
				class="inline-flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
			>
				{#each ['ALL', 'INFO', 'WARN', 'ERROR'] as lvl}
					<button
						type="button"
						onclick={() => (selectedLevel = lvl as any)}
						class="cursor-pointer rounded border-0 px-2 py-1 font-mono text-[11px] transition-colors {selectedLevel ===
						lvl
							? 'bg-[var(--bg-surface)] font-semibold text-[var(--text-primary)] shadow-xs'
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
		<div class="flex shrink-0 items-center gap-2 self-end md:self-auto">
			<div class="flex items-center gap-1.5 px-2 py-1">
				<Checkbox bind:checked={follow} id="log-follow" />
				<label
					for="log-follow"
					class="cursor-pointer font-mono text-[11px] text-[var(--text-secondary)] select-none"
				>
					Follow
				</label>
			</div>

			<Button
				variant="ghost"
				size="sm"
				onclick={fetchLogs}
				disabled={isLoading}
				aria-label="Reload container logs"
			>
				<ArrowClockwise class="mr-1.5 h-3.5 w-3.5 {isLoading ? 'animate-spin' : ''}" />
				<span>Refresh</span>
			</Button>

			<Button variant="ghost" size="sm" onclick={handleCopy} aria-label="Copy logs to clipboard">
				<Copy class="mr-1 h-3.5 w-3.5" />
				<span>{copyFeedback ? 'Copied!' : 'Copy'}</span>
			</Button>

			<Button variant="ghost" size="sm" onclick={handleClear} aria-label="Clear terminal screen">
				<Trash class="h-3.5 w-3.5 text-[var(--text-tertiary)]" />
			</Button>

			<Button
				variant="ghost"
				size="sm"
				onclick={() => (isFullscreen = !isFullscreen)}
				aria-label={isFullscreen ? 'Exit fullscreen logs' : 'Maximize logs'}
				title={isFullscreen ? 'Exit fullscreen' : 'Maximize logs'}
			>
				{#if isFullscreen}
					<ArrowsInSimple class="h-3.5 w-3.5 text-[var(--text-secondary)]" />
				{:else}
					<ArrowsOutSimple class="h-3.5 w-3.5 text-[var(--text-secondary)]" />
				{/if}
			</Button>
		</div>
	</div>

	<!-- Monospace Terminal Log Screen (Auto-stretching to bottom of viewport) -->
	<div
		bind:this={logsContainer}
		style={isFullscreen
			? 'position: fixed; inset: 16px; z-index: 50; height: calc(100dvh - 32px);'
			: 'height: calc(100dvh - 275px); min-height: 480px;'}
		class="relative w-full overflow-auto rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-table-header)] p-4 text-[12px] leading-[1.65] font-[var(--font-mono)] text-[var(--text-primary)] shadow-inner transition-all select-text {isFullscreen
			? 'shadow-2xl ring-1 ring-[var(--border)]'
			: ''}"
	>
		{#if isFullscreen}
			<div class="sticky top-0 right-0 z-10 flex justify-end pb-2">
				<Button
					variant="secondary"
					size="sm"
					onclick={() => (isFullscreen = false)}
					class="gap-1.5 bg-[var(--bg-panel)] shadow-md"
				>
					<ArrowsInSimple class="h-3.5 w-3.5" />
					<span>Exit fullscreen</span>
				</Button>
			</div>
		{/if}
		{#each filteredLogs as log, idx (idx)}
			<div
				class="group flex items-baseline gap-2.5 rounded px-1.5 py-0.5 transition-colors hover:bg-[var(--bg-table-row-hover)]"
			>
				<span
					class="shrink-0 font-mono text-[11px] whitespace-nowrap text-[var(--text-tertiary)] select-none"
				>
					{log.time}
				</span>

				{#if matchingContainers.length > 1 || selectedContainer === 'all'}
					<span
						class="max-w-[120px] shrink-0 truncate text-[11px] font-medium text-[var(--accent)] select-none"
						title={log.container}
					>
						[{log.container}]
					</span>
				{/if}

				<span
					class="min-w-[36px] shrink-0 text-[10px] font-semibold uppercase select-none {log.level ===
					'WARN'
						? 'text-[var(--status-amber)]'
						: log.level === 'ERROR'
							? 'text-[var(--status-red)]'
							: 'text-[var(--text-tertiary)]'}"
				>
					{log.level}
				</span>

				<span
					class="flex-1 font-mono break-all whitespace-pre-wrap text-[var(--text-secondary)] selection:bg-[var(--accent)] selection:text-black"
				>
					{log.text}
				</span>
			</div>
		{/each}

		{#if filteredLogs.length === 0}
			<div
				class="flex h-full flex-col items-center justify-center gap-2 text-center text-xs text-[var(--text-tertiary)]"
			>
				{#if isLoading}
					<div
						class="h-4 w-4 animate-spin rounded-full border-2 border-[var(--accent)] border-t-transparent"
					></div>
					<span class="font-mono text-[11px]">Streaming logs from container runtime…</span>
				{:else if searchQuery}
					<span class="font-mono text-[11px]">No lines matching "{searchQuery}"</span>
				{:else}
					<span class="font-mono text-[11px]"
						>Container has not emitted any stdout/stderr logs yet.</span
					>
				{/if}
			</div>
		{/if}
	</div>
</div>
