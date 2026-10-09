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
		yaml = $bindable(`apiVersion: v1
kind: Pod
metadata:
  name: kube-workload
spec:
  containers:
    - name: web
      image: docker.io/library/nginx:alpine
      ports:
        - containerPort: 80`)
	}: Props = $props();

	let validationStatus = $state<'idle' | 'valid' | 'invalid'>('idle');
	let validationMsg = $state('');

	function validateYaml() {
		try {
			if (!yaml.trim()) {
				validationStatus = 'invalid';
				validationMsg = 'Manifest cannot be empty.';
				return;
			}
			if (!yaml.includes('apiVersion:') || !yaml.includes('kind:')) {
				validationStatus = 'invalid';
				validationMsg = 'Kubernetes manifest must include "apiVersion:" and "kind:".';
				return;
			}
			validationStatus = 'valid';
			validationMsg = 'Valid Kubernetes manifest for Podman kube play.';
		} catch (err: any) {
			validationStatus = 'invalid';
			validationMsg = err.message || 'Syntax error.';
		}
	}
</script>

<div class="flex flex-col gap-4">
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<div class="flex flex-col gap-1.5">
			<label for="k8s-svc-name" class="text-xs font-medium text-[var(--text-secondary)]">
				Service name <span class="text-[var(--status-red)]">*</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input id="k8s-svc-name" bind:value={name} placeholder="e.g. k8s-pod" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="k8s-svc-desc" class="text-xs font-medium text-[var(--text-secondary)]">
				Description <span class="text-[var(--text-tertiary)]">(optional)</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input id="k8s-svc-desc" bind:value={description} placeholder="Podman kube play manifest" />
			</div>
		</div>
	</div>

	<div class="flex flex-col gap-2">
		<div class="flex items-center justify-between">
			<label for="k8s-editor" class="text-xs font-medium text-[var(--text-secondary)]">
				Kubernetes YAML manifest <span class="text-[var(--status-red)]">*</span>
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
				<Button variant="secondary" size="sm" onclick={validateYaml}>Validate Manifest</Button>
			</div>
		</div>

		<CodeEditor bind:value={yaml} language="yaml" height="260px" />
		<span class="text-[11px] text-[var(--text-tertiary)]">
			Executed natively by Podman via <code>podman kube play</code>.
		</span>
	</div>
</div>
