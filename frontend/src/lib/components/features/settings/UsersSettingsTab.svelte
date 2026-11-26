<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { projects } from '$lib/data';
	import { authStore } from '$lib/stores/auth.svelte';
	import type { AdminUserSummary } from '$lib/api/system/types';
	import {
		Button,
		Field,
		Input,
		Select,
		LayerCard,
		Badge,
		Checkbox,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		ConfirmDialog
	} from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { UsersThree, Plus, Trash, ShieldCheck, UserCircle, PencilSimple } from 'phosphor-svelte';

	const toast = useKumoToastManager();

	let users = $state<AdminUserSummary[]>([]);
	let isLoading = $state(true);
	let showCreateForm = $state(false);
	let editingUser = $state<AdminUserSummary | null>(null);
	let deletingUser = $state<AdminUserSummary | null>(null);
	let isSaving = $state(false);

	// Form fields
	let formName = $state('');
	let formEmail = $state('');
	let formPassword = $state('');
	let formRole = $state<'admin' | 'member'>('member');
	let formProjectIds = $state<string[]>([]);

	async function loadUsers() {
		isLoading = true;
		try {
			users = await api.system.listUsers();
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to load users'));
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		void loadUsers();
	});

	function openCreate() {
		editingUser = null;
		formName = '';
		formEmail = '';
		formPassword = '';
		formRole = 'member';
		formProjectIds = [];
		showCreateForm = true;
	}

	function openEdit(u: AdminUserSummary) {
		editingUser = u;
		formName = u.name;
		formEmail = u.email;
		formPassword = '';
		formRole = u.role;
		formProjectIds = [...(u.projectIds || [])];
		showCreateForm = true;
	}

	function toggleProject(id: string) {
		if (formProjectIds.includes(id)) {
			formProjectIds = formProjectIds.filter((p) => p !== id);
		} else {
			formProjectIds = [...formProjectIds, id];
		}
	}

	async function handleSubmit() {
		if (isSaving) return;
		isSaving = true;
		try {
			if (editingUser) {
				const updated = await api.system.updateUser(editingUser.id, {
					name: formName.trim(),
					role: formRole,
					...(formPassword ? { password: formPassword } : {}),
					projectIds: formRole === 'admin' ? [] : formProjectIds
				});
				users = users.map((u) => (u.id === updated.id ? updated : u));
				toast.add({
					title: 'User updated',
					description: `Updated permissions for ${updated.name}.`,
					variant: 'default'
				});
			} else {
				const created = await api.system.createUser({
					name: formName.trim(),
					email: formEmail.trim(),
					password: formPassword,
					role: formRole,
					projectIds: formRole === 'admin' ? [] : formProjectIds
				});
				users = [...users, created];
				toast.add({
					title: 'User created',
					description: `${created.name} can now sign in to GOPOD.`,
					variant: 'default'
				});
			}
			showCreateForm = false;
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to save user'));
		} finally {
			isSaving = false;
		}
	}

	async function confirmDelete() {
		if (!deletingUser) return;
		try {
			await api.system.deleteUser(deletingUser.id);
			users = users.filter((u) => u.id !== deletingUser?.id);
			toast.add({
				title: 'User removed',
				description: `Removed account ${deletingUser.email}.`,
				variant: 'default'
			});
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to delete user'));
		} finally {
			deletingUser = null;
		}
	}

	function projectNamesFor(ids: string[]): string {
		if (!ids || ids.length === 0) return 'None assigned';
		return ids.map((id) => projects.find((p) => p.id === id)?.name || id).join(', ');
	}
</script>

