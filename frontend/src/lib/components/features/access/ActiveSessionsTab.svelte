<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { authStore } from '$lib/stores/auth.svelte';
	import type { SessionSummary } from '$lib/api/auth/types';
	import {
		Badge,
		Button,
		ConfirmDialog,
		DataTable,
		RowActionsMenu
	} from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import {
		ArrowClockwise,
		DeviceMobile,
		Laptop,
		SignOut,
		TerminalWindow
	} from 'phosphor-svelte';

	interface Props {
		scope?: 'admin' | 'self';
	}

	let { scope = 'self' }: Props = $props();
	const toast = useKumoToastManager();

	let sessions = $state<SessionSummary[]>([]);
	let isLoading = $state(true);
	let revokingTarget = $state<SessionSummary | null>(null);
	let isRevokingOthers = $state(false);

	async function loadSessions() {
		isLoading = true;
		try {
			sessions =
				scope === 'admin' ? await api.system.listAllSessions() : await api.auth.listMySessions();
		} catch (err: unknown) {
			toast.add(
				errorToastOptions(
					'Failed to load sessions',
					err instanceof Error ? err.message : undefined
				)
			);
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		void loadSessions();
	});

	async function confirmRevoke() {
		if (!revokingTarget) return;
		const target = revokingTarget;
		revokingTarget = null;
		try {
			const res =
				scope === 'admin'
					? await api.system.revokeAdminSession(target.id)
					: await api.auth.revokeMySession(target.id);
			if (res.revokedCurrent) {
				authStore.sessionExpired();
				return;
			}
			sessions = sessions.filter((s) => s.id !== target.id);
			toast.add({
				title: 'Session revoked',
				description: `Terminated session for ${target.device || target.ipAddress || 'client'}.`,
				variant: 'default'
			});
		} catch (err: unknown) {
			toast.add(
				errorToastOptions(
					'Failed to revoke session',
					err instanceof Error ? err.message : undefined
				)
			);
		}
	}

	async function handleRevokeOthers() {
		if (isRevokingOthers) return;
		isRevokingOthers = true;
		try {
			const res = await api.auth.revokeOtherSessions();
			toast.add({
				title: 'Other sessions revoked',
				description: `Terminated ${res.revokedCount} other session(s).`,
				variant: 'success'
			});
			await loadSessions();
		} catch (err: unknown) {
			toast.add(
				errorToastOptions(
					'Failed to revoke other sessions',
					err instanceof Error ? err.message : undefined
				)
			);
		} finally {
			isRevokingOthers = false;
		}
	}

	function formatTime(iso: string): string {
		if (!iso) return '—';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return d.toLocaleString();
	}

	const columns = $derived([
		...(scope === 'admin'
			? [{ key: 'userName', label: 'Account', render: accountCol }]
			: []),
		{ key: 'device', label: 'Device & client', render: deviceCol },
		{ key: 'ipAddress', label: 'IP address', render: ipCol },
		{ key: 'lastSeenAt', label: 'Last active', render: lastSeenCol },
		{ key: 'expiresAt', label: 'Expires', render: expiresCol }
	]);
</script>

<div class="flex flex-col gap-4">
	<!-- Toolbar -->
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex flex-col gap-0.5">
			<span class="text-sm font-medium text-[var(--text-primary)]">
				{scope === 'admin' ? 'All active cluster sessions' : 'Active login sessions'}
			</span>
			<span class="text-xs text-[var(--text-tertiary)]">
				{scope === 'admin'
					? `${sessions.length} active session(s) across all accounts.`
					: 'Devices and browsers currently signed in to your account.'}
			</span>
		</div>

		<div class="flex items-center gap-2">
			{#if scope === 'self' && sessions.filter((s) => !s.isCurrent).length > 0}
				<Button
					variant="secondary"
					size="sm"
					onclick={handleRevokeOthers}
					disabled={isRevokingOthers}
				>
					<SignOut size={14} />
					<span>Revoke other sessions</span>
				</Button>
			{/if}
			<Button variant="secondary" size="sm" onclick={loadSessions} disabled={isLoading}>
				<ArrowClockwise size={14} class={isLoading ? 'animate-spin' : ''} />
				<span>Refresh</span>
			</Button>
		</div>
	</div>

	<!-- Standard Kumo DataTable -->
	<DataTable
		{columns}
		rows={sessions}
		keyExtractor={(s) => s.id}
		loading={isLoading}
		skeletonRows={4}
		pageSize={10}
		itemName="sessions"
		stickyActions={true}
		emptyMessage="No active sessions found."
		actions={sessionActions}
	/>
</div>

{#snippet accountCol(s: SessionSummary)}
	<div class="flex min-w-0 flex-col">
		<span class="truncate font-medium text-[var(--text-primary)]">{s.userName || 'User'}</span>
		<span class="truncate text-xs text-[var(--text-tertiary)]">{s.userEmail}</span>
	</div>
{/snippet}

{#snippet deviceCol(s: SessionSummary)}
	<div class="flex items-center gap-2">
		{#if s.device.includes('iOS') || s.device.includes('Android')}
			<DeviceMobile size={15} class="shrink-0 text-[var(--text-tertiary)]" />
		{:else if s.device.includes('CLI')}
			<TerminalWindow size={15} class="shrink-0 text-[var(--text-tertiary)]" />
		{:else}
			<Laptop size={15} class="shrink-0 text-[var(--text-tertiary)]" />
		{/if}
		<span class="font-medium text-[var(--text-primary)]">{s.device}</span>
		{#if s.isCurrent}
			<Badge variant="success">Current</Badge>
		{/if}
	</div>
{/snippet}

{#snippet ipCol(s: SessionSummary)}
	<span class="text-[13px] text-[var(--text-secondary)] tabular-nums">
		{s.ipAddress || 'Local loopback'}
	</span>
{/snippet}

{#snippet lastSeenCol(s: SessionSummary)}
	<span class="text-[13px] text-[var(--text-secondary)]">
		{formatTime(s.lastSeenAt)}
	</span>
{/snippet}

{#snippet expiresCol(s: SessionSummary)}
	<span class="text-[13px] text-[var(--text-tertiary)]">
		{formatTime(s.expiresAt)}
	</span>
{/snippet}

{#snippet sessionActions(s: SessionSummary)}
	<RowActionsMenu
		itemName={s.device || s.userEmail || 'session'}
		actions={[
			{
				label: 'Revoke session',
				danger: true,
				onselect: () => (revokingTarget = s)
			}
		]}
	/>
{/snippet}

<ConfirmDialog
	open={revokingTarget !== null}
	title="Revoke session?"
	description={revokingTarget
		? `Revoking this ${revokingTarget.device} session (${revokingTarget.ipAddress || 'local'}) signs that device out immediately.${revokingTarget.isCurrent ? ' This is your current browser session.' : ''}`
		: ''}
	confirmText="Revoke session"
	variant="danger"
	oncancel={() => (revokingTarget = null)}
	onconfirm={confirmRevoke}
/>
