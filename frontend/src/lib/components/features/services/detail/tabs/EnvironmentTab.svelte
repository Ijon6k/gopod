<script lang="ts">
	import type { Service, ServiceSecretMount, EnvVar } from '$lib/types';
	import { Input, Select, Radio } from '$lib/components/ui';
	import { CodeEditor } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { Button } from 'kumo-svelte';
	import {
		Lock,
		Plus,
		Trash,
		DownloadSimple,
		UploadSimple,
		Check,
		MagicWand,
		Eye,
		EyeSlash,
		Copy,
		WarningCircle
	} from 'phosphor-svelte';

	const defaultPlaceholder =
		'# Environment variables (KEY=value)\n# Lines starting with # are comments\nPORT=3000\nDATABASE_URL=postgres://user:pass@localhost:5432/db';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	function buildEnvTextFromService(envVars?: EnvVar[]): string {
		if (!envVars || envVars.length === 0) {
			return '';
		}
		return envVars
			.map((v) => {
				if (v.key?.startsWith('#')) {
					return v.key;
				}
				if (!v.key && !v.value) {
					return '';
				}
				return `${v.key}=${v.value}`;
			})
			.join('\n');
	}

	// Server-side redaction replaces every secret value with this sentinel; the
	// backend treats it as "keep the stored value" on save, so an editor
	// round-trip never destroys secrets.
	const ENV_SECRET_MASK = '••••••';

	let envText = $state('');
	let initialText = $state('');
	let isDirty = $derived(envText !== initialText);
	let isMasked = $state(false);
	let saveFeedback = $state(false);
	let copyFeedback = $state(false);
	let isRevealing = $state(false);
	let revealError = $state<string | null>(null);
	let fileInputRef = $state<HTMLInputElement | null>(null);
	let activeServiceId = $state<string | null>(null);

	$effect(() => {
		if (service.id !== activeServiceId) {
			activeServiceId = service.id;
			const txt = buildEnvTextFromService(service.envVars);
			envText = txt;
			initialText = txt;
			isMasked = txt.trim().length > 0;
			revealError = null;
		}
	});

	function maskedKeys(): string[] {
		return (service.envVars ?? [])
			.filter((v) => v.secret && v.value === ENV_SECRET_MASK)
			.map((v) => v.key)
			.filter((k) => k !== '');
	}

	// Reveal swaps real values in for sentinel lines in both the editor text and
	// its dirty baseline, so revealing never marks the editor dirty by itself.
	async function revealValues() {
		const keys = maskedKeys();
		if (keys.length === 0) return;
		isRevealing = true;
		revealError = null;
		try {
			const values = await api.services.revealEnv(service.id, keys);
			const swap = (text: string) =>
				text
					.split('\n')
					.map((line) => {
						const eq = line.indexOf('=');
						if (eq === -1) return line;
						const key = line.slice(0, eq).trim();
						if (line.slice(eq + 1) === ENV_SECRET_MASK && values[key] !== undefined) {
							return `${line.slice(0, eq)}=${values[key]}`;
						}
						return line;
					})
					.join('\n');
			envText = swap(envText);
			initialText = swap(initialText);
		} catch (err: any) {
			revealError = err?.message || 'Failed to reveal secret values';
		} finally {
			isRevealing = false;
		}
	}

	async function toggleMask() {
		if (isMasked) {
			await revealValues();
			isMasked = false;
		} else {
			isMasked = true;
		}
	}

	let variableCount = $derived.by(() => {
		return envText
			.split('\n')
			.filter((l) => l.trim() && !l.trim().startsWith('#') && l.includes('=')).length;
	});

	async function saveEnv() {
		const lines = envText.split('\n');
		const parsed: EnvVar[] = [];
		for (const rawLine of lines) {
			const trimmed = rawLine.trim();
			if (!trimmed) {
				parsed.push({ key: '', value: '', secret: false });
				continue;
			}
			if (trimmed.startsWith('#')) {
				parsed.push({ key: rawLine, value: '', secret: false });
				continue;
			}
			const eq = rawLine.indexOf('=');
			if (eq !== -1) {
				const key = rawLine.slice(0, eq).trim();
				const value = rawLine.slice(eq + 1);
				if (key) {
					parsed.push({
						key,
						value,
						secret: true
					});
				}
			} else {
				parsed.push({ key: rawLine, value: '', secret: false });
			}
		}

		service.envVars = parsed;
		initialText = envText;

		try {
			await dataStore.updateService(service);
		} catch (err) {
			console.error('Failed to persist env vars:', err);
		}
		saveFeedback = true;
		setTimeout(() => {
			saveFeedback = false;
		}, 1800);
	}

	function formatEnv() {
		envText = envText
			.split('\n')
			.map((l) => {
				const t = l.trim();
				if (!t || t.startsWith('#')) return l;
				const eq = l.indexOf('=');
				if (eq === -1) return l;
				return `${l.slice(0, eq).trim().toUpperCase()}=${l.slice(eq + 1)}`;
			})
			.join('\n');
	}

	function handleImportFile(e: Event) {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;

		const reader = new FileReader();
		reader.onload = (evt) => {
			if (typeof evt.target?.result === 'string') {
				envText = evt.target.result;
			}
		};
		reader.readAsText(file);
		target.value = '';
	}

	async function handleExportFile() {
		// The editor holds sentinel values until revealed; an export is an
		// explicit request for the real content, so reveal first.
		await revealValues();
		const blob = new Blob([envText], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${service.name}.env`;
		a.click();
		URL.revokeObjectURL(url);
	}

	async function handleCopyAll() {
		await revealValues();
		try {
			await navigator.clipboard.writeText(envText);
			copyFeedback = true;
			setTimeout(() => {
				copyFeedback = false;
			}, 1800);
		} catch (err) {
			console.error('Failed to copy to clipboard:', err);
		}
	}

	// ── Podman Secrets Management ──
	let secretMounts = $state<ServiceSecretMount[]>([]);

	$effect(() => {
		secretMounts = service.secretMounts ?? [];
	});

	let isAddingSecret = $state(false);
	let selectedSecretId = $state(dataStore.podmanSecrets[0]?.id ?? '');
	let secretMountType = $state<'env' | 'file'>('env');
	let customEnvVar = $state('');
	let customFilePath = $state('');

	let currentSelectedSecret = $derived(
		dataStore.podmanSecrets.find((s) => s.id === selectedSecretId) ?? dataStore.podmanSecrets[0]
	);

	async function addSecretMount() {
		if (!currentSelectedSecret) return;
		const mount: ServiceSecretMount = {
			secretId: currentSelectedSecret.id,
			secretName: currentSelectedSecret.name,
			type: secretMountType,
			envVar:
				secretMountType === 'env'
					? customEnvVar.trim() || currentSelectedSecret.name.toUpperCase()
					: undefined,
			mountPath:
				secretMountType === 'file'
					? customFilePath.trim() || `/run/secrets/${currentSelectedSecret.name}`
					: undefined
		};
		secretMounts.push(mount);
		service.secretMounts = secretMounts;
		isAddingSecret = false;
		customEnvVar = '';
		customFilePath = '';
		try {
			await dataStore.updateService(service);
		} catch (err) {
			console.error('Failed to persist secret mounts:', err);
		}
	}

	async function removeSecretMount(index: number) {
		secretMounts.splice(index, 1);
		service.secretMounts = secretMounts;
		try {
			await dataStore.updateService(service);
		} catch (err) {
			console.error('Failed to persist secret mounts removal:', err);
		}
	}
</script>

<div class="flex w-full flex-col gap-8">
	<!-- ══════════════════════════════════════════════════════════════
	     1. ENVIRONMENT VARIABLES (Dokploy-Style Text Editor with Mask)
	     ══════════════════════════════════════════════════════════════ -->
	<section class="flex flex-col gap-3">
		{#if service.envDecryptionError}
			<div
				class="flex items-start gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-kumo-warning/10 p-3"
			>
				<WarningCircle size={16} weight="fill" class="mt-0.5 shrink-0 text-kumo-warning" />
				<div class="flex flex-col gap-0.5">
					<span class="text-xs font-medium text-kumo-warning">
						Stored environment values cannot be decrypted
					</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">
						The current master encryption key does not match the one that stored these values. Saving with
						the editor left unchanged is refused to protect the ciphertext — type a full replacement list to
						overwrite them.
					</span>
				</div>
			</div>
		{/if}
		{#if revealError}
			<div class="flex items-center gap-2 text-[11px] text-kumo-warning">
				<WarningCircle size={13} weight="fill" />
				<span>{revealError}</span>
			</div>
		{/if}
		<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
			<div class="flex items-center gap-3">
				<h2 class="m-0 text-sm font-medium text-[var(--text-primary)]">Environment Variables</h2>
				<span class="text-xs text-[var(--text-tertiary)] tabular-nums">
					{variableCount}
					{variableCount === 1 ? 'variable' : 'variables'}
				</span>
			</div>

			<!-- Actions Toolbar (Native Kumo Button Primitives - No Tooltip Interference) -->
			<div class="flex flex-wrap items-center gap-2">
				<!-- Mask Toggle Button -->
				<Button
					variant="secondary"
					size="sm"
					onclick={toggleMask}
					disabled={isRevealing}
					class="gap-1.5"
				>
					{#if isMasked}
						<Eye size={14} />
						<span>{isRevealing ? 'Revealing…' : 'Show Values'}</span>
					{:else}
						<EyeSlash size={14} />
						<span>Hide Values</span>
					{/if}
				</Button>

				<!-- Copy All Button -->
				<Button variant="secondary" size="sm" onclick={handleCopyAll} class="gap-1.5">
					{#if copyFeedback}
						<Check size={14} class="text-[var(--accent)]" />
						<span>Copied!</span>
					{:else}
						<Copy size={14} />
						<span>Copy</span>
					{/if}
				</Button>

				<!-- Hidden File Input for Native Import Button -->
				<input
					bind:this={fileInputRef}
					type="file"
					accept=".env,.txt"
					onchange={handleImportFile}
					class="sr-only"
				/>

				<Button variant="secondary" size="sm" onclick={() => fileInputRef?.click()} class="gap-1.5">
					<UploadSimple size={14} />
					<span>Import</span>
				</Button>

				<Button variant="secondary" size="sm" onclick={handleExportFile} class="gap-1.5">
					<DownloadSimple size={14} />
					<span>Export</span>
				</Button>

				<Button variant="secondary" size="sm" onclick={formatEnv} class="gap-1.5">
					<MagicWand size={14} />
					<span>Format</span>
				</Button>

				<Button
					variant="primary"
					size="sm"
					disabled={!isDirty && !saveFeedback}
					onclick={saveEnv}
					class="gap-1.5"
				>
					{#if saveFeedback}
						<Check size={14} class="text-white" />
						<span>Saved</span>
					{:else}
						<span>Save .env</span>
					{/if}
				</Button>
			</div>
		</div>

		<!-- ── Text Editor Container with Dokploy Masking ── -->
		<div
			class="relative overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]"
		>
			<!-- Blurred Editor View when Masked -->
			<div
				class={isMasked && envText.trim().length > 0
					? 'pointer-events-none blur-[5px] filter transition-all duration-200 select-none'
					: 'transition-all duration-200'}
			>
				<CodeEditor
					bind:value={envText}
					language="env"
					height="80dvh"
					placeholder={defaultPlaceholder}
				/>
			</div>

			<!-- Dokploy-style Overlay Badge when Masked -->
			{#if isMasked && envText.trim().length > 0}
				<div
					class="absolute inset-0 z-10 flex flex-col items-center justify-center bg-black/25 backdrop-blur-[1px] transition-all"
				>
					<button
						type="button"
						onclick={toggleMask}
						class="flex cursor-pointer items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-4 py-2.5 text-xs font-medium text-[var(--text-primary)] shadow-md transition-all hover:border-[var(--accent)] hover:bg-[var(--bg-surface)]"
					>
						<Eye size={15} class="text-[var(--accent)]" />
						<span>Values are hidden · Click to reveal and edit</span>
					</button>
				</div>
			{/if}
		</div>

		<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
			Environment variables are loaded into runtime containers as <code>KEY=value</code> pairs. Direct
			copy-paste supported. Toggle "Show Values" to inspect or edit hidden tokens.
		</p>
	</section>

	<!-- ══════════════════════════════════════════════════════════════
	     2. PODMAN SECRETS (Visually separate from standard env vars)
	     ══════════════════════════════════════════════════════════════ -->
	<section
		class="flex flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5"
	>
		<div class="flex items-start justify-between">
			<div class="flex flex-col gap-0.5">
				<div class="flex items-center gap-2">
					<Lock size={15} class="text-[var(--accent)]" />
					<h3 class="m-0 text-sm font-medium text-[var(--text-primary)]">Podman Secrets</h3>
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">
					Native <code>podman secret</code> objects mounted securely in runtime memory or exposed as environment
					variables.
				</span>
			</div>

			{#if !isAddingSecret}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => (isAddingSecret = true)}
					class="gap-1.5"
				>
					<Plus size={14} /> <span>Mount Secret</span>
				</Button>
			{/if}
		</div>

		<!-- Add Secret Mount Form -->
		{#if isAddingSecret}
			<div
				class="flex flex-col gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-4"
			>
				<span class="text-xs font-medium text-[var(--text-primary)]"
					>Select Secret & Mount Type</span
				>

				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<div class="flex flex-col gap-1.5">
						<label for="secret-select" class="text-xs font-medium text-[var(--text-secondary)]"
							>Available Podman Secret</label
						>
						<Select
							id="secret-select"
							value={selectedSecretId}
							onValueChange={(val: any) => {
								if (typeof val === 'string') selectedSecretId = val;
							}}
							options={dataStore.podmanSecrets.map((s) => ({
								label: `${s.name} (stored secret)`,
								value: s.id
							}))}
							size="sm"
						/>
					</div>

					<div class="flex flex-col gap-1.5">
						<span class="text-xs font-medium text-[var(--text-secondary)]">Mount Exposure</span>
						<Radio
							value={secretMountType}
							onValueChange={(val: any) => {
								if (val === 'env' || val === 'file') secretMountType = val;
							}}
							options={[
								{ label: 'Environment Variable', value: 'env' },
								{ label: 'File Mount (/run/secrets)', value: 'file' }
							]}
							orientation="horizontal"
						/>
					</div>
				</div>

				{#if secretMountType === 'env'}
					<div class="flex flex-col gap-1">
						<label for="custom-env-name" class="text-xs font-medium text-[var(--text-secondary)]">
							Environment Variable Name (defaults to {currentSelectedSecret?.name.toUpperCase()})
						</label>
						<Input
							id="custom-env-name"
							bind:value={customEnvVar}
							placeholder={currentSelectedSecret?.name.toUpperCase()}
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
				{:else}
					<div class="flex flex-col gap-1">
						<label for="custom-file-path" class="text-xs font-medium text-[var(--text-secondary)]">
							Target Path in Container (defaults to /run/secrets/{currentSelectedSecret?.name})
						</label>
						<Input
							id="custom-file-path"
							bind:value={customFilePath}
							placeholder={`/run/secrets/${currentSelectedSecret?.name}`}
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
				{/if}

				<div class="flex items-center justify-end gap-2 border-t border-[var(--border)] pt-2">
					<Button variant="ghost" size="sm" onclick={() => (isAddingSecret = false)}>Cancel</Button>
					<Button variant="primary" size="sm" onclick={addSecretMount}>Attach Secret</Button>
				</div>
			</div>
		{/if}

		<!-- Active Secret Mounts List -->
		<div
			class="flex flex-col divide-y divide-[var(--border)] overflow-hidden rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)]"
		>
			{#each secretMounts as sm, idx}
				<div class="flex items-center justify-between gap-4 p-3.5">
					<div class="flex min-w-0 items-center gap-3">
						<span
							class="flex h-6 w-6 items-center justify-center rounded bg-[rgba(105,115,168,0.12)] text-[var(--accent)]"
						>
							<Lock size={12} />
						</span>
						<div class="flex flex-col gap-0.5">
							<span class="text-xs font-[var(--font-mono)] font-medium text-[var(--text-primary)]">
								{sm.secretName}
							</span>
							<span class="text-[11px] text-[var(--text-tertiary)]">
								{#if sm.type === 'env'}
									Exposed as variable <code class="text-[var(--text-secondary)]">{sm.envVar}</code>
								{:else}
									Mounted at <code class="text-[var(--text-secondary)]">{sm.mountPath}</code>
								{/if}
							</span>
						</div>
					</div>

					<button
						type="button"
						onclick={() => removeSecretMount(idx)}
						class="cursor-pointer border-0 bg-transparent p-1 text-[var(--text-tertiary)] hover:text-red-400"
						title="Detach secret"
					>
						<Trash size={14} />
					</button>
				</div>
			{/each}

			{#if secretMounts.length === 0}
				<div class="p-6 text-center text-xs text-[var(--text-tertiary)]">
					No Podman secrets attached to this service.
				</div>
			{/if}
		</div>
	</section>
</div>
