<script lang="ts">
	import type { DeploymentStep } from '$lib/types';
	import { CheckCircle, CircleNotch, XCircle, Clock } from 'phosphor-svelte';
	import { cn } from '$lib/utils/cn';

	interface Props {
		steps: DeploymentStep[];
		class?: string;
	}

	let { steps, class: className = '' }: Props = $props();
</script>

<div
	class={cn(
		'flex flex-col gap-2 rounded-lg bg-kumo-recessed/60 p-3 ring ring-kumo-hairline/70 sm:flex-row sm:items-center sm:gap-3',
		className
	)}
>
	{#each steps as step, i (step.id || i)}
		<div class="flex min-w-0 items-center gap-2.5 {i < steps.length - 1 ? 'sm:flex-1' : ''}">
			<!-- Status Icon -->
			<div class="flex shrink-0 items-center justify-center">
				{#if step.status === 'success'}
					<span
						class="flex size-5 items-center justify-center rounded-full bg-kumo-success/15 text-kumo-success"
						title="Step completed"
					>
						<CheckCircle size={15} weight="fill" />
					</span>
				{:else if step.status === 'running'}
					<span
						class="flex size-5 items-center justify-center rounded-full bg-[var(--accent)]/15 text-[var(--accent)]"
						title="Step in progress"
					>
						<CircleNotch size={15} class="animate-spin" />
					</span>
				{:else if step.status === 'failed'}
					<span
						class="flex size-5 items-center justify-center rounded-full bg-kumo-danger/15 text-kumo-danger"
						title="Step failed"
					>
						<XCircle size={15} weight="fill" />
					</span>
				{:else}
					<span
						class="flex size-5 items-center justify-center rounded-full border border-kumo-hairline text-kumo-subtle"
						title="Pending step"
					>
						<span class="size-1.5 rounded-full bg-kumo-line"></span>
					</span>
				{/if}
			</div>

			<!-- Step Details: Title & Duration -->
			<div class="flex min-w-0 flex-col leading-tight sm:flex-row sm:items-baseline sm:gap-2">
				<span
					class="text-sm font-medium {step.status === 'running'
						? 'font-semibold text-kumo-default'
						: step.status === 'success'
							? 'text-kumo-default'
							: 'text-kumo-subtle'}"
				>
					{step.name}
				</span>
				{#if step.duration}
					<span class="font-mono text-xs text-kumo-subtle tabular-nums">
						{step.duration}
					</span>
				{/if}
			</div>

			<!-- Subtle Divider Connector (Hidden on Mobile) -->
			{#if i < steps.length - 1}
				<div class="hidden flex-1 items-center px-1.5 sm:flex">
					<div
						class="h-px w-full {step.status === 'success'
							? 'bg-kumo-success/35'
							: 'bg-kumo-hairline'}"
					></div>
				</div>
			{/if}
		</div>
	{/each}
</div>
