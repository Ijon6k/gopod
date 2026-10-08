<script lang="ts">
	import Modal from './Modal.svelte';
	import { Button } from '$lib/components/primitives';
	import { WarningCircle, SpinnerGap, Copy, Check } from 'phosphor-svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		open: boolean;
		title: string;
		description?: string;
		variant?: 'danger' | 'warning' | 'default';
		confirmText?: string;
		cancelText?: string;
		isConfirming?: boolean;
		confirmDisabled?: boolean;
		matchValue?: string;
		matchLabel?: string;
		onconfirm: () => void | Promise<void>;
		oncancel: () => void;
		children?: Snippet;
	}

	let {
		open,
		title,
		description,
		variant = 'danger',
		confirmText = 'Confirm',
		cancelText = 'Cancel',
		isConfirming = false,
		confirmDisabled = false,
		matchValue,
		matchLabel,
		onconfirm,
		oncancel,
		children
	}: Props = $props();

	let inputVal = $state('');
	let copied = $state(false);

	$effect(() => {
		if (open) {
			inputVal = '';
		}
	});

	let isMatchValid = $derived.by(() => {
		if (!matchValue) return true;
		return inputVal.trim() === matchValue.trim();
	});

	let canConfirm = $derived(!confirmDisabled && !isConfirming && isMatchValid);

	function handleCopy() {
		if (!matchValue) return;
		navigator.clipboard.writeText(matchValue);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}
</script>

<Modal {open} onclose={oncancel} size="md">
	<div class="flex items-center gap-3">
		<div
			class={variant === 'danger'
				? 'text-red-400'
				: variant === 'warning'
					? 'text-amber-400'
					: 'text-[var(--accent)]'}
		>
			<WarningCircle size={22} weight="bold" />
		</div>
		<h3 class="text-base font-semibold text-[var(--text-primary)] m-0">
			{title}
		</h3>
	</div>

	{#if description}
		<p class="text-xs text-[var(--text-secondary)] leading-relaxed m-0">
			{description}
		</p>
	{/if}

	<!-- Anti-fat-finger text input confirmation -->
	{#if matchValue}
		<div class="flex flex-col gap-1.5 pt-1">
			<div class="flex items-center justify-between text-xs text-[var(--text-secondary)]">
				<span>{matchLabel ?? 'To confirm, type the exact name below:'}</span>
				<button
					type="button"
					onclick={handleCopy}
					class="inline-flex items-center gap-1 text-[11px] font-[var(--font-mono)] px-2 py-0.5 rounded border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)] cursor-pointer"
					title="Copy to clipboard"
				>
					{#if copied}
						<Check size={11} class="text-emerald-400" />
						<span class="text-emerald-400">Copied</span>
					{:else}
						<Copy size={11} />
						<span>{matchValue}</span>
					{/if}
				</button>
			</div>
			<input
				type="text"
				bind:value={inputVal}
				placeholder={matchValue}
				class="w-full px-3 py-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] text-sm text-[var(--text-primary)] placeholder-[var(--text-tertiary)] focus:border-red-500 focus:outline-none transition-colors"
			/>
		</div>
	{/if}

	{#if children}
		{@render children()}
	{/if}

	{#snippet footer()}
		<Button
			variant="secondary"
			size="sm"
			disabled={isConfirming}
			onclick={oncancel}
		>
			{cancelText}
		</Button>
		<button
			type="button"
			disabled={!canConfirm}
			onclick={onconfirm}
			class={variant === 'danger'
				? 'inline-flex items-center gap-2 px-4 py-2 rounded-md bg-red-600 hover:bg-red-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold cursor-pointer border-0 transition-colors'
				: variant === 'warning'
					? 'inline-flex items-center gap-2 px-4 py-2 rounded-md bg-amber-600 hover:bg-amber-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold cursor-pointer border-0 transition-colors'
					: 'inline-flex items-center gap-2 px-4 py-2 rounded-md bg-[var(--accent)] hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold cursor-pointer border-0 transition-colors'}
		>
			{#if isConfirming}
				<SpinnerGap size={14} class="animate-spin" />
				<span>Processing…</span>
			{:else}
				<span>{confirmText}</span>
			{/if}
		</button>
	{/snippet}
</Modal>
