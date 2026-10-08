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

<div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in" role="dialog" aria-modal="true">
	<div class="bg-[var(--bg-surface)] border border-[var(--border)] rounded-[var(--radius-lg)] w-full max-w-4xl shadow-2xl overflow-hidden flex flex-col">
		<!-- Header -->
		<div class="flex items-center justify-between px-5 py-3.5 border-b border-[var(--border-subtle)] bg-[var(--bg-card)]">
			<div class="flex items-center gap-2.5">
				<div class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] text-[var(--accent)]">
					<Terminal size={18} weight="bold" />
				</div>
				<div>
					<h3 class="text-sm font-semibold text-[var(--text-primary)]">Interactive Terminal</h3>
					<p class="text-xs font-mono text-[var(--text-secondary)]">{containerName}</p>
				</div>
			</div>
			<button
				type="button"
				class="p-1.5 rounded-[var(--radius-sm)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] transition-colors"
				onclick={onclose}
				aria-label="Close"
			>
				<X size={18} />
			</button>
		</div>

		<!-- Real xterm Terminal Body -->
		<div class="p-4 bg-[var(--bg-surface)]">
			<TerminalView
				title={containerName}
				{workloads}
				selectedWorkload={targetId}
				height="460px"
			/>
		</div>
	</div>
</div>
