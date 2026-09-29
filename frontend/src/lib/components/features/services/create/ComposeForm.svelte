<script lang="ts">
	import { Input, Button } from '$lib/components/primitives';
	import { CodeEditor } from '$lib/components/ui';
	import { CheckCircle, WarningCircle } from 'phosphor-svelte';

	interface Props {
		name: string;
		description: string;
		yaml: string;
	}

	let {
		name = $bindable(''),
		description = $bindable(''),
		yaml = $bindable(`services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
    restart: always

  api:
    image: node:20-alpine
    environment:
      - NODE_ENV=production
      - PORT=3000
    restart: unless-stopped`)
	}: Props = $props();

	let validationStatus = $state<'idle' | 'valid' | 'invalid'>('idle');
	let validationMsg = $state('');

	function validateYaml() {
		try {
			if (!yaml.trim()) {
				validationStatus = 'invalid';
				validationMsg = 'Compose YAML cannot be empty.';
				return;
			}
			if (!yaml.includes('services:')) {
				validationStatus = 'invalid';
				validationMsg = 'Compose file must contain a top-level "services:" mapping.';
				return;
			}
			// Count services defined
			const matches = yaml.match(/^\s\s([a-zA-Z0-9_-]+):/gm);
			const count = matches ? matches.length : 1;
			validationStatus = 'valid';
			validationMsg = `Valid compose definition (${count} service${count > 1 ? 's' : ''} detected).`;
		} catch (err: any) {
			validationStatus = 'invalid';
			validationMsg = err.message || 'Syntax error in YAML.';
		}
	}
</script>

<div class="flex flex-col gap-4">
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
		<div class="flex flex-col gap-1.5">
			<label for="compose-svc-name" class="text-xs text-[var(--text-secondary)] font-medium">
				Service name <span class="text-[var(--status-red)]">*</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="compose-svc-name" bind:value={name} placeholder="e.g. app-stack" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="compose-svc-desc" class="text-xs text-[var(--text-secondary)] font-medium">
				Description <span class="text-[var(--text-tertiary)]">(optional)</span>
			</label>
			<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
				<Input id="compose-svc-desc" bind:value={description} placeholder="Multi-container compose application" />
			</div>
		</div>
	</div>

	<div class="flex flex-col gap-2">
		<div class="flex items-center justify-between">
			<label for="compose-editor" class="text-xs text-[var(--text-secondary)] font-medium">
				Compose YAML specification <span class="text-[var(--status-red)]">*</span>
			</label>
			<div class="flex items-center gap-2">
				{#if validationStatus === 'valid'}
					<span class="flex items-center gap-1 text-[11px] text-[var(--status-green)]">
						<CheckCircle size={14} />
						{validationMsg}
					</span>
				{:else if validationStatus === 'invalid'}
					<span class="flex items-center gap-1 text-[11px] text-[var(--status-red)]">
						<WarningCircle size={14} />
						{validationMsg}
					</span>
				{/if}
				<Button variant="secondary" size="sm" onclick={validateYaml}>Validate YAML</Button>
			</div>
		</div>

		<CodeEditor bind:value={yaml} language="yaml" height="260px" />
		<span class="text-[11px] text-[var(--text-tertiary)]">
			The Compose file remains the source of truth for this workload.
		</span>
	</div>
</div>
