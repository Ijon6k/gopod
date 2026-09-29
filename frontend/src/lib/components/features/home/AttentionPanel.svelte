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
		'bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] px-[22px] pt-5 pb-4 flex flex-col justify-start',
		className
	)}
>
	<div class="flex items-center justify-between gap-4 min-h-[22px] mb-4">
		<span
			class="text-[10px] font-medium text-[var(--text-tertiary)] tracking-[0.1em] uppercase"
		>
			Attention{failed.length > 1 ? ` · ${failed.length}` : ''}
		</span>
	</div>

	{#if failed.length > 0}
		<div class="flex flex-col gap-1.5">
			{#each failed as deployment}
				<button
					onclick={() => goto('/deployments')}
					class="flex items-center gap-[11px] py-[11px] px-2.5 -mx-2.5 border-0 rounded-[var(--radius-sm)] bg-transparent text-[var(--text-primary)] text-left cursor-pointer font-[var(--font-sans)] hover:bg-[var(--bg-hover)]"
				>
					<span class="text-[var(--status-red)] shrink-0">
						<Warning size={15} />
					</span>
					<span class="flex flex-col gap-0.5 min-w-0 flex-1">
						<strong class="text-base font-medium">
							{deployment.projectName} / {deployment.serviceName}
						</strong>
						<small class="text-[var(--text-tertiary)] text-[11.5px]">
							Deployment failed · exited with code 137
						</small>
					</span>
					<span class="text-[var(--text-tertiary)] shrink-0">
						<ArrowRight size={14} />
					</span>
				</button>
			{/each}
		</div>
	{:else}
		<div class="flex flex-col gap-1.5 px-0.5 pt-2 pb-3.5">
			<span class="text-[var(--status-green)] mb-1">
				<ShieldCheck size={20} />
			</span>
			<strong class="text-md font-medium text-[var(--text-primary)]">
				Everything is running normally
			</strong>
			<small class="text-[var(--text-tertiary)] text-base">
				No failed deployments or unhealthy services.
			</small>
		</div>
	{/if}
</section>
