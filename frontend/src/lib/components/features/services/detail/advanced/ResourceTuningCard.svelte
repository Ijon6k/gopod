<script lang="ts">
	import { SectionCard, Input } from '$lib/components/ui';

	interface Props {
		openResources?: boolean;
		cpuLimit: string;
		memoryLimit: string;
		pidsLimit: string;
		swapLimit: string;
	}

	let {
		openResources = $bindable(true),
		cpuLimit = $bindable(),
		memoryLimit = $bindable(),
		pidsLimit = $bindable(),
		swapLimit = $bindable()
	}: Props = $props();
</script>

<SectionCard
	title="Resource Limits"
	description="Cgroups v2 limits for CPU cores, memory limits, and maximum process IDs."
	collapsible={true}
	bind:open={openResources}
>
	<div class="grid grid-cols-2 gap-4 pt-2 sm:grid-cols-4">
		<div class="flex flex-col gap-1.5">
			<label for="res-cpu" class="text-xs font-medium text-[var(--text-secondary)]">CPU Quota</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="res-cpu"
					bind:value={cpuLimit}
					placeholder="2.0"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="res-mem" class="text-xs font-medium text-[var(--text-secondary)]"
				>Memory Limit</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="res-mem"
					bind:value={memoryLimit}
					placeholder="512MB"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="res-pids" class="text-xs font-medium text-[var(--text-secondary)]"
				>PIDs Limit</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="res-pids"
					bind:value={pidsLimit}
					placeholder="2048"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="res-swap" class="text-xs font-medium text-[var(--text-secondary)]"
				>Swap Limit</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="res-swap"
					bind:value={swapLimit}
					placeholder="0"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
		</div>
	</div>
</SectionCard>
