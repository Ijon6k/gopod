<script lang="ts">
	import { SectionCard, Input } from '$lib/components/ui';

	interface Props {
		openResources?: boolean;
		cpuText: string;
		cpuError?: string;
		memText: string;
		memError?: string;
		pidsText: string;
		pidsError?: string;
		swapText: string;
		swapError?: string;
	}

	let {
		openResources = $bindable(true),
		cpuText = $bindable(),
		cpuError = '',
		memText = $bindable(),
		memError = '',
		pidsText = $bindable(),
		pidsError = '',
		swapText = $bindable(),
		swapError = ''
	}: Props = $props();
</script>

<SectionCard
	title="Resource Limits"
	description="Cgroups v2 limits enforced by the deploy engine: CPU cores, memory, swap, and process count."
	collapsible={true}
	bind:open={openResources}
>
	<div class="grid grid-cols-1 gap-4 pt-2 sm:grid-cols-2">
		<div class="flex flex-col gap-1.5">
			<label for="res-cpu" class="text-xs font-medium text-[var(--text-secondary)]"
				>CPU limit (cores)</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="res-cpu"
					bind:value={cpuText}
					placeholder="unlimited — e.g. 1.5"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			{#if cpuError}
				<span class="text-[11px] text-[var(--status-red)]">{cpuError}</span>
			{:else}
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Emits <code>--cpus</code> on the runtime path and <code>PodmanArgs=--cpus=…</code> in quadlet
					units.
				</span>
			{/if}
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="res-mem" class="text-xs font-medium text-[var(--text-secondary)]"
				>Memory limit (MB)</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="res-mem"
					bind:value={memText}
					placeholder="unlimited — e.g. 512"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			{#if memError}
				<span class="text-[11px] text-[var(--status-red)]">{memError}</span>
			{:else}
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Emits <code>--memory</code> on the runtime path and <code>PodmanArgs=--memory=…</code> in quadlet
					units.
				</span>
			{/if}
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="res-swap" class="text-xs font-medium text-[var(--text-secondary)]"
				>Memory swap</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="res-swap"
					bind:value={swapText}
					placeholder="unbounded — e.g. 512MB or unlimited"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			{#if swapError}
				<span class="text-[11px] text-[var(--status-red)]">{swapError}</span>
			{:else}
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Emits <code>--memory-swap</code>. Needs a memory limit; <code>unlimited</code> disables swap
					capping.
				</span>
			{/if}
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="res-pids" class="text-xs font-medium text-[var(--text-secondary)]"
				>Process count limit</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="res-pids"
					bind:value={pidsText}
					placeholder="unlimited — e.g. 500"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			{#if pidsError}
				<span class="text-[11px] text-[var(--status-red)]">{pidsError}</span>
			{:else}
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Emits <code>--pids-limit</code> on the runtime path and <code>PidsLimit=</code> in quadlet units.
				</span>
			{/if}
		</div>
	</div>
</SectionCard>
