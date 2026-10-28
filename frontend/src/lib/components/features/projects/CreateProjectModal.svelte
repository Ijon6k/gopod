<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Input, Field, Modal, Banner } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { projectSchema } from '$lib/schemas';
	import { ArrowRight } from 'phosphor-svelte';

	interface Props {
		open?: boolean;
		inline?: boolean;
		onclose?: () => void;
	}

	let { open = $bindable(true), inline = false, onclose }: Props = $props();

	let name = $state('');
	let description = $state('');
	let isSubmitting = $state(false);
	let error = $state<string | null>(null);

	let isFormValid = $derived(name.trim().length > 0);

	function handleCancel() {
		open = false;
		onclose?.();
		if (inline) {
			goto('/projects');
		}
	}

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
			open = false;
			onclose?.();
			await goto(`/projects/${project.id}`);
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

{#if open}
	{#if inline}
		<!-- Inline View (for /projects/new page) -->
		<div class="flex w-full max-w-[640px] flex-col gap-6">
			<div>
				<h1 class="m-0 mb-1 text-xl font-medium text-[var(--text-primary)]">New Project</h1>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">
					Create a new logical workspace for your services and workloads.
				</p>
			</div>

			{#if error}
				<Banner variant="error" size="sm">
					<div class="text-xs">
						<span class="font-medium">Failed to create project:</span>
						<span>{error}</span>
					</div>
				</Banner>
			{/if}

			<form onsubmit={handleSubmit} class="flex flex-col gap-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5">
				<Field label="Project name" description="A descriptive name to identify your project." required error={error || undefined}>
					<Input
						id="inline-project-name"
						bind:value={name}
						placeholder="e.g. Production, Analytics, or Internal Tools"
						class="text-xs"
					/>
				</Field>

				<Field label="Description (Optional)" description="A short summary of the workloads running in this project.">
					<Input
						id="inline-project-desc"
						bind:value={description}
						placeholder="e.g. Core microservices and background job processors"
						class="text-xs"
					/>
				</Field>

				<div class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-3">
					<Button variant="ghost" size="sm" onclick={handleCancel}>Cancel</Button>
					<Button
						variant="primary"
						size="sm"
						type="submit"
						disabled={!isFormValid || isSubmitting}
					>
						Create Project <ArrowRight size={13} />
					</Button>
				</div>
			</form>
		</div>
	{:else}
		<!-- Modal Dialog View -->
		<Modal
			open={true}
			onclose={handleCancel}
			title="New Project"
			subtitle="Create a new logical workspace for your services and workloads."
			size="md"
		>
			{#if error}
				<Banner variant="error" size="sm" class="mb-3">
					<div class="text-xs">
						<span class="font-medium">Failed to create project:</span>
						<span>{error}</span>
					</div>
				</Banner>
			{/if}

			<form onsubmit={handleSubmit} class="flex flex-col gap-4">
				<Field label="Project name" description="A descriptive name to identify your project." required error={error || undefined}>
					<Input
						id="modal-project-name"
						bind:value={name}
						placeholder="e.g. Production, Analytics, or Internal Tools"
						class="text-xs"
					/>
				</Field>

				<Field label="Description (Optional)" description="A short summary of the workloads in this project.">
					<Input
						id="modal-project-desc"
						bind:value={description}
						placeholder="e.g. Core microservices and background workers"
						class="text-xs"
					/>
				</Field>
			</form>

			{#snippet footer()}
				<div class="flex w-full items-center justify-end gap-2">
					<Button variant="ghost" size="sm" onclick={handleCancel}>Cancel</Button>
					<Button
						variant="primary"
						size="sm"
						disabled={!isFormValid || isSubmitting}
						onclick={handleSubmit}
					>
						Create Project <ArrowRight size={13} />
					</Button>
				</div>
			{/snippet}
		</Modal>
	{/if}
{/if}
