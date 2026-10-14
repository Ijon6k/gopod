<script lang="ts">
	import { Input } from '$lib/components/ui';
	import { GitBranch, GithubLogo, GitlabLogo } from 'phosphor-svelte';

	interface Props {
		gitProvider: 'github' | 'gitlab' | 'custom';
		gitRepoUrl: string;
		gitBranch: string;
		gitComposePath: string;
	}

	let {
		gitProvider = $bindable(),
		gitRepoUrl = $bindable(),
		gitBranch = $bindable(),
		gitComposePath = $bindable()
	}: Props = $props();
</script>

<div class="flex flex-col gap-3 rounded-md bg-kumo-base p-3.5 ring ring-kumo-line">
	<!-- Quick Provider selector -->
	<div class="flex items-center gap-2">
		<button
			type="button"
			onclick={() => (gitProvider = 'github')}
			class="flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-xs transition-colors {gitProvider ===
			'github'
				? 'bg-kumo-elevated font-medium text-kumo-default ring ring-kumo-line'
				: 'text-kumo-subtle hover:text-kumo-default'}"
		>
			<GithubLogo size={14} /> GitHub
		</button>
		<button
			type="button"
			onclick={() => (gitProvider = 'gitlab')}
			class="flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-xs transition-colors {gitProvider ===
			'gitlab'
				? 'bg-kumo-elevated font-medium text-kumo-default ring ring-kumo-line'
				: 'text-kumo-subtle hover:text-kumo-default'}"
		>
			<GitlabLogo size={14} /> GitLab
		</button>
		<button
			type="button"
			onclick={() => (gitProvider = 'custom')}
			class="flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-xs transition-colors {gitProvider ===
			'custom'
				? 'bg-kumo-elevated font-medium text-kumo-default ring ring-kumo-line'
				: 'text-kumo-subtle hover:text-kumo-default'}"
		>
			<GitBranch size={14} /> Custom Git / SSH
		</button>
	</div>

	<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
		<div class="flex flex-col gap-1 sm:col-span-2">
			<label for="compose-repo" class="text-xs font-medium text-kumo-subtle">
				Repository URL
			</label>
			<Input
				id="compose-repo"
				bind:value={gitRepoUrl}
				placeholder="https://github.com/user/stack-repo.git"
				class="font-mono text-xs"
			/>
		</div>

		<div class="flex flex-col gap-1">
			<label for="compose-branch" class="text-xs font-medium text-kumo-subtle"> Branch </label>
			<Input
				id="compose-branch"
				bind:value={gitBranch}
				placeholder="main"
				class="font-mono text-xs"
			/>
		</div>
	</div>

	<div class="flex flex-col gap-1">
		<label for="compose-path" class="text-xs font-medium text-kumo-subtle">
			Compose file path in repository
		</label>
		<Input
			id="compose-path"
			bind:value={gitComposePath}
			placeholder="docker-compose.yml"
			class="font-mono text-xs"
		/>
	</div>
</div>
