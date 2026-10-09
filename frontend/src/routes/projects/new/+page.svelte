<script lang="ts">
	import { goto } from '$app/navigation';
	import { PageHeader } from '$lib/components/ui';
	import { Button, Input } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { Plus, ArrowClockwise } from 'phosphor-svelte';

	import { projectSchema } from '$lib/schemas';

	let name = $state('');
	let description = $state('');
	let isSubmitting = $state(false);
	let error = $state<string | null>(null);

	async function handleSubmit(e?: Event) {
		if (e) e.preventDefault();
		const trimmedName = name.trim();
		if (!trimmedName || isSubmitting) return;

		const validation = projectSchema.safeParse({
			name: trimmedName,
			description: description.trim() || undefined
		});

		if (!validation.success) {
			error = validation.error.issues[0]?.message || 'Invalid project data';
			return;
		}

		isSubmitting = true;
		error = null;
		try {
			const project = await dataStore.createProject(validation.data);
			goto(`/projects/${project.id}`);
		} catch (err: unknown) {
			console.error('Failed to create project:', err);
			const errObj = err as { response?: { data?: { error?: string } }; message?: string };
			error =
				errObj?.response?.data?.error ||
				errObj?.message ||
				'Failed to create project. Please try again.';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>New Project — GOPOD</title>
</svelte:head>

<div class="w-full max-w-[640px]">
	<PageHeader
		title="New Project"
		subtitle="Create a new logical workspace for your services and workloads."
	/>

	<form onsubmit={handleSubmit} class="flex flex-col gap-6">
		{#if error}
			<div
				class="rounded border border-[var(--status-red)]/30 bg-[var(--status-red-muted)] p-3 text-xs text-[var(--status-red)]"
			>
				{error}
			</div>
		{/if}

		<div class="flex flex-col gap-2">
			<label class="text-xs font-medium text-[var(--text-secondary)]" for="project-name"
				>Project name</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2.5 focus-within:border-[var(--accent)]"
			>
				<Input
					id="project-name"
					bind:value={name}
					placeholder="e.g. backend-api, my-workspace"
					required
				/>
			</div>
			<p class="text-[11px] text-[var(--text-tertiary)]">
				A descriptive name to identify your project.
			</p>
		</div>

		<div class="flex flex-col gap-2">
			<label class="text-xs font-medium text-[var(--text-secondary)]" for="project-desc"
				>Description</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2.5 focus-within:border-[var(--accent)]"
			>
				<Input
					id="project-desc"
					bind:value={description}
					placeholder="A short description for your project"
				/>
			</div>
			<p class="text-[11px] text-[var(--text-tertiary)]">
				Optional purpose or notes for this workspace.
			</p>
		</div>

		<div class="flex gap-3 pt-2">
			<Button type="button" variant="ghost" onclick={() => goto('/projects')}>Cancel</Button>
			<Button type="submit" variant="primary" disabled={!name.trim() || isSubmitting}>
				{#if isSubmitting}
					<ArrowClockwise class="animate-spin" size={13} /> Creating...
				{:else}
					<Plus size={13} /> Create project
				{/if}
			</Button>
		</div>
	</form>
</div>
