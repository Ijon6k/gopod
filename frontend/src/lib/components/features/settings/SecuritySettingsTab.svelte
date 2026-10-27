<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { authStore } from '$lib/stores/auth.svelte';
	import type { KeyStatusResponse } from '$lib/api/system/types';
	import {
		Button,
		Field,
		InputGroup,
		InputGroupAddon,
		InputGroupButton,
		InputGroupInput,
		LayerCard,
		Badge,
		CopyButton
	} from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import {
		Key,
		ShieldCheck,
		LockKey,
		Warning,
		CircleNotch,
		UserCircle,
		Eye,
		EyeSlash
	} from 'phosphor-svelte';

	const toast = useKumoToastManager();

	let keyStatus = $state<KeyStatusResponse | null>(null);
	let isLoadingKey = $state(true);

	// Change password form state
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let isChangingPassword = $state(false);
	let passwordError = $state('');

	// Show/hide password toggles
	let showCurrent = $state(false);
	let showNew = $state(false);
	let showConfirm = $state(false);

	async function loadKeyStatus() {
		try {
			keyStatus = await api.system.getKeyStatus();
		} catch (err) {
			console.error('Failed to load key status:', err);
		} finally {
			isLoadingKey = false;
		}
	}

	onMount(() => {
		void loadKeyStatus();
	});

	async function handleChangePassword() {
		passwordError = '';
		if (!currentPassword) {
			passwordError = 'Current password is required.';
			return;
		}
		if (newPassword.length < 8) {
			passwordError = 'New password must be at least 8 characters long.';
			return;
		}
		if (newPassword === currentPassword) {
			passwordError = 'New password cannot be the same as current password.';
			return;
		}
		if (newPassword !== confirmPassword) {
			passwordError = 'New password and confirmation do not match.';
			return;
		}

		isChangingPassword = true;
		try {
			await api.auth.changePassword({
				currentPassword,
				newPassword
			});
			toast.add({
				title: 'Password updated',
				description: 'Administrator credentials updated successfully.',
				variant: 'success'
			});
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to update password';
			passwordError = msg;
			toast.add(errorToastOptions('Update failed', msg));
		} finally {
			isChangingPassword = false;
		}
	}
</script>

