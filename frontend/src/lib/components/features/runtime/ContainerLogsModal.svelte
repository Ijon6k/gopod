<script lang="ts">
	import { untrack } from 'svelte';
	import { SkeletonLine } from 'kumo-svelte';
	import { Button, Select, Modal } from '$lib/components/ui';
	import type { Container } from '$lib/types';
	import { runtimeApi } from '$lib/api/runtime/api';

	interface Props {
		/** The container whose logs to show; null keeps the dialog closed. */
		container: Container | null;
		onclose: () => void;
	}

	let { container, onclose }: Props = $props();

	let logsTail = $state(100);
	let logsText = $state('');
	let logsError = $state('');
	let isLogsLoading = $state(false);
	let logsRequestSeq = $state(0);

	const tailOptions = [
		{ label: 'Last 50', value: '50' },
		{ label: 'Last 100', value: '100' },
		{ label: 'Last 500', value: '500' },
		{ label: 'Last 1000', value: '1000' }
	];

	$effect(() => {
		if (!container) return;
		// The sequence counter is bumped untracked: reading it back would turn
		// this effect into its own trigger and reload the log forever.
		const seq = untrack(() => ++logsRequestSeq);
		const target = container;
		const tail = logsTail;
		logsError = '';
		isLogsLoading = true;
		runtimeApi.containers
			.logs(target.id, tail)
			.then((res) => {
				if (seq === logsRequestSeq) logsText = res.logs;
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
	});
</script>

<Modal
	open={!!container}
	{onclose}
	title={container?.name ? `Container logs — ${container.name}` : 'Container logs'}
	subtitle={`Last ${logsTail} lines from ${container?.image ?? ''}`}
	size="lg"
>
	<div class="flex flex-col gap-3">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="w-[140px]">
				<Select
					value={String(logsTail)}
					onValueChange={(val: any) => {
						if (val) logsTail = Number(val);
					}}
					options={tailOptions}
					size="xs"
				/>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]"
				>Fetched on open; select a tail to reload</span
			>
		</div>
		{#if isLogsLoading}
			<div
				class="space-y-2 rounded border border-[var(--border-subtle)] bg-[var(--bg-terminal)] p-3"
			>
				<SkeletonLine />
				<SkeletonLine />
				<SkeletonLine minWidth={60} maxWidth={70} />
			</div>
		{:else if logsError}
			<div
				class="rounded border border-[var(--border-subtle)] bg-[var(--bg-terminal)] p-3 text-xs text-[var(--status-red)]"
			>
				{logsError}
			</div>
		{:else}
			<pre
				class="m-0 max-h-[420px] overflow-auto rounded border border-[var(--border-subtle)] bg-[var(--bg-terminal)] p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap text-[var(--text-primary)]">{logsText ||
					'No log output for this container and tail setting.'}</pre>
		{/if}
		<div class="flex justify-end">
			<Button variant="secondary" onclick={onclose}>Close</Button>
		</div>
	</div>
</Modal>
