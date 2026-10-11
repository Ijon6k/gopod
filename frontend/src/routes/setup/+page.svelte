<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { LayerCard, Input, Button, Badge } from 'kumo-svelte';
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

<div class="mx-auto w-full max-w-[460px] py-8">
	<!-- Branding / Header -->
	<div class="mb-6 text-center">
		<div class="mb-3 inline-flex items-center justify-center gap-2">
			<div
				class="flex h-10 w-10 items-center justify-center rounded-xl border border-[var(--accent)]/30 bg-[var(--accent-muted)] text-[var(--accent)] shadow-xs"
			>
				<ShieldCheck size={24} weight="duotone" />
			</div>
			<div class="text-left">
				<div class="flex items-center gap-1.5">
					<span class="font-mono text-lg font-bold tracking-tight text-[var(--text-primary)]"
						>GOPOD</span
					>
					<Badge variant="secondary" class="!px-1.5 !py-0 !text-[10px] font-mono uppercase">
						Init
					</Badge>
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

	<!-- Setup Card with Pure Kumo LayerCard -->
	<LayerCard class="relative p-6 shadow-lg">
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
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)] z-10"
					>
						<UserIcon size={15} />
					</div>
					<Input
						id="setup-name"
						type="text"
						bind:value={name}
						placeholder="e.g. Administrator"
						required
						autocomplete="name"
						class="!h-10 !pl-9 text-sm w-full"
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
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)] z-10"
					>
						<Envelope size={15} />
					</div>
					<Input
						id="setup-email"
						type="email"
						bind:value={email}
						placeholder="admin@yourdomain.com"
						required
						autocomplete="email"
						class="!h-10 !pl-9 text-sm w-full"
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
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)] z-10"
					>
						<Lock size={15} />
					</div>
					<Input
						id="setup-password"
						type="password"
						bind:value={password}
						placeholder="••••••••••••"
						required
						autocomplete="new-password"
						class="!h-10 !pl-9 text-sm w-full"
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
						class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-[var(--text-tertiary)] z-10"
					>
						<Lock size={15} />
					</div>
					<Input
						id="setup-confirm-password"
						type="password"
						bind:value={confirmPassword}
						placeholder="••••••••••••"
						required
						autocomplete="new-password"
						class="!h-10 !pl-9 text-sm w-full"
					/>
				</div>
			</div>

			<!-- Security Callout -->
			<div
				class="flex items-start gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2.5 text-[11px] text-[var(--text-tertiary)]"
			>
				<ShieldCheck size={14} class="mt-0.5 shrink-0 text-[var(--accent)]" />
				<span
					>Account setup locks permanently once initialized. Further accounts can only be added via administrator invites.</span
				>
			</div>

			<!-- Submit Button with Kumo Button -->
			<Button
				variant="primary"
				type="submit"
				disabled={isSubmitting}
				class="mt-2 w-full !h-10 text-sm font-medium gap-2 justify-center shadow-xs"
			>
				{#if isSubmitting}
					<CircleNotch size={16} class="animate-spin" />
					<span>Configuring GoPod Node…</span>
				{:else}
					<span>Complete Setup & Launch Dashboard</span>
					<ArrowRight size={16} />
				{/if}
			</Button>
		</form>

		<!-- Link to Login -->
		<div class="mt-5 border-t border-[var(--border-subtle)] pt-4 text-center">
			<p class="text-xs text-[var(--text-tertiary)]">
				Already have an account?
				<a
					href="/login"
					class="font-medium text-[var(--accent)] hover:underline ml-1"
				>
					Sign in to GOPOD
				</a>
			</p>
		</div>
	</LayerCard>

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
