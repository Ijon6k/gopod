<script lang="ts">
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { sshKeySchema, registrySchema, gitTokenSchema } from '$lib/schemas';
	import {
		Button,
		Input,
		CopyButton,
		Field,
		Select,
		SensitiveInput,
		Modal
	} from '$lib/components/ui';
	import { Key, Package, ArrowsClockwise, GitBranch } from 'phosphor-svelte';

	interface Props {
		open?: boolean;
		initialTab?: 'ssh' | 'registry' | 'git';
		onclose?: () => void;
		oncreated?: (type: 'ssh' | 'registry' | 'git', id: string) => void;
	}

	let { open = $bindable(false), initialTab = 'ssh', onclose, oncreated }: Props = $props();

	let activeTab = $state<'ssh' | 'registry' | 'git'>('ssh');
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

	// Git token form
	let gitName = $state('');
	let gitProvider = $state<'github' | 'gitlab' | 'gitea' | 'custom'>('github');
	let gitUsername = $state('x-access-token');
	let gitToken = $state('');

	function applyProviderDefaults(provider: 'github' | 'gitlab' | 'gitea' | 'custom') {
		gitProvider = provider;
		if (provider === 'github') gitUsername = 'x-access-token';
		else if (provider === 'gitlab' || provider === 'gitea') gitUsername = 'oauth2';
		else gitUsername = '';
	}

	async function generateRealKey() {
		isGenerating = true;
		errorMessage = '';
		try {
			const res = await api.credentials.sshKeys.generate(keyName.trim() || 'deploy-key');
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

	async function handleSaveGitToken() {
		errorMessage = '';
		const validation = gitTokenSchema.safeParse({
			name: gitName.trim(),
			provider: gitProvider,
			username: gitUsername.trim(),
			token: gitToken.trim()
		});

		if (!validation.success) {
			errorMessage = validation.error.issues[0]?.message || 'Validation error';
			return;
		}

		try {
			const created = await dataStore.addGitToken({
				name: gitName.trim(),
				provider: gitProvider,
				username: gitUsername.trim(),
				token: gitToken.trim()
			});
			open = false;
			oncreated?.('git', created.id);
			onclose?.();
			gitName = '';
			gitToken = '';
		} catch (err: any) {
			errorMessage = err.message || 'Failed to save Git access token';
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
	{:else if activeTab === 'registry'}
		<Button
			variant="primary"
			size="sm"
			disabled={!regName.trim() || !regUsername.trim()}
			onclick={handleSaveRegistry}
		>
			Save Registry
		</Button>
	{:else}
		<Button
			variant="primary"
			size="sm"
			disabled={!gitName.trim() || !gitToken.trim()}
			onclick={handleSaveGitToken}
		>
			Save Git Token
		</Button>
	{/if}
{/snippet}

<Modal
	{open}
	onclose={handleClose}
	title={activeTab === 'ssh'
		? 'Add SSH Deployment Key'
		: activeTab === 'registry'
			? 'Add Container Registry'
			: 'Add Git Access Token'}
	icon={activeTab === 'ssh' ? Key : activeTab === 'registry' ? Package : GitBranch}
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
		<button
			type="button"
			onclick={() => (activeTab = 'git')}
			class={`cursor-pointer border-0 border-b-2 bg-transparent pb-1 text-xs font-medium transition-colors ${activeTab === 'git' ? 'border-[var(--accent)] text-[var(--text-primary)]' : 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}`}
		>
			Git Token
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
		<Field label="Key Name" required>
			<Input
				id="ssh-key-name"
				bind:value={keyName}
				placeholder="e.g. GitHub Personal or GitLab Work"
				class="text-xs"
			/>
		</Field>

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
	{:else if activeTab === 'registry'}
		<Field label="Registry Name" required>
			<Input id="reg-name" bind:value={regName} placeholder="e.g. My Docker Hub" class="text-xs" />
		</Field>

		<div class="grid grid-cols-2 gap-3">
			<Select
				label="Registry Host"
				value={regUrl}
				onValueChange={(val) => {
					if (typeof val === 'string') regUrl = val;
				}}
				options={[
					{ label: 'Docker Hub (docker.io)', value: 'docker.io' },
					{ label: 'GitHub (ghcr.io)', value: 'ghcr.io' },
					{ label: 'Quay.io', value: 'quay.io' },
					{ label: 'Custom Registry', value: 'custom' }
				]}
				size="sm"
			/>

			<Field label="Username" required>
				<Input
					id="reg-username"
					bind:value={regUsername}
					placeholder="username / org"
					class="text-xs"
				/>
			</Field>
		</div>

		<Field label="Password or Access Token (PAT)">
			<SensitiveInput
				bind:value={regToken}
				placeholder="dckr_pat_..."
				size="sm"
				class="text-xs font-[var(--font-mono)]"
			/>
		</Field>
	{:else}
		<Field label="Token Name" required>
			<Input
				id="git-token-name"
				bind:value={gitName}
				placeholder="e.g. GitHub deploy token"
				class="text-xs"
			/>
		</Field>

		<div class="grid grid-cols-2 gap-3">
			<Select
				label="Provider"
				value={gitProvider}
				onValueChange={(val) => {
					if (val === 'github' || val === 'gitlab' || val === 'gitea' || val === 'custom') {
						applyProviderDefaults(val);
					}
				}}
				options={[
					{ label: 'GitHub', value: 'github' },
					{ label: 'GitLab', value: 'gitlab' },
					{ label: 'Gitea', value: 'gitea' },
					{ label: 'Custom Git server', value: 'custom' }
				]}
				size="sm"
			/>

			<Field label="Username" required={gitProvider === 'custom'}>
				<Input
					id="git-token-username"
					bind:value={gitUsername}
					placeholder={gitProvider === 'custom'
						? 'git username'
						: 'defaults to the provider identity'}
					class="text-xs"
				/>
			</Field>
		</div>

		<Field label="Personal Access Token" required>
			<SensitiveInput
				bind:value={gitToken}
				placeholder="ghp_... / glpat-... / gitea token"
				size="sm"
				class="text-xs font-[var(--font-mono)]"
			/>
		</Field>
		<span class="text-[11px] text-[var(--text-tertiary)]">
			Read-only repository scope is sufficient. The token is encrypted at rest and never displayed
			again.
		</span>
	{/if}
</Modal>
