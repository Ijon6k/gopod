<script lang="ts">
	import { dataStore } from '$lib/data';
	import { Button, Input, CopyButton, FormField } from '$lib/components/primitives';
	import { Modal } from '$lib/components/ui';
	import { Key, Package } from 'phosphor-svelte';

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

	// Registry Form
	let regName = $state('');
	let regUrl = $state('docker.io');
	let regUsername = $state('');
	let regToken = $state('');

	$effect(() => {
		if (open) {
			activeTab = initialTab;
		}
	});

	async function handleSaveSSH() {
		if (!keyName.trim()) return;
		const created = await dataStore.addSSHKey({
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

	async function handleSaveRegistry() {
		if (!regName.trim() || !regUsername.trim()) return;
		const created = await dataStore.addRegistry({
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

{#snippet modalFooter()}
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
{/snippet}

<Modal
	{open}
	onclose={handleClose}
	title={activeTab === 'ssh' ? 'Add SSH Deployment Key' : 'Add Container Registry'}
	icon={activeTab === 'ssh' ? Key : Package}
	size="md"
	footer={modalFooter}
>
	<!-- Tab switcher -->
	<div class="flex items-center gap-4 border-b border-[var(--border-subtle)] -mt-2 pb-2">
		<button
			type="button"
			onclick={() => (activeTab = 'ssh')}
			class="pb-1 text-xs font-medium border-b-2 bg-transparent border-0 cursor-pointer transition-colors {activeTab === 'ssh'
				? 'border-[var(--accent)] text-[var(--text-primary)]'
				: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
		>
			Git SSH Key
		</button>
		<button
			type="button"
			onclick={() => (activeTab = 'registry')}
			class="pb-1 text-xs font-medium border-b-2 bg-transparent border-0 cursor-pointer transition-colors {activeTab === 'registry'
				? 'border-[var(--accent)] text-[var(--text-primary)]'
				: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
		>
			Docker Registry
		</button>
	</div>

	{#if activeTab === 'ssh'}
		<FormField label="Key Name" required forId="ssh-key-name">
			<Input
				id="ssh-key-name"
				bind:value={keyName}
				placeholder="e.g. GitHub Personal or GitLab Work"
				class="text-xs"
			/>
		</FormField>

		<div class="flex flex-col gap-1.5">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Public Key</span>
				<CopyButton
					text={generatedPublicKey}
					label="Copy Public Key"
					variant="inline"
				/>
			</div>
			<div class="p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] font-[var(--font-mono)] text-[11px] text-[var(--text-secondary)] break-all select-all">
				{generatedPublicKey}
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Add this public key to your repository (GitHub → Settings → Deploy Keys).
			</span>
		</div>
	{:else}
		<FormField label="Registry Name" required forId="reg-name">
			<Input
				id="reg-name"
				bind:value={regName}
				placeholder="e.g. My Docker Hub"
				class="text-xs"
			/>
		</FormField>

		<div class="grid grid-cols-2 gap-3">
			<FormField label="Registry Host" forId="reg-url">
				<select
					id="reg-url"
					bind:value={regUrl}
					class="w-full px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] text-xs text-[var(--text-primary)]"
				>
					<option value="docker.io">Docker Hub (docker.io)</option>
					<option value="ghcr.io">GitHub (ghcr.io)</option>
					<option value="quay.io">Quay.io</option>
					<option value="custom">Custom Registry</option>
				</select>
			</FormField>

			<FormField label="Username" required forId="reg-username">
				<Input
					id="reg-username"
					bind:value={regUsername}
					placeholder="username / org"
					class="text-xs"
				/>
			</FormField>
		</div>

		<FormField label="Password or Access Token (PAT)" forId="reg-token">
			<Input
				id="reg-token"
				type="password"
				bind:value={regToken}
				placeholder="dckr_pat_..."
				class="text-xs font-[var(--font-mono)]"
			/>
		</FormField>
	{/if}
</Modal>
