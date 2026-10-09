<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { FileCode, Sparkle, Browser, Check } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let buildType = $state<'dockerfile' | 'nixpacks' | 'static'>('dockerfile');
	let dockerfilePath = $state('./Dockerfile');
	let publishDir = $state('dist');
	let saveStatus = $state<'idle' | 'saved'>('idle');

	$effect(() => {
		buildType = service.buildType || 'dockerfile';
		dockerfilePath = service.dockerfilePath || './Dockerfile';
	});

	function handleSave() {
		service.buildType = buildType;
		service.dockerfilePath = dockerfilePath.trim();
		dataStore.updateService(service);
		saveStatus = 'saved';
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}
</script>

<div
	class="flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]"
>
	<!-- Card Header -->
	<div class="flex items-center justify-between border-b border-[var(--border-subtle)] px-5 py-4">
		<div class="flex flex-col gap-0.5">
			<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Build Type</h3>
			<p class="m-0 text-xs text-[var(--text-tertiary)]">
				Choose how Podman constructs the container image for this service.
			</p>
		</div>
	</div>

	<!-- Build Type Choices -->
	<div class="flex flex-col gap-5 p-5">
		<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-3">
			<button
				type="button"
				onclick={() => (buildType = 'dockerfile')}
				class="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border p-3 text-left transition-all {buildType ===
				'dockerfile'
					? 'border-[var(--accent)] bg-[var(--bg-surface)] text-[var(--text-primary)]'
					: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<FileCode size={18} class={buildType === 'dockerfile' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Dockerfile</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Standard OCI build</span>
				</div>
			</button>

			<button
				type="button"
				onclick={() => (buildType = 'nixpacks')}
				class="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border p-3 text-left transition-all {buildType ===
				'nixpacks'
					? 'border-[var(--accent)] bg-[var(--bg-surface)] text-[var(--text-primary)]'
					: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<Sparkle size={18} class={buildType === 'nixpacks' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Nixpacks</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Zero-config auto detect</span>
				</div>
			</button>

			<button
				type="button"
				onclick={() => (buildType = 'static')}
				class="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border p-3 text-left transition-all {buildType ===
				'static'
					? 'border-[var(--accent)] bg-[var(--bg-surface)] text-[var(--text-primary)]'
					: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<Browser size={18} class={buildType === 'static' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Static Site (SPA)</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Vite, Svelte, React</span>
				</div>
			</button>
		</div>

		<!-- Options per build type -->
		{#if buildType === 'dockerfile'}
			<div class="flex flex-col gap-1.5">
				<label for="bld-dockerfile-path" class="text-xs font-medium text-[var(--text-secondary)]">
					Dockerfile Path
				</label>
				<Input
					id="bld-dockerfile-path"
					bind:value={dockerfilePath}
					placeholder="./Dockerfile"
					class="text-xs font-[var(--font-mono)]"
				/>
				<span class="text-[11px] text-[var(--text-tertiary)]"
					>Relative to your repository build path.</span
				>
			</div>
		{:else if buildType === 'nixpacks'}
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)]/60 p-3.5 text-xs leading-relaxed text-[var(--text-secondary)]"
			>
				Nixpacks automatically detects your project language (Node.js, Go, Python, Rust, PHP, Ruby)
				and generates an optimized container image without needing a Dockerfile.
			</div>
		{:else}
			<div class="flex flex-col gap-1.5">
				<label for="bld-publish-dir" class="text-xs font-medium text-[var(--text-secondary)]"
					>Publish Directory</label
				>
				<Input
					id="bld-publish-dir"
					bind:value={publishDir}
					placeholder="dist or build or public"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
		{/if}
	</div>

	<!-- Explicit Card Footer -->
	<div
		class="flex items-center justify-end gap-2 border-t border-[var(--border)] bg-[var(--bg-panel)] px-5 py-3"
	>
		{#if saveStatus === 'saved'}
			<span class="mr-2 flex items-center gap-1 text-xs text-[var(--status-green)]">
				<Check size={14} /> Saved
			</span>
		{/if}
		<Button variant="primary" size="sm" onclick={handleSave}>Save Build Settings</Button>
	</div>
</div>
