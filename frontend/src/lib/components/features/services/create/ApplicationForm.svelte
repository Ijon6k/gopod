<script lang="ts">
	import { Input, Select, Field } from '$lib/components/ui';

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
		<Field label="Service name" required>
			<Input id="app-svc-name" bind:value={name} placeholder="e.g. web-api" />
		</Field>

		<Field label="Description (optional)">
			<Input
				id="app-svc-desc"
				bind:value={description}
				placeholder="Frontend application or API"
			/>
		</Field>
	</div>

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
		<Select
			id="git-provider"
			label="Git Provider"
			value={gitProvider}
			onValueChange={(val) => {
				if (typeof val === 'string') gitProvider = val;
			}}
			options={[
				{ label: 'GitHub', value: 'github' },
				{ label: 'GitLab', value: 'gitlab' },
				{ label: 'Custom Git URL', value: 'custom' }
			]}
			size="sm"
		/>

		<div class="sm:col-span-2">
			<Field label="Repository" required>
				<Input
					id="app-repo"
					bind:value={repo}
					placeholder="github.com/organization/repository"
					class="text-xs font-[var(--font-mono)]"
				/>
			</Field>
		</div>
	</div>

	<div class="max-w-[240px]">
		<Field label="Branch">
			<Input
				id="app-branch"
				bind:value={branch}
				placeholder="main"
				class="text-xs font-[var(--font-mono)]"
			/>
		</Field>
	</div>
</div>
