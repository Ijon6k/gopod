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
			ws.send(JSON.stringify({
				type: 'resize',
				cols: term.cols,
				rows: term.rows
			}));
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
		const token = typeof localStorage !== 'undefined' ? (localStorage.getItem('gopod_token') || '') : '';
		const cols = term ? term.cols : 80;
		const rows = term ? term.rows : 24;

		const params = new URLSearchParams({
			cmd: activeShell,
			cols: cols.toString(),
			rows: rows.toString()
		});
		if (token) {
			params.set('token', token);
		}

		const url = `${protocol}//${location.host}/api/containers/${encodeURIComponent(target)}/exec?${params.toString()}`;

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

<div class="w-full flex flex-col gap-3 {className}">
	<!-- Top Controls Toolbar (Dokploy Parity & Theme Aware) -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-[var(--border-subtle)]">
		<div class="flex items-center gap-3 flex-wrap">
			<!-- Workload / Container Selector -->
			{#if workloads.length > 1}
				<div class="flex items-center gap-2">
					<span class="text-xs text-[var(--text-tertiary)] font-medium">Container:</span>
					<div class="px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
						<select
							value={selectedWorkload || workloads[0]?.name}
							onchange={(e) => handleTargetChange((e.target as HTMLSelectElement).value)}
							class="bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
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
					<span class="text-xs text-[var(--text-secondary)] font-[var(--font-mono)]">{getTargetContainer()}</span>
				</div>
			{/if}

			<!-- Shell Switcher (sh vs bash, exactly like Dokploy) -->
			<div class="flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5">
				<button
					type="button"
					onclick={() => handleShellChange('sh')}
					class="px-2.5 py-1 text-xs font-[var(--font-mono)] rounded-[calc(var(--radius-sm)-2px)] transition-colors {activeShell === 'sh' ? 'bg-[var(--accent)] text-white font-medium' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
				>
					/bin/sh
				</button>
				<button
					type="button"
					onclick={() => handleShellChange('bash')}
					class="px-2.5 py-1 text-xs font-[var(--font-mono)] rounded-[calc(var(--radius-sm)-2px)] transition-colors {activeShell === 'bash' ? 'bg-[var(--accent)] text-white font-medium' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
				>
					bash
				</button>
			</div>

			<!-- Status indicator -->
			{#if isConnected}
				<span class="flex items-center gap-1.5 text-[11px] text-[var(--status-green)] font-medium">
					<span class="w-1.5 h-1.5 rounded-full bg-[var(--status-green)] animate-pulse"></span>
					Connected
				</span>
			{:else}
				<span class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)] font-medium">
					<span class="w-1.5 h-1.5 rounded-full bg-[var(--text-tertiary)]"></span>
					Disconnected
				</span>
			{/if}
		</div>

		<!-- Quick actions & reconnect -->
		<div class="flex items-center gap-2 flex-wrap">
			{#each quickCommands as q (q)}
				<button
					type="button"
					onclick={() => runCommand(q)}
					class="px-2 py-0.5 rounded bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] border border-[var(--border)] text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
				>
					{q}
				</button>
			{/each}

			<button
				type="button"
				onclick={() => runCommand('clear')}
				class="px-2 py-0.5 rounded bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] border border-[var(--border)] text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
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
		class="w-full relative rounded-[var(--radius-card)] border border-[var(--border)] bg-[#0a0d14] p-3 overflow-hidden shadow-inner flex flex-col"
	>
		<div
			bind:this={terminalContainer}
			class="w-full h-full"
		></div>

		{#if !isConnected && exitMessage}
			<div class="absolute bottom-3 right-3 px-3 py-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-panel)] border border-[var(--border)] text-xs text-[var(--text-secondary)] flex items-center gap-2 shadow-lg z-10">
				<span>{exitMessage}</span>
				<Button variant="secondary" size="sm" onclick={() => reconnect()} class="h-6 text-[11px] px-2">
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
