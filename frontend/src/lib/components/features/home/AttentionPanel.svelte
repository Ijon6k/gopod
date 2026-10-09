<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { goto } from '$app/navigation';
	import { Warning, ArrowRight, ShieldCheck } from 'phosphor-svelte';
	import type { Deployment } from '$lib/types';

	interface Props {
		failed: Deployment[];
		class?: string;
	}

	let { failed, class: className = '' }: Props = $props();
</script>

<section
	class={cn(
		'flex flex-col justify-start rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] px-[22px] pt-5 pb-4',
		className
	)}
>
	<div class="mb-4 flex min-h-[22px] items-center justify-between gap-4">
		<span class="text-[10px] font-medium tracking-[0.1em] text-[var(--text-tertiary)] uppercase">
			Attention{failed.length > 1 ? ` · ${failed.length}` : ''}
		</span>
	</div>

	{#if failed.length > 0}
		<div class="flex flex-col gap-1.5">
			{#each failed as deployment}
				<button
					onclick={() => goto('/deployments')}
					class="-mx-2.5 flex cursor-pointer items-center gap-[11px] rounded-[var(--radius-sm)] border-0 bg-transparent px-2.5 py-[11px] text-left font-[var(--font-sans)] text-[var(--text-primary)] hover:bg-[var(--bg-hover)]"
				>
					<span class="shrink-0 text-[var(--status-red)]">
						<Warning size={15} />
					</span>
					<span class="flex min-w-0 flex-1 flex-col gap-0.5">
						<strong class="text-base font-medium">
							{deployment.projectName} / {deployment.serviceName}
						</strong>
						<small class="text-[11.5px] text-[var(--text-tertiary)]">
							Deployment failed · exited with code 137
						</small>
					</span>
					<span class="shrink-0 text-[var(--text-tertiary)]">
						<ArrowRight size={14} />
					</span>
				</button>
			{/each}
		</div>
	{:else}
		<div class="flex flex-col gap-1.5 px-0.5 pt-2 pb-3.5">
			<span class="mb-1 text-[var(--status-green)]">
				<ShieldCheck size={20} />
			</span>
			<strong class="text-md font-medium text-[var(--text-primary)]">
				Everything is running normally
			</strong>
			<small class="text-base text-[var(--text-tertiary)]">
				No failed deployments or unhealthy services.
			</small>
		</div>
	{/if}
</section>
