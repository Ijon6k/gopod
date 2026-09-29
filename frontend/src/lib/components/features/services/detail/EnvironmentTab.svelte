<script lang="ts">
	import type { Service, ServiceSecretMount, EnvVar } from '$lib/types';
	import { Button, Input } from '$lib/components/primitives';
	import { CodeEditor } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { Lock, Plus, Trash, DownloadSimple, UploadSimple, Check, MagicWand } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	function buildEnvText(envVars: EnvVar[]): string {
		if (!envVars || envVars.length === 0) {
			return '# Environment variables (KEY=value)\nNODE_ENV=production\nPORT=3000\nLOG_LEVEL=info';
		}
		return envVars.map((v) => `${v.key}=${v.value}`).join('\n');
	}

	let envText = $state('');
	let initialText = $state('');
	let isDirty = $derived(envText !== initialText);
	let saveFeedback = $state(false);

	$effect(() => {
		const txt = buildEnvText(service.envVars);
		if (!initialText) {
			envText = txt;
			initialText = txt;
		}
	});

	let variableCount = $derived.by(() => {
		return envText
			.split('\n')
			.filter((l) => l.trim() && !l.trim().startsWith('#') && l.includes('=')).length;
	});

	function saveEnv() {
		const lines = envText.split('\n');
		const parsed: EnvVar[] = [];
		for (const line of lines) {
			const trimmed = line.trim();
			if (!trimmed || trimmed.startsWith('#')) continue;
			const eq = trimmed.indexOf('=');
			if (eq !== -1) {
				const key = trimmed.slice(0, eq).trim();
				const value = trimmed.slice(eq + 1).trim();
				if (key) {
					parsed.push({
						key,
						value,
						secret: /secret|password|token|key/i.test(key)
					});
				}
			}
		}
		service.envVars = parsed;
		initialText = envText;
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
				if (!t || t.startsWith('#')) return t;
				const eq = t.indexOf('=');
				if (eq === -1) return t;
				return `${t.slice(0, eq).trim()}=${t.slice(eq + 1).trim()}`;
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
	}

	function handleExportFile() {
		const blob = new Blob([envText], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${service.name}.env`;
		a.click();
		URL.revokeObjectURL(url);
	}

	// ── Podman Secrets Management ──
	let secretMounts = $state<ServiceSecretMount[]>([]);

	$effect(() => {
		secretMounts = service.secretMounts ?? [
			{ secretId: 'sec-2', secretName: 'jwt_secret', type: 'env', envVar: 'JWT_SECRET' }
		];
	});

	let isAddingSecret = $state(false);
	let selectedSecretId = $state(dataStore.podmanSecrets[0]?.id ?? '');
	let secretMountType = $state<'env' | 'file'>('env');
	let customEnvVar = $state('');
	let customFilePath = $state('');

	let currentSelectedSecret = $derived(
		dataStore.podmanSecrets.find((s) => s.id === selectedSecretId) ?? dataStore.podmanSecrets[0]
	);

	function addSecretMount() {
		if (!currentSelectedSecret) return;
		const mount: ServiceSecretMount = {
			secretId: currentSelectedSecret.id,
			secretName: currentSelectedSecret.name,
			type: secretMountType,
			envVar: secretMountType === 'env' ? (customEnvVar.trim() || currentSelectedSecret.name.toUpperCase()) : undefined,
			mountPath: secretMountType === 'file' ? (customFilePath.trim() || `/run/secrets/${currentSelectedSecret.name}`) : undefined
		};
		secretMounts.push(mount);
		service.secretMounts = secretMounts;
		isAddingSecret = false;
		customEnvVar = '';
		customFilePath = '';
	}

	function removeSecretMount(index: number) {
		secretMounts.splice(index, 1);
		service.secretMounts = secretMounts;
	}
</script>

<div class="w-full flex flex-col gap-8">
	<!-- ══════════════════════════════════════════════════════════════
	     1. TEXT-FIRST .ENV EDITOR
	     ══════════════════════════════════════════════════════════════ -->
	<section class="flex flex-col gap-3">
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
			<div class="flex items-center gap-3">
				<h2 class="text-sm font-medium text-[var(--text-primary)] m-0">Environment Variables</h2>
				<span class="text-xs text-[var(--text-tertiary)] tabular-nums">
					{variableCount} {variableCount === 1 ? 'variable' : 'variables'}
				</span>
			</div>

			<!-- Actions Toolbar -->
			<div class="flex items-center gap-2">
				<label
					class="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
					title="Import .env file"
				>
					<UploadSimple size={13} />
					<span>Import</span>
					<input type="file" accept=".env,.txt" onchange={handleImportFile} class="sr-only" />
				</label>

				<Button variant="secondary" size="sm" onclick={handleExportFile} title="Export as .env file">
					<DownloadSimple size={13} /> Export
				</Button>

				<Button variant="secondary" size="sm" onclick={formatEnv} title="Format spacing">
					<MagicWand size={13} /> Format
				</Button>

				<Button
					variant="primary"
					size="sm"
					disabled={!isDirty && !saveFeedback}
					onclick={saveEnv}
				>
					{#if saveFeedback}
						<Check size={13} class="text-white" /> Saved
					{:else}
						Save .env
					{/if}
				</Button>
			</div>
		</div>

		<!-- Shared CodeEditor with .env syntax highlighting -->
		<CodeEditor bind:value={envText} language="env" height="340px" />

		<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
			Paste or edit your complete configuration file directly. One <code>KEY=value</code> pair per line. Lines starting with <code>#</code> are treated as comments.
		</p>
	</section>

	<!-- ══════════════════════════════════════════════════════════════
	     2. PODMAN SECRETS (Visually separate from standard env vars)
	     ══════════════════════════════════════════════════════════════ -->
	<section class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-4">
		<div class="flex items-start justify-between">
			<div class="flex flex-col gap-0.5">
				<div class="flex items-center gap-2">
					<Lock size={15} class="text-[var(--accent)]" />
					<h3 class="text-sm font-medium text-[var(--text-primary)] m-0">Podman Secrets</h3>
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">
					Native <code>podman secret</code> objects mounted securely in runtime memory or exposed as environment variables.
				</span>
			</div>

			{#if !isAddingSecret}
				<Button variant="secondary" size="sm" onclick={() => (isAddingSecret = true)}>
					<Plus size={13} /> Mount Secret
				</Button>
			{/if}
		</div>

		<!-- Add Secret Mount Form -->
		{#if isAddingSecret}
			<div class="p-4 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] flex flex-col gap-3">
				<span class="text-xs font-medium text-[var(--text-primary)]">Select Secret & Mount Type</span>

				<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
					<div class="flex flex-col gap-1.5">
						<label for="secret-select" class="text-xs text-[var(--text-secondary)] font-medium">Available Podman Secret</label>
						<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-panel)]">
							<select
								id="secret-select"
								bind:value={selectedSecretId}
								class="w-full bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
							>
								{#each dataStore.podmanSecrets as s}
									<option value={s.id}>{s.name} (stored secret)</option>
								{/each}
							</select>
						</div>
					</div>

					<div class="flex flex-col gap-1.5">
						<span class="text-xs text-[var(--text-secondary)] font-medium">Mount Exposure</span>
						<div class="flex items-center gap-4 pt-2">
							<label class="flex items-center gap-2 text-xs text-[var(--text-primary)] cursor-pointer">
								<input type="radio" bind:group={secretMountType} value="env" class="accent-[var(--accent)]" />
								Environment Variable
							</label>
							<label class="flex items-center gap-2 text-xs text-[var(--text-primary)] cursor-pointer">
								<input type="radio" bind:group={secretMountType} value="file" class="accent-[var(--accent)]" />
								File Mount (/run/secrets)
							</label>
						</div>
					</div>
				</div>

				{#if secretMountType === 'env'}
					<div class="flex flex-col gap-1">
						<label for="custom-env-name" class="text-xs text-[var(--text-secondary)] font-medium">
							Environment Variable Name (defaults to {currentSelectedSecret?.name.toUpperCase()})
						</label>
						<Input
							id="custom-env-name"
							bind:value={customEnvVar}
							placeholder={currentSelectedSecret?.name.toUpperCase()}
							class="font-[var(--font-mono)] text-xs"
						/>
					</div>
				{:else}
					<div class="flex flex-col gap-1">
						<label for="custom-file-path" class="text-xs text-[var(--text-secondary)] font-medium">
							Target Path in Container (defaults to /run/secrets/{currentSelectedSecret?.name})
						</label>
						<Input
							id="custom-file-path"
							bind:value={customFilePath}
							placeholder={`/run/secrets/${currentSelectedSecret?.name}`}
							class="font-[var(--font-mono)] text-xs"
						/>
					</div>
				{/if}

				<div class="flex items-center justify-end gap-2 pt-2 border-t border-[var(--border-subtle)]">
					<Button variant="ghost" size="sm" onclick={() => (isAddingSecret = false)}>Cancel</Button>
					<Button variant="primary" size="sm" onclick={addSecretMount}>Attach Secret</Button>
				</div>
			</div>
		{/if}

		<!-- Active Secret Mounts List -->
		<div class="flex flex-col divide-y divide-[var(--border-subtle)] border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] overflow-hidden">
			{#each secretMounts as sm, idx}
				<div class="flex items-center justify-between p-3.5 gap-4">
					<div class="flex items-center gap-3 min-w-0">
						<span class="flex items-center justify-center w-6 h-6 rounded bg-[rgba(105,115,168,0.12)] text-[var(--accent)]">
							<Lock size={12} />
						</span>
						<div class="flex flex-col gap-0.5">
							<span class="text-xs font-medium text-[var(--text-primary)] font-[var(--font-mono)]">
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
						class="text-[var(--text-tertiary)] hover:text-[var(--status-red)] p-1 bg-transparent border-0 cursor-pointer"
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
