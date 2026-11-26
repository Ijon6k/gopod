<script lang="ts">
	import type { Service, Container } from '$lib/types';
	import { SearchInput, Select, Button } from '$lib/components/ui';
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
	import { createPoller } from '$lib/utils/poller';
	import {
		parseFormattedLogLines,
		stripAnsi,
		type FormattedLogEntry,
		type LogLevel
	} from '$lib/utils/log-format';
	import FormattedLogView from '$lib/components/features/runtime/FormattedLogView.svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let selectedContainer = $state<string>('all');
	let logSource = $state<'container' | 'journal'>('container');
	let searchQuery = $state('');
	let selectedLevel = $state<'ALL' | LogLevel>('ALL');
	let follow = $state(true);
	let showTimestamps = $state(false);
	let wrapLines = $state(true);
	let prettyJson = $state(false);
	let isLoading = $state(false);
	let rawLogsText = $state<string>('');
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
						(c.labels &&
							c.labels['io.gopod.service'] &&
							memberServiceIds.has(c.labels['io.gopod.service'])) ||
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
			const cleanService = c.serviceName?.trim();
			const showSuffix =
				cleanService &&
				cleanService !== c.name &&
				!c.name.toLowerCase().includes(cleanService.toLowerCase());
			opts.push({
				label: showSuffix ? `${c.name} (${cleanService})` : c.name,
				value: c.id || c.name
			});
		}
		return opts;
	});

	let parsedLogs = $state<FormattedLogEntry[]>([]);

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
				parsedLogs = parseFormattedLogLines(rawLogsText, `${service.name}.service`);
			} else if (selectedContainer !== 'all') {
				// Query specific target container
				const target = selectedContainer;
				const res = await api.runtime.containers.logs(target, 250, showTimestamps);
				const containerObj = matchingContainers.find((c) => c.id === target || c.name === target);
				const contName = containerObj?.name || target;
				rawLogsText = res?.logs || '';
				parsedLogs = parseFormattedLogLines(rawLogsText, contName);
			} else if (matchingContainers.length > 1) {
				// Aggregate logs across all containers in the stack/pod
				const responses = await Promise.all(
					matchingContainers.map(async (c) => {
						try {
							const res = await api.runtime.containers.logs(c.id || c.name, 100, showTimestamps);
							return { name: c.name, logs: res?.logs || '' };
						} catch {
							return { name: c.name, logs: '' };
						}
					})
				);

				const combined: FormattedLogEntry[] = [];
				let seqId = 0;
				for (const item of responses) {
					if (item.logs) {
						for (const entry of parseFormattedLogLines(item.logs, item.name)) {
							combined.push({ ...entry, id: seqId++ });
						}
					}
				}
				if (combined.every((e) => e.sortTs !== null)) {
					combined.sort((a, b) => (a.sortTs ?? 0) - (b.sortTs ?? 0));
				}
				parsedLogs = combined;
				rawLogsText = combined
					.map((e) => `[${e.container}] ${e.time} ${e.level} ${e.text}`)
					.join('\n');
			} else {
				// Single container service
				const target = matchingContainers[0]?.id || matchingContainers[0]?.name || service.name;
				const res = await api.runtime.containers.logs(target, 250, showTimestamps);
				rawLogsText = res?.logs || '';
				parsedLogs = parseFormattedLogLines(
					rawLogsText,
					matchingContainers[0]?.name || service.name
				);
			}
		} catch (err) {
			if (!rawLogsText) {
				const fallback = `[system] Service '${service.name}' status: ${service.status}\n[system] No active output received yet from container runtime.`;
				rawLogsText = fallback;
				parsedLogs = parseFormattedLogLines(fallback, service.name);
			}
		} finally {
			isLoading = false;
		}
	}

	async function handleCopy() {
		const cleanText = stripAnsi(rawLogsText);
		if (!cleanText) return;
		// "Copied!" must only show on a true success. navigator.clipboard is
		// unavailable on insecure origins, so fall back to the legacy path.
		try {
			await navigator.clipboard.writeText(cleanText);
			copyFeedback = true;
		} catch {
			const helper = document.createElement('textarea');
			helper.value = cleanText;
			helper.style.position = 'fixed';
			helper.style.opacity = '0';
			document.body.appendChild(helper);
			helper.select();
			try {
				if (document.execCommand('copy')) copyFeedback = true;
				else console.error('Failed to copy logs to clipboard');
			} catch {
				console.error('Failed to copy logs to clipboard');
			}
			document.body.removeChild(helper);
		}
		if (copyFeedback) {
			setTimeout(() => (copyFeedback = false), 2000);
		}
	}

	function handleClear() {
		rawLogsText = '';
		parsedLogs = [];
	}

	// Return the fetch promise so the poller's in-flight latch engages: a
	// dropped result would let a second tick overlap a slow request, and the
	// later (older) response could clobber the fresher one.
	const logPoller = createPoller(
		3000,
		() => {
			if (follow) return fetchLogs();
		},
		{ pauseWhenHidden: true }
	);

	onMount(() => {
		fetchLogs();
		logPoller.start();
		return () => logPoller.stop();
	});
