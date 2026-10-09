<script lang="ts">
	import { PageHeader, StatusBadge, ResourceBar } from '$lib/components/ui';
	import { server } from '$lib/data';
</script>

<svelte:head>
	<title>Servers — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-8">
	<PageHeader title="Servers" subtitle="Your connected servers." />

	<!-- Server card -->
	<div
		class="flex flex-col gap-6 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)] p-6"
	>
		<div class="flex items-start justify-between">
			<div>
				<div class="flex items-center gap-3">
					<h2 class="m-0 text-lg font-semibold text-[var(--text-primary)]">{server.hostname}</h2>
					<StatusBadge status={server.status} size="sm" />
				</div>
				<p class="mt-1 mb-0 text-xs text-[var(--text-tertiary)]">
					{server.ip} · {server.os} · Kernel {server.kernel}
				</p>
			</div>
			<div class="text-xs text-[var(--text-tertiary)]">
				Uptime: {server.uptime}
			</div>
		</div>

		<div class="grid grid-cols-1 gap-6 md:grid-cols-3">
			<ResourceBar label="CPU" value={server.cpuUsage} max={100} unit="%" />
			<ResourceBar label="Memory" value={server.memoryUsed} max={server.memory} unit=" GB" />
			<ResourceBar label="Storage" value={server.storageUsed} max={server.storage} unit=" GB" />
		</div>

		<!-- Health checks -->
		<div class="grid grid-cols-2 gap-4 border-t border-[var(--border-subtle)] pt-4 md:grid-cols-4">
			<div class="flex flex-col gap-1">
				<span class="text-[10px] font-medium tracking-[0.1em] text-[var(--text-tertiary)] uppercase"
					>Podman</span
				>
				<StatusBadge status={server.podmanHealth} size="sm" />
			</div>
			<div class="flex flex-col gap-1">
				<span class="text-[10px] font-medium tracking-[0.1em] text-[var(--text-tertiary)] uppercase"
					>Caddy</span
				>
				<StatusBadge status={server.caddy} size="sm" />
			</div>
			<div class="flex flex-col gap-1">
				<span class="text-[10px] font-medium tracking-[0.1em] text-[var(--text-tertiary)] uppercase"
					>Network</span
				>
				<StatusBadge status={server.networkHealth} size="sm" />
			</div>
			<div class="flex flex-col gap-1">
				<span class="text-[10px] font-medium tracking-[0.1em] text-[var(--text-tertiary)] uppercase"
					>Storage</span
				>
				<StatusBadge status={server.storageHealth} size="sm" />
			</div>
		</div>

		<!-- Info -->
		<div class="grid grid-cols-2 gap-4 text-xs md:grid-cols-4">
			<div class="flex flex-col gap-0.5">
				<span class="text-[var(--text-tertiary)]">Podman</span>
				<span class="text-[var(--text-secondary)]">v{server.podmanVersion}</span>
			</div>
			<div class="flex flex-col gap-0.5">
				<span class="text-[var(--text-tertiary)]">Mode</span>
				<span class="text-[var(--text-secondary)]">{server.rootless ? 'Rootless' : 'Root'}</span>
			</div>
			<div class="flex flex-col gap-0.5">
				<span class="text-[var(--text-tertiary)]">Systemd</span>
				<span class="text-[var(--text-secondary)]">{server.systemd ? 'Enabled' : 'Disabled'}</span>
			</div>
			<div class="flex flex-col gap-0.5">
				<span class="text-[var(--text-tertiary)]">Quadlet</span>
				<span class="text-[var(--text-secondary)]">{server.quadlet ? 'Enabled' : 'Disabled'}</span>
			</div>
		</div>
	</div>
</div>
