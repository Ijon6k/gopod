<script lang="ts">
	import { dataStore } from '$lib/data';
	import { Button, Input } from '$lib/components/primitives';
	import { X, Key, Package, Copy, Check } from 'phosphor-svelte';

	interface Props {
		open?: boolean;
		initialTab?: 'ssh' | 'registry';
		onclose?: () => void;
		oncreated?: (type: 'ssh' | 'registry', id: string) => void;
	}

	let { open = $bindable(false), initialTab = 'ssh', onclose, oncreated }: Props = $props();

	let activeTab = $state<'ssh' | 'registry'>('ssh');

	// SSH Form
	let keyName = $state('');
	let keyType = $state<'ed25519' | 'rsa'>('ed25519');
	let generatedPublicKey = $state('ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI' + Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15) + ' gopod-deploy');
	let copied = $state(false);

	// Registry Form
	let regName = $state('');
	let regUrl = $state('docker.io');
	let regUsername = $state('');
	let regToken = $state('');

	$effect(() => {
		if (open) {
			activeTab = initialTab;
			copied = false;
		}
	});

	function handleCopy() {
		navigator.clipboard.writeText(generatedPublicKey);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	function handleSaveSSH() {
		if (!keyName.trim()) return;
		const created = dataStore.addSSHKey({
			name: keyName.trim(),
			publicKey: generatedPublicKey,
			fingerprint: `SHA256:${Math.random().toString(16).substring(2, 14)}...`,
			type: keyType
		});
		open = false;
		oncreated?.('ssh', created.id);
		onclose?.();
		keyName = '';
	}

	function handleSaveRegistry() {
		if (!regName.trim() || !regUsername.trim()) return;
		const created = dataStore.addRegistry({
			name: regName.trim(),
			url: regUrl.trim(),
			username: regUsername.trim(),
			token: regToken.trim()
		});
		open = false;
		oncreated?.('registry', created.id);
		onclose?.();
		regName = '';
		regUsername = '';
		regToken = '';
	}

	function handleClose() {
		open = false;
		onclose?.();
	}
</script>

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs"
		role="dialog"
		aria-modal="true"
	>
		<div
			class="w-full max-w-[500px] rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xl flex flex-col overflow-hidden animate-in fade-in zoom-in-95 duration-150"
		>
			<!-- Header -->
			<div class="px-5 py-4 border-b border-[var(--border)] flex items-center justify-between">
				<div class="flex items-center gap-2">
					<div class="p-1.5 rounded-[var(--radius-sm)] bg-[rgba(105,115,168,0.12)] text-[var(--accent)]">
						{#if activeTab === 'ssh'}
							<Key size={16} />
						{:else}
							<Package size={16} />
						{/if}
					</div>
					<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">
						{activeTab === 'ssh' ? 'Add SSH Deployment Key' : 'Add Container Registry'}
					</h3>
				</div>
				<button
					type="button"
					onclick={handleClose}
					class="p-1 rounded text-[var(--text-tertiary)] hover:text-[var(--text-primary)] bg-transparent border-0 cursor-pointer"
					aria-label="Close"
				>
					<X size={16} />
				</button>
			</div>

			<!-- Tab switcher -->
			<div class="px-5 pt-3 flex items-center gap-4 border-b border-[var(--border-subtle)]">
				<button
					type="button"
					onclick={() => (activeTab = 'ssh')}
					class="pb-2 text-xs font-medium border-b-2 bg-transparent border-0 cursor-pointer transition-colors {activeTab === 'ssh'
						? 'border-[var(--accent)] text-[var(--text-primary)]'
						: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Git SSH Key
				</button>
				<button
					type="button"
					onclick={() => (activeTab = 'registry')}
					class="pb-2 text-xs font-medium border-b-2 bg-transparent border-0 cursor-pointer transition-colors {activeTab === 'registry'
						? 'border-[var(--accent)] text-[var(--text-primary)]'
						: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Docker Registry
				</button>
			</div>

			<!-- Body -->
			<div class="p-5 flex flex-col gap-4">
				{#if activeTab === 'ssh'}
					<div class="flex flex-col gap-1.5">
						<label for="ssh-key-name" class="text-xs font-medium text-[var(--text-secondary)]">
							Key Name <span class="text-[var(--status-red)]">*</span>
						</label>
						<Input
							id="ssh-key-name"
							bind:value={keyName}
							placeholder="e.g. GitHub Personal or GitLab Work"
							class="text-xs"
						/>
					</div>

					<div class="flex flex-col gap-1.5">
						<div class="flex items-center justify-between">
							<span class="text-xs font-medium text-[var(--text-secondary)]">Public Key</span>
							<button
								type="button"
								onclick={handleCopy}
								class="flex items-center gap-1 text-[11px] text-[var(--accent)] hover:underline bg-transparent border-0 cursor-pointer p-0"
							>
								{#if copied}
									<Check size={12} class="text-[var(--status-green)]" /> Copied!
								{:else}
									<Copy size={12} /> Copy Public Key
								{/if}
							</button>
						</div>
						<div class="p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] font-[var(--font-mono)] text-[11px] text-[var(--text-secondary)] break-all select-all">
							{generatedPublicKey}
						</div>
						<span class="text-[11px] text-[var(--text-tertiary)]">
							Tambahkan public key ini ke repository Anda (GitHub → Settings → Deploy Keys).
						</span>
					</div>
				{:else}
					<div class="flex flex-col gap-1.5">
						<label for="reg-name" class="text-xs font-medium text-[var(--text-secondary)]">
							Registry Name <span class="text-[var(--status-red)]">*</span>
						</label>
						<Input
							id="reg-name"
							bind:value={regName}
							placeholder="e.g. My Docker Hub"
							class="text-xs"
						/>
					</div>

					<div class="grid grid-cols-2 gap-3">
						<div class="flex flex-col gap-1.5">
							<label for="reg-url" class="text-xs font-medium text-[var(--text-secondary)]">Registry Host</label>
							<select
								id="reg-url"
								bind:value={regUrl}
								class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] text-xs text-[var(--text-primary)]"
							>
								<option value="docker.io">Docker Hub (docker.io)</option>
								<option value="ghcr.io">GitHub (ghcr.io)</option>
								<option value="quay.io">Quay.io</option>
								<option value="custom">Custom Registry</option>
							</select>
						</div>

						<div class="flex flex-col gap-1.5">
							<label for="reg-username" class="text-xs font-medium text-[var(--text-secondary)]">
								Username <span class="text-[var(--status-red)]">*</span>
							</label>
							<Input
								id="reg-username"
								bind:value={regUsername}
								placeholder="username / org"
								class="text-xs"
							/>
						</div>
					</div>

					<div class="flex flex-col gap-1.5">
						<label for="reg-token" class="text-xs font-medium text-[var(--text-secondary)]">
							Password or Access Token (PAT)
						</label>
						<Input
							id="reg-token"
							type="password"
							bind:value={regToken}
							placeholder="dckr_pat_..."
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
				{/if}
			</div>

			<!-- Footer -->
			<div class="px-5 py-3 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-end gap-2">
				<Button variant="ghost" size="sm" onclick={handleClose}>Cancel</Button>
				{#if activeTab === 'ssh'}
					<Button variant="primary" size="sm" disabled={!keyName.trim()} onclick={handleSaveSSH}>
						Save SSH Key
					</Button>
				{:else}
					<Button variant="primary" size="sm" disabled={!regName.trim() || !regUsername.trim()} onclick={handleSaveRegistry}>
						Save Registry
					</Button>
				{/if}
			</div>
		</div>
	</div>
{/if}
