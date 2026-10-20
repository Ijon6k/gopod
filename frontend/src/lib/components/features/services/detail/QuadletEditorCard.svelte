<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor } from '$lib/components/ui';
	import { Input, CopyButton } from '$lib/components/ui';
	import { LayerCard, Button, Banner, useKumoToastManager } from 'kumo-svelte';
	import {
		FileText,
		Eye,
		Check,
		ShieldCheck,
		GitBranch,
		Lightning,
		Plus,
		FileCode,
		WarningCircle,
		Info
	} from 'phosphor-svelte';

	import { QUADLET_TEMPLATES, type QuadletTemplate } from '../manifests/quadlet-templates';
	import { getDefaultQuadletConfig } from '../manifests/defaults';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

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

	function extractUnitName(cfg: string, svc: Service): string {
		if (cfg) {
			const m = cfg.match(/ContainerName\s*=\s*([^\r\n#]+)/i);
			if (m && m[1]?.trim()) {
				return m[1].trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-');
			}
			const p = cfg.match(/PodName\s*=\s*([^\r\n#]+)/i);
			if (p && p[1]?.trim()) {
				return p[1].trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-');
			}
		}
		return (svc.id || svc.name).toLowerCase().replace(/[^a-z0-9_-]/g, '-');
	}

	let cleanUnitName = $derived(extractUnitName(quadletConfig, service));
	let unitPath = $derived(`~/.config/containers/systemd/${cleanUnitName}.container`);
	let systemctlStatusCmd = $derived(`systemctl --user status ${cleanUnitName}.service`);
	const templates = QUADLET_TEMPLATES;

	// Supervision truth from the server (populated on service GET):
	// unit supervised by host systemd vs served by a direct container.
	let supervisedUnit = $derived(service.quadletUnit || `${cleanUnitName}.service`);
	let supervisedFile = $derived(service.quadletFile || unitPath);
	let isSupervised = $derived(service.quadletActive === true);
	let activateCmd = $derived(
		`systemctl --user daemon-reload && systemctl --user start ${supervisedUnit}`
	);

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
			toast.add({
				title: 'Quadlet Saved & Reloaded',
				description: `Systemd unit ${cleanUnitName}.container compiled and daemon-reloaded.`,
				variant: 'success'
			});
			setTimeout(() => (saveStatus = 'idle'), 2500);
		} catch (err: any) {
			console.error('Failed to save Quadlet config:', err);
			saveStatus = 'idle';
			toast.add({
				title: 'Quadlet Save Failed',
				description: err?.message || 'Failed to save Quadlet config',
				variant: 'error'
			});
		}
	}
</script>

<div id="quadlet-editor-section" class="flex flex-col gap-6">
	<!-- 1. Provider Card -->
	<LayerCard class="overflow-hidden p-0">
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
			<Button variant="secondary" onclick={() => (previewOpen = !previewOpen)} class="gap-1.5">
				<Eye size={14} />
				{previewOpen ? 'Hide Systemd Output' : 'Preview Systemd Unit'}
			</Button>
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
	</LayerCard>

	<!-- 2. Quadlet Unit File Card with Kumo LayerCard -->
	<LayerCard class="overflow-hidden p-0">
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
				<span class="mr-1 hidden text-[11px] text-kumo-subtle sm:inline">Snippets:</span>
				<Button
					variant="secondary"
					size="xs"
					onclick={() => insertSnippet('Volume=app-data:/data:Z')}
				>
					+ Volume
				</Button>
				<Button
					variant="secondary"
					size="xs"
					onclick={() => insertSnippet('Environment=KEY=value')}
				>
					+ Env
				</Button>
				<Button variant="secondary" size="xs" onclick={() => insertSnippet('UserNS=auto')}>
					+ UserNS
				</Button>
				<Button variant="secondary" size="xs" onclick={() => insertSnippet('AutoUpdate=registry')}>
					+ AutoUpdate
				</Button>
			</div>
		</div>

		<!-- Editor Container -->
		<div class="flex flex-col gap-3 p-5">
			{#if !isSupervised}
				<Banner variant={service.status === 'running' ? 'default' : 'alert'} class="shadow-xs">
					{#snippet icon()}
						{#if service.status === 'running'}
							<Info size={18} weight="fill" class="shrink-0 text-[var(--accent)]" />
						{:else}
							<WarningCircle size={18} weight="fill" class="shrink-0 text-kumo-warning" />
						{/if}
					{/snippet}
					<div class="flex flex-col gap-1 text-xs">
						<span class="font-semibold text-[var(--color-kumo-default)]">
							{#if service.status === 'running'}
								Served by a direct container — systemd is not supervising this unit
							{:else}
								Unit is not supervised and no container is running
							{/if}
						</span>
						<span class="leading-relaxed text-[var(--color-kumo-subtle)]">
							Unit file is written at
							<code class="font-mono text-[var(--accent)]">{supervisedFile}</code>.
							{#if service.status === 'running'}
								Traffic works, but restarts and reboots are not managed. To hand
								supervision to host systemd, stop the direct container first to free
								the name and ports, then activate the unit on the host.
							{:else}
								Activate the unit on the host to bring this workload up under systemd.
							{/if}
						</span>
						<code class="mt-1 font-mono text-[11px] text-[var(--color-kumo-default)]"
							>podman stop {cleanUnitName} && {activateCmd}</code
						>
					</div>
				</Banner>
			{/if}
			<!-- System path orientation banner -->
			<div
				class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-kumo-line bg-kumo-elevated px-3 py-2 text-xs font-[var(--font-mono)] text-kumo-subtle"
			>
				<div class="flex min-w-0 items-center gap-2">
					<span class="shrink-0 text-kumo-subtle">Host Path:</span>
					<span class="truncate font-semibold text-kumo-default">{unitPath}</span>
					<CopyButton
						text={unitPath}
						title="Copy unit file path"
						variant="icon"
						size={13}
						class="p-0 text-kumo-subtle hover:text-kumo-default"
					/>
				</div>
				<div class="flex items-center gap-3">
					<div class="flex items-center gap-1.5 text-kumo-subtle">
						<span class="shrink-0">Supervise:</span>
						<code class="rounded bg-black/30 px-1 py-0.5 text-[11px] text-[var(--accent)]">{systemctlStatusCmd}</code>
						<CopyButton
							text={systemctlStatusCmd}
							title="Copy systemctl status command"
							variant="icon"
							size={13}
							class="p-0 text-kumo-subtle hover:text-kumo-default"
						/>
					</div>
					<span class="flex shrink-0 items-center gap-1 text-[var(--status-green)]">
						<ShieldCheck size={14} /> Rootless (--user)
					</span>
				</div>
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
							>{systemctlStatusCmd}</span
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
				<Button variant="primary" onclick={handleSave} disabled={saveStatus === 'saving'}>
					{saveStatus === 'saving' ? 'Reloading...' : 'Save & Reload Systemd'}
				</Button>
			</div>
		</div>
	</LayerCard>
</div>
