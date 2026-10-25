<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { goto } from '$app/navigation';
	import { Banner, BannerAction, LayerCard } from 'kumo-svelte';
	import { formatTimeAgo, pluralize } from '$lib/utils/format';
	import type { Deployment } from '$lib/types';

	interface Props {
		failed: Deployment[];
		/** Container names whose engine reports (unhealthy). */
		unhealthy: string[];
		/** Container names the engine reports as restarting. */
		restarting: string[];
		class?: string;
	}

	let { failed, unhealthy, restarting, class: className = '' }: Props = $props();

	/** Attachment list that names at most three and elides the rest. */
	function names(names: string[]): string {
		return names.length <= 3 ? names.join(', ') : `${names.slice(0, 3).join(', ')} + ${names.length - 3} more`;
	}

	function failedLabel(): string {
		if (failed.length === 1) {
			const d = failed[0];
			return `${d.serviceName || d.projectName || 'Service'} deployment failed`;
		}
		return `${failed.length} deployments failed`;
	}

	function failedDetail(): string {
		if (failed.length === 1) {
			const d = failed[0];
			// Live-derives so a long-open dashboard does not freeze the "when"
			// on the value stamped at fetch time.
			const when = d.startedAt ? formatTimeAgo(d.startedAt) : d.timeAgo || '';
			return [when, d.version || d.commitHash?.slice(0, 7)].filter(Boolean).join(' · ') || 'Open it for build logs';
		}
		return names(failed.map((d) => d.serviceName || d.projectName || d.serviceId));
	}

	// Single failure lands straight on its service; many failures fall back to
	// the plain deployments list so the user keeps the overview.
	function failedAction() {
		if (failed.length === 1 && failed[0]?.projectId && failed[0]?.serviceId) {
			const d = failed[0];
			goto(`/projects/${d.projectId}/services/${d.serviceId}`);
			return;
		}
		goto('/deployments');
	}
</script>

<section class={cn('flex flex-col gap-2', className)} aria-labelledby="needs-attention-title">
	<div class="mb-1 flex items-center justify-between gap-4">
		<h2 id="needs-attention-title" class="m-0 text-base font-semibold text-[var(--text-primary)]">
			Needs attention
		</h2>
		<span class="text-[13px] whitespace-nowrap text-[var(--text-tertiary)]">
			{failed.length + unhealthy.length + restarting.length}
			{pluralize(failed.length + unhealthy.length + restarting.length, 'issue')}
		</span>
	</div>

	{#if failed.length > 0}
		<Banner variant="error" title={failedLabel()} description={failedDetail()}>
			{#snippet action()}
				<BannerAction size="sm" onclick={failedAction}>View deployment</BannerAction>
			{/snippet}
		</Banner>
	{/if}

	{#if unhealthy.length > 0}
		<Banner
			variant="error"
			title="{pluralize(unhealthy.length, 'container', 'containers')} unhealthy"
			description={names(unhealthy)}
		>
			{#snippet action()}
				<BannerAction size="sm" onclick={() => goto('/runtime/containers')}>Inspect containers</BannerAction>
			{/snippet}
		</Banner>
	{/if}

	{#if restarting.length > 0}
		<Banner
			variant="alert"
			title="{pluralize(restarting.length, 'container', 'containers')} restarting"
			description={names(restarting)}
		>
			{#snippet action()}
				<BannerAction size="sm" variant="secondary" onclick={() => goto('/runtime/containers')}>
					Inspect containers
				</BannerAction>
			{/snippet}
		</Banner>
	{/if}
</section>
