<script lang="ts">
	import { Input } from '$lib/components/primitives';

	interface Props {
		name: string;
		description: string;
		repo: string;
		branch: string;
		gitProvider: string;
	}

	let {
		name = $bindable(''),
		description = $bindable(''),
		repo = $bindable(''),
		branch = $bindable('main'),
		gitProvider = $bindable('github')
	}: Props = $props();
</script>

<div class="flex flex-col gap-4">
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
		<div class="flex flex-col gap-1.5">
			<label for="app-svc-name" class="text-xs text-[var(--text-secondary)] font-medium">
				Service name <span class="text-[var(--status-red)]">*</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="app-svc-name" bind:value={name} placeholder="e.g. web-api" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="app-svc-desc" class="text-xs text-[var(--text-secondary)] font-medium">
				Description <span class="text-[var(--text-tertiary)]">(optional)</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="app-svc-desc" bind:value={description} placeholder="Frontend application or API" />
			</div>
		</div>
	</div>

	<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
		<div class="flex flex-col gap-1.5">
			<label for="git-provider" class="text-xs text-[var(--text-secondary)] font-medium">Git Provider</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
				<select
					id="git-provider"
					bind:value={gitProvider}
					class="w-full bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
				>
					<option value="github">GitHub</option>
					<option value="gitlab">GitLab</option>
					<option value="custom">Custom Git URL</option>
				</select>
			</div>
		</div>

		<div class="flex flex-col gap-1.5 sm:col-span-2">
			<label for="app-repo" class="text-xs text-[var(--text-secondary)] font-medium">
				Repository <span class="text-[var(--status-red)]">*</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input
					id="app-repo"
					bind:value={repo}
					placeholder="github.com/organization/repository"
					class="font-[var(--font-mono)] text-xs"
				/>
			</div>
		</div>
	</div>

	<div class="flex flex-col gap-1.5 max-w-[240px]">
		<label for="app-branch" class="text-xs text-[var(--text-secondary)] font-medium">Branch</label>
		<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
			<Input id="app-branch" bind:value={branch} placeholder="main" class="font-[var(--font-mono)] text-xs" />
		</div>
	</div>
</div>
