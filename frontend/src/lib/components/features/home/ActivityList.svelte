<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { goto } from '$app/navigation';
	import { StatusBadge } from '$lib/components/ui';
	import { ArrowRight } from 'phosphor-svelte';
	import type { Deployment } from '$lib/types';

	interface Props {
		deployments: Deployment[];
		class?: string;
	}

	let { deployments, class: className = '' }: Props = $props();
</script>

<section class={cn(className)}>
	<div class="mb-4 flex min-h-[22px] items-center justify-between gap-4">
		<span class="text-[10px] font-medium tracking-[0.1em] text-[var(--text-tertiary)] uppercase">
			Recent activity
		</span>
		<button
			onclick={() => goto('/deployments')}
			class="inline-flex cursor-pointer items-center gap-[5px] border-0 bg-transparent p-0 text-xs font-[var(--font-sans)] whitespace-nowrap text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
		>
			View all <ArrowRight size={12} />
		</button>
	</div>
	<div class="flex flex-col">
		{#each deployments.slice(0, 5) as deployment}
			<button
				onclick={() => goto('/deployments')}
				class="-mx-3 flex min-h-[56px] cursor-pointer items-center gap-3.5 rounded-[var(--radius-sm)] border-0 bg-transparent px-3 py-2 text-left font-[var(--font-sans)] text-[var(--text-primary)] hover:bg-[var(--bg-panel)]"
			>
				<span class="flex min-w-0 flex-1 flex-col gap-[3px]">
					<strong class="text-base font-medium">
						{deployment.projectName} / {deployment.serviceName}
					</strong>
					<small class="text-[11.5px] text-[var(--text-tertiary)]">
						<span class="text-[10.5px] font-[var(--font-mono)]">
							{deployment.version === 'latest' ? deployment.commit : deployment.version}
						</span>
						· {deployment.timeAgo}
					</small>
				</span>
				<StatusBadge status={deployment.status} size="sm" />
			</button>
		{/each}
	</div>
</section>
