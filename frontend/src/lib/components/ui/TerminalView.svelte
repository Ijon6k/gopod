<script lang="ts">
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

	let history = $state<HistoryItem[]>([
		{ type: 'output', text: `Linux podman-host 6.6.14-200.fc39.x86_64 #1 SMP PREEMPT_DYNAMIC` },
		{ type: 'output', text: `Connected to container session (user: app, rootless)` },
		{ type: 'cmd', text: 'whoami' },
		{ type: 'output', text: 'app (uid=1000 gid=1000)' },
		{ type: 'cmd', text: 'uname -a' },
		{ type: 'output', text: 'Linux container 6.6.14 x86_64 Linux' }
	]);

	function runCommand(cmd: string) {
		const trimmed = cmd.trim();
		if (!trimmed) return;

		history.push({ type: 'cmd', text: trimmed });

		if (oncommand) {
			oncommand(trimmed);
		} else {
			// Built-in command simulation
			if (trimmed === 'clear') {
				history = [];
			} else if (trimmed === 'ps aux' || trimmed === 'ps') {
				history.push({
					type: 'output',
					text: `USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND\napp          1  0.2  0.8 124800 28400 ?        Ssl  10:14   0:02 node server.js\napp         42  0.0  0.1  14200  4200 pts/0    Ss   10:18   0:00 /bin/sh`
				});
			} else if (trimmed === 'env') {
				history.push({
					type: 'output',
					text: `NODE_ENV=production\nPORT=3000\nHOSTNAME=${title || 'app'}\nHOME=/home/app\nPATH=/usr/local/bin:/usr/bin:/bin`
				});
			} else if (trimmed === 'df -h') {
				history.push({
					type: 'output',
					text: `Filesystem      Size  Used Avail Use% Mounted on\noverlay          50G  8.4G   42G  17% /\ntmpfs            64M     0   64M   0% /dev\nshm              64M     0   64M   0% /dev/shm`
				});
			} else if (trimmed === 'whoami') {
				history.push({ type: 'output', text: 'app (uid=1000 gid=1000)' });
			} else {
				history.push({ type: 'output', text: `Executed: ${trimmed} (exit code: 0)` });
			}
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
			{ type: 'output', text: `Reconnecting to container session: ${targetName}…` },
			{ type: 'output', text: `Session connected (rootless)` }
		];
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
