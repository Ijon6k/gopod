<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { LayerCard, Input, Button, Banner } from 'kumo-svelte';
	import { CircleNotch, WarningCircle } from 'phosphor-svelte';

	import { setupSchema } from '$lib/schemas';

	let name = $state('');
	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let validationError = $state<string | null>(null);
	let isSubmitting = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		validationError = null;

		const result = setupSchema.safeParse({
			name,
			email,
			password,
			confirmPassword
		});

		if (!result.success) {
			validationError = result.error.issues[0]?.message || 'Please check your inputs.';
			return;
		}

		isSubmitting = true;
		try {
			await authStore.setup({
				name: result.data.name,
				email: result.data.email,
				password: result.data.password
			});
			await goto('/', { replaceState: true });
		} catch (err: any) {
			validationError = err.message || 'Setup failed. Please check your details and try again.';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>Administrator Setup — GOPOD</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[400px] flex-col items-center py-20">
	<!-- Brand -->
	<div class="mb-8 flex items-center gap-3">
		<img
			src="/logo.png"
			alt="GOPOD logo"
			draggable="false"
			class="h-9 w-9 rounded-lg select-none"
		/>
		<span class="font-mono text-lg font-semibold text-[var(--text-primary)]">GOPOD</span>
	</div>

	<LayerCard class="w-full p-6">
		{#if validationError}
			<Banner variant="error" size="sm" class="mb-4">
				{#snippet icon()}
					<WarningCircle size={16} weight="fill" class="shrink-0" />
				{/snippet}
				{validationError}
			</Banner>
		{/if}

		<form onsubmit={handleSubmit} class="flex flex-col gap-4">
			<Input
				id="setup-name"
				type="text"
				label="Name"
				bind:value={name}
				placeholder="Administrator"
				required
				autocomplete="name"
				class="w-full text-sm"
			/>

			<Input
				id="setup-email"
				type="email"
				label="Email"
				bind:value={email}
				placeholder="you@example.com"
				required
				autocomplete="email"
				class="w-full text-sm"
			/>

			<Input
				id="setup-password"
				type="password"
				label="Password"
				description="At least 8 characters."
				bind:value={password}
				required
				autocomplete="new-password"
				class="w-full text-sm"
			/>

			<Input
				id="setup-confirm-password"
				type="password"
				label="Confirm password"
				bind:value={confirmPassword}
				required
				autocomplete="new-password"
				class="w-full text-sm"
			/>

			<Button
				variant="primary"
				type="submit"
				disabled={isSubmitting}
				class="mt-1 w-full justify-center text-sm"
			>
				{#if isSubmitting}
					<CircleNotch size={16} class="animate-spin" />
					<span>Creating account…</span>
				{:else}
					<span>Create administrator account</span>
				{/if}
			</Button>
		</form>
	</LayerCard>

	<p class="mt-6 text-center text-xs leading-relaxed text-[var(--text-tertiary)]">
		Setup runs once. This instance supports a single administrator account.
	</p>
</div>
