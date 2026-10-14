<script lang="ts">
	import { SectionCard, Button, Input } from '$lib/components/ui';
	import { Plus, Trash } from 'phosphor-svelte';

	interface Props {
		openStorage?: boolean;
		serviceName: string;
		volumes: { source: string; target: string; options: string }[];
	}

	let { openStorage = $bindable(true), serviceName, volumes = $bindable() }: Props = $props();

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
</script>

<SectionCard
	title="Storage & Volumes"
	description="Persistent named volumes, host bind mounts, and SELinux volume options."
	collapsible={true}
	bind:open={openStorage}
>
	<div class="flex items-center justify-between pt-1">
		<span class="text-xs font-medium text-[var(--text-secondary)]">Configured Volume Mounts</span>
		<Button variant="secondary" size="sm" onclick={addVolume}>
			<Plus size={13} /> Add Mount
		</Button>
	</div>

	<div
		class="flex flex-col divide-y divide-[var(--border-subtle)] overflow-hidden rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)]"
	>
		{#each volumes as vol, idx}
			<div class="grid grid-cols-1 items-center gap-3 p-3 sm:grid-cols-3">
				<div class="flex flex-col gap-1">
					<span class="text-[10px] font-medium text-[var(--text-tertiary)] uppercase"
						>Source / Volume</span
					>
					<Input
						bind:value={vol.source}
						placeholder="volume_name"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
				<div class="flex flex-col gap-1">
					<span class="text-[10px] font-medium text-[var(--text-tertiary)] uppercase"
						>Container Mount</span
					>
					<Input
						bind:value={vol.target}
						placeholder="/data"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
				<div class="flex items-center gap-2">
					<div class="flex flex-1 flex-col gap-1">
						<span class="text-[10px] font-medium text-[var(--text-tertiary)] uppercase">Flags</span>
						<Input
							bind:value={vol.options}
							placeholder="Z"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
					<button
						type="button"
						onclick={() => removeVolume(idx)}
						class="mt-4 cursor-pointer border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] hover:text-[var(--status-red)]"
					>
						<Trash size={14} />
					</button>
				</div>
			</div>
		{/each}
	</div>
</SectionCard>
