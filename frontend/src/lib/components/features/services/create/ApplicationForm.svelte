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
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<div class="flex flex-col gap-1.5">
			<label for="app-svc-name" class="text-xs font-medium text-[var(--text-secondary)]">
				Service name <span class="text-[var(--status-red)]">*</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input id="app-svc-name" bind:value={name} placeholder="e.g. web-api" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="app-svc-desc" class="text-xs font-medium text-[var(--text-secondary)]">
				Description <span class="text-[var(--text-tertiary)]">(optional)</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input
					id="app-svc-desc"
					bind:value={description}
					placeholder="Frontend application or API"
				/>
			</div>
		</div>
	</div>

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
		<div class="flex flex-col gap-1.5">
			<label for="git-provider" class="text-xs font-medium text-[var(--text-secondary)]"
				>Git Provider</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<select
					id="git-provider"
					bind:value={gitProvider}
					class="w-full cursor-pointer border-0 bg-transparent text-xs font-[var(--font-sans)] text-[var(--text-primary)] outline-none"
				>
					<option value="github">GitHub</option>
					<option value="gitlab">GitLab</option>
					<option value="custom">Custom Git URL</option>
				</select>
			</div>
		</div>

		<div class="flex flex-col gap-1.5 sm:col-span-2">
			<label for="app-repo" class="text-xs font-medium text-[var(--text-secondary)]">
				Repository <span class="text-[var(--status-red)]">*</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input
					id="app-repo"
					bind:value={repo}
					placeholder="github.com/organization/repository"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
		</div>
	</div>

	<div class="flex max-w-[240px] flex-col gap-1.5">
		<label for="app-branch" class="text-xs font-medium text-[var(--text-secondary)]">Branch</label>
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
		>
			<Input
				id="app-branch"
				bind:value={branch}
				placeholder="main"
				class="text-xs font-[var(--font-mono)]"
			/>
		</div>
	</div>
</div>
