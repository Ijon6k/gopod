<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { cn } from '$lib/utils/cn';
	import { ui } from '$lib/stores/ui.svelte';
	import { server } from '$lib/data';
	import type { SystemInfoResponse } from '$lib/api/system/types';
	import { LayerCard, Field, Input, Badge } from '$lib/components/ui';
	import { Sun, Moon, Desktop, Globe, Clock, ShieldCheck } from 'phosphor-svelte';

	let sysInfo = $state<SystemInfoResponse | null>(null);
	let hostname = $state(server.hostname || 'gopod-host');

	onMount(async () => {
		try {
			const info = await api.system.info();
			if (info) {
				sysInfo = info;
				if (info.hostname) hostname = info.hostname;
			}
		} catch {
			// keep fallback
		}
	});
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
</div>
