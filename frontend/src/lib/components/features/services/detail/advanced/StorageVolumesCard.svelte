<script lang="ts">
	import { SectionCard, Button, Input } from '$lib/components/ui';
	import { Plus, Trash } from 'phosphor-svelte';

	interface Props {
		open?: boolean;
		serviceName: string;
		volumes: { source: string; target: string; options: string }[];
		/** Per-row validation message ('' when the row is valid). */
		volumeErrors?: string[];
	}

	let {
		open = $bindable(true),
		serviceName,
		volumes = $bindable(),
		volumeErrors = []
	}: Props = $props();

	function addVolume() {
		volumes.push({
			source: `${serviceName}_vol_${volumes.length + 1}`,
			target: '/data',
			options: 'Z'
		});
	}

	function removeVolume(idx: number) {
		volumes.splice(idx, 1);
	}

	let summary = $derived(
		volumes.length === 0
			? 'None — data is ephemeral'
			: `${volumes.length} mount${volumes.length === 1 ? '' : 's'}`
	);
</script>

<SectionCard
	title="Storage & Volumes"
	description="Named volumes and host bind mounts. Flags apply volume options such as Z, z, or ro."
	collapsible={true}
	bind:open
>
	{#snippet headerActions()}
		<span class="max-w-[240px] truncate font-mono text-[11px] text-[var(--text-tertiary)]"
			>{summary}</span
		>
	{/snippet}

	<div class="flex items-center justify-between">
		<span class="text-xs font-medium text-[var(--text-secondary)]">Volume mounts</span>
		<Button variant="secondary" size="sm" onclick={addVolume}>
			<Plus size={13} /> Add mount
		</Button>
	</div>

	{#if volumes.length === 0}
		<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
			No volumes mounted. Files written inside the container are lost when it is removed or
			redeployed — add a mount for anything that must survive.
		</p>
	{:else}
		<!-- Header row: column labels render once instead of per row -->
		<div class="hidden grid-cols-[1fr_1fr_10rem] gap-3 px-1 sm:grid">
			<span class="text-[11px] font-medium text-[var(--text-tertiary)]">Source / volume</span>
			<span class="text-[11px] font-medium text-[var(--text-tertiary)]">Container path</span>
			<span class="text-[11px] font-medium text-[var(--text-tertiary)]">Flags</span>
		</div>

		{#each volumes as vol, idx}
			<div class="grid grid-cols-1 items-start gap-3 sm:grid-cols-[1fr_1fr_10rem]">
				<Input
					bind:value={vol.source}
					placeholder="volume_name"
					aria-label={`Mount ${idx + 1} source`}
					class="font-mono text-xs"
				/>
				<Input
					bind:value={vol.target}
					placeholder="/data"
					aria-label={`Mount ${idx + 1} container path`}
					class="font-mono text-xs"
				/>
				<div class="flex items-center gap-2">
					<Input
						bind:value={vol.options}
						placeholder="Z"
						aria-label={`Mount ${idx + 1} flags`}
						class="font-mono text-xs"
					/>
					<button
						type="button"
						onclick={() => removeVolume(idx)}
						aria-label={`Remove mount ${idx + 1}`}
						title="Remove mount"
						class="cursor-pointer border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] hover:text-[var(--status-red)]"
					>
						<Trash size={14} />
					</button>
				</div>
				{#if volumeErrors[idx]}
					<p class="m-0 text-[11px] text-[var(--status-red)] sm:col-span-3">{volumeErrors[idx]}</p>
				{/if}
			</div>
		{/each}
	{/if}
</SectionCard>
