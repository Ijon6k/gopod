<script lang="ts">
	import { onDestroy } from 'svelte';
	import { Button } from '$lib/components/primitives';
	import { Terminal, ArrowClockwise } from 'phosphor-svelte';

	interface HistoryItem {
		type: 'cmd' | 'output';
		text: string;
	}

	interface WorkloadOption {
		name: string;
		image?: string;
		status?: string;
	}

	interface Props {
		title?: string;
		workloads?: WorkloadOption[];
		selectedWorkload?: string;
		height?: string;
		class?: string;
		quickCommands?: string[];
		statusText?: string;
		oncommand?: (cmd: string) => void;
	}

	let {
		title = '',
		workloads = [],
		selectedWorkload = $bindable(''),
		height = '460px',
		class: className = '',
		quickCommands = ['ps aux', 'env', 'df -h', 'clear'],
		statusText = 'Session active',
		oncommand
	}: Props = $props();

	let commandInput = $state('');
	let historyContainer = $state<HTMLDivElement | null>(null);
	let ws = $state<WebSocket | null>(null);
	let isConnected = $state(false);

	let history = $state<HistoryItem[]>([]);

	function connectWebSocket(target: string) {
		if (typeof window === 'undefined') return;
		if (ws) {
			ws.close();
			ws = null;
		}

		const containerTarget = target || selectedWorkload || title;
		if (!containerTarget) return;

		const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
		const url = `${protocol}//${location.host}/api/containers/${encodeURIComponent(containerTarget)}/exec`;
		try {
			const socket = new WebSocket(url);
			socket.binaryType = 'arraybuffer';
			socket.onopen = () => {
				isConnected = true;
				history.push({ type: 'output', text: `🚀 Connected to container session: ${containerTarget}` });
			};
			socket.onmessage = (e) => {
				let text = '';
				if (typeof e.data === 'string') {
					text = e.data;
				} else if (e.data instanceof ArrayBuffer) {
					text = new TextDecoder().decode(e.data);
				}
				if (text) {
					history.push({ type: 'output', text: text.trimEnd() });
					requestAnimationFrame(() => {
						if (historyContainer) historyContainer.scrollTop = historyContainer.scrollHeight;
					});
				}
			};
			socket.onclose = () => {
				isConnected = false;
				history.push({ type: 'output', text: `[Session closed]` });
			};
			socket.onerror = () => {
				isConnected = false;
			};
			ws = socket;
		} catch (err) {
			console.warn('Exec websocket error:', err);
		}
	}

	$effect(() => {
		const target = selectedWorkload || title;
		if (target) {
			connectWebSocket(target);
		}
		return () => {
			if (ws) {
				ws.close();
				ws = null;
			}
		};
	});

	onDestroy(() => {
		if (ws) ws.close();
	});

	function runCommand(cmd: string) {
		const trimmed = cmd.trim();
		if (!trimmed) return;

		history.push({ type: 'cmd', text: trimmed });

		if (trimmed === 'clear') {
			history = [];
		} else if (oncommand) {
			oncommand(trimmed);
		} else if (ws && ws.readyState === WebSocket.OPEN) {
			ws.send(trimmed + '\n');
		} else {
			history.push({ type: 'output', text: `Terminal not connected to active container.` });
		}

		commandInput = '';
		requestAnimationFrame(() => {
			if (historyContainer) {
				historyContainer.scrollTop = historyContainer.scrollHeight;
			}
		});
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			runCommand(commandInput);
		}
	}

	export function resetSession(name?: string) {
		const targetName = name || selectedWorkload || title || 'container';
		history = [
			{ type: 'output', text: `Reconnecting to container session: ${targetName}…` }
		];
		connectWebSocket(targetName);
	}
</script>

<div class="w-full flex flex-col gap-3 {className}">
	<!-- Top Bar / Controls -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-[var(--border-subtle)]">
		<div class="flex items-center gap-3">
			{#if workloads.length > 1}
				<div class="flex items-center gap-2">
					<span class="text-xs text-[var(--text-tertiary)] font-medium">Target Container:</span>
					<div class="px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
						<select
							bind:value={selectedWorkload}
							onchange={() => resetSession(selectedWorkload)}
							class="bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
						>
							{#each workloads as w}
								<option value={w.name}>{w.name} {w.image ? `(${w.image})` : ''}</option>
							{/each}
						</select>
					</div>
				</div>
			{:else if title}
				<div class="flex items-center gap-2">
					<Terminal size={15} class="text-[var(--text-tertiary)]" />
					<span class="text-xs text-[var(--text-secondary)] font-[var(--font-mono)]">{title}</span>
				</div>
			{/if}

			<span class="flex items-center gap-1.5 text-[11px] text-[var(--status-green)]">
				<span class="w-1.5 h-1.5 rounded-full bg-[var(--status-green)]"></span>
				{statusText}
			</span>
		</div>

		<!-- Quick action buttons -->
		<div class="flex items-center gap-2">
			{#each quickCommands as q}
				<button
					type="button"
					onclick={() => runCommand(q)}
					class="px-2 py-0.5 rounded bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] border border-[var(--border)] text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
				>
					{q}
				</button>
			{/each}

			<Button variant="ghost" size="sm" onclick={() => resetSession()} title="Reconnect session">
				<ArrowClockwise size={13} />
			</Button>
		</div>
	</div>

	<!-- Interactive Terminal Canvas (Theme-Aware) -->
	<div
		style="height: {height};"
		class="w-full p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-terminal)] font-[var(--font-mono)] text-[12px] leading-[1.65] overflow-hidden flex flex-col justify-between transition-colors"
	>
		<!-- Output History -->
		<div bind:this={historyContainer} class="flex-1 overflow-y-auto flex flex-col gap-1 pr-1">
			{#each history as item}
				{#if item.type === 'cmd'}
					<div class="flex items-center gap-2 text-[var(--code-text)]">
						<span class="text-[var(--accent)] select-none font-semibold">$</span>
						<span class="text-[var(--code-text)] font-medium">{item.text}</span>
					</div>
				{:else}
					<div class="text-[var(--text-secondary)] whitespace-pre-wrap pl-3 border-l-2 border-[var(--border-subtle)] my-0.5">
						{item.text}
					</div>
				{/if}
			{/each}
		</div>

		<!-- Prompt Input -->
		<div class="flex items-center gap-2 pt-2.5 border-t border-[var(--border-subtle)] mt-2">
			<span class="text-[var(--accent)] font-semibold select-none">$</span>
			<input
				type="text"
				bind:value={commandInput}
				onkeydown={handleKeydown}
				placeholder="Type command and press Enter…"
				class="flex-1 bg-transparent border-0 outline-none text-[var(--code-text)] font-[var(--font-mono)] text-[12px] caret-[var(--accent)] placeholder:text-[var(--text-tertiary)]"
			/>
		</div>
	</div>
</div>
