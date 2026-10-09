<script lang="ts">
	import { dataStore } from '$lib/data';
	import { credentialsApi } from '$lib/api/credentials';
	import { sshKeySchema, registrySchema } from '$lib/schemas';
	import { Button, Input, CopyButton, FormField } from '$lib/components/primitives';
	import { Modal } from '$lib/components/ui';
	import { Key, Package, ArrowsClockwise } from 'phosphor-svelte';

	interface Props {
		open?: boolean;
		initialTab?: 'ssh' | 'registry';
		onclose?: () => void;
		oncreated?: (type: 'ssh' | 'registry', id: string) => void;
	}

	let { open = $bindable(false), initialTab = 'ssh', onclose, oncreated }: Props = $props();

	let activeTab = $state<'ssh' | 'registry'>('ssh');
	let errorMessage = $state('');
	let isGenerating = $state(false);

	// SSH Form
	let keyName = $state('');
	let keyType = $state<'ed25519' | 'rsa'>('ed25519');
	let generatedPublicKey = $state('');
	let generatedPrivateKey = $state('');
	let generatedFingerprint = $state('');

	// Registry Form
	let regName = $state('');
	let regUrl = $state('docker.io');
	let regUsername = $state('');
	let regToken = $state('');

	async function generateRealKey() {
		isGenerating = true;
		errorMessage = '';
		try {
			const res = await credentialsApi.sshKeys.generate(keyName.trim() || 'deploy-key');
			generatedPublicKey = res.publicKey;
			generatedPrivateKey = res.privateKey || '';
			generatedFingerprint = res.fingerprint;
		} catch (err: any) {
			errorMessage = err.message || 'Failed to generate real SSH keypair';
		} finally {
			isGenerating = false;
		}
	}

	$effect(() => {
		if (open) {
			activeTab = initialTab;
			errorMessage = '';
			if (!generatedPublicKey && activeTab === 'ssh') {
				generateRealKey();
			}
		}
	});

	async function handleSaveSSH() {
		errorMessage = '';
		const validation = sshKeySchema.safeParse({
			name: keyName.trim(),
			publicKey: generatedPublicKey,
			privateKey: generatedPrivateKey,
			type: keyType
		});

		if (!validation.success) {
			errorMessage = validation.error.issues[0]?.message || 'Validation error';
			return;
		}

		try {
			const created = await dataStore.addSSHKey({
				name: keyName.trim(),
				publicKey: generatedPublicKey,
				privateKey: generatedPrivateKey,
				fingerprint: generatedFingerprint,
				type: keyType
			});
			open = false;
			oncreated?.('ssh', created.id);
			onclose?.();
			keyName = '';
			generatedPublicKey = '';
		} catch (err: any) {
			errorMessage = err.message || 'Failed to save SSH key';
		}
	}

	async function handleSaveRegistry() {
		errorMessage = '';
		const validation = registrySchema.safeParse({
			name: regName.trim(),
			url: regUrl.trim(),
			username: regUsername.trim(),
			token: regToken.trim()
		});

		if (!validation.success) {
			errorMessage = validation.error.issues[0]?.message || 'Validation error';
			return;
		}

		try {
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
		} catch (err: any) {
			errorMessage = err.message || 'Failed to save registry';
		}
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
		<Button
			variant="primary"
			size="sm"
			disabled={!regName.trim() || !regUsername.trim()}
			onclick={handleSaveRegistry}
		>
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
	<div class="-mt-2 flex items-center gap-4 border-b border-[var(--border-subtle)] pb-2">
		<button
			type="button"
			onclick={() => (activeTab = 'ssh')}
			class={`cursor-pointer border-0 border-b-2 bg-transparent pb-1 text-xs font-medium transition-colors ${activeTab === 'ssh' ? 'border-[var(--accent)] text-[var(--text-primary)]' : 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}`}
		>
			Git SSH Key
		</button>
		<button
			type="button"
			onclick={() => (activeTab = 'registry')}
			class={`cursor-pointer border-0 border-b-2 bg-transparent pb-1 text-xs font-medium transition-colors ${activeTab === 'registry' ? 'border-[var(--accent)] text-[var(--text-primary)]' : 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}`}
		>
			Docker Registry
		</button>
	</div>

	{#if errorMessage}
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--status-rose)]/30 bg-[var(--status-rose)]/10 p-2.5 text-[11px] text-[var(--status-rose)]"
		>
			{errorMessage}
		</div>
	{/if}

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
				<span class="text-xs font-medium text-[var(--text-secondary)]">Public Key (Ed25519)</span>
				<div class="flex items-center gap-2">
					<button
						type="button"
						onclick={generateRealKey}
						disabled={isGenerating}
						class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline"
						title="Generate new keypair"
					>
						<ArrowsClockwise size={12} class={isGenerating ? 'animate-spin' : ''} />
						<span>{isGenerating ? 'Generating...' : 'Regenerate'}</span>
					</button>
					<CopyButton text={generatedPublicKey} label="Copy" variant="inline" />
				</div>
			</div>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-2.5 text-[11px] font-[var(--font-mono)] break-all text-[var(--text-secondary)] select-all"
			>
				{generatedPublicKey ||
					(isGenerating ? 'Generating real Ed25519 keypair...' : 'Click regenerate to create key')}
			</div>
			{#if generatedFingerprint}
				<div class="font-mono text-[10px] text-[var(--text-tertiary)]">
					Fingerprint: {generatedFingerprint}
				</div>
			{/if}
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Add this public key to your repository (GitHub → Settings → Deploy Keys).
			</span>
		</div>
	{:else}
		<FormField label="Registry Name" required forId="reg-name">
			<Input id="reg-name" bind:value={regName} placeholder="e.g. My Docker Hub" class="text-xs" />
		</FormField>

		<div class="grid grid-cols-2 gap-3">
			<FormField label="Registry Host" forId="reg-url">
				<select
					id="reg-url"
					bind:value={regUrl}
					class="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs text-[var(--text-primary)]"
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
