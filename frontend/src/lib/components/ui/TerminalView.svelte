<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { Button } from '$lib/components/primitives';
	import { Terminal as TerminalIcon, ArrowClockwise } from 'phosphor-svelte';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import { ClipboardAddon } from '@xterm/addon-clipboard';
	import '@xterm/xterm/css/xterm.css';

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
	}

	let {
		title = '',
		workloads = [],
		selectedWorkload = $bindable(''),
		height = '480px',
		class: className = '',
		quickCommands = ['ps aux', 'df -h', 'env', 'uname -a']
	}: Props = $props();

	let terminalContainer = $state<HTMLDivElement | null>(null);
	let activeShell = $state<'sh' | 'bash'>('sh');
	let isConnected = $state(false);
	let exitMessage = $state<string | null>(null);

	let term: Terminal | null = null;
	let fitAddon: FitAddon | null = null;
	let clipboardAddon: ClipboardAddon | null = null;
	let ws: WebSocket | null = null;
	let resizeObserver: ResizeObserver | null = null;

	function getTargetContainer(): string {
		return selectedWorkload || (workloads.length > 0 ? workloads[0].name : '') || title;
	}

	function sendResize() {
		if (!term || !ws || ws.readyState !== WebSocket.OPEN) return;
		try {
			ws.send(
				JSON.stringify({
					type: 'resize',
					cols: term.cols,
					rows: term.rows
				})
			);
		} catch (e) {
			console.warn('[Terminal] Failed to send resize:', e);
		}
	}

	function connectSession() {
		if (!browser || !terminalContainer) return;

		// Cleanup existing session
		if (ws) {
			ws.onclose = null;
			ws.onerror = null;
			ws.onmessage = null;
			ws.close();
			ws = null;
		}

		isConnected = false;
		exitMessage = null;

		const target = getTargetContainer();
		if (!target) {
			if (term) term.write('\r\n\x1b[33m[GOPOD] No container target selected.\x1b[0m\r\n');
			return;
		}

		if (!term) {
			initTerminal();
		} else {
			term.reset();
		}

		if (fitAddon) {
			try {
				fitAddon.fit();
			} catch {}
		}

		const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
		const token =
			typeof localStorage !== 'undefined' ? localStorage.getItem('gopod_token') || '' : '';
		const cols = term ? term.cols : 80;
		const rows = term ? term.rows : 24;

		const tokenParam = token ? `&token=${encodeURIComponent(token)}` : '';
		const url = `${protocol}//${location.host}/api/containers/${encodeURIComponent(target)}/exec?cmd=${encodeURIComponent(activeShell)}&cols=${cols}&rows=${rows}${tokenParam}`;

		try {
			const socket = new WebSocket(url);
			socket.binaryType = 'arraybuffer';

			socket.onopen = () => {
				isConnected = true;
				exitMessage = null;
				term?.focus();
				sendResize();
			};

			socket.onmessage = (event) => {
				if (!term) return;
				if (typeof event.data === 'string') {
					term.write(event.data);
				} else if (event.data instanceof ArrayBuffer) {
					term.write(new Uint8Array(event.data));
				}
			};

			socket.onclose = (event) => {
				isConnected = false;
				if (event.code !== 1000 && !term?.buffer.active.length) {
					exitMessage = `Session closed (code: ${event.code})`;
				}
			};

			socket.onerror = () => {
				isConnected = false;
			};

			ws = socket;
		} catch (err) {
			console.error('[Terminal] WebSocket init error:', err);
			isConnected = false;
		}
	}

	function initTerminal() {
		if (!terminalContainer) return;

		if (term) {
			term.dispose();
			term = null;
		}
		// eslint-disable-next-line svelte/no-dom-manipulating
		terminalContainer.innerHTML = '';

		term = new Terminal({
			cursorBlink: true,
			cursorStyle: 'bar',
			fontSize: 13,
			lineHeight: 1.45,
			fontFamily: 'JetBrains Mono, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
			theme: {
				background: '#0a0d14',
				foreground: '#e2e8f0',
				cursor: '#38bdf8',
				cursorAccent: '#0a0d14',
				selectionBackground: 'rgba(56, 189, 248, 0.3)',
				black: '#0a0d14',
				red: '#f87171',
				green: '#4ade80',
				yellow: '#facc15',
				blue: '#60a5fa',
				magenta: '#c084fc',
				cyan: '#38bdf8',
				white: '#f1f5f9',
				brightBlack: '#64748b',
				brightRed: '#ef4444',
				brightGreen: '#22c55e',
				brightYellow: '#eab308',
				brightBlue: '#3b82f6',
				brightMagenta: '#a855f7',
				brightCyan: '#06b6d4',
				brightWhite: '#ffffff'
			},
			convertEol: true,
			allowProposedApi: true
		});

		fitAddon = new FitAddon();
		clipboardAddon = new ClipboardAddon();

		term.loadAddon(fitAddon);
		term.loadAddon(clipboardAddon);

		term.open(terminalContainer);
		fitAddon.fit();

		term.onData((data) => {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(data);
			}
		});

		term.onResize(() => {
			sendResize();
		});

		if (typeof ResizeObserver !== 'undefined') {
			resizeObserver = new ResizeObserver(() => {
				if (fitAddon) {
					try {
						fitAddon.fit();
						sendResize();
					} catch {}
				}
			});
			resizeObserver.observe(terminalContainer);
		}
	}

	export function runCommand(cmd: string) {
		const trimmed = cmd.trim();
		if (!trimmed) return;

		if (trimmed === 'clear') {
			term?.clear();
			return;
		}

		if (ws && ws.readyState === WebSocket.OPEN) {
			ws.send(trimmed + '\n');
			term?.focus();
		} else {
			reconnect();
		}
	}

	export function reconnect() {
		connectSession();
	}

	onMount(() => {
		if (browser) {
			requestAnimationFrame(() => {
				initTerminal();
				connectSession();
			});
		}
	});

	onDestroy(() => {
		if (resizeObserver) {
			resizeObserver.disconnect();
			resizeObserver = null;
		}
		if (ws) {
			ws.onclose = null;
			ws.close();
			ws = null;
		}
		if (term) {
			term.dispose();
			term = null;
		}
	});

	function handleTargetChange(newTarget: string) {
		selectedWorkload = newTarget;
		connectSession();
	}

	function handleShellChange(shell: 'sh' | 'bash') {
		if (activeShell === shell) return;
		activeShell = shell;
		connectSession();
	}
