<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { Input } from '$lib/components/primitives';
	import {
		ShieldCheck,
		Lock,
		Envelope,
		User as UserIcon,
		ArrowRight,
		CircleNotch,
		WarningCircle,
		CheckCircle,
		Cpu
	} from 'phosphor-svelte';

	let name = $state('');
	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let validationError = $state<string | null>(null);
	let isSubmitting = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		validationError = null;

		const cleanName = name.trim();
		const cleanEmail = email.trim();

		if (!cleanName) {
			validationError = 'Please provide an administrator name.';
			return;
		}

		if (!cleanEmail || !cleanEmail.includes('@') || !cleanEmail.includes('.')) {
			validationError = 'Please provide a valid administrator email address.';
			return;
		}

		if (password.length < 8) {
			validationError = 'Password must be at least 8 characters long.';
			return;
		}

		if (password !== confirmPassword) {
			validationError = 'Passwords do not match. Please re-enter.';
			return;
		}

		isSubmitting = true;
		try {
			await authStore.setup({
				name: cleanName,
				email: cleanEmail,
				password
			});
			// Successfully configured initial root admin
			await goto('/', { replaceState: true });
		} catch (err: any) {
			validationError = err.message || 'Failed to complete cluster setup. Please verify your details.';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>Initial Administrator Setup — GOPOD</title>
</svelte:head>

<div class="w-full max-w-[460px] mx-auto">
	<!-- Branding / Header -->
	<div class="text-center mb-6">
		<div class="inline-flex items-center justify-center gap-2 mb-3">
			<div class="w-10 h-10 rounded-xl bg-[var(--accent-muted)] border border-[var(--accent)]/30 flex items-center justify-center text-[var(--accent)] shadow-sm">
				<ShieldCheck size={24} weight="duotone" />
			</div>
			<div class="text-left">
				<div class="flex items-center gap-1.5">
					<span class="text-lg font-bold tracking-tight text-[var(--text-primary)] font-mono">GOPOD</span>
					<span class="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-[var(--accent-muted)] text-[var(--accent)] font-semibold border border-[var(--accent)]/20">Init</span>
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">Podman Infrastructure Manager</span>
			</div>
		</div>

		<h1 class="text-xl font-semibold text-[var(--text-primary)] tracking-tight">Create Administrator Account</h1>
		<p class="text-xs text-[var(--text-secondary)] mt-1.5 max-w-[380px] mx-auto leading-relaxed">
			Welcome to GOPOD. Configure your master administrative credentials to take ownership of this node and manage your containers.
		</p>
	</div>

	<!-- Setup Card -->
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
			<!-- Name Field -->
			<div>
				<label for="setup-name" class="block text-xs font-medium text-[var(--text-secondary)] mb-1.5">
					Administrator Name
				</label>
				<div class="relative">
					<div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[var(--text-tertiary)]">
						<UserIcon size={16} />
					</div>
					<Input
						id="setup-name"
						type="text"
						bind:value={name}
						placeholder="e.g. Administrator"
						required
						autocomplete="name"
						class="pl-9 h-10 text-sm"
					/>
				</div>
			</div>

			<!-- Email Field -->
			<div>
				<label for="setup-email" class="block text-xs font-medium text-[var(--text-secondary)] mb-1.5">
					Administrator Email
				</label>
				<div class="relative">
					<div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[var(--text-tertiary)]">
						<Envelope size={16} />
					</div>
					<Input
						id="setup-email"
						type="email"
						bind:value={email}
						placeholder="admin@yourdomain.com"
						required
						autocomplete="email"
						class="pl-9 h-10 text-sm"
					/>
				</div>
			</div>

			<!-- Password Field -->
			<div>
				<label for="setup-password" class="block text-xs font-medium text-[var(--text-secondary)] mb-1.5">
					Password <span class="text-[var(--text-muted)] font-normal">(min 8 characters)</span>
				</label>
				<div class="relative">
					<div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[var(--text-tertiary)]">
						<Lock size={16} />
					</div>
					<Input
						id="setup-password"
						type="password"
						bind:value={password}
						placeholder="••••••••••••"
						required
						autocomplete="new-password"
						class="pl-9 h-10 text-sm"
					/>
				</div>
			</div>

			<!-- Confirm Password Field -->
			<div>
				<label for="setup-confirm-password" class="block text-xs font-medium text-[var(--text-secondary)] mb-1.5">
					Confirm Password
				</label>
				<div class="relative">
					<div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[var(--text-tertiary)]">
						<Lock size={16} />
					</div>
					<Input
						id="setup-confirm-password"
						type="password"
						bind:value={confirmPassword}
						placeholder="••••••••••••"
						required
						autocomplete="new-password"
						class="pl-9 h-10 text-sm"
					/>
				</div>
			</div>

			<!-- Security Callout -->
			<div class="p-2.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] flex items-start gap-2 text-[11px] text-[var(--text-tertiary)]">
				<ShieldCheck size={14} class="shrink-0 text-[var(--accent)] mt-0.5" />
				<span>Account setup endpoint locks permanently once initialized. Further accounts can only be added via administrator invites.</span>
			</div>

			<!-- Submit Button -->
			<button
				type="submit"
				disabled={isSubmitting}
				class="w-full h-10 rounded-[var(--radius-sm)] bg-[var(--accent)] hover:bg-[var(--accent-hover)] text-white font-medium text-sm flex items-center justify-center gap-2 border-0 cursor-pointer transition-colors shadow-sm disabled:opacity-50 disabled:cursor-not-allowed mt-2"
			>
				{#if isSubmitting}
					<CircleNotch size={16} class="animate-spin" />
					<span>Configuring GoPod Node…</span>
				{:else}
					<span>Complete Setup & Launch Dashboard</span>
					<ArrowRight size={16} />
				{/if}
			</button>
		</form>
	</div>

	<!-- Node Info Footer -->
	<div class="mt-6 flex items-center justify-center gap-4 text-[11px] text-[var(--text-muted)] font-mono">
		<span class="flex items-center gap-1.5">
			<Cpu size={12} /> Rootless Podman 5.x
		</span>
		<span>•</span>
		<span>Modern SQLite (WAL)</span>
		<span>•</span>
		<span>Caddy Ingress</span>
	</div>
</div>
