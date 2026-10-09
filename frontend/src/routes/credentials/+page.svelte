<script lang="ts">
	import { PageHeader, Tabs } from '$lib/components/ui';
	import { Button, CopyButton } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { Key, Package, ShieldCheck, Plus, Trash } from 'phosphor-svelte';
	import AddCredentialModal from '$lib/components/features/services/detail/AddCredentialModal.svelte';

	let activeTab = $state<'ssh' | 'registries' | 'secrets'>('ssh');
	let isModalOpen = $state(false);

	const tabs = [
		{ id: 'ssh', label: 'SSH Deploy Keys' },
		{ id: 'registries', label: 'Container Registries' },
		{ id: 'secrets', label: 'Podman Secrets' }
	];
</script>

<svelte:head>
	<title>Credentials & Secrets — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<!-- Page Header with Primary Action -->
	<div class="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
		<PageHeader
			title="Credentials & Secrets"
			subtitle="Centralized management of SSH deploy keys, private container registries, and Podman secrets."
		/>
		<Button
			variant="primary"
			onclick={() => (isModalOpen = true)}
			class="gap-2 self-start sm:self-auto"
		>
			<Plus size={15} />
			<span>Add Credential</span>
		</Button>
	</div>

	<!-- Subnav Tabs -->
	<Tabs {tabs} bind:active={activeTab} />

	<!-- 1. SSH Deploy Keys Tab -->
	{#if activeTab === 'ssh'}
		<div class="flex flex-col gap-4">
			<div
				class="flex items-start gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)] p-3.5 text-xs text-[var(--text-secondary)]"
			>
				<div
					class="mt-0.5 shrink-0 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] p-1.5 text-[var(--accent)]"
				>
					<Key size={15} />
				</div>
				<div class="flex flex-col gap-1">
					<span class="font-semibold text-[var(--text-primary)]">Using SSH Deploy Keys</span>
					<p class="leading-relaxed text-[var(--text-tertiary)]">
						Add public keys to your GitHub, GitLab, or Gitea repositories under <strong
							class="text-[var(--text-secondary)]">Settings &gt; Deploy Keys</strong
						> (read-only) to allow GoPod to clone and build private repositories without requiring OAuth
						logins.
					</p>
				</div>
			</div>

			<div
				class="overflow-x-auto rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs md:overflow-x-visible"
			>
				<table class="w-full border-collapse text-left text-xs">
					<thead>
						<tr
							class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)] font-medium text-[var(--text-tertiary)]"
						>
							<th class="px-4 py-3">Key Name</th>
							<th class="px-4 py-3">Fingerprint</th>
							<th class="px-4 py-3">Type</th>
							<th class="px-4 py-3">Created</th>
							<th class="px-4 py-3 text-right">Actions</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border)]">
						{#if dataStore.sshKeys.length === 0}
							<tr>
								<td colspan="5" class="py-8 text-center text-[var(--text-tertiary)]">
									No SSH deploy keys configured. Click "Add Credential" to create one.
								</td>
							</tr>
						{:else}
							{#each dataStore.sshKeys as key (key.id)}
								<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)]">
									<td
										class="flex items-center gap-2 px-4 py-3.5 font-semibold text-[var(--text-primary)]"
									>
										<Key size={14} class="text-[var(--text-tertiary)]" />
										<span>{key.name}</span>
									</td>
									<td class="px-4 py-3.5 font-mono text-[11px] text-[var(--text-secondary)]">
										{key.fingerprint}
									</td>
									<td class="px-4 py-3.5">
										<span
											class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-0.5 font-mono text-[10.5px] text-[var(--text-secondary)] uppercase"
										>
											{key.type}
										</span>
									</td>
									<td class="px-4 py-3.5 text-[var(--text-tertiary)]">
										{new Date(key.createdAt).toLocaleDateString()}
									</td>
									<td class="px-4 py-3.5 text-right">
										<div class="flex items-center justify-end gap-1.5">
											<CopyButton
												text={key.publicKey}
												title="Copy Public Key"
												variant="icon"
												size={14}
												class="border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
											/>
											<button
												type="button"
												onclick={() => dataStore.deleteSSHKey(key.id)}
												class="cursor-pointer rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
												title="Delete SSH Key"
												aria-label="Delete SSH Key"
											>
												<Trash size={14} />
											</button>
										</div>
									</td>
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>
		</div>

		<!-- 2. Container Registries Tab -->
	{:else if activeTab === 'registries'}
		<div class="flex flex-col gap-4">
			<div
				class="flex items-start gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)] p-3.5 text-xs text-[var(--text-secondary)]"
			>
				<div
					class="mt-0.5 shrink-0 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] p-1.5 text-[var(--accent)]"
				>
					<Package size={15} />
				</div>
				<div class="flex flex-col gap-1">
					<span class="font-semibold text-[var(--text-primary)]"
						>Container Registry Authentication</span
					>
					<p class="leading-relaxed text-[var(--text-tertiary)]">
						Authenticate with Docker Hub, GitHub Container Registry (<code
							class="font-mono text-[var(--text-secondary)]">ghcr.io</code
						>), Quay, or private Harbor instances to pull protected container images into Podman.
					</p>
				</div>
			</div>

			<div
				class="overflow-x-auto rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs md:overflow-x-visible"
			>
				<table class="w-full border-collapse text-left text-xs">
					<thead>
						<tr
							class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)] font-medium text-[var(--text-tertiary)]"
						>
							<th class="px-4 py-3">Registry Name</th>
							<th class="px-4 py-3">Registry URL</th>
							<th class="px-4 py-3">Username</th>
							<th class="px-4 py-3">Created</th>
							<th class="px-4 py-3 text-right">Actions</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border)]">
						{#if dataStore.registries.length === 0}
							<tr>
								<td colspan="5" class="py-8 text-center text-[var(--text-tertiary)]">
									No private registries configured. Click "Add Credential" to register one.
								</td>
							</tr>
						{:else}
							{#each dataStore.registries as reg (reg.id)}
								<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)]">
									<td
										class="flex items-center gap-2 px-4 py-3.5 font-semibold text-[var(--text-primary)]"
									>
										<Package size={14} class="text-[var(--text-tertiary)]" />
										<span>{reg.name}</span>
									</td>
									<td class="px-4 py-3.5 font-mono text-[11.5px] text-[var(--text-secondary)]">
										{reg.url}
									</td>
									<td class="px-4 py-3.5 font-mono text-[11px] text-[var(--text-secondary)]">
										{reg.username}
									</td>
									<td class="px-4 py-3.5 text-[var(--text-tertiary)]">
										{new Date(reg.createdAt).toLocaleDateString()}
									</td>
									<td class="px-4 py-3.5 text-right">
										<button
											type="button"
											onclick={() => dataStore.deleteRegistry(reg.id)}
											class="cursor-pointer rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
											title="Delete Registry"
											aria-label="Delete Registry"
										>
											<Trash size={14} />
										</button>
									</td>
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>
		</div>

		<!-- 3. Podman Secrets Tab -->
	{:else}
		<div class="flex flex-col gap-4">
			<div
				class="flex items-start gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)] p-3.5 text-xs text-[var(--text-secondary)]"
			>
				<div
					class="mt-0.5 shrink-0 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] p-1.5 text-[var(--accent)]"
				>
					<ShieldCheck size={15} />
				</div>
				<div class="flex flex-col gap-1">
					<span class="font-semibold text-[var(--text-primary)]">Podman Secret Storage</span>
					<p class="leading-relaxed text-[var(--text-tertiary)]">
						Manage sensitive credentials (database passwords, API tokens, JWT secrets) stored safely
						and mounted into containers or injected into environment variables.
					</p>
				</div>
			</div>

			<div
				class="overflow-x-auto rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs md:overflow-x-visible"
			>
				<table class="w-full border-collapse text-left text-xs">
					<thead>
						<tr
							class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)] font-medium text-[var(--text-tertiary)]"
						>
							<th class="px-4 py-3">Secret Name</th>
							<th class="px-4 py-3">Storage Driver</th>
							<th class="px-4 py-3">Created</th>
							<th class="px-4 py-3 text-right">Actions</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border)]">
						{#if dataStore.podmanSecrets.length === 0}
							<tr>
								<td colspan="4" class="py-8 text-center text-[var(--text-tertiary)]">
									No Podman secrets created yet. Click "Add Credential" to create one.
								</td>
							</tr>
						{:else}
							{#each dataStore.podmanSecrets as secret (secret.name)}
								<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)]">
									<td
										class="flex items-center gap-2 px-4 py-3.5 font-mono font-semibold text-[var(--text-primary)]"
									>
										<ShieldCheck size={14} class="text-[var(--text-tertiary)]" />
										<span>{secret.name}</span>
									</td>
									<td class="px-4 py-3.5">
										<span
											class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-0.5 font-mono text-[10.5px] text-[var(--text-secondary)]"
										>
											{secret.driver || 'file'}
										</span>
									</td>
									<td class="px-4 py-3.5 text-[var(--text-tertiary)]">
										{new Date(secret.createdAt).toLocaleDateString()}
									</td>
									<td class="px-4 py-3.5 text-right">
										<button
											type="button"
											onclick={() => {
												dataStore.podmanSecrets = dataStore.podmanSecrets.filter(
													(s) => s.id !== secret.id
												);
											}}
											class="cursor-pointer rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
											title="Delete Secret"
											aria-label="Delete Secret"
										>
											<Trash size={14} />
										</button>
									</td>
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>

<!-- Modal Dialog -->
<AddCredentialModal
	bind:open={isModalOpen}
	initialTab={activeTab === 'registries' ? 'registry' : 'ssh'}
	onclose={() => (isModalOpen = false)}
/>
