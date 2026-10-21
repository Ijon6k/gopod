<script lang="ts">
	import { PageHeader, Tabs } from '$lib/components/ui';
	import { Button, CopyButton } from '$lib/components/ui';
	import {
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		LayerCard,
		Badge
	} from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import { Key, Package, ShieldCheck, Plus, Trash, GitBranch } from 'phosphor-svelte';
	import AddCredentialModal from '$lib/components/features/services/detail/modals/AddCredentialModal.svelte';

	let activeTab = $state<'ssh' | 'registries' | 'git' | 'secrets'>('ssh');
	let isModalOpen = $state(false);

	const tabs = [
		{ id: 'ssh', label: 'SSH Deploy Keys' },
		{ id: 'registries', label: 'Container Registries' },
		{ id: 'git', label: 'Git Access Tokens' },
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

			<LayerCard class="overflow-hidden p-0">
				<div class="w-full overflow-x-auto md:overflow-x-visible">
					<Table class="w-full text-left text-xs">
						<TableHeader
							class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
						>
							<TableRow>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Key name</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Fingerprint</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Type</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Created</TableHead
								>
								<TableHead
									class="px-4 py-3 text-right text-[12px] font-semibold text-kumo-subtle normal-case"
									>Actions</TableHead
								>
							</TableRow>
						</TableHeader>
						<TableBody class="divide-y divide-kumo-hairline">
							{#if dataStore.sshKeys.length === 0}
								<TableRow>
									<TableCell colspan="5" class="py-12 text-center text-kumo-subtle">
										No SSH deploy keys configured. Click "Add Credential" to create one.
									</TableCell>
								</TableRow>
							{:else}
								{#each dataStore.sshKeys as key (key.id)}
									<TableRow class="transition-colors hover:bg-kumo-fill">
										<TableCell
											class="flex items-center gap-2 px-4 py-3.5 font-semibold text-kumo-default"
										>
											<Key size={14} class="text-kumo-subtle" />
											<span>{key.name}</span>
										</TableCell>
										<TableCell class="px-4 py-3.5 font-mono text-[11px] text-kumo-subtle">
											{key.fingerprint}
										</TableCell>
										<TableCell class="px-4 py-3.5">
											<Badge variant="neutral" size="sm" class="font-mono uppercase">
												{key.type}
											</Badge>
										</TableCell>
										<TableCell class="px-4 py-3.5 text-kumo-subtle">
											{new Date(key.createdAt).toLocaleDateString()}
										</TableCell>
										<TableCell class="px-4 py-3.5 text-right">
											<div class="flex items-center justify-end gap-1">
												<CopyButton
													text={key.publicKey}
													title="Copy Public Key"
													variant="icon"
													size={14}
													class="text-kumo-subtle hover:text-kumo-default"
												/>
												<Button
													variant="ghost"
													size="xs"
													onclick={() => dataStore.deleteSSHKey(key.id)}
													title="Delete SSH Key"
													aria-label="Delete SSH Key"
												>
													<Trash size={14} class="text-kumo-danger" />
												</Button>
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

			<LayerCard class="overflow-hidden p-0">
				<div class="w-full overflow-x-auto md:overflow-x-visible">
					<Table class="w-full text-left text-xs">
						<TableHeader
							class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
						>
							<TableRow>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Registry name</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Registry URL</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Username</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Created</TableHead
								>
								<TableHead
									class="px-4 py-3 text-right text-[12px] font-semibold text-kumo-subtle normal-case"
									>Actions</TableHead
								>
							</TableRow>
						</TableHeader>
						<TableBody class="divide-y divide-kumo-hairline">
							{#if dataStore.registries.length === 0}
								<TableRow>
									<TableCell colspan="5" class="py-12 text-center text-kumo-subtle">
										No private registries configured. Click "Add Credential" to register one.
									</TableCell>
								</TableRow>
							{:else}
								{#each dataStore.registries as reg (reg.id)}
									<TableRow class="transition-colors hover:bg-kumo-fill">
										<TableCell
											class="flex items-center gap-2 px-4 py-3.5 font-semibold text-kumo-default"
										>
											<Package size={14} class="text-kumo-subtle" />
											<span>{reg.name}</span>
										</TableCell>
										<TableCell class="px-4 py-3.5 font-mono text-[11.5px] text-kumo-subtle">
											{reg.url}
										</TableCell>
										<TableCell class="px-4 py-3.5 font-mono text-[11px] text-kumo-subtle">
											{reg.username}
										</TableCell>
										<TableCell class="px-4 py-3.5 text-kumo-subtle">
											{new Date(reg.createdAt).toLocaleDateString()}
										</TableCell>
										<TableCell class="px-4 py-3.5 text-right">
											<Button
												variant="ghost"
												size="xs"
												onclick={() => dataStore.deleteRegistry(reg.id)}
												title="Delete Registry"
												aria-label="Delete Registry"
											>
												<Trash size={14} class="text-kumo-danger" />
											</Button>
										</TableCell>
									</TableRow>
								{/each}
							{/if}
						</TableBody>
					</Table>
				</div>
			</LayerCard>
		</div>

		<!-- 3. Git Access Tokens Tab -->
	{:else if activeTab === 'git'}
		<div class="flex flex-col gap-4">
			<div
				class="flex items-start gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)] p-3.5 text-xs text-[var(--text-secondary)]"
			>
				<div
					class="mt-0.5 shrink-0 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] p-1.5 text-[var(--accent)]"
				>
					<GitBranch size={15} />
				</div>
				<div class="flex flex-col gap-1">
					<span class="font-semibold text-[var(--text-primary)]">HTTPS Git Access Tokens</span>
					<p class="leading-relaxed text-[var(--text-tertiary)]">
						Clone private repositories over HTTPS with a personal access token. Tokens are encrypted
						at rest, never serialized, and are only used for the repository that references them.
					</p>
				</div>
			</div>

			<LayerCard class="overflow-hidden p-0">
				<div class="w-full overflow-x-auto md:overflow-x-visible">
					<Table class="w-full text-left text-xs">
						<TableHeader
							class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
						>
							<TableRow>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Token name</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Provider</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Username</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Created</TableHead
								>
								<TableHead
									class="px-4 py-3 text-right text-[12px] font-semibold text-kumo-subtle normal-case"
									>Actions</TableHead
								>
							</TableRow>
						</TableHeader>
						<TableBody class="divide-y divide-kumo-hairline">
							{#if dataStore.gitTokens.length === 0}
								<TableRow>
									<TableCell colspan="5" class="py-12 text-center text-kumo-subtle">
										No Git access tokens configured. Click "Add Credential" to create one.
									</TableCell>
								</TableRow>
							{:else}
								{#each dataStore.gitTokens as token (token.id)}
									<TableRow class="transition-colors hover:bg-kumo-fill">
										<TableCell
											class="flex items-center gap-2 px-4 py-3.5 font-semibold text-kumo-default"
										>
											<GitBranch size={14} class="text-kumo-subtle" />
											<span>{token.name}</span>
										</TableCell>
										<TableCell class="px-4 py-3.5">
											<Badge variant="neutral" size="sm" class="font-mono uppercase">
												{token.provider}
											</Badge>
										</TableCell>
										<TableCell class="px-4 py-3.5 font-mono text-[11px] text-kumo-subtle">
											{token.username}
										</TableCell>
										<TableCell class="px-4 py-3.5 text-kumo-subtle">
											{new Date(token.createdAt).toLocaleDateString()}
										</TableCell>
										<TableCell class="px-4 py-3.5 text-right">
											<Button
												variant="ghost"
												size="xs"
												onclick={() => dataStore.deleteGitToken(token.id)}
												title="Delete Git token"
												aria-label="Delete Git token"
											>
												<Trash size={14} class="text-kumo-danger" />
											</Button>
										</TableCell>
									</TableRow>
								{/each}
							{/if}
						</TableBody>
					</Table>
				</div>
			</LayerCard>
		</div>

		<!-- 4. Podman Secrets Tab -->
	{:else if activeTab === 'secrets'}
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

			<LayerCard class="overflow-hidden p-0">
				<div class="w-full overflow-x-auto md:overflow-x-visible">
					<Table class="w-full text-left text-xs">
						<TableHeader
							class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
						>
							<TableRow>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Secret name</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Storage driver</TableHead
								>
								<TableHead class="px-4 py-3 text-[12px] font-semibold text-kumo-subtle normal-case"
									>Created</TableHead
								>
								<TableHead
									class="px-4 py-3 text-right text-[12px] font-semibold text-kumo-subtle normal-case"
									>Actions</TableHead
								>
							</TableRow>
						</TableHeader>
						<TableBody class="divide-y divide-kumo-hairline">
							{#if dataStore.podmanSecrets.length === 0}
								<TableRow>
									<TableCell colspan="4" class="py-12 text-center text-kumo-subtle">
										No Podman secrets created yet. Click "Add Credential" to create one.
									</TableCell>
								</TableRow>
							{:else}
								{#each dataStore.podmanSecrets as secret (secret.name)}
									<TableRow class="transition-colors hover:bg-kumo-fill">
										<TableCell
											class="flex items-center gap-2 px-4 py-3.5 font-mono font-semibold text-kumo-default"
										>
											<ShieldCheck size={14} class="text-kumo-subtle" />
											<span>{secret.name}</span>
										</TableCell>
										<TableCell class="px-4 py-3.5">
											<Badge variant="neutral" size="sm" class="font-mono">
												{secret.driver || 'file'}
											</Badge>
										</TableCell>
										<TableCell class="px-4 py-3.5 text-kumo-subtle">
											{new Date(secret.createdAt).toLocaleDateString()}
										</TableCell>
										<TableCell class="px-4 py-3.5 text-right">
											<Button
												variant="ghost"
												size="xs"
												onclick={() => {
													dataStore.podmanSecrets = dataStore.podmanSecrets.filter(
														(s) => s.id !== secret.id
													);
												}}
												title="Delete Secret"
												aria-label="Delete Secret"
											>
												<Trash size={14} class="text-kumo-danger" />
											</Button>
										</TableCell>
									</TableRow>
								{/each}
							{/if}
						</TableBody>
					</Table>
				</div>
			</LayerCard>
		</div>
	{/if}
</div>

<!-- Modal Dialog -->
<AddCredentialModal
	bind:open={isModalOpen}
	initialTab={activeTab === 'registries' ? 'registry' : activeTab === 'git' ? 'git' : 'ssh'}
	onclose={() => (isModalOpen = false)}
/>
