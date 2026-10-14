<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from 'kumo-svelte';
	import { ArrowRight } from 'phosphor-svelte';

	interface Props {
		server: { ip: string; podmanVersion: string; os: string };
	}

	let { server }: Props = $props();
</script>

<div
	class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
>
	<span class="text-xs font-semibold text-[var(--text-primary)]">Public Internet Traffic</span>
	<div class="space-y-1.5 text-xs text-[var(--text-secondary)]">
		<div
			class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5"
		>
			<span>Host Public IP</span>
			<span class="font-mono text-[var(--text-primary)]">{server.ip}</span>
		</div>
		<div
			class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5"
		>
			<span>Podman Engine</span>
			<span class="font-mono text-[var(--text-primary)]">Podman {server.podmanVersion}</span>
		</div>
		<div
			class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5"
		>
			<span>Host Operating System</span>
			<span class="text-[var(--text-primary)]">{server.os}</span>
		</div>
	</div>
</div>

<div class="mt-2">
	<Button
		variant="primary"
		size="sm"
		class="w-full justify-center"
		onclick={() => goto('/settings')}
	>
		<span>Host System Overview</span>
		<ArrowRight size={13} />
	</Button>
</div>
