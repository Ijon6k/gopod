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
	<div class="flex items-center justify-between gap-4 min-h-[22px] mb-4">
		<span class="text-[10px] font-medium text-[var(--text-tertiary)] tracking-[0.1em] uppercase">
			Recent activity
		</span>
		<button
			onclick={() => goto('/deployments')}
			class="inline-flex items-center gap-[5px] p-0 border-0 bg-transparent text-[var(--text-secondary)] text-xs font-[var(--font-sans)] cursor-pointer whitespace-nowrap hover:text-[var(--text-primary)]"
		>
			View all <ArrowRight size={12} />
		</button>
	</div>
	<div class="flex flex-col">
		{#each deployments.slice(0, 5) as deployment}
			<button
				onclick={() => goto('/deployments')}
				class="flex items-center gap-3.5 min-h-[56px] py-2 px-3 -mx-3 border-0 rounded-[var(--radius-sm)] bg-transparent text-[var(--text-primary)] cursor-pointer text-left font-[var(--font-sans)] hover:bg-[var(--bg-panel)]"
			>
				<span class="flex flex-col gap-[3px] min-w-0 flex-1">
					<strong class="text-base font-medium">
						{deployment.projectName} / {deployment.serviceName}
					</strong>
					<small class="text-[var(--text-tertiary)] text-[11.5px]">
						<span class="font-[var(--font-mono)] text-[10.5px]">
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