<div class="flex flex-col gap-6">
	<LayerCard class="flex flex-col gap-4 p-5">
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-subtle)] pb-3"
		>
			<div class="flex items-center gap-2">
				<UsersThree size={16} class="text-[var(--accent)]" />
				<div class="flex flex-col gap-0.5">
					<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
						Users &amp; Project Access
					</h3>
					<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
						Grant friends scoped member access to specific projects while keeping host engine,
						backups, and cluster credentials restricted to administrators.
					</p>
				</div>
			</div>
			<Button variant="primary" size="sm" onclick={openCreate}>
				<Plus size={14} />
				<span>Add user</span>
			</Button>
		</div>

		{#if showCreateForm}
			<div
				class="flex flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)] p-4"
			>
				<div class="flex items-center justify-between">
					<h4 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
						{editingUser ? `Edit user — ${editingUser.email}` : 'Create user account'}
					</h4>
					<Button variant="ghost" size="sm" onclick={() => (showCreateForm = false)}>Cancel</Button>
				</div>

				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
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
					<Field label={editingUser ? 'New password (optional)' : 'Password (min 8 chars)'}>
						<Input
							bind:value={formPassword}
							type="password"
							placeholder={editingUser ? 'Leave blank to keep existing' : '••••••••'}
							class="text-xs"
						/>
					</Field>
					<Field label="Role">
						<Select
							value={formRole}
							onValueChange={(v: any) => {
								if (v === 'admin' || v === 'member') formRole = v;
							}}
							options={[
								{ label: 'Member (assigned projects only)', value: 'member' },
								{ label: 'Administrator (full cluster)', value: 'admin' }
							]}
							size="sm"
						/>
					</Field>
				</div>

				{#if formRole === 'member'}
					<div class="flex flex-col gap-2">
						<span class="text-xs font-medium text-[var(--text-secondary)]">
							Accessible projects
						</span>
						{#if projects.length === 0}
							<span class="text-[11px] text-[var(--text-tertiary)]"
								>No user projects exist yet.</span
							>
						{:else}
							<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3">
								{#each projects as proj (proj.id)}
									<label
										class="flex cursor-pointer items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-2 text-xs select-none"
									>
										<Checkbox
											checked={formProjectIds.includes(proj.id)}
											onCheckedChange={() => toggleProject(proj.id)}
										/>
										<span class="truncate text-[var(--text-primary)]">{proj.name}</span>
									</label>
								{/each}
							</div>
						{/if}
					</div>
				{/if}

				<div class="flex justify-end gap-2 border-t border-[var(--border-subtle)] pt-3">
					<Button variant="secondary" size="sm" onclick={() => (showCreateForm = false)}
						>Cancel</Button
					>
					<Button variant="primary" size="sm" onclick={handleSubmit} disabled={isSaving}>
						{isSaving ? 'Saving...' : editingUser ? 'Save changes' : 'Create user'}
					</Button>
				</div>
			</div>
		{/if}

		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<Table class="w-full text-left text-[13px]">
				<TableHeader
					class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
				>
					<TableRow>
						<TableHead class="px-3.5 py-2.5 text-xs font-semibold text-[var(--text-tertiary)]"
							>User</TableHead
						>
						<TableHead class="px-3 py-2.5 text-xs font-semibold text-[var(--text-tertiary)]"
							>Role</TableHead
						>
						<TableHead class="px-3 py-2.5 text-xs font-semibold text-[var(--text-tertiary)]"
							>Project Scope</TableHead
						>
						<TableHead
							class="px-3.5 py-2.5 text-right text-xs font-semibold text-[var(--text-tertiary)]"
							>Actions</TableHead
						>
					</TableRow>
				</TableHeader>
				<TableBody class="divide-y divide-[var(--border-subtle)]">
					{#if isLoading}
						<TableRow>
							<TableCell colspan={4} class="py-6 text-center text-xs text-[var(--text-tertiary)]">
								Loading user accounts…
							</TableCell>
						</TableRow>
					{:else if users.length === 0}
						<TableRow>
							<TableCell colspan={4} class="py-6 text-center text-xs text-[var(--text-tertiary)]">
								No user accounts found.
							</TableCell>
						</TableRow>
					{:else}
						{#each users as u (u.id)}
							<TableRow class="hover:bg-[var(--bg-table-row-hover)]">
								<TableCell class="px-3.5 py-2.5">
									<div class="flex items-center gap-2">
										{#if u.role === 'admin'}
											<ShieldCheck size={15} class="text-[var(--status-green)]" />
										{:else}
											<UserCircle size={15} class="text-[var(--text-tertiary)]" />
										{/if}
										<div class="flex flex-col">
											<span class="font-medium text-[var(--text-primary)]">
												{u.name}
												{#if authStore.user?.id === u.id}
													<span class="ml-1 font-mono text-[10px] text-[var(--accent)]">(you)</span>
												{/if}
											</span>
											<span class="font-mono text-[11px] text-[var(--text-tertiary)]"
												>{u.email}</span
											>
										</div>
									</div>
								</TableCell>
								<TableCell class="px-3 py-2.5">
									<Badge variant={u.role === 'admin' ? 'success' : 'neutral'}>
										{u.role}
									</Badge>
								</TableCell>
								<TableCell class="px-3 py-2.5 text-xs text-[var(--text-secondary)]">
									{u.role === 'admin' ? 'All workspaces & host' : projectNamesFor(u.projectIds)}
								</TableCell>
								<TableCell class="px-3.5 py-2.5 text-right">
									<div class="inline-flex items-center gap-1">
										<button
											type="button"
											onclick={() => openEdit(u)}
											class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
											title="Edit permissions"
											aria-label="Edit user"
										>
											<PencilSimple size={14} />
										</button>
										{#if authStore.user?.id !== u.id}
											<button
												type="button"
												onclick={() => (deletingUser = u)}
												class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] hover:bg-[var(--status-red-muted)] hover:text-[var(--status-red)]"
												title="Delete user"
												aria-label="Delete user"
											>
												<Trash size={14} />
											</button>
										{/if}
									</div>
								</TableCell>
							</TableRow>
						{/each}
					{/if}
				</TableBody>
			</Table>
		</div>
	</LayerCard>
</div>

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
