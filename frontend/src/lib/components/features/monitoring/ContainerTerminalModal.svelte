<script lang="ts">
	import { Terminal, X } from 'phosphor-svelte';
	import { TerminalView } from '$lib/components/ui';

	interface Props {
		containerName: string;
		containerId?: string;
		onclose: () => void;
	}

	let { containerName, containerId, onclose }: Props = $props();

	let targetId = $derived(containerId || containerName);
	let workloads = $derived([{ name: targetId, status: 'running' }]);
</script>

<div
	class="animate-fade-in fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
	role="dialog"
	aria-modal="true"
>
	<div
		class="flex w-full max-w-4xl flex-col overflow-hidden rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--bg-surface)] shadow-2xl"
	>
		<!-- Header -->
		<div
			class="flex items-center justify-between border-b border-[var(--border-subtle)] bg-[var(--bg-card)] px-5 py-3.5"
		>
			<div class="flex items-center gap-2.5">
				<div class="rounded-[var(--radius-sm)] bg-[var(--accent-muted)] p-1.5 text-[var(--accent)]">
					<Terminal size={18} weight="bold" />
				</div>
				<div>
					<h3 class="text-sm font-semibold text-[var(--text-primary)]">Interactive Terminal</h3>
					<p class="font-mono text-xs text-[var(--text-secondary)]">{containerName}</p>
				</div>
			</div>
			<button
				type="button"
				class="rounded-[var(--radius-sm)] p-1.5 text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				onclick={onclose}
				aria-label="Close"
			>
				<X size={18} />
			</button>
		</div>

		<!-- Real xterm Terminal Body -->
		<div class="bg-[var(--bg-surface)] p-4">
			<TerminalView title={containerName} {workloads} selectedWorkload={targetId} height="460px" />
		</div>
	</div>
</div>
