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

<div class="w-full max-w-[420px] mx-auto">
	<!-- Branding / Header -->
	<div class="text-center mb-6">
		<div class="inline-flex items-center justify-center gap-2 mb-3">
			<div class="w-10 h-10 rounded-xl bg-[var(--accent-muted)] border border-[var(--accent)]/30 flex items-center justify-center text-[var(--accent)] shadow-sm">
				<TerminalWindow size={24} weight="duotone" />
			</div>
			<div class="text-left">
				<div class="flex items-center gap-1.5">
					<span class="text-lg font-bold tracking-tight text-[var(--text-primary)] font-mono">GOPOD</span>
					<span class="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-[var(--bg-surface)] text-[var(--text-tertiary)] font-semibold border border-[var(--border)]">v0.1.0</span>
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">Infrastructure Control Plane</span>
			</div>
		</div>

		<h1 class="text-xl font-semibold text-[var(--text-primary)] tracking-tight">Sign in to GOPOD</h1>
		<p class="text-xs text-[var(--text-secondary)] mt-1.5 max-w-[340px] mx-auto leading-relaxed">
			Enter your credentials to access your container workloads, services, and proxy domains.
		</p>
	</div>

	<!-- Login Card -->
	<div class="bg-[var(--bg-shell)] border border-[var(--border)] rounded-[var(--radius-card)] p-6 shadow-xl relative overflow-hidden">
		<!-- Top accent indicator -->
		<div class="absolute top-0 left-0 right-0 h-[2px] bg-gradient-to-r from-transparent via-[var(--accent)] to-transparent opacity-80"></div>

		{#if validationError}
			<div class="mb-5 p-3 rounded-[var(--radius-sm)] bg-[var(--status-red-muted)] border border-[var(--status-red)]/30 flex items-start gap-2.5 text-xs text-[var(--status-red)]">
				<WarningCircle size={16} class="shrink-0 mt-0.5" />
				<span class="leading-normal">{validationError}</span>
			</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-4">
			<!-- Email Field -->
			<div>
				<label for="login-email" class="block text-xs font-medium text-[var(--text-secondary)] mb-1.5">
					Email address
				</label>
				<div class="relative">
					<div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[var(--text-tertiary)]">
						<Envelope size={16} />
					</div>
					<Input
						id="login-email"
						type="email"
						bind:value={email}
						placeholder="admin@yourdomain.com"
						required
						autocomplete="username"
						class="pl-9 h-10 text-sm"
					/>
				</div>
			</div>

			<!-- Password Field -->
			<div>
				<div class="flex items-center justify-between mb-1.5">
					<label for="login-password" class="block text-xs font-medium text-[var(--text-secondary)]">
						Password
					</label>
				</div>
				<div class="relative">
					<div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[var(--text-tertiary)]">
						<Lock size={16} />
					</div>
					<Input
						id="login-password"
						type="password"
						bind:value={password}
						placeholder="••••••••••••"
						required
						autocomplete="current-password"
						class="pl-9 h-10 text-sm"
					/>
				</div>
			</div>

			<!-- Submit Button -->
			<button
				type="submit"
				disabled={isSubmitting}
				class="w-full h-10 rounded-[var(--radius-sm)] bg-[var(--accent)] hover:bg-[var(--accent-hover)] text-white font-medium text-sm flex items-center justify-center gap-2 border-0 cursor-pointer transition-colors shadow-sm disabled:opacity-50 disabled:cursor-not-allowed mt-2"
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
		<div class="mt-5 pt-4 border-t border-[var(--border-subtle)] text-center">
			<p class="text-[11px] text-[var(--text-muted)] leading-relaxed">
				Registration is invite-only. New accounts must be created or invited by a cluster administrator.
			</p>
		</div>
	</div>

	<!-- Secure Node Footer -->
	<div class="mt-6 flex items-center justify-center gap-4 text-[11px] text-[var(--text-muted)] font-mono">
		<span class="flex items-center gap-1.5">
			<ShieldCheck size={12} /> Rootless Security
		</span>
		<span>•</span>
		<span class="flex items-center gap-1.5">
			<Cpu size={12} /> Podman Socket
		</span>
	</div>
</div>
