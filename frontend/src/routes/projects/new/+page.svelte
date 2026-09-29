<script lang="ts">
	import { goto } from '$app/navigation';
	import { PageHeader } from '$lib/components/ui';
	import { Button, Input } from '$lib/components/primitives';
	import { Plus } from 'phosphor-svelte';

	let name = $state('');
	let description = $state('');
</script>

<svelte:head>
	<title>New Project — GOPOD</title>
</svelte:head>

<div class="w-full max-w-[640px]">
	<PageHeader title="New Project" subtitle="Create a new project workspace." />

	<div class="flex flex-col gap-6">
		<div class="flex flex-col gap-2">
			<label class="text-xs text-[var(--text-secondary)] font-medium" for="project-name">Project name</label>
			<div class="px-3 py-2.5 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input bind:value={name} placeholder="my-project" />
			</div>
		</div>

		<div class="flex flex-col gap-2">
			<label class="text-xs text-[var(--text-secondary)] font-medium" for="project-desc">Description</label>
			<div class="px-3 py-2.5 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input bind:value={description} placeholder="A short description for your project" />
			</div>
		</div>

		<div class="flex gap-3 pt-2">
			<Button variant="ghost" onclick={() => goto('/projects')}>Cancel</Button>
			<Button variant="primary" disabled={!name.trim()}>
				<Plus size={13} /> Create project
			</Button>
		</div>
	</div>
</div>
