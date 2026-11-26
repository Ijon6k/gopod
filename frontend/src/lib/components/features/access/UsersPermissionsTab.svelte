<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { projects, services } from '$lib/data';
	import { authStore } from '$lib/stores/auth.svelte';
	import type { AdminUserSummary } from '$lib/api/system/types';
	import type { PermissionKey, ProjectScope, UserPermissions } from '$lib/api/auth/types';
	import {
		Badge,
		Button,
		Checkbox,
		ConfirmDialog,
		DataTable,
		Field,
		Input,
		Modal,
		RowActionsMenu,
		SearchInput,
		Select
	} from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { Plus, ShieldCheck, UserCircle, UsersThree } from 'phosphor-svelte';

	const toast = useKumoToastManager();

	const EMPTY_PERMS: UserPermissions = {
		canCreateProjects: false,
		canDeleteProjects: false,
		canCreateServices: false,
		canDeployServices: false,
		canDeleteServices: false,
		canRevealSecrets: false,
		canManageDomains: false,
		canManageBackups: false,
		canAccessRuntime: false,
		canControlRuntime: false,
		canViewMonitoring: false,
		canViewActivity: false
	};

	const PRESETS: Record<string, { label: string; perms: UserPermissions }> = {
		viewer: {
			label: 'Viewer',
			perms: {
				...EMPTY_PERMS,
				canViewMonitoring: true,
				canViewActivity: true
			}
		},
		developer: {
			label: 'Developer',
			perms: {
				...EMPTY_PERMS,
				canCreateServices: true,
				canDeployServices: true,
				canManageDomains: true,
				canManageBackups: true,
				canViewMonitoring: true,
				canViewActivity: true
			}
		},
		operator: {
			label: 'Operator',
			perms: {
				...EMPTY_PERMS,
				canCreateProjects: true,
				canCreateServices: true,
				canDeployServices: true,
				canDeleteServices: true,
				canRevealSecrets: true,
				canManageDomains: true,
				canManageBackups: true,
				canAccessRuntime: true,
				canViewMonitoring: true,
				canViewActivity: true
			}
		},
		clear: {
			label: 'Minimal',
			perms: { ...EMPTY_PERMS }
		}
	};

	const PERM_GROUPS: {
		title: string;
		items: { key: PermissionKey; label: string; hint: string }[];
	}[] = [
		{
			title: 'Projects & services',
			items: [
				{
					key: 'canCreateProjects',
					label: 'Create projects',
					hint: 'Create new project workspaces'
				},
				{ key: 'canDeleteProjects', label: 'Delete projects', hint: 'Remove assigned projects' },
				{
					key: 'canCreateServices',
					label: 'Create services',
					hint: 'Add services inside assigned projects'
				},
				{
					key: 'canDeployServices',
					label: 'Deploy & manage',
					hint: 'Build, deploy, start/stop, edit env'
				},
				{
					key: 'canDeleteServices',
					label: 'Delete services',
					hint: 'Destroy services and workloads'
				}
			]
		},
		{
			title: 'Secrets, domains & backups',
			items: [
				{
					key: 'canRevealSecrets',
					label: 'Reveal secrets',
					hint: 'View secret env vars & webhook tokens'
				},
				{
					key: 'canManageDomains',
					label: 'Manage domains',
					hint: 'Configure Caddy routes & TLS'
				},
				{
					key: 'canManageBackups',
					label: 'Manage backups',
					hint: 'Create, restore & schedule snapshots'
				}
			]
		},
		{
			title: 'Engine & observability',
			items: [
				{
					key: 'canViewMonitoring',
					label: 'View monitoring',
					hint: 'Telemetry charts & HTTP traffic'
				},
				{
					key: 'canViewActivity',
					label: 'View activity',
					hint: 'Deployment activity & engine events'
				},
				{
					key: 'canAccessRuntime',
					label: 'Read host runtime',
					hint: 'Inspect containers, images & volumes'
				},
				{
					key: 'canControlRuntime',
					label: 'Control runtime & exec',
					hint: 'Start/stop containers & open terminal'
				}
			]
		}
	];

	let users = $state<AdminUserSummary[]>([]);
	let search = $state('');
	let isLoading = $state(true);
	let showUserModal = $state(false);
	let editingUser = $state<AdminUserSummary | null>(null);
	let deletingUser = $state<AdminUserSummary | null>(null);
	let isSaving = $state(false);

	// Form state
	let formName = $state('');
	let formEmail = $state('');
	let formPassword = $state('');
	let formRole = $state<'admin' | 'member'>('member');
	let formPerms = $state<UserPermissions>({ ...EMPTY_PERMS });
	let formScopes = $state<Record<string, { enabled: boolean; serviceIds: string[] }>>({});

	let filteredUsers = $derived.by(() => {
		const q = search.trim().toLowerCase();
		if (!q) return users;
		return users.filter(
			(u) =>
				u.name.toLowerCase().includes(q) ||
				u.email.toLowerCase().includes(q) ||
				u.role.toLowerCase().includes(q)
		);
	});

	async function loadUsers() {
		isLoading = true;
		try {
			users = await api.system.listUsers();
		} catch (err: unknown) {
			toast.add(
				errorToastOptions('Failed to load users', err instanceof Error ? err.message : undefined)
			);
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		void loadUsers();
	});

	function buildScopeMap(existingScopes?: ProjectScope[], fallbackProjectIds?: string[]) {
		const map: Record<string, { enabled: boolean; serviceIds: string[] }> = {};
		for (const p of projects) {
			map[p.id] = { enabled: false, serviceIds: [] };
		}
		if (existingScopes && existingScopes.length > 0) {
			for (const sc of existingScopes) {
				map[sc.projectId] = { enabled: true, serviceIds: [...(sc.serviceIds || [])] };
			}
		} else if (fallbackProjectIds) {
			for (const pid of fallbackProjectIds) {
				map[pid] = { enabled: true, serviceIds: [] };
			}
		}
		return map;
	}

	function openCreate() {
		editingUser = null;
		formName = '';
		formEmail = '';
		formPassword = '';
		formRole = 'member';
		formPerms = { ...EMPTY_PERMS };
		formScopes = buildScopeMap();
		showUserModal = true;
	}

	function openEdit(u: AdminUserSummary) {
		editingUser = u;
		formName = u.name;
		formEmail = u.email;
		formPassword = '';
		formRole = u.role;
		formPerms = { ...EMPTY_PERMS, ...(u.permissions || {}) };
		formScopes = buildScopeMap(u.projectScopes, u.projectIds);
		showUserModal = true;
	}

	function applyPreset(key: string) {
		const preset = PRESETS[key];
		if (!preset) return;
		formPerms = { ...preset.perms };
	}

	function togglePerm(key: PermissionKey) {
		const next = !formPerms[key];
		formPerms = {
			...formPerms,
			[key]: next,
			...(key === 'canControlRuntime' && next ? { canAccessRuntime: true } : {})
		};
	}

	function toggleProjectScope(projectId: string) {
		const cur = formScopes[projectId] || { enabled: false, serviceIds: [] };
		formScopes = {
			...formScopes,
			[projectId]: { enabled: !cur.enabled, serviceIds: cur.enabled ? [] : cur.serviceIds }
		};
	}

	function toggleServiceScope(projectId: string, serviceId: string) {
		const cur = formScopes[projectId] || { enabled: true, serviceIds: [] };
		const exists = cur.serviceIds.includes(serviceId);
		const nextSvcs = exists
			? cur.serviceIds.filter((s) => s !== serviceId)
			: [...cur.serviceIds, serviceId];
		formScopes = {
			...formScopes,
			[projectId]: { enabled: true, serviceIds: nextSvcs }
		};
	}

	function serializeScopes(): ProjectScope[] {
		const scopes: ProjectScope[] = [];
		for (const [projectId, entry] of Object.entries(formScopes)) {
			if (entry.enabled) {
				scopes.push({ projectId, serviceIds: entry.serviceIds });
			}
		}
		return scopes;
	}

	async function handleSubmit() {
		if (isSaving) return;
		isSaving = true;
		try {
			const scopes = formRole === 'admin' ? [] : serializeScopes();
			const projectIds = scopes.map((s) => s.projectId);
			if (editingUser) {
				const updated = await api.system.updateUser(editingUser.id, {
					name: formName.trim(),
					role: formRole,
					permissions: formPerms,
					...(formPassword ? { password: formPassword } : {}),
					projectIds,
					projectScopes: scopes
				});
				users = users.map((u) => (u.id === updated.id ? updated : u));
				toast.add({
					title: 'Permissions updated',
					description: `Saved role and capabilities for ${updated.name}.`,
					variant: 'success'
				});
			} else {
				const created = await api.system.createUser({
					name: formName.trim(),
					email: formEmail.trim(),
					password: formPassword,
					role: formRole,
					permissions: formPerms,
					projectIds,
					projectScopes: scopes
				});
				users = [...users, created];
				toast.add({
					title: 'User created',
					description: `${created.name} added to access control.`,
					variant: 'success'
				});
			}
			showUserModal = false;
		} catch (err: unknown) {
			toast.add(
				errorToastOptions('Failed to save user', err instanceof Error ? err.message : undefined)
			);
		} finally {
			isSaving = false;
		}
	}

	async function confirmDelete() {
		if (!deletingUser) return;
		const target = deletingUser;
		deletingUser = null;
		try {
			await api.system.deleteUser(target.id);
			users = users.filter((u) => u.id !== target.id);
			toast.add({
				title: 'User removed',
				description: `Revoked sessions and removed ${target.email}.`,
				variant: 'default'
			});
		} catch (err: unknown) {
			toast.add(
				errorToastOptions('Failed to delete user', err instanceof Error ? err.message : undefined)
			);
		}
	}

	function scopeSummary(u: AdminUserSummary): string {
		if (u.role === 'admin') return 'All workspaces & host runtime';
		const scopes = u.projectScopes || [];
		if (scopes.length === 0) {
			if (!u.projectIds || u.projectIds.length === 0) return 'No projects assigned';
			return u.projectIds.map((id) => projects.find((p) => p.id === id)?.name || id).join(', ');
		}
		return scopes
			.map((sc) => {
				const pName = projects.find((p) => p.id === sc.projectId)?.name || sc.projectId;
				return sc.serviceIds && sc.serviceIds.length > 0
					? `${pName} (${sc.serviceIds.length} svc)`
					: pName;
			})
			.join(', ');
	}

	function permissionCount(u: AdminUserSummary): number {
		if (u.role === 'admin') return 12;
		const perms = u.permissions;
		if (!perms) return 0;
		return Object.values(perms).filter(Boolean).length;
	}
