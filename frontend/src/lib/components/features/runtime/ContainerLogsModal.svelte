<script lang="ts">
	import { untrack } from 'svelte';
	import { Button, Select, Modal, SearchInput } from '$lib/components/ui';
	import type { Container } from '$lib/types';
	import { runtimeApi } from '$lib/api/runtime/api';
	import { parseFormattedLogLines, stripAnsi, type LogLevel } from '$lib/utils/log-format';
	import FormattedLogView from './FormattedLogView.svelte';
	import { ArrowClockwise, Copy } from 'phosphor-svelte';

	interface Props {
		/** The container whose logs to show; null keeps the dialog closed. */
		container: Container | null;
		onclose: () => void;
	}

	let { container, onclose }: Props = $props();

	let logsTail = $state(100);
	let showTimestamps = $state(false);
	let wrapLines = $state(true);
	let prettyJson = $state(false);
	let selectedLevel = $state<'ALL' | LogLevel>('ALL');
	let searchQuery = $state('');
	let logsText = $state('');
	let logsError = $state('');
	let isLogsLoading = $state(false);
	let copyFeedback = $state(false);
	let logsRequestSeq = $state(0);

	const tailOptions = [
		{ label: 'Last 50', value: '50' },
		{ label: 'Last 100', value: '100' },
		{ label: 'Last 500', value: '500' },
		{ label: 'Last 1000', value: '1000' }
	];

	function fetchContainerLogs() {
		if (!container) return;
		const seq = untrack(() => ++logsRequestSeq);
		const target = container;
		const tail = logsTail;
		const withTs = showTimestamps;
		logsError = '';
		isLogsLoading = true;
		runtimeApi.containers
			.logs(target.id, tail, withTs)
			.then((res) => {
				if (seq === logsRequestSeq) logsText = res.logs || '';
			})
			.catch((err: any) => {
				if (seq === logsRequestSeq) {
					logsText = '';
					logsError =
						err?.message || 'Failed to load container logs (the container may have been removed)';
				}
			})
			.finally(() => {
				if (seq === logsRequestSeq) isLogsLoading = false;
			});
	}

	$effect(() => {
		if (!container) return;
		// Track container, logsTail, and showTimestamps cleanly.
		void container.id;
		void logsTail;
		void showTimestamps;
		fetchContainerLogs();
	});

	let parsedEntries = $derived.by(() =>
		parseFormattedLogLines(logsText, container?.name || 'container')
	);

	let filteredEntries = $derived.by(() => {
		let list = parsedEntries;
		if (selectedLevel !== 'ALL') {
			list = list.filter((e) => e.level === selectedLevel);
		}
		if (searchQuery.trim()) {
			const q = searchQuery.toLowerCase().trim();
			list = list.filter((e) => e.text.toLowerCase().includes(q));
		}
		return list;
	});

	async function handleCopy() {
		const clean = stripAnsi(logsText);
		if (!clean) return;
		try {
			await navigator.clipboard.writeText(clean);
			copyFeedback = true;
			setTimeout(() => (copyFeedback = false), 2000);
		} catch {
			// ignore clipboard permission errors on insecure contexts
		}
	}
</script>

<Modal
	open={!!container}
	{onclose}
	title={container?.name ? `Container logs — ${container.name}` : 'Container logs'}
	subtitle={`Last ${logsTail} lines from ${container?.image ?? ''}`}
	size="xl"
>
	<div class="flex flex-col gap-3">
		<!-- Toolbar -->
		<div
			class="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--border)] pb-2.5"
		>
			<div class="flex flex-wrap items-center gap-2">
				<div class="w-[120px]">
					<Select
						value={String(logsTail)}
						onValueChange={(val: any) => {
							if (val) logsTail = Number(val);
						}}
						options={tailOptions}
						size="xs"
					/>
				</div>

				<div
					class="inline-flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
				>
					{#each [
						{ value: 'ALL', label: 'All' },
						{ value: 'INFO', label: 'Info' },
						{ value: 'WARN', label: 'Warn' },
						{ value: 'ERROR', label: 'Error' }
					] as lvl}
						<button
							type="button"
							onclick={() => (selectedLevel = lvl.value as any)}
							class="cursor-pointer rounded border-0 px-2.5 py-0.5 text-xs transition-colors {selectedLevel ===
							lvl.value
								? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)] shadow-xs'
								: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						>
							{lvl.label}
						</button>
					{/each}
				</div>

				<div class="w-44 sm:w-52">
					<SearchInput bind:value={searchQuery} placeholder="Filter output…" />
				</div>
			</div>

			<div class="flex flex-wrap items-center gap-1.5">
				<button
					type="button"
					onclick={() => (showTimestamps = !showTimestamps)}
					aria-pressed={showTimestamps}
					class="cursor-pointer rounded-[var(--radius-sm)] border px-2 py-1 text-xs font-medium transition-colors {showTimestamps
						? 'border-[var(--accent)]/40 bg-[var(--accent-muted)] text-[var(--accent-text)]'
						: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Timestamps
				</button>

				<button
					type="button"
					onclick={() => (wrapLines = !wrapLines)}
					aria-pressed={wrapLines}
					class="cursor-pointer rounded-[var(--radius-sm)] border px-2 py-1 text-xs font-medium transition-colors {wrapLines
						? 'border-[var(--accent)]/40 bg-[var(--accent-muted)] text-[var(--accent-text)]'
						: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Wrap
				</button>

				<button
					type="button"
					onclick={() => (prettyJson = !prettyJson)}
					aria-pressed={prettyJson}
					class="cursor-pointer rounded-[var(--radius-sm)] border px-2 py-1 text-xs font-medium transition-colors {prettyJson
						? 'border-[var(--accent)]/40 bg-[var(--accent-muted)] text-[var(--accent-text)]'
						: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Pretty JSON
				</button>

				<Button
					variant="ghost"
					size="sm"
					onclick={fetchContainerLogs}
					disabled={isLogsLoading}
					aria-label="Reload container logs"
				>
					<ArrowClockwise class="mr-1 h-3.5 w-3.5 {isLogsLoading ? 'animate-spin' : ''}" />
					<span>Refresh</span>
				</Button>

				<Button
					variant="ghost"
					size="sm"
					onclick={handleCopy}
					disabled={!logsText}
					aria-label="Copy logs"
				>
					<Copy class="mr-1 h-3.5 w-3.5" />
					<span>{copyFeedback ? 'Copied!' : 'Copy'}</span>
				</Button>
			</div>
		</div>

		<FormattedLogView
			entries={filteredEntries}
			loading={isLogsLoading}
			error={logsError}
			{wrapLines}
			{prettyJson}
			emptyMessage={searchQuery
				? `No log lines matching "${searchQuery}"`
				: 'No log output for this container and tail setting.'}
			containerStyle="height: 440px;"
		/>

		<div class="flex items-center justify-between text-xs text-[var(--text-tertiary)]">
			<span class="tabular-nums">{filteredEntries.length} of {parsedEntries.length} lines shown</span>
			<Button variant="secondary" onclick={onclose}>Close</Button>
		</div>
	</div>
</Modal>
