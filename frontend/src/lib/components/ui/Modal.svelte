<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { X } from 'phosphor-svelte';
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

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) {
			onclose();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<div
		class="fixed inset-0 z-50 bg-[rgba(5,6,7,0.75)] backdrop-blur-xs flex items-center justify-center p-3 sm:p-5 overflow-y-auto"
		role="presentation"
		onclick={(e) => {
			if (e.target === e.currentTarget) onclose();
		}}
	>
		<div
			class={cn(
				'w-full rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-2xl flex flex-col overflow-hidden max-h-[92vh]',
				sizeClasses[size],
				className
			)}
			role="dialog"
			aria-modal="true"
			aria-label={title || 'Dialog'}
			tabindex="-1"
		>
			{#if title || showClose || headerActions}
				<div class="flex items-center justify-between px-5 py-4 border-b border-[var(--border)] bg-[var(--bg-panel)] shrink-0">
					<div class="flex items-center gap-3 min-w-0 pr-3">
						{#if icon}
							{@const IconComp = icon}
							<div class="w-8 h-8 rounded-lg bg-[var(--accent-muted)] text-[var(--accent)] flex items-center justify-center shrink-0">
								<IconComp size={18} />
							</div>
						{/if}
						<div class="flex flex-col gap-0.5 min-w-0">
							{#if title}
								<h3 class="text-sm font-semibold text-[var(--text-primary)] tracking-tight m-0 truncate">
									{title}
								</h3>
							{/if}
							{#if subtitle}
								<p class="text-xs text-[var(--text-tertiary)] m-0 leading-normal">
									{subtitle}
								</p>
							{/if}
						</div>
					</div>

					<div class="flex items-center gap-2 shrink-0">
						{#if headerActions}
							{@render headerActions()}
						{/if}
						{#if showClose}
							<button
								type="button"
								onclick={onclose}
								class="p-1 rounded-[var(--radius-sm)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] transition-colors cursor-pointer border-0 bg-transparent"
								title="Close dialog"
								aria-label="Close dialog"
							>
								<X size={16} />
							</button>
						{/if}
					</div>
				</div>
			{/if}

			<!-- Modal Body (Scrollable if overflowing) -->
			<div class="p-5 overflow-y-auto flex-1 flex flex-col gap-4 text-xs text-[var(--text-secondary)]">
				{@render children()}
			</div>

			{#if footer}
				<div class="px-5 py-3.5 border-t border-[var(--border)] bg-[var(--bg-surface)] shrink-0 flex items-center justify-end gap-2.5">
					{@render footer()}
				</div>
			{/if}
		</div>
	</div>
{/if}
