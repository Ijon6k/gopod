<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { Button, Badge } from '$lib/components/ui';
	import {
		Terminal as TerminalIcon,
		ArrowClockwise,
		ArrowsOut,
		ArrowsIn,
		CaretDown
	} from 'phosphor-svelte';
	import { DropdownMenu } from 'kumo-svelte';
	import { cn } from '$lib/utils/cn';
	import { api } from '$lib/api';
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
		height = 'calc(100dvh - 280px)',
		class: className = '',
		quickCommands = ['ps aux', 'df -h', 'env', 'uname -a']
	}: Props = $props();

	let terminalContainer = $state<HTMLDivElement | null>(null);
	let activeShell = $state<'sh' | 'bash'>('sh');
	let isConnected = $state(false);
	let exitMessage = $state<string | null>(null);
	let isFullscreen = $state(false);

	let term: Terminal | null = null;
	let fitAddon: FitAddon | null = null;
	let clipboardAddon: ClipboardAddon | null = null;
	let ws: WebSocket | null = null;
	let resizeObserver: ResizeObserver | null = null;
	// Destroy epoch: connectSession spans awaits and socket creation, so
	// anything resolving after onDestroy must never write into a disposed
	// component. Handlers compare their snapshot against this counter.
	let epoch = 0;
	let rafId: number | null = null;

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

	async function connectSession() {
		if (!browser || !terminalContainer) return;

		// Snapshot the epoch: if the component unmounts at any of the awaits
		// below, this attempt is stale and must not re-enter the DOM state.
		const myEpoch = epoch;

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

		const cols = term ? term.cols : 80;
		const rows = term ? term.rows : 24;

		let ticket: string | undefined;
		try {
			ticket = await api.runtime.containers.getTerminalTicket(target);
		} catch {
			// fallback silently to session token
		}

		// The ticket fetch can take seconds; bail out silently if the
		// component was destroyed while it was pending — no UI writes and
		// no term/subscription changes from this stale attempt.
		if (myEpoch !== epoch) return;

		const url = api.runtime.containers.getTerminalWsUrl(target, cols, rows, activeShell, ticket);

		try {
			const socket = new WebSocket(url);
			socket.binaryType = 'arraybuffer';

			socket.onopen = () => {
				// A late-opening socket must not write into a disposed xterm.
				if (myEpoch !== epoch) {
					socket.close();
					return;
				}
				isConnected = true;
				exitMessage = null;
				term?.focus();
				sendResize();
			};

			socket.onmessage = (event) => {
				if (myEpoch !== epoch || !term) return;
				if (typeof event.data === 'string') {
					term.write(event.data);
				} else if (event.data instanceof ArrayBuffer) {
					term.write(new Uint8Array(event.data));
				}
			};

			socket.onclose = (event) => {
				if (myEpoch !== epoch) return;
				isConnected = false;
				if (event.code !== 1000 && !term?.buffer.active.length) {
					exitMessage = `Session closed (code: ${event.code})`;
				}
			};

			socket.onerror = () => {
				if (myEpoch !== epoch) return;
				isConnected = false;
			};

			// A socket created right after destroy would never be closed by
			// onDestroy's cleanup — close it here instead of wiring it up.
			if (myEpoch !== epoch) {
				socket.onopen = null;
				socket.onclose = null;
				socket.onerror = null;
				socket.onmessage = null;
				socket.close();
				return;
			}

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
				background: '#0d1117',
				foreground: '#f1f5f9',
				cursor: '#f6821f',
				cursorAccent: '#0d1117',
				selectionBackground: 'rgba(246, 130, 31, 0.25)',
				black: '#0d1117',
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
			// Track the rAF id so destroy can cancel a still-pending connect.
			rafId = requestAnimationFrame(() => {
				rafId = null;
				initTerminal();
				connectSession();
			});
		}
	});

	onDestroy(() => {
		// Invalidate first: any socket or await resuming after this point
		// belongs to a destroyed component and must bail out on its own.
		epoch++;
		if (rafId !== null) {
			cancelAnimationFrame(rafId);
			rafId = null;
		}
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

	function toggleFullscreen() {
		isFullscreen = !isFullscreen;
		setTimeout(() => {
			if (fitAddon) {
				try {
					fitAddon.fit();
					sendResize();
				} catch {}
			}
		}, 60);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && isFullscreen) {
			toggleFullscreen();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div
	class={cn(
		'flex w-full flex-col gap-3 transition-all',
		isFullscreen
			? 'fixed inset-0 z-50 h-[100dvh] w-screen bg-[var(--bg-canvas)] p-3 md:p-4'
			: className
	)}
>
	<!-- Top Controls Toolbar (Theme Aware) -->
	<div
		class="flex flex-col justify-between gap-3 border-b border-[var(--border-subtle)] pb-2 sm:flex-row sm:items-center"
	>
		<div class="flex flex-wrap items-center gap-3">
			<!-- Workload / Container Selector using Kumo DropdownMenu -->
			{#if workloads.length > 1}
				<div class="flex items-center gap-2">
					<span class="text-xs font-medium text-[var(--text-tertiary)]">Container:</span>
					<DropdownMenu>
						<DropdownMenu.Trigger
							class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1 text-xs font-[var(--font-sans)] text-[var(--text-primary)] outline-none hover:bg-[var(--bg-hover)]"
						>
							<span class="max-w-[140px] truncate sm:max-w-[200px]">
								{selectedWorkload || workloads[0]?.name}
							</span>
							<CaretDown size={11} class="shrink-0 text-[var(--text-tertiary)]" />
						</DropdownMenu.Trigger>
						<DropdownMenu.Content class="w-56" align="start">
							{#each workloads as w (w.name)}
								<DropdownMenu.Item
									selected={(selectedWorkload || workloads[0]?.name) === w.name}
									onclick={() => handleTargetChange(w.name)}
								>
									<span class="truncate text-xs">{w.name} {w.image ? `(${w.image})` : ''}</span>
								</DropdownMenu.Item>
							{/each}
						</DropdownMenu.Content>
					</DropdownMenu>
				</div>
			{:else if title}
				<div class="flex items-center gap-2">
					<TerminalIcon size={15} class="text-[var(--text-tertiary)]" />
					<span class="text-xs font-[var(--font-mono)] text-[var(--text-secondary)]"
						>{getTargetContainer()}</span
					>
				</div>
			{/if}

			<!-- Shell Switcher (sh vs bash) -->
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

			<!-- Status indicator using Kumo Badge -->
			{#if isConnected}
				<Badge variant="primary" size="sm">Connected</Badge>
			{:else}
				<Badge variant="neutral" size="sm">Disconnected</Badge>
			{/if}
		</div>

		<!-- Quick actions, reconnect & Fullscreen (100dvh) -->
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

			<Button
				variant="ghost"
				size="sm"
				onclick={toggleFullscreen}
				title={isFullscreen ? 'Exit full screen (100dvh)' : 'Full screen (100dvh)'}
				class="text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
			>
				{#if isFullscreen}
					<ArrowsIn size={13} />
				{:else}
					<ArrowsOut size={13} />
				{/if}
			</Button>
		</div>
	</div>

	<!-- Real Interactive xterm.js Terminal Canvas -->
	<div
		style={isFullscreen ? 'height: calc(100dvh - 58px);' : `height: ${height}; min-height: 520px;`}
		class="relative flex w-full flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-terminal)] p-3 shadow-inner"
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
