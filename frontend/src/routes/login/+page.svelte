<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { LayerCard, Input, Button, Banner } from 'kumo-svelte';
	import { CircleNotch, WarningCircle } from 'phosphor-svelte';

	import { loginSchema } from '$lib/schemas';

	let email = $state('');
	let password = $state('');
	let validationError = $state<string | null>(null);
	let isSubmitting = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		validationError = null;

		const result = loginSchema.safeParse({ email, password });
		if (!result.success) {
			validationError = result.error.issues[0]?.message || 'Please check your inputs.';
			return;
		}

		isSubmitting = true;
		try {
			await authStore.login(result.data);
			await goto('/', { replaceState: true });
		} catch (err: any) {
			validationError = err.message || 'Invalid email or password.';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>Sign In — GOPOD</title>
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
				id="login-email"
				type="email"
				label="Email"
				bind:value={email}
				placeholder="you@example.com"
				required
				autocomplete="username"
				class="w-full text-sm"
			/>

			<Input
				id="login-password"
				type="password"
				label="Password"
				bind:value={password}
				required
				autocomplete="current-password"
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
					<span>Signing in…</span>
				{:else}
					<span>Sign in</span>
				{/if}
			</Button>
		</form>
	</LayerCard>

	<!-- Initial setup link: only shown before the administrator account exists -->
	{#if authStore.isInitialized === false}
		<p class="mt-6 text-xs text-[var(--text-tertiary)]">
			No administrator account yet.
			<a href="/setup" class="ml-1 font-medium text-[var(--accent)] hover:underline">
				Run first-time setup
			</a>
		</p>
	{/if}
</div>
