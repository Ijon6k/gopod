<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { cn } from '$lib/utils/cn';
	import { ui } from '$lib/stores/ui.svelte';
	import { server } from '$lib/data';
	import type { SystemInfoResponse } from '$lib/api/system/types';
	import { LayerCard, Field, Input, Badge, Button, Switch } from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { Sun, Moon, Desktop, Globe, Clock, ShieldCheck, Lock } from 'phosphor-svelte';

	const toast = useKumoToastManager();

	let sysInfo = $state<SystemInfoResponse | null>(null);
	let hostname = $state(server.hostname || 'gopod-host');

	let panelDomain = $state('');
	let panelTls = $state(false);
	let panelLockCatchall = $state(false);
	let savingPanel = $state(false);

	onMount(async () => {
		try {
			const [info, panelCfg] = await Promise.all([
				api.system.info().catch(() => null),
				api.system.getPanelSettings().catch(() => null)
			]);
			if (info) {
				sysInfo = info;
				if (info.hostname) hostname = info.hostname;
			}
			if (panelCfg) {
				panelDomain = panelCfg.domain || '';
				panelTls = Boolean(panelCfg.tls);
				panelLockCatchall = Boolean(panelCfg.lockCatchall);
			}
		} catch {
			// keep fallback
		}
	});

	async function handleSavePanelSettings() {
		savingPanel = true;
		try {
			const saved = await api.system.updatePanelSettings({
				domain: panelDomain.trim(),
				tls: panelTls,
				lockCatchall: panelLockCatchall
			});
			panelDomain = saved.domain;
			panelTls = saved.tls;
			panelLockCatchall = saved.lockCatchall;
			toast.add({
				title: 'Control plane ingress updated',
				description: saved.lockCatchall
					? `Dashboard locked to ${saved.domain}; unmapped :80 probes return 404.`
					: saved.domain
						? `Control plane bound to ${saved.domain} (catch-all :80 still active).`
						: 'Default :80 control plane ingress active.',
				variant: 'default'
			});
		} catch (err: any) {
			toast.add(errorToastOptions(err, 'Failed to update panel ingress settings'));
		} finally {
			savingPanel = false;
		}
	}
</script>

