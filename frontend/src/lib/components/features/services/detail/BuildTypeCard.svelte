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

<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col">
	<!-- Card Header -->
	<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex items-center justify-between">
		<div class="flex flex-col gap-0.5">
			<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Build Type</h3>
			<p class="text-xs text-[var(--text-tertiary)] m-0">Choose how Podman constructs the container image for this service.</p>
		</div>
	</div>

	<!-- Build Type Choices -->
	<div class="p-5 flex flex-col gap-5">
		<div class="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
			<button
				type="button"
				onclick={() => (buildType = 'dockerfile')}
				class="flex items-center gap-3 p-3 rounded-[var(--radius-sm)] border text-left cursor-pointer transition-all {buildType === 'dockerfile'
					? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)]'
					: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
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
				class="flex items-center gap-3 p-3 rounded-[var(--radius-sm)] border text-left cursor-pointer transition-all {buildType === 'nixpacks'
					? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)]'
					: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
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
				class="flex items-center gap-3 p-3 rounded-[var(--radius-sm)] border text-left cursor-pointer transition-all {buildType === 'static'
					? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)]'
					: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
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
				<span class="text-[11px] text-[var(--text-tertiary)]">Relative to your repository build path.</span>
			</div>
		{:else if buildType === 'nixpacks'}
			<div class="p-3.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)]/60 text-xs text-[var(--text-secondary)] leading-relaxed">
				Nixpacks automatically detects your project language (Node.js, Go, Python, Rust, PHP, Ruby) and generates an optimized container image without needing a Dockerfile.
			</div>
		{:else}
			<div class="flex flex-col gap-1.5">
				<label for="bld-publish-dir" class="text-xs font-medium text-[var(--text-secondary)]">Publish Directory</label>
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
	<div class="px-5 py-3 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-end gap-2">
		{#if saveStatus === 'saved'}
			<span class="text-xs text-[var(--status-green)] flex items-center gap-1 mr-2">
				<Check size={14} /> Saved
			</span>
		{/if}
		<Button variant="primary" size="sm" onclick={handleSave}>
			Save Build Settings
		</Button>
	</div>
</div>
