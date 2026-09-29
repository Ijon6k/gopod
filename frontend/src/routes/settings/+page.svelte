<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { PageHeader, Tabs } from '$lib/components/ui';
	import { Button, Input } from '$lib/components/primitives';
	import { server } from '$lib/data';
	import { ui } from '$lib/stores/ui.svelte';
	import { Sun, Moon } from 'phosphor-svelte';

	let activeTab = $state('general');
	const tabs = [
		{ id: 'general', label: 'General' },
		{ id: 'security', label: 'Security' },
		{ id: 'notifications', label: 'Notifications' }
	];

	let hostname = $state(server.hostname);
</script>

<svelte:head>
	<title>Settings — GOPOD</title>
</svelte:head>

<div class="w-full max-w-[800px] flex flex-col gap-6">
	<PageHeader title="Settings" subtitle="Configure your GOPOD instance." />
	<Tabs {tabs} bind:active={activeTab} />

	{#if activeTab === 'general'}
		<div class="flex flex-col gap-6">
			<!-- Theme Appearance -->
			<div class="flex flex-col gap-2">
				<span class="text-xs text-[var(--text-secondary)] font-medium">Theme Appearance</span>
				<div class="grid grid-cols-2 gap-3 max-w-[320px]">
					<button
						type="button"
						onclick={() => ui.setTheme('dark')}
						class={cn(
							'flex items-center justify-center gap-2 px-3 py-2.5 rounded-[var(--radius-sm)] border cursor-pointer font-[var(--font-sans)] text-base transition-colors',
							ui.theme === 'dark'
								? 'border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--text-primary)] font-medium'
								: 'border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
						)}
					>
						<Moon size={16} /> Dark
					</button>
					<button
						type="button"
						onclick={() => ui.setTheme('light')}
						class={cn(
							'flex items-center justify-center gap-2 px-3 py-2.5 rounded-[var(--radius-sm)] border cursor-pointer font-[var(--font-sans)] text-base transition-colors',
							ui.theme === 'light'
								? 'border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--text-primary)] font-medium'
								: 'border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
						)}
					>
						<Sun size={16} /> Light
					</button>
				</div>
			</div>

			<div class="flex flex-col gap-2">
				<label for="setting-hostname" class="text-xs text-[var(--text-secondary)] font-medium">Hostname</label>
				<div class="px-3 py-2.5 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
					<Input id="setting-hostname" bind:value={hostname} />
				</div>
			</div>

			<div class="flex flex-col gap-2">
				<label for="setting-ip" class="text-xs text-[var(--text-secondary)] font-medium">Server IP</label>
				<div class="px-3 py-2.5 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] opacity-60">
					<Input id="setting-ip" value={server.ip} disabled />
				</div>
			</div>

			<div class="flex flex-col gap-2">
				<label for="setting-os" class="text-xs text-[var(--text-secondary)] font-medium">Operating System</label>
				<div class="px-3 py-2.5 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] opacity-60">
					<Input id="setting-os" value={server.os} disabled />
				</div>
			</div>

			<div class="pt-2">
				<Button variant="primary">Save changes</Button>
			</div>
		</div>
	{:else if activeTab === 'security'}
		<div class="text-[var(--text-tertiary)] py-12 text-center text-xs">
			Security settings coming soon.
		</div>
	{:else}
		<div class="text-[var(--text-tertiary)] py-12 text-center text-xs">
			Notification preferences coming soon.
		</div>
	{/if}
</div>