<div class="flex flex-col gap-6">
	<!-- Appearance Settings Card -->
	<LayerCard class="flex flex-col gap-4 p-5">
		<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
			<div class="flex flex-col gap-0.5">
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">Interface theme</h3>
				<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
					Choose appearance preference for the GoPod management console.
				</p>
			</div>
			<Badge variant="neutral">{ui.theme === 'dark' ? 'Dark mode' : 'Light mode'}</Badge>
		</div>

		<div class="grid max-w-[320px] grid-cols-2 gap-3 pt-1">
			<button
				type="button"
				onclick={() => ui.setTheme('dark')}
				class={cn(
					'flex cursor-pointer items-center justify-center gap-2 rounded-[var(--radius-sm)] border px-3 py-2 text-xs font-medium',
					ui.theme === 'dark'
						? 'border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--text-primary)]'
						: 'border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
				)}
			>
				<Moon size={15} /> Dark
			</button>
			<button
				type="button"
				onclick={() => ui.setTheme('light')}
				class={cn(
					'flex cursor-pointer items-center justify-center gap-2 rounded-[var(--radius-sm)] border px-3 py-2 text-xs font-medium',
					ui.theme === 'light'
						? 'border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--text-primary)]'
						: 'border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
				)}
			>
				<Sun size={15} /> Light
			</button>
		</div>
	</LayerCard>

	<!-- Host System Overview Card -->
	<LayerCard class="flex flex-col gap-4 p-5">
		<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
			<div class="flex items-center gap-2">
				<Desktop size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Host & server environment
				</h3>
			</div>
			<span
				class="inline-flex items-center gap-1.5 text-[11px] font-medium text-[var(--status-green)]"
			>
				<span class="h-1.5 w-1.5 rounded-full bg-[var(--status-green)]"></span>
				Active instance
			</span>
		</div>

		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<!-- The hostname currently has no persistence endpoint; like the other
			     host facts it renders read-only instead of silently discarding edits -->
			<Field label="Hostname">
				<Input id="setting-hostname" value={hostname} class="font-mono text-xs" disabled />
			</Field>

			<Field label="Host IP address">
				<Input
					id="setting-ip"
					value={sysInfo?.ip || server.ip || '127.0.0.1'}
					class="font-mono text-xs"
					disabled
				/>
			</Field>

			<Field label="Operating system">
				<Input
					id="setting-os"
					value={sysInfo?.os || server.os || 'Linux'}
					class="font-mono text-xs"
					disabled
				/>
			</Field>

			<Field label="Kernel version">
				<Input
					id="setting-kernel"
					value={sysInfo?.kernel || server.kernel || 'N/A'}
					class="font-mono text-xs"
					disabled
				/>
			</Field>
		</div>

		<div
			class="grid grid-cols-1 gap-3 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 text-xs sm:grid-cols-3"
		>
			<div class="flex items-center gap-2 text-[var(--text-secondary)]">
				<Clock size={15} class="text-[var(--text-tertiary)]" />
				<span class="text-[11px]"
					>Uptime: <strong class="text-[var(--text-primary)]"
						>{sysInfo?.uptime || server.uptime || 'Active'}</strong
					></span
				>
			</div>
			<div class="flex items-center gap-2 text-[var(--text-secondary)]">
				<Globe size={15} class="text-[var(--text-tertiary)]" />
				<span class="text-[11px]"
					>Engine: <strong class="text-[var(--text-primary)]"
						>{sysInfo?.rootless ? 'Podman rootless' : 'Podman root'}</strong
					></span
				>
			</div>
			<div class="flex items-center gap-2 text-[var(--text-secondary)]">
				<ShieldCheck size={15} class="text-[var(--text-tertiary)]" />
				<span class="text-[11px]"
					>Cgroups: <strong class="text-[var(--text-primary)]"
						>{sysInfo?.cgroupVersion || server.cgroupVersion || 'v2'}</strong
					></span
				>
			</div>
		</div>
	</LayerCard>

	<!-- Control Plane Domain & Access Lock -->
	<LayerCard class="flex flex-col gap-4 p-5">
		<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
			<div class="flex items-center gap-2">
				<Lock size={16} class="text-[var(--accent)]" />
				<div class="flex flex-col gap-0.5">
					<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
						Control plane domain &amp; access lock
					</h3>
					<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
						Bind the GOPOD dashboard to a dedicated hostname and optionally disable the catch-all
						:80 listener.
					</p>
				</div>
			</div>
			<Badge variant={panelLockCatchall && panelDomain ? 'success' : 'neutral'}>
				{panelLockCatchall && panelDomain ? 'Locked to domain' : 'Open on :80'}
			</Badge>
		</div>

		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<Field label="Dashboard hostname">
				<Input
					id="setting-panel-domain"
					bind:value={panelDomain}
					placeholder="panel.homelab.local or gopod.example.com"
					class="font-mono text-xs"
				/>
			</Field>
			<div class="flex flex-col justify-end gap-2.5 pb-1">
				<label class="flex cursor-pointer items-center justify-between gap-3 text-xs">
					<span class="text-[var(--text-secondary)]">Automatic HTTPS / TLS</span>
					<Switch bind:checked={panelTls} />
				</label>
				<label class="flex cursor-pointer items-center justify-between gap-3 text-xs">
					<span class="text-[var(--text-secondary)]"
						>Lock catch-all :80 listener (404 unmapped)</span
					>
					<Switch bind:checked={panelLockCatchall} disabled={!panelDomain.trim()} />
				</label>
			</div>
		</div>

		{#if panelLockCatchall && panelDomain.trim()}
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--status-amber)]/40 bg-[var(--status-amber-muted)] px-3 py-2 text-[11px] text-[var(--status-amber)]"
			>
				Requests hitting raw IP or unmapped hostnames on port 80 will receive HTTP 404. Ensure
				<code class="font-mono font-semibold">{panelDomain.trim()}</code> resolves to this host before
				saving.
			</div>
		{/if}

		<div class="flex justify-end border-t border-[var(--border-subtle)] pt-3">
			<Button variant="primary" size="sm" onclick={handleSavePanelSettings} disabled={savingPanel}>
				{savingPanel ? 'Applying Caddy config…' : 'Save & Reconcile Ingress'}
			</Button>
		</div>
	</LayerCard>
</div>
