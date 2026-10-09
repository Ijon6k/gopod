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
		<h3 class="m-0 text-base font-semibold text-[var(--text-primary)]">
			{title}
		</h3>
	</div>

	{#if description}
		<p class="m-0 text-xs leading-relaxed text-[var(--text-secondary)]">
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
					class="inline-flex cursor-pointer items-center gap-1 rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-0.5 text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
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
				class="w-full rounded-md border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)] placeholder-[var(--text-tertiary)] transition-colors focus:border-red-500 focus:outline-none"
			/>
		</div>
	{/if}

	{#if children}
		{@render children()}
	{/if}

	{#snippet footer()}
		<Button variant="secondary" size="sm" disabled={isConfirming} onclick={oncancel}>
			{cancelText}
		</Button>
		<button
			type="button"
			disabled={!canConfirm}
			onclick={onconfirm}
			class={variant === 'danger'
				? 'inline-flex cursor-pointer items-center gap-2 rounded-md border-0 bg-red-600 px-4 py-2 text-xs font-semibold text-white transition-colors hover:bg-red-500 disabled:cursor-not-allowed disabled:opacity-40'
				: variant === 'warning'
					? 'inline-flex cursor-pointer items-center gap-2 rounded-md border-0 bg-amber-600 px-4 py-2 text-xs font-semibold text-white transition-colors hover:bg-amber-500 disabled:cursor-not-allowed disabled:opacity-40'
					: 'inline-flex cursor-pointer items-center gap-2 rounded-md border-0 bg-[var(--accent)] px-4 py-2 text-xs font-semibold text-white transition-colors hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40'}
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
