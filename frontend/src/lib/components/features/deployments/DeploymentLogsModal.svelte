<script lang="ts">
	import type { Deployment } from '$lib/types';
	import { Button } from '$lib/components/primitives';
	import { StatusBadge, SearchInput } from '$lib/components/ui';
	import { api } from '$lib/api';
	import { getDeploymentSteps, getDeploymentLogs } from '$lib/utils/deploymentLogs';
	import {
		X,
		Copy,
		Check,
		DownloadSimple,
		MagnifyingGlass,
		CircleNotch,
		CheckCircle,
		XCircle,
		Clock,
		ArrowSquareOut
	} from 'phosphor-svelte';

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

	let steps = $derived(deployment ? getDeploymentSteps(deployment) : []);
	let logs = $derived.by(() => {
		const source =
			fetchedLogs.length > 0
				? fetchedLogs
				: deployment?.logs && deployment.logs.length > 0
					? deployment.logs
					: deployment
						? getDeploymentLogs(deployment)
						: [];
		return source.map(sanitizeLogLine);
	});

	let filteredLogs = $derived.by(() => {
		if (!searchQuery.trim()) return logs;
		const q = searchQuery.toLowerCase().trim();
		return logs.filter((l) => l.toLowerCase().includes(q));
	});

	$effect(() => {
		if (open && deployment) {
			const depId = deployment.id;
			let isSubscribed = true;
			isLogsLoading = true;

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
				})
				.catch((err) => {
					console.warn('Could not load stored deployment logs:', err);
				})
				.finally(() => {
					if (isSubscribed) isLogsLoading = false;
				});

			// If active building/deploying, subscribe to SSE stream
			let es: EventSource | null = null;
			if (deployment.status === 'building' || deployment.status === 'deploying') {
				try {
					es = new EventSource(`/api/deployments/${depId}/logs/stream`);
					es.onmessage = (event) => {
						if (!isSubscribed) return;
						if (event.data) {
							const clean = sanitizeLogLine(event.data);
							if (!fetchedLogs.includes(clean)) {
								fetchedLogs = [...fetchedLogs, clean];
							}
						}
					};
					es.onerror = () => {
						es?.close();
					};
				} catch (e) {
					console.warn('SSE stream error:', e);
				}
			}

			return () => {
				isSubscribed = false;
				if (es) es.close();
			};
		} else {
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
		if (!deployment || !logs.length) return;
		const blob = new Blob([logs.join('\n')], { type: 'text/plain;charset=utf-8' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `gopod-deployment-${deployment.id}-${deployment.number}.log`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			onclose();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open && deployment}
	<!-- Modal Backdrop -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(5,6,7,0.82)] p-3 backdrop-blur-[3px] sm:p-5"
		role="dialog"
		aria-modal="true"
	>
		<div
			class="animate-in fade-in zoom-in-95 flex h-[88vh] max-h-[860px] w-full max-w-[880px] flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] duration-150"
		>
			<!-- Top Modal Header -->
			<div
				class="flex shrink-0 items-center justify-between border-b border-[var(--border)] bg-[var(--bg-panel)] px-5 py-3.5"
			>
				<div class="flex min-w-0 items-center gap-3">
					<div class="flex items-center gap-2">
						<span class="text-sm font-[var(--font-mono)] font-semibold text-[var(--text-primary)]">
							#{deployment.number}
						</span>
						<StatusBadge status={deployment.status} size="sm" />
					</div>

					<span class="hidden text-[var(--border)] sm:inline">•</span>

					<div class="flex min-w-0 items-center gap-2">
						<span
							class="max-w-[280px] truncate text-xs font-medium text-[var(--text-secondary)]"
							title={deployment.commitMessage}
						>
							{deployment.commitMessage}
						</span>
						{#if deployment.commit && deployment.commit !== '—'}
							<span
								class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-1.5 py-0.5 text-[10.5px] font-[var(--font-mono)] text-[var(--text-tertiary)]"
							>
								{deployment.commit.substring(0, 7)}
							</span>
						{/if}
					</div>
				</div>

				<div class="flex shrink-0 items-center gap-2">
					<span class="hidden text-xs text-[var(--text-tertiary)] tabular-nums sm:inline">
						Duration: {deployment.duration}
					</span>

					<button
						type="button"
						onclick={onclose}
						class="cursor-pointer rounded-[var(--radius-sm)] border-0 bg-transparent p-1 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]"
						aria-label="Close logs dialog"
					>
						<X size={18} />
					</button>
				</div>
			</div>

			<!-- Pipeline Stages Stepper (Dokploy & Coolify UX) -->
			<div class="shrink-0 border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-5 py-3">
				<div class="flex flex-col gap-1.5">
					<span
						class="text-[10px] font-semibold tracking-wider text-[var(--text-tertiary)] uppercase"
					>
						Deployment Pipeline
					</span>

					<div class="grid grid-cols-1 gap-2 sm:grid-cols-5">
						{#each steps as step, i}
							<div
								class="flex items-center gap-2 rounded-[var(--radius-sm)] border p-2 text-xs {step.status ===
								'success'
									? 'border-[rgba(76,154,114,0.25)] bg-[rgba(76,154,114,0.06)] text-[var(--text-primary)]'
									: step.status === 'running'
										? 'border-[rgba(105,115,168,0.35)] bg-[rgba(105,115,168,0.08)] text-[var(--text-primary)]'
										: step.status === 'failed'
											? 'border-[rgba(184,84,84,0.30)] bg-[rgba(184,84,84,0.08)] text-[var(--text-primary)]'
											: 'border-[var(--border-subtle)] bg-[var(--bg-panel)] text-[var(--text-tertiary)]'}"
							>
								<!-- Icon based on status -->
								{#if step.status === 'success'}
									<CheckCircle size={14} class="shrink-0 text-[var(--status-green)]" />
								{:else if step.status === 'running'}
									<CircleNotch size={14} class="shrink-0 animate-spin text-[var(--accent)]" />
								{:else if step.status === 'failed'}
									<XCircle size={14} class="shrink-0 text-[var(--status-red)]" />
								{:else}
									<Clock size={14} class="shrink-0 text-[var(--text-tertiary)] opacity-60" />
								{/if}

								<div class="flex min-w-0 flex-1 flex-col leading-tight">
									<span class="truncate text-[11px] font-medium">
										{step.name}
									</span>
									{#if step.duration}
										<span class="text-[10px] text-[var(--text-tertiary)] tabular-nums">
											{step.duration}
										</span>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				</div>
			</div>

			<!-- Terminal Logs Toolbar -->
			<div
				class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-[var(--border)] bg-[var(--bg-panel)] px-5 py-2.5 text-xs"
			>
				<!-- Search Filter -->
				<SearchInput
					bind:value={searchQuery}
					placeholder="Search in deployment logs..."
					class="max-w-[320px] flex-1 text-xs"
				/>

				<!-- Actions: AutoScroll, Copy, Download -->
				<div class="flex items-center gap-2 text-xs">
					<label
						class="flex cursor-pointer items-center gap-1.5 text-[var(--text-secondary)] select-none hover:text-[var(--text-primary)]"
					>
						<input
							type="checkbox"
							bind:checked={autoScroll}
							class="cursor-pointer rounded border-[var(--border)] text-[var(--accent)] accent-[var(--accent)]"
						/>
						<span class="text-[11px]">Auto-scroll</span>
					</label>

					<div class="h-3.5 w-px bg-[var(--border)]"></div>

					<Button variant="ghost" size="sm" onclick={copyAllLogs} class="h-7 gap-1 text-[11px]">
						{#if copiedLogs}
							<Check size={12} class="text-[var(--status-green)]" />
							<span class="text-[var(--status-green)]">Copied</span>
						{:else}
							<Copy size={12} />
							<span>Copy Logs</span>
						{/if}
					</Button>

					<Button variant="ghost" size="sm" onclick={downloadLogFile} class="h-7 gap-1 text-[11px]">
						<DownloadSimple size={12} />
						<span>Download</span>
					</Button>
				</div>
			</div>

			<!-- Terminal Logs Canvas -->
			<div
				bind:this={logContainer}
				class="min-h-0 flex-1 overflow-y-auto bg-[var(--bg-terminal)] p-4 text-[12px] leading-[20px] font-[var(--font-mono)] select-text selection:bg-[rgba(105,115,168,0.3)]"
			>
				{#if filteredLogs.length === 0}
					<div
						class="py-12 text-center text-xs font-[var(--font-sans)] text-[var(--text-tertiary)]"
					>
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
								class="group flex items-start gap-3 rounded px-1.5 py-0.5 transition-colors hover:bg-[rgba(255,255,255,0.02)]"
							>
								<span
									class="min-w-[26px] text-right text-[11px] text-[var(--text-tertiary)] opacity-40 select-none group-hover:opacity-75"
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
														: 'text-[var(--code-text)]'}"
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
				class="flex shrink-0 items-center justify-between border-t border-[var(--border)] bg-[var(--bg-panel)] px-5 py-2.5 text-[11px] text-[var(--text-tertiary)]"
			>
				<div class="flex items-center gap-2">
					<span
						class="h-1.5 w-1.5 rounded-full {deployment.status === 'running'
							? 'bg-[var(--status-green)]'
							: deployment.status === 'failed'
								? 'bg-[var(--status-red)]'
								: 'bg-[var(--accent)]'}"
					></span>
					<span>Stream finished • {logs.length} lines captured</span>
				</div>

				<Button variant="secondary" size="sm" onclick={onclose} class="h-7 text-xs">Close</Button>
			</div>
		</div>
	</div>
{/if}
