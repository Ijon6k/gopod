<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor } from '$lib/components/ui';
	import { Button, Input, CopyButton } from '$lib/components/primitives';
	import {
		FileText,
		Eye,
		Check,
		ShieldCheck,
		GitBranch,
		Lightning,
		Plus,
		FileCode
	} from 'phosphor-svelte';

	import { QUADLET_TEMPLATES, type QuadletTemplate } from '../manifests/quadlet-templates';
	import { getDefaultQuadletConfig } from '../manifests/defaults';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let providerTab = $state<'raw' | 'git' | 'templates'>('raw');
	let gitRepoUrl = $state('');
	let gitBranch = $state('main');
	let gitFilePath = $state('');
	let quadletConfig = $state('');
	let previewOpen = $state(false);
	let saveStatus = $state<'idle' | 'saving' | 'saved'>('idle');

	let lastLoadedServiceId = $state<string | null>(null);

	$effect(() => {
		if (service.id !== lastLoadedServiceId) {
			lastLoadedServiceId = service.id;
			gitRepoUrl = service.source && service.source.endsWith('.git') ? service.source : '';
			gitBranch = service.branch || 'main';
			gitFilePath = `${service.name}.container`;
			quadletConfig = service.quadletConfig || getDefaultQuadletConfig(service);
		}
	});

	let unitPath = $derived(`~/.config/containers/systemd/${service.name}.container`);
	const templates = QUADLET_TEMPLATES;

	function applyTemplate(tpl: QuadletTemplate) {
		quadletConfig = tpl.content;
		service.port = tpl.port;
		service.image = tpl.image;
		providerTab = 'raw';
	}

	function insertSnippet(snippet: string) {
		quadletConfig += `\n${snippet}`;
	}

	async function handleSave() {
		saveStatus = 'saving';
		service.quadletConfig = quadletConfig;
		if (providerTab === 'git') {
			service.source = gitRepoUrl;
			service.branch = gitBranch;
		}

		try {
			await dataStore.updateService(service);
			saveStatus = 'saved';
			setTimeout(() => (saveStatus = 'idle'), 2500);
		} catch (err) {
			console.error('Failed to save Quadlet config:', err);
			saveStatus = 'idle';
		}
	}
</script>

