<script lang="ts">
	import { Copy, Check } from 'phosphor-svelte';

	interface Props {
		text: string;
		label?: string;
		copiedLabel?: string;
		variant?: 'icon' | 'inline' | 'button';
		size?: number;
		class?: string;
		title?: string;
		oncopied?: () => void;
	}

	let {
		text,
		label,
		copiedLabel = 'Copied!',
		variant = 'icon',
		size = 13,
		class: className = '',
		title = 'Copy to clipboard',
		oncopied
	}: Props = $props();

	let copied = $state(false);
	let timeoutId: ReturnType<typeof setTimeout> | null = null;

	async function handleCopy(e: MouseEvent) {
		e.stopPropagation();
		if (!text) return;
		try {
			await navigator.clipboard.writeText(text);
			copied = true;
			oncopied?.();
			if (timeoutId) clearTimeout(timeoutId);
			timeoutId = setTimeout(() => {
				copied = false;
			}, 2000);
		} catch (err) {
			console.error('Failed to copy to clipboard:', err);
		}
	}
</script>

{#if variant === 'inline'}
	<button
		type="button"
		onclick={handleCopy}
		{title}
		class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] font-medium text-[var(--accent)] transition-colors hover:underline {className}"
	>
		{#if copied}
			<Check {size} class="shrink-0 text-[var(--status-green)]" />
			<span class="text-[var(--status-green)]">{copiedLabel}</span>
		{:else}
			<Copy {size} class="shrink-0" />
			{#if label}
				<span>{label}</span>
			{/if}
		{/if}
	</button>
{:else if variant === 'button'}
	<button
		type="button"
		onclick={handleCopy}
		{title}
		class="flex w-fit cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1 text-[11px] font-medium text-[var(--text-primary)] transition-colors hover:bg-[var(--bg-hover)] {className}"
	>
		{#if copied}
			<Check {size} class="shrink-0 text-[var(--status-green)]" />
			<span class="text-[var(--status-green)]">{copiedLabel}</span>
		{:else}
			<Copy {size} class="shrink-0" />
			{#if label}
				<span>{label}</span>
			{/if}
		{/if}
	</button>
{:else}
	<button
		type="button"
		onclick={handleCopy}
		{title}
		aria-label={title}
		class="flex cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] {className}"
	>
		{#if copied}
			<Check {size} class="shrink-0 text-[var(--status-green)]" />
			{#if label}
				<span class="ml-1 text-xs text-[var(--status-green)]">{copiedLabel}</span>
			{/if}
		{:else}
			<Copy {size} class="shrink-0" />
			{#if label}
				<span class="ml-1 text-xs">{label}</span>
			{/if}
		{/if}
	</button>
{/if}
