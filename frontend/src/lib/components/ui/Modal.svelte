<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { LayerDialog } from 'kumo-svelte';
	import type { Component, Snippet } from 'svelte';

	interface Props {
		open: boolean;
		onclose: () => void;
		title?: string;
		subtitle?: string;
		icon?: Component<{ size?: number; class?: string }>;
		size?: 'sm' | 'md' | 'lg' | 'xl' | '2xl';
		showClose?: boolean;
		class?: string;
		children: Snippet;
		headerActions?: Snippet;
		footer?: Snippet;
	}

	let {
		open,
		onclose,
		title,
		subtitle,
		icon,
		size = 'md',
		showClose = true,
		class: className = '',
		children,
		headerActions,
		footer
	}: Props = $props();

	const kumoSize = $derived<'sm' | 'base' | 'lg' | 'xl'>(
		size === 'sm' ? 'sm' : size === 'lg' ? 'lg' : size === 'xl' || size === '2xl' ? 'xl' : 'base'
	);
	const customMaxW = $derived(size === '2xl' ? 'sm:max-w-4xl' : '');
</script>

{#if open}
	<LayerDialog.Root
		open={true}
		onOpenChange={(isOpen) => {
			if (!isOpen) onclose();
		}}
	>
		<LayerDialog.Content
			size={kumoSize}
			class={cn('flex flex-col overflow-hidden p-0', customMaxW, className)}
		>
		{#if title || showClose || headerActions}
			<div
				class={cn(
					'flex shrink-0 items-center justify-between border-b border-kumo-hairline bg-kumo-base px-5 py-3.5',
					showClose ? 'pr-12' : 'pr-5'
				)}
			>
				<div class="flex min-w-0 items-center gap-3">
					{#if icon}
						{@const IconComp = icon}
						<div
							class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--accent-muted)] text-[var(--accent)]"
						>
							<IconComp size={18} />
						</div>
					{/if}
					<div class="flex min-w-0 flex-col gap-0.5">
						{#if title}
							<LayerDialog.Title class="m-0 truncate text-sm font-semibold text-kumo-default">
								{title}
							</LayerDialog.Title>
						{/if}
						{#if subtitle}
							<LayerDialog.Description class="m-0 text-xs leading-normal text-kumo-subtle">
								{subtitle}
							</LayerDialog.Description>
						{/if}
					</div>
				</div>

				{#if headerActions}
					<div class="flex shrink-0 items-center gap-2">
						{@render headerActions()}
					</div>
				{/if}
			</div>
		{/if}

		<!-- Modal Body (Scrollable if overflowing) -->
		<div class="flex flex-1 flex-col gap-4 overflow-y-auto p-5 text-xs text-kumo-subtle">
			{@render children()}
		</div>

		{#if footer}
			<div
				class="flex shrink-0 items-center justify-end gap-2.5 border-t border-kumo-hairline bg-kumo-recessed/30 px-5 py-3.5"
			>
				{@render footer()}
			</div>
		{/if}
	</LayerDialog.Content>
</LayerDialog.Root>
{/if}
