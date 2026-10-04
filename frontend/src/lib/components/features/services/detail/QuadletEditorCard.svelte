<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor } from '$lib/components/ui';
	import { Button, Input } from '$lib/components/primitives';
	import {
		FileText,
		Eye,
		Check,
		ShieldCheck,
		GitBranch,
		Lightning,
		Copy,
		Plus,
		FileCode
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let providerTab = $state<'raw' | 'git' | 'templates'>('raw');
	let gitRepoUrl = $state(service.source && service.source.endsWith('.git') ? service.source : '');
	let gitBranch = $state(service.branch || 'main');
	let gitFilePath = $state(`${service.name}.container`);

	let quadletConfig = $state(
		service.quadletConfig ||
			`[Unit]
Description=${service.name} Quadlet Service
After=network-online.target

[Container]
Image=${service.image || 'docker.io/library/nginx:alpine'}
PublishPort=${service.port || 8080}:80
AutoUpdate=registry
Restart=always

[Service]
Restart=always
TimeoutStartSec=300

[Install]
WantedBy=default.target`
	);

	let previewOpen = $state(false);
	let saveStatus = $state<'idle' | 'saving' | 'saved'>('idle');
	let copiedPath = $state(false);

	let unitPath = $derived(`~/.config/containers/systemd/${service.name}.container`);

	const templates = [
		{
			id: 'metube',
			name: 'MeTube (YouTube Downloader)',
			image: 'ghcr.io/alexta69/metube:latest',
			port: 8081,
			content: `[Unit]
Description=MeTube Video Downloader Quadlet
After=network-online.target

[Container]
Image=ghcr.io/alexta69/metube:latest
PublishPort=8081:8081
Volume=metube-downloads:/downloads:Z
AutoUpdate=registry
Restart=always

[Service]
Restart=always
TimeoutStartSec=300

[Install]
WantedBy=default.target`
		},
		{
			id: 'nginx',
			name: 'Nginx Web Server',
			image: 'docker.io/library/nginx:alpine',
			port: 8080,
			content: `[Unit]
Description=Nginx Web Server Quadlet
After=network-online.target

[Container]
Image=docker.io/library/nginx:alpine
PublishPort=8080:80
AutoUpdate=registry
Restart=always

[Service]
Restart=always

[Install]
WantedBy=default.target`
		},
		{
			id: 'postgres',
			name: 'PostgreSQL 17 Database',
			image: 'docker.io/library/postgres:17-alpine',
			port: 5432,
			content: `[Unit]
Description=PostgreSQL Database Quadlet
After=network-online.target

[Container]
Image=docker.io/library/postgres:17-alpine
PublishPort=5432:5432
Environment=POSTGRES_PASSWORD=postgres_secure_pass
Volume=postgres-data:/var/lib/postgresql/data:Z
Restart=always

[Service]
Restart=always
TimeoutStartSec=600

[Install]
WantedBy=default.target`
		},
		{
			id: 'redis',
			name: 'Redis In-Memory Cache',
			image: 'docker.io/library/redis:7-alpine',
			port: 6379,
			content: `[Unit]
Description=Redis In-Memory Cache Quadlet
After=network-online.target

[Container]
Image=docker.io/library/redis:7-alpine
PublishPort=6379:6379
Restart=always

[Service]
Restart=always

[Install]
WantedBy=default.target`
		}
	];

	function applyTemplate(tpl: (typeof templates)[0]) {
		quadletConfig = tpl.content;
		service.port = tpl.port;
		service.image = tpl.image;
		providerTab = 'raw';
	}

	function insertSnippet(snippet: string) {
		quadletConfig += `\n${snippet}`;
	}

	function copyUnitPath() {
		navigator.clipboard.writeText(unitPath);
		copiedPath = true;
		setTimeout(() => (copiedPath = false), 2000);
	}

	function handleSave() {
		saveStatus = 'saving';
		service.quadletConfig = quadletConfig;
		if (providerTab === 'git') {
			service.source = gitRepoUrl;
			service.branch = gitBranch;
		}

		setTimeout(() => {
			dataStore.updateService(service);
			saveStatus = 'saved';
			setTimeout(() => (saveStatus = 'idle'), 2500);
		}, 400);
	}
</script>

<div class="flex flex-col gap-6">
	<!-- 1. Provider Card (Dokploy Style) -->
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col shadow-xs">
		<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex flex-wrap items-center justify-between gap-3">
			<div class="flex flex-col gap-0.5">
				<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Provider</h3>
				<p class="text-xs text-[var(--text-tertiary)] m-0">Select the source for your Quadlet systemd unit</p>
			</div>

			<!-- Right Action: Preview Systemd Generator Output -->
			<button
				type="button"
				onclick={() => (previewOpen = !previewOpen)}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
			>
				<Eye size={14} /> {previewOpen ? 'Hide Systemd Output' : 'Preview Systemd Unit'}
			</button>
		</div>

		<!-- Provider Tabs Bar -->
		<div class="px-5 py-3 border-b border-[var(--border-subtle)] flex items-center gap-2 bg-[var(--bg-surface)]/30">
			<button
				type="button"
				onclick={() => (providerTab = 'raw')}
				class="flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 {providerTab ===
				'raw'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}"
			>
				<FileCode size={14} /> Raw / Unit Editor
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'git')}
				class="flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 {providerTab ===
				'git'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}"
			>
				<GitBranch size={14} /> Git Repository
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'templates')}
				class="flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 {providerTab ===
				'templates'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}"
			>
				<Lightning size={14} /> Presets & Templates
			</button>
		</div>

		<!-- Provider Tab Contents -->
		{#if providerTab === 'git'}
			<div class="p-5 flex flex-col gap-4">
				<div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
					<div class="sm:col-span-2 flex flex-col gap-1.5">
						<label for="git-repo" class="text-xs font-medium text-[var(--text-secondary)]">Repository URL</label>
						<Input
							id="git-repo"
							bind:value={gitRepoUrl}
							placeholder="git@github.com:user/homelab-infra.git"
							class="font-[var(--font-mono)] text-xs"
						/>
					</div>
					<div class="flex flex-col gap-1.5">
						<label for="git-branch" class="text-xs font-medium text-[var(--text-secondary)]">Branch</label>
						<Input
							id="git-branch"
							bind:value={gitBranch}
							placeholder="main"
							class="font-[var(--font-mono)] text-xs"
						/>
					</div>
				</div>
				<div class="flex flex-col gap-1.5">
					<label for="git-path" class="text-xs font-medium text-[var(--text-secondary)]">Unit File Path in Repo</label>
					<Input
						id="git-path"
						bind:value={gitFilePath}
						placeholder="quadlets/metube.container"
						class="font-[var(--font-mono)] text-xs"
					/>
				</div>
			</div>
		{:else if providerTab === 'templates'}
			<div class="p-5 flex flex-col gap-3">
				<span class="text-xs text-[var(--text-secondary)] font-medium">Select a Quadlet boilerplate template:</span>
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
					{#each templates as tpl}
						<button
							type="button"
							onclick={() => applyTemplate(tpl)}
							class="flex flex-col text-left p-3.5 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] hover:border-[var(--accent)] transition-all cursor-pointer"
						>
							<div class="flex items-center justify-between mb-1">
								<span class="text-xs font-semibold text-[var(--text-primary)]">{tpl.name}</span>
								<span class="text-[10px] font-[var(--font-mono)] text-[var(--accent)]">:{tpl.port}</span>
							</div>
							<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)] truncate">{tpl.image}</span>
						</button>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<!-- 2. Quadlet Unit File Card -->
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col shadow-xs">
		<!-- Unit Card Header -->
		<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex flex-wrap items-center justify-between gap-3">
			<div class="flex flex-col gap-0.5">
				<div class="flex items-center gap-2">
					<FileText size={16} class="text-[var(--accent)]" />
					<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Quadlet Unit File</h3>
				</div>
				<p class="text-xs text-[var(--text-tertiary)] m-0">
					Configure your Podman Quadlet <code class="text-[var(--accent)]">.container</code> unit for this service.
				</p>
			</div>

			<!-- Quick Snippets Inserter -->
			<div class="flex items-center gap-1.5 flex-wrap">
				<span class="text-[11px] text-[var(--text-tertiary)] mr-1 hidden sm:inline">Snippets:</span>
				<button
					type="button"
					onclick={() => insertSnippet('Volume=app-data:/data:Z')}
					class="px-2 py-1 rounded bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] border border-[var(--border)] text-[11px] text-[var(--text-secondary)] cursor-pointer"
				>
					+ Volume
				</button>
				<button
					type="button"
					onclick={() => insertSnippet('Environment=KEY=value')}
					class="px-2 py-1 rounded bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] border border-[var(--border)] text-[11px] text-[var(--text-secondary)] cursor-pointer"
				>
					+ Env
				</button>
				<button
					type="button"
					onclick={() => insertSnippet('UserNS=auto')}
					class="px-2 py-1 rounded bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] border border-[var(--border)] text-[11px] text-[var(--text-secondary)] cursor-pointer"
				>
					+ UserNS
				</button>
				<button
					type="button"
					onclick={() => insertSnippet('AutoUpdate=registry')}
					class="px-2 py-1 rounded bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] border border-[var(--border)] text-[11px] text-[var(--text-secondary)] cursor-pointer"
				>
					+ AutoUpdate
				</button>
			</div>
		</div>

		<!-- Editor Container -->
		<div class="p-5 flex flex-col gap-3">
			<!-- System path orientation banner -->
			<div class="flex items-center justify-between text-xs text-[var(--text-tertiary)] font-[var(--font-mono)] bg-[var(--bg-surface)]/60 px-3 py-2 rounded-md border border-[var(--border-subtle)]">
				<div class="flex items-center gap-2 min-w-0">
					<span class="text-[var(--text-tertiary)] shrink-0">Host Path:</span>
					<span class="text-[var(--text-primary)] truncate font-semibold">{unitPath}</span>
					<button
						type="button"
						onclick={copyUnitPath}
						class="text-[var(--text-tertiary)] hover:text-[var(--text-primary)] bg-transparent border-0 cursor-pointer p-0 shrink-0"
						title="Copy unit file path"
					>
						{#if copiedPath}
							<Check size={13} class="text-[var(--status-green)]" />
						{:else}
							<Copy size={13} />
						{/if}
					</button>
				</div>
				<span class="text-[var(--status-green)] flex items-center gap-1 shrink-0">
					<ShieldCheck size={14} /> Rootless systemd (--user)
				</span>
			</div>

			<!-- Monospace Code Editor -->
			<div class="rounded-md border border-[var(--border)] overflow-hidden shadow-inner">
				<CodeEditor bind:value={quadletConfig} language="ini" height="340px" />
			</div>

			<!-- Systemd Generator Preview Drawer -->
			{#if previewOpen}
				<div class="p-4 rounded-md border border-[var(--border-subtle)] bg-[var(--bg-surface)]/90 flex flex-col gap-2">
					<div class="flex items-center justify-between">
						<span class="text-xs font-semibold text-[var(--text-primary)] flex items-center gap-1.5">
							<Eye size={14} class="text-[var(--accent)]" /> Generated Service Unit (/usr/lib/systemd/system-generators/podman-systemd-generator)
						</span>
						<span class="text-[11px] text-[var(--text-tertiary)] font-[var(--font-mono)]">systemctl --user status {service.name}</span>
					</div>
					<pre class="m-0 p-3 rounded-md bg-[var(--bg-panel)] border border-[var(--border-subtle)] font-[var(--font-mono)] text-[11px] text-[var(--text-secondary)] overflow-x-auto leading-relaxed">
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
ExecStart=/usr/bin/podman run --name={service.name} -d --replace -p {service.port || 8080}:80 {service.image || 'ghcr.io/alexta69/metube:latest'}
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
		<div class="px-5 py-3 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-between gap-3">
			<span class="text-xs text-[var(--text-tertiary)]">
				Edits take effect after systemd unit reload (<code class="text-[var(--text-secondary)]">systemctl --user daemon-reload</code>).
			</span>

			<div class="flex items-center gap-2">
				{#if saveStatus === 'saved'}
					<span class="text-xs text-[var(--status-green)] flex items-center gap-1 mr-1">
						<Check size={14} weight="bold" /> Saved & Reloaded
					</span>
				{/if}
				<button
					type="button"
					onclick={handleSave}
					disabled={saveStatus === 'saving'}
					class="px-4 py-2 rounded-md bg-[var(--accent)] hover:opacity-90 text-white text-xs font-semibold cursor-pointer border-0 transition-opacity disabled:opacity-50"
				>
					{saveStatus === 'saving' ? 'Reloading...' : 'Save & Reload Systemd'}
				</button>
			</div>
		</div>
	</div>
</div>