<div class="flex flex-col gap-6">
	<!-- 1. Provider Card (Dokploy Style) -->
	<div
		class="flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs"
	>
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-subtle)] px-5 py-4"
		>
			<div class="flex flex-col gap-0.5">
				<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Provider</h3>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">
					Select the source for your Quadlet systemd unit
				</p>
			</div>

			<!-- Right Action: Preview Systemd Generator Output -->
			<button
				type="button"
				onclick={() => (previewOpen = !previewOpen)}
				class="flex cursor-pointer items-center gap-1.5 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-1.5 text-xs font-medium text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			>
				<Eye size={14} />
				{previewOpen ? 'Hide Systemd Output' : 'Preview Systemd Unit'}
			</button>
		</div>

		<!-- Provider Tabs Bar -->
		<div
			class="flex items-center gap-2 border-b border-[var(--border-subtle)] bg-[var(--bg-surface)]/30 px-5 py-3"
		>
			<button
				type="button"
				onclick={() => (providerTab = 'raw')}
				class="flex cursor-pointer items-center gap-2 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {providerTab ===
				'raw'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
			>
				<FileCode size={14} /> Raw / Unit Editor
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'git')}
				class="flex cursor-pointer items-center gap-2 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {providerTab ===
				'git'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
			>
				<GitBranch size={14} /> Git Repository
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'templates')}
				class="flex cursor-pointer items-center gap-2 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {providerTab ===
				'templates'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
			>
				<Lightning size={14} /> Presets & Templates
			</button>
		</div>

		<!-- Provider Tab Contents -->
		{#if providerTab === 'git'}
			<div class="flex flex-col gap-4 p-5">
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<div class="flex flex-col gap-1.5 sm:col-span-2">
						<label for="git-repo" class="text-xs font-medium text-[var(--text-secondary)]"
							>Repository URL</label
						>
						<Input
							id="git-repo"
							bind:value={gitRepoUrl}
							placeholder="git@github.com:user/homelab-infra.git"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
					<div class="flex flex-col gap-1.5">
						<label for="git-branch" class="text-xs font-medium text-[var(--text-secondary)]"
							>Branch</label
						>
						<Input
							id="git-branch"
							bind:value={gitBranch}
							placeholder="main"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
				</div>
				<div class="flex flex-col gap-1.5">
					<label for="git-path" class="text-xs font-medium text-[var(--text-secondary)]"
						>Unit File Path in Repo</label
					>
					<Input
						id="git-path"
						bind:value={gitFilePath}
						placeholder="quadlets/metube.container"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>
		{:else if providerTab === 'templates'}
			<div class="flex flex-col gap-3 p-5">
				<span class="text-xs font-medium text-[var(--text-secondary)]"
					>Select a Quadlet boilerplate template:</span
				>
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
					{#each templates as tpl}
						<button
							type="button"
							onclick={() => applyTemplate(tpl)}
							class="flex cursor-pointer flex-col rounded-md border border-[var(--border)] bg-[var(--bg-surface)] p-3.5 text-left transition-all hover:border-[var(--accent)] hover:bg-[var(--bg-hover)]"
						>
							<div class="mb-1 flex items-center justify-between">
								<span class="text-xs font-semibold text-[var(--text-primary)]">{tpl.name}</span>
								<span class="text-[10px] font-[var(--font-mono)] text-[var(--accent)]"
									>:{tpl.port}</span
								>
							</div>
							<span class="truncate text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]"
								>{tpl.image}</span
							>
						</button>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<!-- 2. Quadlet Unit File Card -->
	<div
		class="flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs"
	>
		<!-- Unit Card Header -->
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-subtle)] px-5 py-4"
		>
			<div class="flex flex-col gap-0.5">
				<div class="flex items-center gap-2">
					<FileText size={16} class="text-[var(--accent)]" />
					<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Quadlet Unit File</h3>
				</div>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">
					Configure your Podman Quadlet <code class="text-[var(--accent)]">.container</code> unit for
					this service.
				</p>
			</div>

			<!-- Quick Snippets Inserter -->
			<div class="flex flex-wrap items-center gap-1.5">
				<span class="mr-1 hidden text-[11px] text-[var(--text-tertiary)] sm:inline">Snippets:</span>
				<button
					type="button"
					onclick={() => insertSnippet('Volume=app-data:/data:Z')}
					class="cursor-pointer rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-1 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]"
				>
					+ Volume
				</button>
				<button
					type="button"
					onclick={() => insertSnippet('Environment=KEY=value')}
					class="cursor-pointer rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-1 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]"
				>
					+ Env
				</button>
				<button
					type="button"
					onclick={() => insertSnippet('UserNS=auto')}
					class="cursor-pointer rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-1 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]"
				>
					+ UserNS
				</button>
				<button
					type="button"
					onclick={() => insertSnippet('AutoUpdate=registry')}
					class="cursor-pointer rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-1 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]"
				>
					+ AutoUpdate
				</button>
			</div>
		</div>

		<!-- Editor Container -->
		<div class="flex flex-col gap-3 p-5">
			<!-- System path orientation banner -->
			<div
				class="flex items-center justify-between rounded-md border border-[var(--border-subtle)] bg-[var(--bg-surface)]/60 px-3 py-2 text-xs font-[var(--font-mono)] text-[var(--text-tertiary)]"
			>
				<div class="flex min-w-0 items-center gap-2">
					<span class="shrink-0 text-[var(--text-tertiary)]">Host Path:</span>
					<span class="truncate font-semibold text-[var(--text-primary)]">{unitPath}</span>
					<CopyButton
						text={unitPath}
						title="Copy unit file path"
						variant="icon"
						size={13}
						class="p-0 text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
					/>
				</div>
				<span class="flex shrink-0 items-center gap-1 text-[var(--status-green)]">
					<ShieldCheck size={14} /> Rootless systemd (--user)
				</span>
			</div>

			<!-- Monospace Code Editor -->
			<div class="overflow-hidden rounded-md border border-[var(--border)] shadow-inner">
				<CodeEditor bind:value={quadletConfig} language="quadlet" height="auto" />
			</div>

			<!-- Systemd Generator Preview Drawer -->
			{#if previewOpen}
				<div
					class="flex flex-col gap-2 rounded-md border border-[var(--border-subtle)] bg-[var(--bg-surface)]/90 p-4"
				>
					<div class="flex items-center justify-between">
						<span
							class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]"
						>
							<Eye size={14} class="text-[var(--accent)]" /> Generated Service Unit (/usr/lib/systemd/system-generators/podman-systemd-generator)
						</span>
						<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]"
							>systemctl --user status {service.name}</span
						>
					</div>
					<pre
						class="m-0 overflow-x-auto rounded-md border border-[var(--border-subtle)] bg-[var(--bg-panel)] p-3 text-[11px] leading-relaxed font-[var(--font-mono)] text-[var(--text-secondary)]">
# Automatically generated by podman-systemd-generator
[Unit]
Description={service.name} Quadlet Service
SourcePath={unitPath}
After=network-online.target
RequiresMountsFor=%t/containers

[Service]
Environment=PODMAN_SYSTEMD_UNIT=%n
Restart=always
TimeoutStartSec=300
ExecStart=/usr/bin/podman run --name={service.name} -d --replace -p {service.port ||
							8080}:80 {service.image || 'ghcr.io/alexta69/metube:latest'}
ExecStop=/usr/bin/podman stop -t 10 {service.name}
ExecStopPost=/usr/bin/podman rm -f {service.name}
Type=notify
NotifyAccess=all

[Install]
WantedBy=default.target</pre>
				</div>
			{/if}
		</div>

		<!-- Card Footer (1 Explicit Save Button per Card) -->
		<div
			class="flex items-center justify-between gap-3 border-t border-[var(--border)] bg-[var(--bg-panel)] px-5 py-3"
		>
			<span class="text-xs text-[var(--text-tertiary)]">
				Edits take effect after systemd unit reload (<code class="text-[var(--text-secondary)]"
					>systemctl --user daemon-reload</code
				>).
			</span>

			<div class="flex items-center gap-2">
				{#if saveStatus === 'saved'}
					<span class="mr-1 flex items-center gap-1 text-xs text-[var(--status-green)]">
						<Check size={14} weight="bold" /> Saved & Reloaded
					</span>
				{/if}
				<button
					type="button"
					onclick={handleSave}
					disabled={saveStatus === 'saving'}
					class="cursor-pointer rounded-md border-0 bg-[var(--accent)] px-4 py-2 text-xs font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
				>
					{saveStatus === 'saving' ? 'Reloading...' : 'Save & Reload Systemd'}
				</button>
			</div>
		</div>
	</div>
</div>
