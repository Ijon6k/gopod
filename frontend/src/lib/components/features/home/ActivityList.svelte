<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { goto } from '$app/navigation';
	import { StatusBadge } from '$lib/components/ui';
	import { LayerCard } from 'kumo-svelte';
	import { ArrowRight } from 'phosphor-svelte';
	import type { Deployment } from '$lib/types';

	interface Props {
		deployments: Deployment[];
		class?: string;
	}

	let { deployments, class: className = '' }: Props = $props();
</script>

<LayerCard class={cn('flex flex-col p-5', className)}>
	<div class="mb-4 flex min-h-[22px] items-center justify-between gap-4">
		<span class="text-xs font-medium text-[var(--text-tertiary)]"> Recent activity </span>
		<button
			onclick={() => goto('/deployments')}
			class="inline-flex cursor-pointer items-center gap-1.5 border-0 bg-transparent p-0 text-xs font-[var(--font-sans)] whitespace-nowrap text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
		>
			View all <ArrowRight size={12} />
		</button>
	</div>
	<div class="flex flex-col divide-y divide-[var(--border-subtle)]">
		{#each deployments.slice(0, 5) as deployment}
			<button
				onclick={() => goto('/deployments')}
				class="-mx-2 flex min-h-[50px] cursor-pointer items-center gap-3.5 rounded-[var(--radius-sm)] border-0 bg-transparent px-2 py-2 text-left font-[var(--font-sans)] text-[var(--text-primary)] hover:bg-[var(--bg-hover)]"
			>
				<span class="flex min-w-0 flex-1 flex-col gap-0.5">
					<strong class="text-sm font-medium">
						{deployment.projectName} / {deployment.serviceName}
					</strong>
					<small class="text-xs text-[var(--text-tertiary)]">
						<span class="text-[11px] font-[var(--font-mono)]">
							{deployment.version === 'latest' ? deployment.commit : deployment.version}
						</span>
						· {deployment.timeAgo}
					</small>
				</span>
				<StatusBadge status={deployment.status} size="sm" />
			</button>
		{/each}
	</div>
</LayerCard>
