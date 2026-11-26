<script lang="ts">
	import type { FormattedLogEntry } from '$lib/utils/log-format';

	interface Props {
		entries: FormattedLogEntry[];
		loading?: boolean;
		error?: string;
		emptyMessage?: string;
		loadingMessage?: string;
		showContainer?: boolean;
		wrapLines?: boolean;
		prettyJson?: boolean;
		follow?: boolean;
		containerClass?: string;
		containerStyle?: string;
		headerSlot?: import('svelte').Snippet;
	}

	let {
		entries,
		loading = false,
		error = '',
		emptyMessage = 'Container has not emitted any stdout/stderr logs yet.',
		loadingMessage = 'Streaming logs from container runtime…',
		showContainer = false,
		wrapLines = true,
		prettyJson = false,
		follow = false,
		containerClass = '',
		containerStyle = '',
		headerSlot
	}: Props = $props();

	let viewport = $state<HTMLDivElement | null>(null);

	$effect(() => {
		if (follow && viewport && entries.length > 0) {
			viewport.scrollTop = viewport.scrollHeight;
		}
	});

	function levelBadgeClass(level: FormattedLogEntry['level']): string {
		if (level === 'ERROR') {
			return 'border-[var(--status-red)]/30 bg-[var(--status-red-muted)] text-[var(--status-red)]';
		}
		if (level === 'WARN') {
			return 'border-[var(--status-amber)]/30 bg-[var(--status-amber-muted)] text-[var(--status-amber)]';
		}
		if (level === 'DEBUG') {
			return 'border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-tertiary)]';
		}
		return 'border-transparent text-[var(--text-tertiary)]';
	}

	function rowAccentClass(level: FormattedLogEntry['level']): string {
		if (level === 'ERROR')
			return 'bg-[var(--status-red-muted)]/35 hover:bg-[var(--status-red-muted)]/55';
		if (level === 'WARN')
			return 'bg-[var(--status-amber-muted)]/25 hover:bg-[var(--status-amber-muted)]/45';
		return 'hover:bg-[var(--bg-table-row-hover)]';
	}
</script>

<div
	bind:this={viewport}
	style={containerStyle}
	class="relative w-full overflow-auto rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-terminal)] p-3 text-[12px] leading-[1.6] font-[var(--font-mono)] text-[var(--text-primary)] select-text {containerClass}"
>
	{#if headerSlot}
		{@render headerSlot()}
	{/if}

	{#if loading && entries.length === 0}
		<div
			class="flex min-h-[180px] flex-col items-center justify-center gap-2 text-center text-xs text-[var(--text-tertiary)]"
		>
			<div
				class="h-4 w-4 animate-spin rounded-full border-2 border-[var(--accent)] border-t-transparent"
			></div>
			<span class="text-xs">{loadingMessage}</span>
		</div>
	{:else if error}
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--status-red)]/30 bg-[var(--status-red-muted)] p-3 text-xs text-[var(--status-red)]"
		>
			{error}
		</div>
	{:else if entries.length === 0}
		<div
			class="flex min-h-[180px] flex-col items-center justify-center gap-2 text-center text-xs text-[var(--text-tertiary)]"
		>
			<span class="text-xs">{emptyMessage}</span>
		</div>
	{:else}
		<div class="flex flex-col {wrapLines ? '' : 'min-w-max'}">
			{#each entries as log (log.id)}
				<div
					class="group flex items-baseline gap-2.5 rounded-[var(--radius-sm)] px-2 py-[3px] transition-colors {rowAccentClass(
						log.level
					)}"
				>
					{#if log.time}
						<span
							class="w-[82px] shrink-0 font-mono text-[11px] whitespace-nowrap text-[var(--text-tertiary)] tabular-nums select-none"
						>
							{log.time}
						</span>
					{/if}

					{#if showContainer}
						<span
							class="max-w-[130px] shrink-0 truncate text-[11px] font-medium text-[var(--accent-text)] select-none"
							title={log.container}
						>
							[{log.container}]
						</span>
					{/if}

					<span
						class="inline-flex min-w-[44px] shrink-0 items-center justify-center rounded border px-1 py-0 text-[10px] font-semibold uppercase select-none {levelBadgeClass(
							log.level
						)}"
					>
						{log.level}
					</span>

					<div
						class="min-w-0 flex-1 {wrapLines
							? 'break-words whitespace-pre-wrap'
							: 'whitespace-pre'}"
					>
						{#if prettyJson && log.prettyJson}
							<pre
								class="m-0 font-mono text-[11.5px] leading-[1.5] text-[var(--code-value)]">{log.prettyJson}</pre>
						{:else}
							{#each log.spans as span}
								<span
									class="{span.colorClass || 'text-[var(--text-secondary)]'} {span.bold
										? 'font-semibold text-[var(--text-primary)]'
										: ''} {span.dim
										? 'opacity-70'
										: ''} selection:bg-[var(--accent)] selection:text-white">{span.text}</span
								>
							{/each}
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