</script>

<div class="flex w-full flex-col gap-3 {className}">
	<!-- Top Controls Toolbar (Dokploy Parity & Theme Aware) -->
	<div
		class="flex flex-col justify-between gap-3 border-b border-[var(--border-subtle)] pb-2 sm:flex-row sm:items-center"
	>
		<div class="flex flex-wrap items-center gap-3">
			<!-- Workload / Container Selector -->
			{#if workloads.length > 1}
				<div class="flex items-center gap-2">
					<span class="text-xs font-medium text-[var(--text-tertiary)]">Container:</span>
					<div
						class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1"
					>
						<select
							value={selectedWorkload || workloads[0]?.name}
							onchange={(e) => handleTargetChange((e.target as HTMLSelectElement).value)}
							class="cursor-pointer border-0 bg-transparent text-xs font-[var(--font-sans)] text-[var(--text-primary)] outline-none"
						>
							{#each workloads as w (w.name)}
								<option value={w.name}>{w.name} {w.image ? `(${w.image})` : ''}</option>
							{/each}
						</select>
					</div>
				</div>
			{:else if title}
				<div class="flex items-center gap-2">
					<TerminalIcon size={15} class="text-[var(--text-tertiary)]" />
					<span class="text-xs font-[var(--font-mono)] text-[var(--text-secondary)]"
						>{getTargetContainer()}</span
					>
				</div>
			{/if}

			<!-- Shell Switcher (sh vs bash, exactly like Dokploy) -->
			<div
				class="flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
			>
				<button
					type="button"
					onclick={() => handleShellChange('sh')}
					class="rounded-[calc(var(--radius-sm)-2px)] px-2.5 py-1 text-xs font-[var(--font-mono)] transition-colors {activeShell ===
					'sh'
						? 'bg-[var(--accent)] font-medium text-white'
						: 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
				>
					/bin/sh
				</button>
				<button
					type="button"
					onclick={() => handleShellChange('bash')}
					class="rounded-[calc(var(--radius-sm)-2px)] px-2.5 py-1 text-xs font-[var(--font-mono)] transition-colors {activeShell ===
					'bash'
						? 'bg-[var(--accent)] font-medium text-white'
						: 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
				>
					bash
				</button>
			</div>

			<!-- Status indicator -->
			{#if isConnected}
				<span class="flex items-center gap-1.5 text-[11px] font-medium text-[var(--status-green)]">
					<span class="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--status-green)]"></span>
					Connected
				</span>
			{:else}
				<span class="flex items-center gap-1.5 text-[11px] font-medium text-[var(--text-tertiary)]">
					<span class="h-1.5 w-1.5 rounded-full bg-[var(--text-tertiary)]"></span>
					Disconnected
				</span>
			{/if}
		</div>

		<!-- Quick actions & reconnect -->
		<div class="flex flex-wrap items-center gap-2">
			{#each quickCommands as q (q)}
				<button
					type="button"
					onclick={() => runCommand(q)}
					class="cursor-pointer rounded border border-[var(--border)] bg-[var(--bg-panel)] px-2 py-0.5 text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				>
					{q}
				</button>
			{/each}

			<button
				type="button"
				onclick={() => runCommand('clear')}
				class="cursor-pointer rounded border border-[var(--border)] bg-[var(--bg-panel)] px-2 py-0.5 text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			>
				clear
			</button>

			<Button variant="ghost" size="sm" onclick={() => reconnect()} title="Reconnect session">
				<ArrowClockwise size={13} />
			</Button>
		</div>
	</div>

	<!-- Real Interactive xterm.js Terminal Canvas -->
	<div
		style="height: {height};"
		class="relative flex w-full flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[#0a0d14] p-3 shadow-inner"
	>
		<div bind:this={terminalContainer} class="h-full w-full"></div>

		{#if !isConnected && exitMessage}
			<div
				class="absolute right-3 bottom-3 z-10 flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 text-xs text-[var(--text-secondary)] shadow-lg"
			>
				<span>{exitMessage}</span>
				<Button
					variant="secondary"
					size="sm"
					onclick={() => reconnect()}
					class="h-6 px-2 text-[11px]"
				>
					Reconnect
				</Button>
			</div>
		{/if}
	</div>
</div>

<style>
	:global(.xterm) {
		height: 100%;
		padding: 4px;
	}
	:global(.xterm-viewport) {
		background-color: transparent !important;
		scrollbar-width: thin;
	}
</style>
