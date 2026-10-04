<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Terminal, X, ArrowClockwise, Trash, ArrowElbowDownLeft } from 'phosphor-svelte';
	import { Button } from '$lib/components/primitives';

	interface Props {
		containerName: string;
		containerId?: string;
		onclose: () => void;
	}

	let { containerName, containerId, onclose }: Props = $props();

	let socket: WebSocket | null = null;
	let isConnected = $state(false);
	let output = $state<string[]>([]);
	let currentInput = $state('');
	let history: string[] = [];
	let historyIndex = -1;
	let terminalContainer: HTMLDivElement | null = null;
	let inputEl: HTMLInputElement | null = null;
	let activeShell = $state('/bin/sh');

	function scrollToBottom() {
		if (terminalContainer) {
			terminalContainer.scrollTop = terminalContainer.scrollHeight;
		}
	}

	function connect() {
		if (typeof window === 'undefined') return;
		if (socket) {
			socket.close();
		}

		isConnected = false;
		const id = containerId || containerName;
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = window.location.host;
		const url = `${protocol}//${host}/api/containers/${id}/exec?cmd=${activeShell}`;

		output = [`\x1b[36mConnecting to ${containerName} (${activeShell})...\x1b[0m`];

		try {
			socket = new WebSocket(url);
			socket.binaryType = 'arraybuffer';

			socket.onopen = () => {
				isConnected = true;
				output.push('\x1b[32m✔ WebSocket connection established.\x1b[0m\n');
				inputEl?.focus();
				scrollToBottom();
			};

			socket.onmessage = (event) => {
				let text = '';
				if (typeof event.data === 'string') {
					text = event.data;
				} else if (event.data instanceof ArrayBuffer) {
					text = new TextDecoder('utf-8').decode(event.data);
				}
				output.push(text);
				scrollToBottom();
			};

			socket.onclose = () => {
				isConnected = false;
				output.push('\n\x1b[33m[Connection closed by host]\x1b[0m');
				scrollToBottom();
			};

			socket.onerror = () => {
				isConnected = false;
				output.push('\n\x1b[31m[WebSocket connection error]\x1b[0m');
				scrollToBottom();
			};
		} catch (err: any) {
			output.push(`\n\x1b[31mFailed to initialize WebSocket: ${err.message}\x1b[0m`);
		}
	}

	function sendInput() {
		if (!socket || socket.readyState !== WebSocket.OPEN) return;
		const cmd = currentInput;
		if (cmd) {
			history.push(cmd);
			historyIndex = -1;
		}
		socket.send(cmd + '\n');
		currentInput = '';
		scrollToBottom();
	}

	function sendCtrlC() {
		if (!socket || socket.readyState !== WebSocket.OPEN) return;
		socket.send('\x03');
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			sendInput();
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (history.length > 0) {
				if (historyIndex === -1) historyIndex = history.length - 1;
				else if (historyIndex > 0) historyIndex--;
				currentInput = history[historyIndex];
			}
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (historyIndex !== -1) {
				if (historyIndex < history.length - 1) {
					historyIndex++;
					currentInput = history[historyIndex];
				} else {
					historyIndex = -1;
					currentInput = '';
				}
			}
		}
	}

	function clearTerminal() {
		output = [];
	}

	onMount(() => {
		connect();
		const handleGlobalKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') onclose();
		};
		window.addEventListener('keydown', handleGlobalKey);
		return () => {
			window.removeEventListener('keydown', handleGlobalKey);
			if (socket) socket.close();
		};
	});

	onDestroy(() => {
		if (socket) socket.close();
	});
</script>

