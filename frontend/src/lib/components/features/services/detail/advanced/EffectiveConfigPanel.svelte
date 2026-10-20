<script lang="ts">
	import type { Service } from '$lib/types';
	import type { QuadletPreviewResponse } from '$lib/api/services/types';
	import { api } from '$lib/api';
	import { SectionCard, Button, Banner } from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { ArrowsClockwise, CheckCircle, FileCode, WarningCircle } from 'phosphor-svelte';

	interface Props {
		openEffective?: boolean;
		service: Service;
	}

	let { openEffective = $bindable(true), service }: Props = $props();

	const toast = useKumoToastManager();

	let loading = $state(false);
	let preview = $state<QuadletPreviewResponse | null>(null);

	// Renders the backend generator output for THIS service as currently
	async function handlePreview() {
		loading = true;
		try {
			preview = await api.services.quadletPreview(service.id);
		} catch (err: any) {
			toast.add({
				title: 'Preview failed',
				description: err?.message || 'The generator rejected the request',
				variant: 'error'
			});
		} finally {
			loading = false;
		}
	}
</script>

<SectionCard
	title="Effective Configuration"
	description="Generated unit files from saved settings, checked by quadlet -dryrun — what the next promotion deploys."
	collapsible={true}
	bind:open={openEffective}
>
	<div class="flex flex-col gap-3 pt-1">
		<div class="flex flex-wrap items-center gap-2">
			<Button variant="secondary" size="sm" disabled={loading} onclick={handlePreview}>
				{#if loading}
					<ArrowsClockwise size={13} class="animate-spin" /> Generating…
				{:else if preview}
					<ArrowsClockwise size={13} /> Regenerate
				{:else}
					<FileCode size={13} /> Generate from saved settings
				{/if}
			</Button>
			<span class="text-[10px] text-[var(--text-tertiary)]">
				Runtime image deploys print the exact sanitized <code>podman run</code> arguments in the deploy
				log.
			</span>
		</div>

		{#if preview}
			{#if preview.error}
				<Banner variant="alert">
					{#snippet icon()}
						<WarningCircle size={18} weight="fill" class="shrink-0 text-kumo-warning" />
					{/snippet}
					<pre
						class="m-0 font-mono text-[11px] leading-relaxed whitespace-pre-wrap">{preview.error}</pre>
				</Banner>
			{:else if preview.valid}
				<div class="flex items-center gap-2 text-xs">
					<CheckCircle size={14} weight="fill" class="text-kumo-success" />
					<span class="font-medium text-[var(--text-primary)]">
						{Object.keys(preview.files).length} unit file(s) pass <code>quadlet -dryrun</code>
					</span>
				</div>
			{/if}

			{#each Object.entries(preview.files) as [name, content] (name)}
				<div class="flex flex-col gap-1">
					<span class="font-mono text-[11px] font-semibold text-[var(--accent)]">{name}</span>
					<pre
						class="max-h-48 overflow-auto rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3 font-mono text-[11px] leading-relaxed text-[var(--text-secondary)]">{content}</pre>
				</div>
			{/each}

			{#if preview.dryrun}
				<div class="flex flex-col gap-1">
					<span class="text-[10px] font-medium text-[var(--text-tertiary)] uppercase"
						>quadlet -dryrun output</span
					>
					<pre
						class="max-h-32 overflow-auto rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3 font-mono text-[11px] leading-relaxed text-[var(--text-tertiary)]">{preview.dryrun}</pre>
				</div>
			{/if}
		{/if}
	</div>
</SectionCard>
