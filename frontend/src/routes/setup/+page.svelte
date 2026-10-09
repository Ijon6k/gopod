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
			validationError =
				err.message || 'Failed to complete cluster setup. Please verify your details.';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>Initial Administrator Setup — GOPOD</title>
</svelte:head>

<div class="mx-auto w-full max-w-[460px]">
	<!-- Branding / Header -->
	<div class="mb-6 text-center">
		<div class="mb-3 inline-flex items-center justify-center gap-2">
			<div
				class="flex h-10 w-10 items-center justify-center rounded-xl border border-[var(--accent)]/30 bg-[var(--accent-muted)] text-[var(--accent)] shadow-sm"
			>
				<ShieldCheck size={24} weight="duotone" />
			</div>
			<div class="text-left">
				<div class="flex items-center gap-1.5">
					<span class="font-mono text-lg font-bold tracking-tight text-[var(--text-primary)]"
						>GOPOD</span
					>
					<span
						class="rounded border border-[var(--accent)]/20 bg-[var(--accent-muted)] px-1.5 py-0.5 font-mono text-[10px] font-semibold text-[var(--accent)] uppercase"
						>Init</span
					>
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">Podman Infrastructure Manager</span>
			</div>
		</div>

		<h1 class="text-xl font-semibold tracking-tight text-[var(--text-primary)]">
			Create Administrator Account
		</h1>
		<p class="mx-auto mt-1.5 max-w-[380px] text-xs leading-relaxed text-[var(--text-secondary)]">
			Welcome to GOPOD. Configure your master administrative credentials to take ownership of this
			node and manage your containers.
		</p>
	</div>

	<!-- Setup Card -->
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
			<!-- Name Field -->
			<div>
				<label
					for="setup-name"
					class="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]"
				>
					Administrator Name
				</label>
				<div class="relative">
					<div
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)]"
					>
						<UserIcon size={16} />
					</div>
					<Input
						id="setup-name"
						type="text"
						bind:value={name}
						placeholder="e.g. Administrator"
						required
						autocomplete="name"
						class="h-10 pl-9 text-sm"
					/>
				</div>
			</div>

			<!-- Email Field -->
			<div>
				<label
					for="setup-email"
					class="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]"
				>
					Administrator Email
				</label>
				<div class="relative">
					<div
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)]"
					>
						<Envelope size={16} />
					</div>
					<Input
						id="setup-email"
						type="email"
						bind:value={email}
						placeholder="admin@yourdomain.com"
						required
						autocomplete="email"
						class="h-10 pl-9 text-sm"
					/>
				</div>
			</div>

			<!-- Password Field -->
			<div>
				<label
					for="setup-password"
					class="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]"
				>
					Password <span class="font-normal text-[var(--text-muted)]">(min 8 characters)</span>
				</label>
				<div class="relative">
					<div
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)]"
					>
						<Lock size={16} />
					</div>
					<Input
						id="setup-password"
						type="password"
						bind:value={password}
						placeholder="••••••••••••"
						required
						autocomplete="new-password"
						class="h-10 pl-9 text-sm"
					/>
				</div>
			</div>

			<!-- Confirm Password Field -->
			<div>
				<label
					for="setup-confirm-password"
					class="mb-1.5 block text-xs font-medium text-[var(--text-secondary)]"
				>
					Confirm Password
				</label>
				<div class="relative">
					<div
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)]"
					>
						<Lock size={16} />
					</div>
					<Input
						id="setup-confirm-password"
						type="password"
						bind:value={confirmPassword}
						placeholder="••••••••••••"
						required
						autocomplete="new-password"
						class="h-10 pl-9 text-sm"
					/>
				</div>
			</div>

			<!-- Security Callout -->
			<div
				class="flex items-start gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2.5 text-[11px] text-[var(--text-tertiary)]"
			>
				<ShieldCheck size={14} class="mt-0.5 shrink-0 text-[var(--accent)]" />
				<span
					>Account setup endpoint locks permanently once initialized. Further accounts can only be
					added via administrator invites.</span
				>
			</div>

			<!-- Submit Button -->
			<button
				type="submit"
				disabled={isSubmitting}
				class="mt-2 flex h-10 w-full cursor-pointer items-center justify-center gap-2 rounded-[var(--radius-sm)] border-0 bg-[var(--accent)] text-sm font-medium text-white shadow-sm transition-colors hover:bg-[var(--accent-hover)] disabled:cursor-not-allowed disabled:opacity-50"
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
	<div
		class="mt-6 flex items-center justify-center gap-4 font-mono text-[11px] text-[var(--text-muted)]"
	>
		<span class="flex items-center gap-1.5">
			<Cpu size={12} /> Rootless Podman 5.x
		</span>
		<span>•</span>
		<span>Modern SQLite (WAL)</span>
		<span>•</span>
		<span>Caddy Ingress</span>
	</div>
</div>