<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
	<!-- Change Password Card (Left Column) -->
	<LayerCard class="flex flex-col gap-4 p-5">
		<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
			<div class="flex items-center gap-2">
				<LockKey size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">Administrator password</h3>
			</div>
			<Badge variant="neutral">Authentication</Badge>
		</div>

		<p class="m-0 text-xs leading-relaxed text-[var(--text-secondary)]">
			Update password credentials for the instance administrator account. Password changes
			invalidate other active sessions.
		</p>

		<div class="flex flex-col gap-3.5">
			<Field label="Current password" required>
				<InputGroup class="w-full">
					<InputGroupInput
						type={showCurrent ? 'text' : 'password'}
						bind:value={currentPassword}
						placeholder="••••••••••••"
						class="font-mono text-xs"
						disabled={isChangingPassword}
					/>
					<InputGroupAddon align="end">
						<InputGroupButton
							icon={showCurrent ? EyeSlash : Eye}
							aria-label={showCurrent ? 'Hide password' : 'Show password'}
							onclick={() => (showCurrent = !showCurrent)}
						/>
					</InputGroupAddon>
				</InputGroup>
			</Field>

			<Field label="New password" description="Must be at least 8 characters long" required>
				<InputGroup class="w-full">
					<InputGroupInput
						type={showNew ? 'text' : 'password'}
						bind:value={newPassword}
						placeholder="••••••••••••"
						class="font-mono text-xs"
						disabled={isChangingPassword}
					/>
					<InputGroupAddon align="end">
						<InputGroupButton
							icon={showNew ? EyeSlash : Eye}
							aria-label={showNew ? 'Hide password' : 'Show password'}
							onclick={() => (showNew = !showNew)}
						/>
					</InputGroupAddon>
				</InputGroup>
			</Field>

			<Field label="Confirm new password" required>
				<InputGroup class="w-full">
					<InputGroupInput
						type={showConfirm ? 'text' : 'password'}
						bind:value={confirmPassword}
						placeholder="••••••••••••"
						class="font-mono text-xs"
						disabled={isChangingPassword}
					/>
					<InputGroupAddon align="end">
						<InputGroupButton
							icon={showConfirm ? EyeSlash : Eye}
							aria-label={showConfirm ? 'Hide password' : 'Show password'}
							onclick={() => (showConfirm = !showConfirm)}
						/>
					</InputGroupAddon>
				</InputGroup>
			</Field>

			{#if passwordError}
				<div
					class="rounded border border-[var(--status-red)]/30 bg-[var(--status-red-muted)] p-2.5 text-xs text-[var(--status-red)]"
				>
					{passwordError}
				</div>
			{/if}

			<div class="pt-2">
				<Button
					variant="primary"
					size="base"
					class="w-full"
					disabled={isChangingPassword}
					onclick={handleChangePassword}
				>
					{#if isChangingPassword}
						<CircleNotch size={14} class="animate-spin" /> Updating credentials...
					{:else}
						<ShieldCheck size={14} /> Update password
					{/if}
				</Button>
			</div>
		</div>
	</LayerCard>

	<!-- Security Architecture & Master Key (Right Column) -->
	<div class="flex flex-col gap-6">
		<LayerCard class="flex flex-col gap-4 p-5">
			<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
				<div class="flex items-center gap-2">
					<Key size={16} class="text-[var(--accent)]" />
					<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
						Master encryption key (AES-256-GCM)
					</h3>
				</div>
				{#if keyStatus?.keyExists}
					<Badge variant="success">Active &amp; protected</Badge>
				{:else if isLoadingKey}
					<!-- Neutral while loading: "missing" is only claimable after the
					     backend actually answered the status probe -->
					<Badge variant="neutral">Checking key status...</Badge>
				{:else}
					<Badge variant="error">Key missing</Badge>
				{/if}
			</div>

			<p class="m-0 text-xs leading-relaxed text-[var(--text-secondary)]">
				GoPod encrypts all sensitive data (passwords, registry tokens, and S3 credentials) at rest
				using a 256-bit symmetric cipher.
			</p>

			<div
				class="grid grid-cols-1 gap-3 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 text-xs sm:grid-cols-2"
			>
				<div class="flex flex-col gap-1">
					<span class="text-[11px] text-[var(--text-tertiary)]">Key storage location</span>
					<span class="font-mono text-xs text-[var(--text-primary)]">
						{keyStatus?.keyFilePath || 'data/keys/master.key'}
					</span>
				</div>
				<div class="flex flex-col gap-1">
					<span class="text-[11px] text-[var(--text-tertiary)]">Cipher algorithm</span>
					<span class="font-mono text-xs text-[var(--text-primary)]">
						{keyStatus?.algorithm || 'AES-256-GCM'}
					</span>
				</div>
			</div>

			{#if keyStatus?.fingerprint}
				<div class="flex flex-col gap-1.5">
					<span class="text-[11px] font-medium text-[var(--text-tertiary)]"
						>SHA-256 key fingerprint</span
					>
					<div
						class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-2 font-mono text-xs text-[var(--text-secondary)]"
					>
						<span class="truncate">sha256:{keyStatus.fingerprint}</span>
						<CopyButton text={`sha256:${keyStatus.fingerprint}`} />
					</div>
				</div>
			{/if}

			<div
				class="flex items-start gap-2.5 rounded-[var(--radius-sm)] border border-[var(--status-amber)]/30 bg-[var(--status-amber-muted)] p-3 text-xs text-[var(--text-primary)]"
			>
				<Warning size={16} class="shrink-0 text-[var(--status-amber)]" />
				<div class="flex flex-col gap-0.5">
					<span class="font-semibold text-[var(--status-amber)]">Disaster recovery notice</span>
					<span class="text-[11px] text-[var(--text-secondary)]">
						Retain an offline backup of <code class="font-mono text-[var(--text-primary)]"
							>keys/master.key</code
						>. If this file is lost during host failure or migration, encrypted credentials in your
						database become permanently unrecoverable.
					</span>
				</div>
			</div>
		</LayerCard>

		<!-- Active User Session Info Card -->
		<LayerCard class="flex flex-col gap-4 p-5">
			<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
				<div class="flex items-center gap-2">
					<UserCircle size={16} class="text-[var(--accent)]" />
					<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
						Active administrative session
					</h3>
				</div>
				<span
					class="inline-flex items-center gap-1.5 text-[11px] font-medium text-[var(--status-green)]"
				>
					<span class="h-1.5 w-1.5 rounded-full bg-[var(--status-green)]"></span>
					Authenticated
				</span>
			</div>

			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				<div class="flex flex-col gap-1">
					<span class="text-[11px] text-[var(--text-tertiary)]">Email address</span>
					<span class="text-xs font-medium text-[var(--text-primary)]">
						{authStore.user?.email || 'admin@gopod.dev'}
					</span>
				</div>
				<div class="flex flex-col gap-1">
					<span class="text-[11px] text-[var(--text-tertiary)]">Account role</span>
					<span class="font-mono text-xs text-[var(--accent)] uppercase">
						{authStore.user?.role || 'admin'}
					</span>
				</div>
			</div>
		</LayerCard>
	</div>
</div>