</script>

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape' && isFullscreen) isFullscreen = false;
	}}
/>

<div class="flex w-full flex-col gap-3">
	<!-- Clean 2-Row Toolbar (prevents layout break on long container names) -->
	<div class="flex flex-col gap-2.5 border-b border-[var(--border)] pb-3">
		<!-- Row 1: Source & Container on left, Live/Actions on right -->
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex min-w-0 flex-wrap items-center gap-2">
				<!-- Dual Log Source Switcher -->
				<div
					class="inline-flex shrink-0 items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
				>
					<button
						type="button"
						onclick={() => {
							logSource = 'container';
							fetchLogs();
						}}
						class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {logSource ===
						'container'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)] shadow-xs'
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
						class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {logSource ===
						'journal'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)] shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						Systemd Journal
					</button>
				</div>

				<!-- Container Selector Dropdown (fixed max-width + truncated trigger) -->
				{#if logSource === 'container'}
					<div class="flex min-w-0 items-center gap-1.5">
						<span class="hidden text-xs text-[var(--text-tertiary)] sm:inline">Container</span>
						<div class="w-[180px] max-w-[220px] min-w-0 sm:w-[220px] [&_button]:w-full [&_button]:truncate">
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
			</div>

			<!-- Right: Stream & Actions -->
			<div class="flex shrink-0 flex-wrap items-center gap-1.5">
				<button
					type="button"
					onclick={() => (follow = !follow)}
					aria-pressed={follow}
					class="inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border px-2.5 text-xs font-medium transition-colors {follow
						? 'border-[var(--status-green)]/35 bg-[var(--status-green-muted)] text-[var(--status-green)]'
						: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					<span
						class="h-1.5 w-1.5 rounded-full {follow
							? 'animate-pulse bg-[var(--status-green)]'
							: 'bg-[var(--text-muted)]'}"
					></span>
					Follow
				</button>

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

		<!-- Row 2: Search & Level Filter on left, Display Toggles on right -->
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
				<div class="w-full max-w-[260px] min-w-[160px] sm:w-60">
					<SearchInput bind:value={searchQuery} placeholder="Filter output…" />
				</div>

				<!-- Log Level Filter Pills -->
				<div
					class="inline-flex shrink-0 items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
				>
					{#each [
						{ value: 'ALL', label: 'All' },
						{ value: 'INFO', label: 'Info' },
						{ value: 'WARN', label: 'Warn' },
						{ value: 'ERROR', label: 'Error' }
					] as lvl}
						<button
							type="button"
							onclick={() => (selectedLevel = lvl.value as any)}
							class="cursor-pointer rounded border-0 px-2.5 py-0.5 text-xs transition-colors {selectedLevel ===
							lvl.value
								? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)] shadow-xs'
								: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						>
							{lvl.label}
						</button>
					{/each}
				</div>
			</div>

			<!-- Display Options -->
			<div class="flex shrink-0 items-center gap-1.5">
				{#if logSource === 'container'}
					<button
						type="button"
						onclick={() => {
							showTimestamps = !showTimestamps;
							fetchLogs();
						}}
						aria-pressed={showTimestamps}
						class="cursor-pointer rounded-[var(--radius-sm)] border px-2 py-1 text-xs font-medium transition-colors {showTimestamps
							? 'border-[var(--accent)]/40 bg-[var(--accent-muted)] text-[var(--accent-text)]'
							: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						Timestamps
					</button>
				{/if}

				<button
					type="button"
					onclick={() => (wrapLines = !wrapLines)}
					aria-pressed={wrapLines}
					class="cursor-pointer rounded-[var(--radius-sm)] border px-2 py-1 text-xs font-medium transition-colors {wrapLines
						? 'border-[var(--accent)]/40 bg-[var(--accent-muted)] text-[var(--accent-text)]'
						: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Wrap
				</button>

				<button
					type="button"
					onclick={() => (prettyJson = !prettyJson)}
					aria-pressed={prettyJson}
					class="cursor-pointer rounded-[var(--radius-sm)] border px-2 py-1 text-xs font-medium transition-colors {prettyJson
						? 'border-[var(--accent)]/40 bg-[var(--accent-muted)] text-[var(--accent-text)]'
						: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Pretty JSON
				</button>
			</div>
		</div>
	</div>

	<!-- Monospace Terminal Log Screen (Auto-stretching to bottom of viewport) -->
	<FormattedLogView
		entries={filteredLogs}
		loading={isLoading}
		showContainer={matchingContainers.length > 1 || selectedContainer === 'all'}
		{wrapLines}
		{prettyJson}
		{follow}
		emptyMessage={searchQuery
			? `No lines matching "${searchQuery}"`
			: 'Container has not emitted any stdout/stderr logs yet.'}
		containerStyle={isFullscreen
			? 'position: fixed; inset: 16px; z-index: 50; height: calc(100dvh - 32px);'
			: 'height: calc(100dvh - 275px); min-height: 480px;'}
		containerClass={isFullscreen ? 'shadow-2xl ring-1 ring-[var(--border)]' : 'shadow-inner'}
	>
		{#snippet headerSlot()}
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
		{/snippet}
	</FormattedLogView>
</div>
