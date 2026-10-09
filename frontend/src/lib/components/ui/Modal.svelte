<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { X } from 'phosphor-svelte';
	import { Dialog } from 'bits-ui';
	import type { Component, Snippet } from 'svelte';

	interface Props {
		open: boolean;
		onclose: () => void;
		title?: string;
		subtitle?: string;
		icon?: Component<any>;
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

	const sizeClasses = {
		sm: 'max-w-sm',
		md: 'max-w-md',
		lg: 'max-w-lg',
		xl: 'max-w-2xl',
		'2xl': 'max-w-4xl'
	};
</script>

<Dialog.Root
	{open}
	onOpenChange={(isOpen) => {
		if (!isOpen) onclose();
	}}
>
	<Dialog.Portal>
		<Dialog.Overlay
			class="animate-fade-in fixed inset-0 z-50 bg-[rgba(5,6,7,0.75)] backdrop-blur-xs"
		/>
		<div
			class="pointer-events-none fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-3 sm:p-5"
		>
			<Dialog.Content
				class={cn(
					'pointer-events-auto flex max-h-[92vh] w-full flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-2xl',
					sizeClasses[size],
					className
				)}
			>
				{#if title || showClose || headerActions}
					<div
						class="flex shrink-0 items-center justify-between border-b border-[var(--border)] bg-[var(--bg-panel)] px-5 py-4"
					>
						<div class="flex min-w-0 items-center gap-3 pr-3">
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
									<Dialog.Title
										class="m-0 truncate text-sm font-semibold tracking-tight text-[var(--text-primary)]"
									>
										{title}
									</Dialog.Title>
								{/if}
								{#if subtitle}
									<Dialog.Description
										class="m-0 text-xs leading-normal text-[var(--text-tertiary)]"
									>
										{subtitle}
									</Dialog.Description>
								{/if}
							</div>
						</div>

						<div class="flex shrink-0 items-center gap-2">
							{#if headerActions}
								{@render headerActions()}
							{/if}
							{#if showClose}
								<Dialog.Close
									class="inline-flex cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
									title="Close dialog"
									aria-label="Close dialog"
								>
									<X size={16} />
								</Dialog.Close>
							{/if}
						</div>
					</div>
				{/if}

				<!-- Modal Body (Scrollable if overflowing) -->
				<div
					class="flex flex-1 flex-col gap-4 overflow-y-auto p-5 text-xs text-[var(--text-secondary)]"
				>
					{@render children()}
				</div>

				{#if footer}
					<div
						class="flex shrink-0 items-center justify-end gap-2.5 border-t border-[var(--border)] bg-[var(--bg-surface)] px-5 py-3.5"
					>
						{@render footer()}
					</div>
				{/if}
			</Dialog.Content>
		</div>
	</Dialog.Portal>
</Dialog.Root>
