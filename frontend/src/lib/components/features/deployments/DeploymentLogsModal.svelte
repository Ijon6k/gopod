<script lang="ts">
	import type { Deployment } from '$lib/types';
	import { Button } from '$lib/components/primitives';
	import { StatusBadge } from '$lib/components/ui';
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

	let steps = $derived(deployment ? getDeploymentSteps(deployment) : []);
	let logs = $derived(deployment ? getDeploymentLogs(deployment) : []);

	let filteredLogs = $derived.by(() => {
		if (!searchQuery.trim()) return logs;
		const q = searchQuery.toLowerCase().trim();
		return logs.filter((l) => l.toLowerCase().includes(q));
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
		class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-5 bg-[rgba(5,6,7,0.82)] backdrop-blur-[3px]"
		role="dialog"
		aria-modal="true"
	>
		<div
			class="w-full max-w-[880px] h-[88vh] max-h-[860px] flex flex-col rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden animate-in fade-in zoom-in-95 duration-150"
		>
			<!-- Top Modal Header -->
			<div class="flex items-center justify-between px-5 py-3.5 border-b border-[var(--border)] bg-[var(--bg-panel)] shrink-0">
				<div class="flex items-center gap-3 min-w-0">
					<div class="flex items-center gap-2">
						<span class="text-sm font-semibold font-[var(--font-mono)] text-[var(--text-primary)]">
							#{deployment.number}
						</span>
						<StatusBadge status={deployment.status} size="sm" />
					</div>

					<span class="text-[var(--border)] hidden sm:inline">•</span>

					<div class="flex items-center gap-2 min-w-0">
						<span class="text-xs font-medium text-[var(--text-secondary)] truncate max-w-[280px]" title={deployment.commitMessage}>
							{deployment.commitMessage}
						</span>
						{#if deployment.commit && deployment.commit !== '—'}
							<span class="font-[var(--font-mono)] text-[10.5px] text-[var(--text-tertiary)] bg-[var(--bg-surface)] px-1.5 py-0.5 rounded border border-[var(--border)]">
								{deployment.commit.substring(0, 7)}
							</span>
						{/if}
					</div>
				</div>

				<div class="flex items-center gap-2 shrink-0">
					<span class="text-xs text-[var(--text-tertiary)] hidden sm:inline tabular-nums">
						Duration: {deployment.duration}
					</span>

					<button
						type="button"
						onclick={onclose}
						class="p-1 rounded-[var(--radius-sm)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-surface)] transition-colors cursor-pointer border-0 bg-transparent"
						aria-label="Close logs dialog"
					>
						<X size={18} />
					</button>
				</div>
			</div>

			<!-- Pipeline Stages Stepper (Dokploy & Coolify UX) -->
			<div class="px-5 py-3 border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] shrink-0">
				<div class="flex flex-col gap-1.5">
					<span class="text-[10px] uppercase tracking-wider font-semibold text-[var(--text-tertiary)]">
						Deployment Pipeline
					</span>

					<div class="grid grid-cols-1 sm:grid-cols-5 gap-2">
						{#each steps as step, i}
							<div
								class="flex items-center gap-2 p-2 rounded-[var(--radius-sm)] border text-xs {step.status === 'success'
									? 'border-[rgba(76,154,114,0.25)] bg-[rgba(76,154,114,0.06)] text-[var(--text-primary)]'
									: step.status === 'running'
										? 'border-[rgba(105,115,168,0.35)] bg-[rgba(105,115,168,0.08)] text-[var(--text-primary)]'
										: step.status === 'failed'
											? 'border-[rgba(184,84,84,0.30)] bg-[rgba(184,84,84,0.08)] text-[var(--text-primary)]'
											: 'border-[var(--border-subtle)] bg-[var(--bg-panel)] text-[var(--text-tertiary)]'}"
							>
								<!-- Icon based on status -->
								{#if step.status === 'success'}
									<CheckCircle size={14} class="text-[var(--status-green)] shrink-0" />
								{:else if step.status === 'running'}
									<CircleNotch size={14} class="text-[var(--accent)] animate-spin shrink-0" />
								{:else if step.status === 'failed'}
									<XCircle size={14} class="text-[var(--status-red)] shrink-0" />
								{:else}
									<Clock size={14} class="text-[var(--text-tertiary)] shrink-0 opacity-60" />
								{/if}

								<div class="flex flex-col min-w-0 flex-1 leading-tight">
									<span class="text-[11px] truncate font-medium">
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
			<div class="flex flex-wrap items-center justify-between gap-3 px-5 py-2.5 border-b border-[var(--border)] bg-[var(--bg-panel)] shrink-0 text-xs">
				<!-- Search Filter -->
				<div class="flex items-center gap-2 flex-1 max-w-[320px] px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
					<MagnifyingGlass size={13} class="text-[var(--text-tertiary)]" />
					<input
						type="text"
						bind:value={searchQuery}
						placeholder="Search in deployment logs..."
						class="w-full bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)] font-[var(--font-mono)]"
					/>
					{#if searchQuery}
						<button
							type="button"
							onclick={() => (searchQuery = '')}
							class="text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border-0 bg-transparent cursor-pointer p-0"
						>
							<X size={11} />
						</button>
					{/if}
				</div>

				<!-- Actions: AutoScroll, Copy, Download -->
				<div class="flex items-center gap-2 text-xs">
					<label class="flex items-center gap-1.5 text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer select-none">
						<input
							type="checkbox"
							bind:checked={autoScroll}
							class="rounded border-[var(--border)] text-[var(--accent)] accent-[var(--accent)] cursor-pointer"
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
				class="flex-1 min-h-0 overflow-y-auto p-4 bg-[var(--bg-terminal)] font-[var(--font-mono)] text-[12px] leading-[20px] select-text selection:bg-[rgba(105,115,168,0.3)]"
			>
				{#if filteredLogs.length === 0}
					<div class="py-12 text-center text-xs text-[var(--text-tertiary)] font-[var(--font-sans)]">
						No log lines matching "{searchQuery}".
					</div>
				{:else}
					<div class="flex flex-col">
						{#each filteredLogs as line, idx}
							{@const isErr = line.includes('[error]') || line.includes('FAILED') || line.includes('SIGTERM')}
							{@const isSuccess = line.includes('successfully') || line.includes('HTTP 200 OK')}
							{@const isHealth = line.includes('[healthcheck]')}
							{@const isPodman = line.includes('[podman]')}
							{@const isSystemd = line.includes('[systemd]')}
							<div class="flex items-start gap-3 hover:bg-[rgba(255,255,255,0.02)] px-1.5 py-0.5 rounded transition-colors group">
								<span class="text-[11px] text-[var(--text-tertiary)] select-none opacity-40 group-hover:opacity-75 min-w-[26px] text-right">
									{idx + 1}
								</span>
								<span
									class="flex-1 whitespace-pre-wrap break-all {isErr
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
			<div class="px-5 py-2.5 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-between shrink-0 text-[11px] text-[var(--text-tertiary)]">
				<div class="flex items-center gap-2">
					<span class="w-1.5 h-1.5 rounded-full {deployment.status === 'running' ? 'bg-[var(--status-green)]' : deployment.status === 'failed' ? 'bg-[var(--status-red)]' : 'bg-[var(--accent)]'}"></span>
					<span>Stream finished • {logs.length} lines captured</span>
				</div>

				<Button variant="secondary" size="sm" onclick={onclose} class="h-7 text-xs">
					Close
				</Button>
			</div>
		</div>
	</div>
{/if}
