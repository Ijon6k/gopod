<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { Input } from '$lib/components/primitives';
	import {
		Lock,
		Envelope,
		ArrowRight,
		CircleNotch,
		WarningCircle,
		TerminalWindow,
		ShieldCheck,
		Cpu
	} from 'phosphor-svelte';

	let email = $state('');
	let password = $state('');
	let validationError = $state<string | null>(null);
	let isSubmitting = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		validationError = null;

		const cleanEmail = email.trim();
		if (!cleanEmail || !password) {
			validationError = 'Please enter your email and password.';
			return;
		}

		isSubmitting = true;
		try {
			await authStore.login({
				email: cleanEmail,
				password
			});
			// Successfully authenticated
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

<div class="mx-auto w-full max-w-[420px]">
	<!-- Branding / Header -->
	<div class="mb-6 text-center">
		<div class="mb-3 inline-flex items-center justify-center gap-2">
			<div
				class="flex h-10 w-10 items-center justify-center rounded-xl border border-[var(--accent)]/30 bg-[var(--accent-muted)] text-[var(--accent)] shadow-sm"
			>
				<TerminalWindow size={24} weight="duotone" />
			</div>
			<div class="text-left">
				<div class="flex items-center gap-1.5">
					<span class="font-mono text-lg font-bold tracking-tight text-[var(--text-primary)]"
						>GOPOD</span
					>
					<span
						class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-1.5 py-0.5 font-mono text-[10px] font-semibold text-[var(--text-tertiary)] uppercase"
						>v0.1.0</span
					>
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">Infrastructure Control Plane</span>
			</div>
		</div>

		<h1 class="text-xl font-semibold tracking-tight text-[var(--text-primary)]">
			Sign in to GOPOD
		</h1>
		<p class="mx-auto mt-1.5 max-w-[340px] text-xs leading-relaxed text-[var(--text-secondary)]">
			Enter your credentials to access your container workloads, services, and proxy domains.
		</p>
	</div>

	<!-- Login Card -->
	<div
		class="relative overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-6 shadow-xl"
	>
		<!-- Top accent indicator -->
		<div
			class="absolute top-0 right-0 left-0 h-[2px] bg-gradient-to-r from-transparent via-[var(--accent)] to-transparent opacity-80"
		></div>

		{#if validationError}
			<div
				class="mb-5 flex items-start gap-2.5 rounded-[var(--radius-sm)] border border-[var(--status-red)]/30 bg-[var(--status-red-muted)] p-3 text-xs text-[var(--status-red)]"
			>
				<WarningCircle size={16} class="mt-0.5 shrink-0" />
				<span class="leading-normal">{validationError}</span>
			</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-4">
			<!-- Email Field -->
			<div>
				<label
					for="login-email"
					class="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]"
				>
					Email address
				</label>
				<div class="relative">
					<div
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)]"
					>
						<Envelope size={16} />
					</div>
					<Input
						id="login-email"
						type="email"
						bind:value={email}
						placeholder="admin@yourdomain.com"
						required
						autocomplete="username"
						class="h-10 pl-9 text-sm"
					/>
				</div>
			</div>

			<!-- Password Field -->
			<div>
				<div class="mb-1.5 flex items-center justify-between">
					<label
						for="login-password"
						class="block text-xs font-medium text-[var(--text-secondary)]"
					>
						Password
					</label>
				</div>
				<div class="relative">
					<div
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)]"
					>
						<Lock size={16} />
					</div>
					<Input
						id="login-password"
						type="password"
						bind:value={password}
						placeholder="••••••••••••"
						required
						autocomplete="current-password"
						class="h-10 pl-9 text-sm"
					/>
				</div>
			</div>

			<!-- Submit Button -->
			<button
				type="submit"
				disabled={isSubmitting}
				class="mt-2 flex h-10 w-full cursor-pointer items-center justify-center gap-2 rounded-[var(--radius-sm)] border-0 bg-[var(--accent)] text-sm font-medium text-white shadow-sm transition-colors hover:bg-[var(--accent-hover)] disabled:cursor-not-allowed disabled:opacity-50"
			>
				{#if isSubmitting}
					<CircleNotch size={16} class="animate-spin" />
					<span>Authenticating…</span>
				{:else}
					<span>Sign In</span>
					<ArrowRight size={16} />
				{/if}
			</button>
		</form>

		<!-- Dokploy-style Invitation Notice (Strictly no register button) -->
		<div class="mt-5 border-t border-[var(--border-subtle)] pt-4 text-center">
			<p class="text-[11px] leading-relaxed text-[var(--text-muted)]">
				Registration is invite-only. New accounts must be created or invited by a cluster
				administrator.
			</p>
		</div>
	</div>

	<!-- Secure Node Footer -->
	<div
		class="mt-6 flex items-center justify-center gap-4 font-mono text-[11px] text-[var(--text-muted)]"
	>
		<span class="flex items-center gap-1.5">
			<ShieldCheck size={12} /> Rootless Security
		</span>
		<span>•</span>
		<span class="flex items-center gap-1.5">
			<Cpu size={12} /> Podman Socket
		</span>
	</div>
</div>
