<script lang="ts">
	import { Input, Button } from '$lib/components/primitives';
	import { Plus, Trash } from 'phosphor-svelte';

	interface ContainerEntry {
		name: string;
		image: string;
	}

	interface Props {
		name: string;
		description: string;
		containers: ContainerEntry[];
	}

	let {
		name = $bindable(''),
		description = $bindable(''),
		containers = $bindable([
			{ name: 'app', image: 'quay.io/lib/app:latest' },
			{ name: 'redis', image: 'redis:7-alpine' }
		])
	}: Props = $props();

	function addContainer() {
		containers.push({
			name: `worker-${containers.length + 1}`,
			image: ''
		});
	}

	function removeContainer(index: number) {
		if (containers.length > 1) {
			containers.splice(index, 1);
		}
	}
</script>

<div class="flex flex-col gap-5">
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<div class="flex flex-col gap-1.5">
			<label for="pod-svc-name" class="text-xs font-medium text-[var(--text-secondary)]">
				Pod name <span class="text-[var(--status-red)]">*</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input id="pod-svc-name" bind:value={name} placeholder="e.g. realtime-cluster" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="pod-svc-desc" class="text-xs font-medium text-[var(--text-secondary)]">
				Description <span class="text-[var(--text-tertiary)]">(optional)</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input
					id="pod-svc-desc"
					bind:value={description}
					placeholder="Podman Pod grouping co-located containers"
				/>
			</div>
		</div>
	</div>

	<!-- Containers List -->
	<div class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Member Containers</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Define initial containers in this Pod. Detailed runtime, volumes, and ports can be
					configured in Service Detail.
				</span>
			</div>
			<Button variant="secondary" size="sm" onclick={addContainer}>
				<Plus size={13} /> Add Container
			</Button>
		</div>

		<div class="flex flex-col gap-2">
			{#each containers as c, idx}
				<div
					class="flex items-center gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3"
				>
					<div class="grid flex-1 grid-cols-1 gap-3 sm:grid-cols-2">
						<div class="flex flex-col gap-1">
							<span
								class="text-[10px] font-medium tracking-[0.08em] text-[var(--text-tertiary)] uppercase"
								>Container Name</span
							>
							<Input
								bind:value={c.name}
								placeholder="e.g. app"
								class="text-xs font-[var(--font-mono)]"
							/>
						</div>
						<div class="flex flex-col gap-1">
							<span
								class="text-[10px] font-medium tracking-[0.08em] text-[var(--text-tertiary)] uppercase"
								>Image Reference</span
							>
							<Input
								bind:value={c.image}
								placeholder="image:tag"
								class="text-xs font-[var(--font-mono)]"
							/>
						</div>
					</div>

					{#if containers.length > 1}
						<button
							type="button"
							onclick={() => removeContainer(idx)}
							class="cursor-pointer border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:text-[var(--status-red)]"
							title="Remove container"
						>
							<Trash size={15} />
						</button>
					{/if}
				</div>
			{/each}
		</div>
	</div>
</div>
