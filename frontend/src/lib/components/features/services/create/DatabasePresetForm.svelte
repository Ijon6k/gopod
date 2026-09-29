<script lang="ts">
	import { Input } from '$lib/components/primitives';
	import { ArrowClockwise } from 'phosphor-svelte';

	interface Props {
		name: string;
		description: string;
		databaseType: 'postgres' | 'redis' | 'mysql' | 'mongodb';
		dbName: string;
		dbUser: string;
		dbPassword: string;
		imageTag: string;
	}

	let {
		name = $bindable(''),
		description = $bindable(''),
		databaseType = $bindable('postgres'),
		dbName = $bindable('app'),
		dbUser = $bindable('postgres'),
		dbPassword = $bindable(''),
		imageTag = $bindable('17-alpine')
	}: Props = $props();

	function generatePassword() {
		const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#%_+';
		let res = '';
		for (let i = 0; i < 20; i++) {
			res += chars.charAt(Math.floor(Math.random() * chars.length));
		}
		dbPassword = res;
	}

	// Generate a secure password on init if empty
	if (!dbPassword) {
		generatePassword();
	}

	function handleTypeChange(type: 'postgres' | 'redis' | 'mysql' | 'mongodb') {
		databaseType = type;
		if (type === 'postgres') {
			name = name || 'postgres-db';
			dbName = 'app';
			dbUser = 'postgres';
			imageTag = '17-alpine';
		} else if (type === 'redis') {
			name = name || 'redis-cache';
			dbName = '';
			dbUser = 'default';
			imageTag = '7-alpine';
		} else if (type === 'mysql') {
			name = name || 'mysql-db';
			dbName = 'app';
			dbUser = 'root';
			imageTag = '8.4';
		} else if (type === 'mongodb') {
			name = name || 'mongo-db';
			dbName = 'app';
			dbUser = 'root';
			imageTag = '7.0';
		}
	}
</script>

<div class="flex flex-col gap-4">
	<!-- Database Engine selector -->
	<div class="flex flex-col gap-1.5">
		<span class="text-xs text-[var(--text-secondary)] font-medium">Database Engine</span>
		<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
			{#each [
				{ id: 'postgres', label: 'PostgreSQL', defaultImg: '17-alpine' },
				{ id: 'redis', label: 'Redis', defaultImg: '7-alpine' },
				{ id: 'mysql', label: 'MySQL', defaultImg: '8.4' },
				{ id: 'mongodb', label: 'MongoDB', defaultImg: '7.0' }
			] as db}
				<button
					type="button"
					onclick={() => handleTypeChange(db.id as any)}
					class="p-2.5 rounded-[var(--radius-sm)] border text-left cursor-pointer transition-colors {databaseType === db.id
						? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)]'
						: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
				>
					<span class="block text-xs font-medium">{db.label}</span>
				</button>
			{/each}
		</div>
	</div>

	<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
		<div class="flex flex-col gap-1.5">
			<label for="db-svc-name" class="text-xs text-[var(--text-secondary)] font-medium">
				Service name <span class="text-[var(--status-red)]">*</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="db-svc-name" bind:value={name} placeholder="e.g. postgres-db" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="db-svc-desc" class="text-xs text-[var(--text-secondary)] font-medium">
				Description <span class="text-[var(--text-tertiary)]">(optional)</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="db-svc-desc" bind:value={description} placeholder="Database convenience preset" />
			</div>
		</div>
	</div>

	<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
		{#if databaseType !== 'redis'}
			<div class="flex flex-col gap-1.5">
				<label for="db-name" class="text-xs text-[var(--text-secondary)] font-medium">Database Name</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
					<Input id="db-name" bind:value={dbName} class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>
		{/if}

		<div class="flex flex-col gap-1.5">
			<label for="db-user" class="text-xs text-[var(--text-secondary)] font-medium">Initial User</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="db-user" bind:value={dbUser} class="font-[var(--font-mono)] text-xs" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="db-img-tag" class="text-xs text-[var(--text-secondary)] font-medium">Image Version / Tag</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="db-img-tag" bind:value={imageTag} class="font-[var(--font-mono)] text-xs" />
			</div>
		</div>
	</div>

	<div class="flex flex-col gap-1.5">
		<div class="flex items-center justify-between">
			<label for="db-password" class="text-xs text-[var(--text-secondary)] font-medium">Root / Admin Password</label>
			<button
				type="button"
				onclick={generatePassword}
				class="inline-flex items-center gap-1 text-[11px] text-[var(--accent)] hover:underline bg-transparent border-0 cursor-pointer"
			>
				<ArrowClockwise size={12} /> Regenerate
			</button>
		</div>
		<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
			<Input id="db-password" bind:value={dbPassword} class="font-[var(--font-mono)] text-xs" />
		</div>
	</div>

	<p class="m-0 text-[11px] text-[var(--text-tertiary)]">
		GoPod creates this database as a standard container workload with pre-configured environment credentials. Volume storage and port publishing can be tailored in Service Detail.
	</p>
</div>
