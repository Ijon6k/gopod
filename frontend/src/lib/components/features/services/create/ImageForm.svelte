<script lang="ts">
	import { Input } from '$lib/components/primitives';

	interface Props {
		name: string;
		description: string;
		image: string;
		command: string;
	}

	let {
		name = $bindable(''),
		description = $bindable(''),
		image = $bindable(''),
		command = $bindable('')
	}: Props = $props();
</script>

<div class="flex flex-col gap-4">
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
		<div class="flex flex-col gap-1.5">
			<label for="img-svc-name" class="text-xs text-[var(--text-secondary)] font-medium">
				Service name <span class="text-[var(--status-red)]">*</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="img-svc-name" bind:value={name} placeholder="e.g. redis-cache" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="img-svc-desc" class="text-xs text-[var(--text-secondary)] font-medium">
				Description <span class="text-[var(--text-tertiary)]">(optional)</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="img-svc-desc" bind:value={description} placeholder="In-memory cache or worker" />
			</div>
		</div>
	</div>

	<div class="flex flex-col gap-1.5">
		<label for="img-reference" class="text-xs text-[var(--text-secondary)] font-medium">
			Image reference <span class="text-[var(--status-red)]">*</span>
		</label>
		<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
			<Input
				id="img-reference"
				bind:value={image}
				placeholder="docker.io/library/redis:7-alpine or ghcr.io/org/repo:tag"
				class="font-[var(--font-mono)] text-xs"
			/>
		</div>
		<span class="text-[11px] text-[var(--text-tertiary)]">
			Supports Docker Hub, Quay.io, GitHub Container Registry (ghcr.io), or private registries.
		</span>
	</div>

	<div class="flex flex-col gap-1.5">
		<label for="img-command" class="text-xs text-[var(--text-secondary)] font-medium">
			Command / Entrypoint override <span class="text-[var(--text-tertiary)]">(optional)</span>
		</label>
		<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
			<Input
				id="img-command"
				bind:value={command}
				placeholder="e.g. redis-server --appendonly yes"
				class="font-[var(--font-mono)] text-xs"
			/>
		</div>
	</div>
</div>
