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
		title={title}
		class="flex items-center gap-1 text-[11px] text-[var(--accent)] hover:underline bg-transparent border-0 cursor-pointer p-0 font-medium transition-colors {className}"
	>
		{#if copied}
			<Check size={size} class="text-[var(--status-green)] shrink-0" />
			<span class="text-[var(--status-green)]">{copiedLabel}</span>
		{:else}
			<Copy size={size} class="shrink-0" />
			{#if label}
				<span>{label}</span>
			{/if}
		{/if}
	</button>
{:else if variant === 'button'}
	<button
		type="button"
		onclick={handleCopy}
		title={title}
		class="flex items-center gap-1.5 px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-primary)] hover:bg-[var(--bg-hover)] cursor-pointer text-[11px] font-medium transition-colors w-fit {className}"
	>
		{#if copied}
			<Check size={size} class="text-[var(--status-green)] shrink-0" />
			<span class="text-[var(--status-green)]">{copiedLabel}</span>
		{:else}
			<Copy size={size} class="shrink-0" />
			{#if label}
				<span>{label}</span>
			{/if}
		{/if}
	</button>
{:else}
	<button
		type="button"
		onclick={handleCopy}
		title={title}
		aria-label={title}
		class="flex items-center justify-center p-1.5 rounded-[var(--radius-sm)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] transition-colors cursor-pointer border-0 bg-transparent {className}"
	>
		{#if copied}
			<Check size={size} class="text-[var(--status-green)] shrink-0" />
			{#if label}
				<span class="ml-1 text-[var(--status-green)] text-xs">{copiedLabel}</span>
			{/if}
		{:else}
			<Copy size={size} class="shrink-0" />
			{#if label}
				<span class="ml-1 text-xs">{label}</span>
			{/if}
		{/if}
	</button>
{/if}