</script>

<div class="flex flex-col gap-4">
	<!-- Toolbar -->
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex flex-wrap items-center gap-3">
			<SearchInput
				bind:value={search}
				placeholder="Search users by name or email..."
				class="w-full sm:w-72"
			/>
			<span class="text-xs text-[var(--text-tertiary)] tabular-nums">
				{filteredUsers.length}
				{filteredUsers.length === 1 ? 'account' : 'accounts'}
			</span>
		</div>

		<Button variant="primary" size="sm" onclick={openCreate}>
			<Plus size={14} />
			<span>Add user</span>
		</Button>
	</div>

	<!-- Standard Kumo DataTable -->
	<DataTable
		columns={[
			{ key: 'name', label: 'User', render: userCol },
			{ key: 'role', label: 'Role', render: roleCol },
			{ key: 'capabilities', label: 'Capabilities', render: capabilitiesCol },
			{ key: 'scope', label: 'Workspace scope', render: scopeCol }
		]}
		rows={filteredUsers}
		keyExtractor={(u) => u.id}
		loading={isLoading}
		skeletonRows={3}
		pageSize={10}
		itemName="users"
		stickyActions={true}
		emptyMessage={search ? 'No accounts match your filter.' : 'No user accounts found.'}
		actions={userActions}
	/>
