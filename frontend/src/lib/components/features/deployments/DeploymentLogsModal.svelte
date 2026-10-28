<script lang="ts">
	import type { Deployment } from '$lib/types';
	import { untrack } from 'svelte';
	import { Button } from '$lib/components/ui';
	import { SearchInput } from '$lib/components/ui';
	import { DeploymentPipelineStepper } from '$lib/components/features/deployments';
	import { LayerDialog, Badge, Switch } from 'kumo-svelte';
	import { api } from '$lib/api';
	import { createPoller } from '$lib/utils/poller';
	import { getDeploymentSteps, getDeploymentLogs } from '$lib/utils/deploymentLogs';
	import { dataStore } from '$lib/stores/data.svelte';
	import { Copy, Check, DownloadSimple, CaretDown } from 'phosphor-svelte';

	interface Props {
		deployment: Deployment | null;
		open: boolean;
		onclose: () => void;
	}

	let { deployment, open = $bindable(false), onclose }: Props = $props();

	let searchQuery = $state('');
	let autoScroll = $state(true);
	let copiedLogs = $state(false);
	let logContainer = $state<HTMLDivElement | null>(null);

	let fetchedLogs = $state<string[]>([]);
	let isLogsLoading = $state(false);

	// In-memory ring bound for a live session: very long builds keep only the
	// newest MAX_LOG_LINES lines so streaming never grows without limit.
	const MAX_LOG_LINES = 5000;
	// Bounded tail of recently streamed lines; the only true duplicate source
	// in the SSE feed is an immediate re-delivery of one event.
	const STREAM_DEDUPE_TAIL = 50;

	let activeDep = $derived.by(() => {
		if (!deployment) return null;
		const fromStore = dataStore
			.getServiceDeployments(deployment.serviceId)
			.find((d) => d.id === deployment.id);
		return fromStore || deployment;
	});

	function sanitizeLogLine(raw: string): string {
		const trimmed = raw.trim();
		if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
			try {
				const parsed = JSON.parse(trimmed);
				if (parsed && typeof parsed.line === 'string') {
					return parsed.line;
				}
			} catch {
				// plain text fallback
			}
		}
		return raw;
	}

	let steps = $derived(activeDep ? getDeploymentSteps(activeDep) : []);
	let completedSteps = $derived(steps.filter((s) => s.status === 'success').length);
	let showPipelineMobile = $state(false);
	let logs = $derived.by(() => {
		const source =
			fetchedLogs.length > 0
				? fetchedLogs
				: activeDep?.logs && activeDep.logs.length > 0
					? activeDep.logs
					: activeDep
						? getDeploymentLogs(activeDep)
						: [];
		return source.map(sanitizeLogLine);
	});

	let filteredLogs = $derived.by(() => {
		if (!searchQuery.trim()) return logs;
		const q = searchQuery.toLowerCase().trim();
		return logs.filter((l) => l.toLowerCase().includes(q));
	});

	// A content-keyed dedup against the stored log was wrong: the backend
	// streams plain text lines with no sequence ids and never replays lines on
	// reconnect (a fresh subscriber channel per connection, see
	// backend/internal/services/log_manager.go Subscribe/PipeReader), so
	// identical lines are legitimate distinct events (health retries,
	// repeated build steps). The stream guard is therefore limited to an
	// immediate re-delivery of one SSE event via the bounded tail below —
	// O(1) membership checks, then the oldest entry evicts from the queue.
	let recentStreamed = new Set<string>();
	let recentStreamedQueue: string[] = [];

	let depId = $derived(deployment?.id ?? '');
	let isLive = $derived(activeDep?.status === 'building' || activeDep?.status === 'deploying');

	let now = $state(Date.now());
	$effect(() => {
		if (!open || !isLive) return;
		const timer = setInterval(() => {
			now = Date.now();
		}, 1000);
		return () => clearInterval(timer);
	});

	let liveDuration = $derived.by(() => {
		if (!activeDep) return '—';
		if (isLive && activeDep.startedAt) {
			const start = new Date(activeDep.startedAt).getTime();
			if (!isNaN(start) && start > 0) {
				const diff = Math.max(0, Math.floor((now - start) / 1000));
				const m = Math.floor(diff / 60);
				const s = diff % 60;
				return m > 0 ? `${m}m ${s}s` : `${s}s`;
			}
			return 'Running…';
		}
		return activeDep.duration || '—';
	});

	// Fingerprint of the last store write. Identical poll responses are skipped
	// so the store keeps stable identities and nothing downstream re-subscribes.
	let lastSyncedFingerprint = '';

	$effect(() => {
		const id = depId;
		if (!open || !id) {
			untrack(() => {
				if (fetchedLogs.length > 0) fetchedLogs = [];
			});
			return;
		}

		let isSubscribed = true;
		isLogsLoading = true;
		// Reset per effect run so a reopen always syncs.
		lastSyncedFingerprint = '';
		recentStreamed = new Set<string>();
		recentStreamedQueue = [];

		const syncStatus = async () => {
			try {
				const latest = await api.services.getDeployment(id);
				if (!latest || !isSubscribed) return null;
				const fingerprint = JSON.stringify(latest);
				if (fingerprint !== lastSyncedFingerprint) {
					lastSyncedFingerprint = fingerprint;
					dataStore.upsertDeployment(latest);
				}
				return latest;
			} catch (_) {
				return null;
			}
		};

		// Read the stored log file from the server.
		const loadStoredLogs = () =>
			api.services.deploymentLogs(id).then((res) => {
				if (!isSubscribed || !res?.logs) return false;
				// Stored lines load as-is; they must NOT seed the stream guard,
				// or identical later SSE lines would be silently dropped.
				fetchedLogs = res.logs
					.split('\n')
					.map(sanitizeLogLine)
					.filter((line: string) => line.length > 0);
				return true;
			});

		loadStoredLogs()
			.then(() => syncStatus())
			.catch((err) => {
				console.warn('Could not load stored deployment logs:', err);
			})
			.finally(() => {
				if (isSubscribed) isLogsLoading = false;
			});

		if (isLive) {
			// Poll and stream only while the build is in flight. The effect keys on
			// depId + isLive, so per-tick progress updates never rebuild either.
			// createPoller latches ticks so a slow sync can never overlap itself.
			const statusPoller = createPoller(1200, async () => {
				const latest = await syncStatus();
				if (latest && isSubscribed && (latest.status === 'success' || latest.status === 'failed')) {
					statusPoller.stop();
				}
			});
			statusPoller.start();

			// If active building/deploying, subscribe to SSE stream via centralized API
			let unsubscribeStream: (() => void) | null = null;
			try {
				unsubscribeStream = api.services.streamDeploymentLogs(
					id,
					(raw) => {
						if (!isSubscribed) return;
						const clean = sanitizeLogLine(raw);
						// Only an immediate re-delivery of the same streamed line is
						// suppressed; identical repeats further apart still render.
						if (recentStreamed.has(clean)) return;
						recentStreamed.add(clean);
						recentStreamedQueue.push(clean);
						if (recentStreamedQueue.length > STREAM_DEDUPE_TAIL) {
							const evicted = recentStreamedQueue.shift() as string;
							recentStreamed.delete(evicted);
						}
						// Ring bound: keep the newest MAX_LOG_LINES lines in memory.
						fetchedLogs =
							fetchedLogs.length + 1 > MAX_LOG_LINES
								? [...fetchedLogs.slice(-(MAX_LOG_LINES - 1)), clean]
								: [...fetchedLogs, clean];
					},
					() => {
						unsubscribeStream?.();
						// A dropped stream must not end live log updates: recover from
						// the stored log file when nothing was streamed yet, and the
						// status poller keeps converging the rest.
						if (fetchedLogs.length === 0) {
							loadStoredLogs().catch(() => {});
						}
						syncStatus();
					}
				);
			} catch (e) {
				console.warn('SSE stream error:', e);
			}

			return () => {
				isSubscribed = false;
				statusPoller.stop();
				unsubscribeStream?.();
			};
		}

		return () => {
			isSubscribed = false;
		};
	});

	$effect(() => {
		if (open && autoScroll && logContainer) {
			requestAnimationFrame(() => {
				if (logContainer) {
					logContainer.scrollTop = logContainer.scrollHeight;
				}
			});
		}
	});

	async function copyAllLogs() {
		if (!logs.length) return;
		try {
			await navigator.clipboard.writeText(logs.join('\n'));
			copiedLogs = true;
			setTimeout(() => {
				copiedLogs = false;
			}, 1800);
		} catch (err) {
			console.error('Failed to copy logs', err);
		}
	}

	function downloadLogFile() {
		if (!activeDep || !logs.length) return;
		const blob = new Blob([logs.join('\n')], { type: 'text/plain;charset=utf-8' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `gopod-deployment-${activeDep.id}-${activeDep.number}.log`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);
	}
</script>

{#if open && activeDep}
	<LayerDialog.Root
		open={true}
		onOpenChange={(isOpen) => {
			if (!isOpen) onclose();
		}}
	>
		<LayerDialog.Content
			size="xl"
			class="flex h-dvh! max-h-dvh! min-h-0! w-full! max-w-none! inset-0! rounded-none! p-0! sm:inset-auto! sm:h-[85vh]! sm:max-h-[860px]! sm:min-h-[560px]! sm:max-w-4xl! sm:rounded-xl! sm:p-1.5! flex-col overflow-hidden [&>div]:h-full [&>div]:min-h-0 [&>div]:flex-1 [&>div]:rounded-none sm:[&>div]:rounded-lg [&>div>[aria-hidden]]:hidden"
		>
			{#if activeDep}
				<!-- Top Modal Header (Padding-right reserved for Kumo's close X button) -->
				<div
					class="flex shrink-0 items-center justify-between border-b border-kumo-hairline bg-kumo-base px-4 py-3 pr-14 sm:px-5 sm:py-3.5"
				>
					<div class="flex min-w-0 items-center gap-2 sm:gap-3">
						<div class="flex items-center gap-1.5 sm:gap-2">
							<span class="font-mono text-sm font-semibold text-kumo-default">
								#{activeDep.number}
							</span>
							<Badge
								variant={activeDep.status === 'success' ||
								activeDep.status === 'running' ||
								activeDep.status === 'healthy'
									? 'success'
									: activeDep.status === 'failed'
										? 'error'
										: activeDep.status === 'building' || activeDep.status === 'deploying'
											? 'warning'
											: 'neutral'}
								appearance="dot"
							>
								{activeDep.status === 'success' ||
								activeDep.status === 'running' ||
								activeDep.status === 'healthy'
									? 'Success'
									: activeDep.status === 'building' || activeDep.status === 'deploying'
										? 'Building'
										: activeDep.status === 'failed'
											? 'Failed'
											: activeDep.status}
							</Badge>
						</div>

						<span class="hidden text-kumo-hairline sm:inline">•</span>

						<div class="flex min-w-0 items-center gap-1.5 sm:gap-2">
							<span
								class="max-w-[130px] truncate text-xs font-medium text-kumo-default sm:max-w-[320px] sm:text-sm"
								title={activeDep.commitMessage}
							>
								{activeDep.commitMessage}
							</span>
							{#if activeDep.commit && activeDep.commit !== '—'}
								<span
									class="rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-[11px] text-kumo-subtle ring ring-kumo-hairline/60 sm:text-xs"
								>
									{activeDep.commit.substring(0, 7)}
								</span>
							{/if}
						</div>
					</div>

					<div
						class="hidden shrink-0 items-center gap-2 text-xs text-kumo-subtle tabular-nums sm:flex"
					>
						<span>Duration: {liveDuration}</span>
					</div>
				</div>

				<!-- Minimalist Pipeline Stages Stepper (Collapsible on mobile) -->
				<div class="shrink-0 border-b border-kumo-hairline bg-kumo-base px-4 py-2 sm:px-5 sm:py-3">
					<div class="flex flex-col gap-1.5">
						<button
							type="button"
							class="flex w-full items-center justify-between text-left cursor-pointer sm:cursor-default"
							onclick={() => (showPipelineMobile = !showPipelineMobile)}
						>
							<div class="flex items-center gap-1.5">
								<span class="text-xs font-medium text-kumo-subtle">Pipeline stages</span>
								<span class="text-[11px] text-kumo-subtle/80 sm:hidden">
									({completedSteps}/{steps.length} completed)
								</span>
								<CaretDown
									size={12}
									class="text-kumo-subtle transition-transform sm:hidden {showPipelineMobile
										? 'rotate-180'
										: ''}"
								/>
							</div>
							<span class="font-mono text-xs text-kumo-subtle tabular-nums sm:hidden">
								Duration: {liveDuration}
							</span>
						</button>
						<div class="{showPipelineMobile ? 'block' : 'hidden sm:block'} pt-1 sm:pt-0">
							<DeploymentPipelineStepper {steps} />
						</div>
					</div>
				</div>

				<!-- Terminal Logs Toolbar -->
				<div
					class="flex shrink-0 flex-col gap-2 border-b border-kumo-hairline bg-kumo-base px-4 py-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3 sm:px-5 sm:py-2.5"
				>
					<!-- Search Filter -->
					<div class="flex flex-1 items-center gap-2">
						<SearchInput
							bind:value={searchQuery}
							placeholder="Search deployment logs…"
							class="w-full text-xs sm:text-sm"
						/>
					</div>

					<!-- Actions: AutoScroll, Copy, Download -->
					<div class="flex items-center justify-between gap-2 sm:justify-end sm:gap-3">
						<Switch size="sm" bind:checked={autoScroll} label="Auto-scroll" />

						<div class="h-3.5 w-px bg-kumo-hairline"></div>

						<div class="flex items-center gap-1 sm:gap-1.5">
							<Button
								variant="ghost"
								size="sm"
								onclick={copyAllLogs}
								class="h-7 px-2 text-xs text-kumo-default"
								aria-label="Copy logs"
							>
								{#if copiedLogs}
									<Check size={13} class="text-kumo-success" />
									<span class="font-medium text-kumo-success">Copied</span>
								{:else}
									<Copy size={13} />
									<span class="hidden xs:inline sm:inline">Copy logs</span>
								{/if}
							</Button>

							<Button
								variant="ghost"
								size="sm"
								onclick={downloadLogFile}
								class="h-7 px-2 text-xs text-kumo-default"
								aria-label="Download logs"
							>
								<DownloadSimple size={13} />
								<span class="hidden xs:inline sm:inline">Download</span>
							</Button>
						</div>
					</div>
				</div>

				<!-- Terminal Logs Canvas -->
				<div
					bind:this={logContainer}
					class="min-h-0 flex-1 overflow-x-auto overflow-y-auto bg-[var(--bg-terminal)] p-3 sm:p-4 font-mono text-xs sm:text-[13px] leading-5 select-text selection:bg-[var(--accent)]/30"
				>
					{#if filteredLogs.length === 0}
						<div class="py-14 text-center font-sans text-sm text-kumo-subtle">
							No log lines matching "{searchQuery}".
						</div>
					{:else}
						<div class="flex flex-col">
							{#each filteredLogs as line, idx}
								{@const isErr =
									line.includes('[error]') || line.includes('FAILED') || line.includes('SIGTERM')}
								{@const isSuccess = line.includes('successfully') || line.includes('HTTP 200 OK')}
								{@const isHealth = line.includes('[healthcheck]')}
								{@const isPodman = line.includes('[podman]')}
								{@const isSystemd = line.includes('[systemd]')}
								<div
									class="group flex items-start gap-2.5 rounded px-1 sm:px-1.5 py-0.5 transition-colors hover:bg-white/[0.03]"
								>
									<span
										class="min-w-[24px] sm:min-w-[28px] text-right text-[11px] sm:text-xs text-kumo-subtle/50 select-none group-hover:text-kumo-subtle"
									>
										{idx + 1}
									</span>
									<span
										class="flex-1 break-all whitespace-pre-wrap {isErr
											? 'text-[var(--status-red)]'
											: isSuccess
												? 'text-[var(--status-green)]'
												: isHealth
													? 'text-[var(--code-key)]'
													: isPodman
														? 'text-[var(--code-string)]'
														: isSystemd
															? 'text-[var(--code-section)]'
															: 'text-kumo-default'}"
									>
										{line}
									</span>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<!-- Modal Footer -->
				<div
					class="flex shrink-0 items-center justify-between border-t border-kumo-hairline bg-kumo-base px-4 py-2 sm:px-5 sm:py-2.5 text-xs text-kumo-subtle"
				>
					<div class="flex min-w-0 items-center gap-2">
						<span
							class="size-1.5 shrink-0 rounded-full {activeDep.status === 'running' ||
							activeDep.status === 'healthy' ||
							activeDep.status === 'success'
								? 'bg-kumo-success'
								: activeDep.status === 'failed'
									? 'bg-kumo-danger'
									: 'bg-[var(--accent)]'}"
						></span>
						<span class="truncate">
							{activeDep.status === 'building' || activeDep.status === 'deploying'
								? 'Streaming live'
								: 'Completed'} • {logs.length} lines captured
						</span>
					</div>

					<Button variant="secondary" size="sm" onclick={onclose}>Close</Button>
				</div>
			{/if}
		</LayerDialog.Content>
	</LayerDialog.Root>
{/if}
