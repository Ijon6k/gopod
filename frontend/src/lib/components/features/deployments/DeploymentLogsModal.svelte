<script lang="ts">
	import type { Deployment } from '$lib/types';
	import { Button } from '$lib/components/ui';
	import { SearchInput } from '$lib/components/ui';
	import { DeploymentPipelineStepper } from '$lib/components/features/deployments';
	import { LayerDialog, Badge, Switch } from 'kumo-svelte';
	import { api } from '$lib/api';
	import { getDeploymentSteps, getDeploymentLogs } from '$lib/utils/deploymentLogs';
	import { dataStore } from '$lib/stores/data.svelte';
	import { Copy, Check, DownloadSimple } from 'phosphor-svelte';

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

	$effect(() => {
		if (open && activeDep) {
			const depId = activeDep.id;
			let isSubscribed = true;
			isLogsLoading = true;

			const syncStatus = async () => {
				try {
					const latest = await api.services.getDeployment(depId);
					if (latest && isSubscribed) {
						dataStore.upsertDeployment(latest);
					}
				} catch (_) {}
			};

			// Fetch stored log file
			api.services
				.deploymentLogs(depId)
				.then((res) => {
					if (!isSubscribed) return;
					if (res?.logs) {
						fetchedLogs = res.logs
							.split('\n')
							.map(sanitizeLogLine)
							.filter((line: string) => line.length > 0);
					}
					syncStatus();
				})
				.catch((err) => {
					console.warn('Could not load stored deployment logs:', err);
				})
				.finally(() => {
					if (isSubscribed) isLogsLoading = false;
				});

			// Periodic poller while active
			let pollInterval: ReturnType<typeof setInterval> | null = null;
			if (activeDep.status === 'building' || activeDep.status === 'deploying') {
				pollInterval = setInterval(async () => {
					await syncStatus();
					if (activeDep?.status === 'success' || activeDep?.status === 'failed') {
						if (pollInterval) clearInterval(pollInterval);
					}
				}, 1200);
			}

			// If active building/deploying, subscribe to SSE stream via centralized API
			let unsubscribeStream: (() => void) | null = null;
			if (activeDep.status === 'building' || activeDep.status === 'deploying') {
				try {
					unsubscribeStream = api.services.streamDeploymentLogs(
						depId,
						(raw) => {
							if (!isSubscribed) return;
							const clean = sanitizeLogLine(raw);
							if (!fetchedLogs.includes(clean)) {
								fetchedLogs = [...fetchedLogs, clean];
							}
						},
						() => {
							unsubscribeStream?.();
							syncStatus();
						}
					);
				} catch (e) {
					console.warn('SSE stream error:', e);
				}
			}

			return () => {
				isSubscribed = false;
				if (pollInterval) clearInterval(pollInterval);
				if (unsubscribeStream) unsubscribeStream();
			};
		} else if (fetchedLogs.length > 0) {
			fetchedLogs = [];
		}
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
			class="flex h-[85vh] max-h-[860px] min-h-[560px] flex-col overflow-hidden p-0 sm:max-w-4xl [&>div]:h-full [&>div]:flex-1 [&>div>div]:h-full [&>div>div]:flex-1"
		>
			{#if activeDep}
				<!-- Top Modal Header (Padding-right reserved for Kumo's close X button) -->
				<div
					class="flex shrink-0 items-center justify-between border-b border-kumo-hairline bg-kumo-base px-5 py-3.5 pr-14"
				>
					<div class="flex min-w-0 items-center gap-3">
						<div class="flex items-center gap-2">
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

						<div class="flex min-w-0 items-center gap-2">
							<span
								class="max-w-[320px] truncate text-sm font-medium text-kumo-default"
								title={activeDep.commitMessage}
							>
								{activeDep.commitMessage}
							</span>
							{#if activeDep.commit && activeDep.commit !== '—'}
								<span
									class="rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-xs text-kumo-subtle ring ring-kumo-hairline/60"
								>
									{activeDep.commit.substring(0, 7)}
								</span>
							{/if}
						</div>
					</div>

					<div
						class="hidden shrink-0 items-center gap-2 text-xs text-kumo-subtle tabular-nums sm:flex"
					>
						<span>Duration: {activeDep.duration}</span>
					</div>
				</div>

				<!-- Minimalist Pipeline Stages Stepper -->
				<div class="shrink-0 border-b border-kumo-hairline bg-kumo-base px-5 py-3">
					<div class="flex flex-col gap-1.5">
						<div class="flex items-center justify-between">
							<span class="text-xs font-medium text-kumo-subtle">Pipeline stages</span>
							<span class="font-mono text-xs text-kumo-subtle tabular-nums sm:hidden">
								Duration: {activeDep.duration}
							</span>
						</div>
						<DeploymentPipelineStepper {steps} />
					</div>
				</div>

				<!-- Terminal Logs Toolbar -->
				<div
					class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline bg-kumo-base px-5 py-2.5"
				>
					<!-- Search Filter -->
					<SearchInput
						bind:value={searchQuery}
						placeholder="Search deployment logs…"
						class="max-w-[320px] flex-1 text-sm"
					/>

					<!-- Actions: AutoScroll, Copy, Download -->
					<div class="flex items-center gap-3">
						<Switch size="sm" bind:checked={autoScroll} label="Auto-scroll" />

						<div class="h-4 w-px bg-kumo-hairline"></div>

						<Button
							variant="ghost"
							size="sm"
							onclick={copyAllLogs}
							class="h-7 gap-1.5 text-xs text-kumo-default"
						>
							{#if copiedLogs}
								<Check size={13} class="text-kumo-success" />
								<span class="font-medium text-kumo-success">Copied</span>
							{:else}
								<Copy size={13} />
								<span>Copy logs</span>
							{/if}
						</Button>

						<Button
							variant="ghost"
							size="sm"
							onclick={downloadLogFile}
							class="h-7 gap-1.5 text-xs text-kumo-default"
						>
							<DownloadSimple size={13} />
							<span>Download</span>
						</Button>
					</div>
				</div>

				<!-- Terminal Logs Canvas -->
				<div
					bind:this={logContainer}
					class="min-h-0 flex-1 overflow-y-auto bg-[var(--bg-terminal)] p-4 font-mono text-[13px] leading-5 select-text selection:bg-[var(--accent)]/30"
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
									class="group flex items-start gap-3 rounded px-1.5 py-0.5 transition-colors hover:bg-white/[0.03]"
								>
									<span
										class="min-w-[28px] text-right text-xs text-kumo-subtle/50 select-none group-hover:text-kumo-subtle"
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
					class="flex shrink-0 items-center justify-between border-t border-kumo-hairline bg-kumo-base px-5 py-2.5 text-xs text-kumo-subtle"
				>
					<div class="flex items-center gap-2">
						<span
							class="size-1.5 rounded-full {activeDep.status === 'running' ||
							activeDep.status === 'healthy' ||
							activeDep.status === 'success'
								? 'bg-kumo-success'
								: activeDep.status === 'failed'
									? 'bg-kumo-danger'
									: 'bg-[var(--accent)]'}"
						></span>
						<span
							>{activeDep.status === 'building' || activeDep.status === 'deploying'
								? 'Streaming live deployment logs'
								: 'Deployment completed'} • {logs.length} lines captured</span
						>
					</div>

					<Button variant="secondary" size="sm" onclick={onclose}>Close</Button>
				</div>
			{/if}
		</LayerDialog.Content>
	</LayerDialog.Root>
{/if}
