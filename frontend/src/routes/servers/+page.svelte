<script lang="ts">
	import { PageHeader, StatusBadge, ResourceBar } from '$lib/components/ui';
	import { server } from '$lib/data';
</script>

<svelte:head>
	<title>Servers — GOPOD</title>
</svelte:head>

<div class="w-full flex flex-col gap-8">
	<PageHeader title="Servers" subtitle="Your connected servers." />

	<!-- Server card -->
	<div class="bg-[var(--bg-surface)] border border-[var(--border)] rounded-[var(--radius-card)] p-6 flex flex-col gap-6">
		<div class="flex items-start justify-between">
			<div>
				<div class="flex items-center gap-3">
					<h2 class="text-lg font-semibold text-[var(--text-primary)] m-0">{server.hostname}</h2>
					<StatusBadge status={server.status} size="sm" />
				</div>
				<p class="text-xs text-[var(--text-tertiary)] mt-1 mb-0">{server.ip} · {server.os} · Kernel {server.kernel}</p>
			</div>
			<div class="text-xs text-[var(--text-tertiary)]">
				Uptime: {server.uptime}
			</div>
		</div>

		<div class="grid grid-cols-1 md:grid-cols-3 gap-6">
			<ResourceBar label="CPU" value={server.cpuUsage} max={100} unit="%" />
			<ResourceBar label="Memory" value={server.memoryUsed} max={server.memory} unit=" GB" />
			<ResourceBar label="Storage" value={server.storageUsed} max={server.storage} unit=" GB" />
		</div>

		<!-- Health checks -->
		<div class="grid grid-cols-2 md:grid-cols-4 gap-4 pt-4 border-t border-[var(--border-subtle)]">
			<div class="flex flex-col gap-1">
				<span class="text-[10px] text-[var(--text-tertiary)] tracking-[0.1em] uppercase font-medium">Podman</span>
				<StatusBadge status={server.podmanHealth} size="sm" />
			</div>
			<div class="flex flex-col gap-1">
				<span class="text-[10px] text-[var(--text-tertiary)] tracking-[0.1em] uppercase font-medium">Caddy</span>
				<StatusBadge status={server.caddy} size="sm" />
			</div>
			<div class="flex flex-col gap-1">
				<span class="text-[10px] text-[var(--text-tertiary)] tracking-[0.1em] uppercase font-medium">Network</span>
				<StatusBadge status={server.networkHealth} size="sm" />
			</div>
			<div class="flex flex-col gap-1">
				<span class="text-[10px] text-[var(--text-tertiary)] tracking-[0.1em] uppercase font-medium">Storage</span>
				<StatusBadge status={server.storageHealth} size="sm" />
			</div>
		</div>

		<!-- Info -->
		<div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs">
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
