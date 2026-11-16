<script lang="ts">
	import { SectionCard, Input } from '$lib/components/ui';

	interface Props {
		open?: boolean;
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
		open = $bindable(true),
		cpuText = $bindable(),
		cpuError = '',
		memText = $bindable(),
		memError = '',
		pidsText = $bindable(),
		pidsError = '',
		swapText = $bindable(),
		swapError = ''
	}: Props = $props();

	// Compact header summary of the currently set limits.
	let summary = $derived.by(() => {
		const parts: string[] = [];
		if (cpuText.trim()) parts.push(`cpu ${cpuText.trim()}`);
		if (memText.trim()) parts.push(`mem ${memText.trim()}`);
		if (swapText.trim()) parts.push(`swap ${swapText.trim()}`);
		if (pidsText.trim()) parts.push(`pids ${pidsText.trim()}`);
		return parts.length ? parts.join(' · ') : 'Unlimited';
	});
</script>

<SectionCard
	title="Resource Limits"
	description="CPU, memory, swap, and process count enforced by cgroups v2. Empty fields mean unlimited."
	collapsible={true}
	bind:open
>
	{#snippet headerActions()}
		<span class="max-w-[240px] truncate font-mono text-[11px] text-[var(--text-tertiary)]"
			>{summary}</span
		>
	{/snippet}

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<Input
			id="res-cpu"
			label="CPU limit (cores)"
			bind:value={cpuText}
			placeholder="e.g. 1.5"
			error={cpuError || undefined}
			class="font-mono text-xs"
		/>

		<Input
			id="res-mem"
			label="Memory limit (MB)"
			bind:value={memText}
			placeholder="e.g. 512"
			error={memError || undefined}
			class="font-mono text-xs"
		/>

		<Input
			id="res-swap"
			label="Memory swap"
			description="Capped against the memory limit; unlimited disables swap capping."
			bind:value={swapText}
			placeholder="e.g. 512MB or unlimited"
			error={swapError || undefined}
			class="font-mono text-xs"
		/>

		<Input
			id="res-pids"
			label="Process count limit"
			bind:value={pidsText}
			placeholder="e.g. 500"
			error={pidsError || undefined}
			class="font-mono text-xs"
		/>
	</div>
</SectionCard>