<!-- Backdrop Modal -->
<div class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150">
	<div class="w-full max-w-4xl h-[620px] max-h-[90vh] flex flex-col bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] shadow-2xl overflow-hidden font-sans">
		
		<!-- Modal Header -->
		<div class="flex items-center justify-between px-4 py-3 border-b border-[var(--border)] bg-[var(--bg-surface)]">
			<div class="flex items-center gap-2.5">
				<div class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] text-[var(--accent)]">
					<Terminal size={16} />
				</div>
				<div class="flex flex-col">
					<div class="flex items-center gap-2">
						<span class="text-sm font-semibold text-[var(--text-primary)] font-mono">{containerName}</span>
						<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10.5px] font-mono {isConnected ? 'bg-[var(--status-green)]/15 text-[var(--status-green)]' : 'bg-[var(--status-amber)]/15 text-[var(--status-amber)]'}">
							<span class="w-1.5 h-1.5 rounded-full {isConnected ? 'bg-[var(--status-green)] animate-pulse' : 'bg-[var(--status-amber)]'}"></span>
							{isConnected ? 'Connected' : 'Offline'}
						</span>
					</div>
					<span class="text-[11px] text-[var(--text-tertiary)]">Interactive TTY Session via Podman Engine</span>
				</div>
			</div>

			<!-- Actions & Controls -->
			<div class="flex items-center gap-2">
				<select
					bind:value={activeShell}
					onchange={connect}
					class="text-xs font-mono px-2 py-1 rounded-[var(--radius-sm)] bg-[var(--bg-panel)] border border-[var(--border)] text-[var(--text-secondary)] outline-none cursor-pointer"
					aria-label="Shell selection"
				>
					<option value="/bin/sh">/bin/sh</option>
					<option value="/bin/bash">/bin/bash</option>
					<option value="sh">sh</option>
				</select>

				<button
					type="button"
					onclick={connect}
					class="p-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
					title="Reconnect console"
					aria-label="Reconnect"
				>
					<ArrowClockwise size={13} />
				</button>

				<button
					type="button"
					onclick={clearTerminal}
					class="p-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
					title="Clear screen"
					aria-label="Clear"
				>
					<Trash size={13} />
				</button>

				<button
					type="button"
					onclick={onclose}
					class="p-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
					title="Close terminal"
					aria-label="Close"
				>
					<X size={14} />
				</button>
			</div>
		</div>

		<!-- Terminal Screen Output -->
		<div
			bind:this={terminalContainer}
			class="flex-1 p-4 bg-[#0d1117] text-[#c9d1d9] font-mono text-[12.5px] leading-relaxed overflow-y-auto select-text whitespace-pre-wrap break-all focus:outline-none"
			tabindex="-1"
		>
			{#each output as line}
				<span>{line}</span>
			{/each}
		</div>

		<!-- Terminal Command Prompt Input Bar -->
		<div class="flex items-center gap-2 px-3 py-2.5 bg-[#161b22] border-t border-[#30363d]">
			<span class="text-[#58a6ff] font-mono text-xs font-bold pl-1">$</span>
			<input
				bind:this={inputEl}
				bind:value={currentInput}
				onkeydown={handleKeydown}
				disabled={!isConnected}
				placeholder={isConnected ? 'Type command and hit Enter...' : 'Terminal disconnected...'}
				class="flex-1 bg-transparent text-[#f0f6fc] font-mono text-xs outline-none border-0 placeholder:text-[#484f58]"
				spellcheck="false"
				autocomplete="off"
			/>
			<div class="flex items-center gap-1.5 pr-1">
				<button
					type="button"
					onclick={sendCtrlC}
					disabled={!isConnected}
					class="px-2 py-0.5 rounded text-[10px] font-mono bg-[#21262d] text-[#8b949e] hover:text-white border border-[#30363d] cursor-pointer disabled:opacity-40"
					title="Send SIGINT (Ctrl+C)"
				>
					^C
				</button>
				<button
					type="button"
					onclick={sendInput}
					disabled={!isConnected || !currentInput.trim()}
					class="flex items-center gap-1 px-2.5 py-1 rounded text-[11px] font-medium bg-[var(--accent)] text-white hover:opacity-90 disabled:opacity-40 cursor-pointer"
				>
					<span>Send</span>
					<ArrowElbowDownLeft size={11} />
				</button>
			</div>
		</div>
	</div>
</div>