</div>

{#snippet userCol(u: AdminUserSummary)}
	<div class="flex items-center gap-2.5">
		{#if u.role === 'admin'}
			<ShieldCheck size={16} class="shrink-0 text-[var(--status-green)]" />
		{:else}
			<UserCircle size={16} class="shrink-0 text-[var(--text-tertiary)]" />
		{/if}
		<div class="flex min-w-0 flex-col">
			<div class="flex items-center gap-1.5">
				<span class="truncate font-medium text-[var(--text-primary)]">{u.name}</span>
				{#if authStore.user?.id === u.id}
					<Badge variant="secondary">You</Badge>
				{/if}
			</div>
			<span class="truncate text-xs text-[var(--text-tertiary)]">{u.email}</span>
		</div>
	</div>
{/snippet}

{#snippet roleCol(u: AdminUserSummary)}
	<Badge variant={u.role === 'admin' ? 'success' : 'secondary'}>
		{u.role === 'admin' ? 'Admin' : 'Member'}
	</Badge>
{/snippet}

{#snippet capabilitiesCol(u: AdminUserSummary)}
	{#if u.role === 'admin'}
		<span class="text-[13px] text-[var(--text-secondary)]">Full access (12/12)</span>
	{:else}
		<span class="text-[13px] text-[var(--text-secondary)] tabular-nums">
			{permissionCount(u)} / 12 granted
		</span>
	{/if}
{/snippet}

{#snippet scopeCol(u: AdminUserSummary)}
	<span class="block max-w-md truncate text-[13px] text-[var(--text-secondary)]">
		{scopeSummary(u)}
	</span>
{/snippet}

{#snippet userActions(u: AdminUserSummary)}
	<RowActionsMenu
		itemName={u.name}
		actions={[
			{
				label: 'Edit permissions',
				onselect: () => openEdit(u)
			},
			...(authStore.user?.id !== u.id
				? [
						{
							label: 'Remove user',
							danger: true,
							onselect: () => (deletingUser = u)
						}
					]
				: [])
		]}
	/>
{/snippet}

<!-- Create / Edit User Modal -->
<Modal
	open={showUserModal}
	onclose={() => (showUserModal = false)}
	size="xl"
	icon={UsersThree}
	title={editingUser ? `Edit permissions — ${editingUser.name}` : 'Create user account'}
	subtitle="Changing roles or permissions revokes existing sessions automatically."
>
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<Field label="Display name">
			<Input bind:value={formName} placeholder="Alex" class="text-xs" />
		</Field>
		<Field label="Email address">
			<Input
				bind:value={formEmail}
				type="email"
				placeholder="alex@homelab.local"
				disabled={Boolean(editingUser)}
				class="text-xs"
			/>
		</Field>
		<Field label={editingUser ? 'New password (optional)' : 'Password (min 8 characters)'}>
			<Input
				bind:value={formPassword}
				type="password"
				placeholder={editingUser ? 'Leave blank to keep existing' : 'Enter password'}
				class="text-xs"
			/>
		</Field>
		<Field label="Account role">
			<Select
				value={formRole}
				onValueChange={(v: unknown) => {
					if (v === 'admin' || v === 'member') formRole = v;
				}}
				options={[
					{ label: 'Member (fine-grained capabilities & scoped projects)', value: 'member' },
					{ label: 'Administrator (unrestricted cluster access)', value: 'admin' }
				]}
				size="sm"
			/>
		</Field>
	</div>

	{#if formRole === 'member'}
		<div class="flex flex-col gap-3 border-t border-[var(--border-subtle)] pt-4">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<div class="flex flex-col gap-0.5">
					<span class="text-xs font-medium text-[var(--text-primary)]">
						Capability permissions
					</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">
						Default is deny-all. Grant only the exact capabilities this member needs.
					</span>
				</div>
				<div class="flex flex-wrap items-center gap-1.5">
					{#each Object.entries(PRESETS) as [key, preset] (key)}
						<Button variant="secondary" size="xs" onclick={() => applyPreset(key)}>
							{preset.label}
						</Button>
					{/each}
				</div>
			</div>

			<div class="grid grid-cols-1 gap-3 md:grid-cols-3">
				{#each PERM_GROUPS as group (group.title)}
					<div
						class="flex flex-col gap-2.5 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-panel)] p-3"
					>
						<span class="text-xs font-medium text-[var(--text-secondary)]">
							{group.title}
						</span>
						{#each group.items as item (item.key)}
							<label class="flex cursor-pointer items-start gap-2 select-none">
								<Checkbox
									checked={formPerms[item.key]}
									onCheckedChange={() => togglePerm(item.key)}
									class="mt-0.5"
								/>
								<div class="flex flex-col">
									<span class="text-xs font-medium text-[var(--text-primary)]">{item.label}</span>
									<span class="text-[11px] text-[var(--text-tertiary)]">{item.hint}</span>
								</div>
							</label>
						{/each}
					</div>
				{/each}
			</div>
		</div>

		<div class="flex flex-col gap-3 border-t border-[var(--border-subtle)] pt-4">
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">
					Project &amp; service workspace scope
				</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Enable a project to grant access. Leave individual services unselected to allow the whole
					project.
				</span>
			</div>

			{#if projects.length === 0}
				<span class="text-xs text-[var(--text-tertiary)]">No user projects exist yet.</span>
			{:else}
				<div class="grid grid-cols-1 gap-2.5 md:grid-cols-2">
					{#each projects as proj (proj.id)}
						{@const sc = formScopes[proj.id] || { enabled: false, serviceIds: [] }}
						{@const projServices = services.filter((s) => s.projectId === proj.id)}
						<div
							class="flex flex-col gap-2 rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-3"
						>
							<label class="flex cursor-pointer items-center justify-between gap-2 select-none">
								<div class="flex min-w-0 items-center gap-2">
									<Checkbox
										checked={sc.enabled}
										onCheckedChange={() => toggleProjectScope(proj.id)}
									/>
									<span class="truncate text-xs font-medium text-[var(--text-primary)]">
										{proj.name}
									</span>
								</div>
								{#if sc.enabled}
									<Badge variant="secondary">
										{sc.serviceIds.length === 0
											? 'All services'
											: `${sc.serviceIds.length} selected`}
									</Badge>
								{/if}
							</label>

							{#if sc.enabled && projServices.length > 0}
								<div class="flex flex-wrap gap-1.5 border-t border-[var(--border-subtle)] pt-2">
									{#each projServices as svc (svc.id)}
										{@const svcActive = sc.serviceIds.includes(svc.id)}
										<button
											type="button"
											onclick={() => toggleServiceScope(proj.id, svc.id)}
											class="cursor-pointer rounded border px-2 py-0.5 text-[11px] {svcActive
												? 'border-[var(--accent)]/50 bg-[var(--accent-muted)] font-medium text-[var(--accent-text)]'
												: 'border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)]'}"
										>
											{svc.name}
										</button>
									{/each}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}

	{#snippet footer()}
		<Button variant="secondary" size="sm" onclick={() => (showUserModal = false)}>Cancel</Button>
		<Button variant="primary" size="sm" onclick={handleSubmit} disabled={isSaving}>
			{isSaving ? 'Saving...' : editingUser ? 'Save changes' : 'Create user'}
		</Button>
	{/snippet}
</Modal>

<ConfirmDialog
	open={deletingUser !== null}
	title="Remove user account?"
	description={deletingUser
		? `Removing "${deletingUser.email}" revokes all active sessions and project access immediately.`
		: ''}
	confirmText="Remove user"
	variant="danger"
	oncancel={() => (deletingUser = null)}
	onconfirm={confirmDelete}
/>
