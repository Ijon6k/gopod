<script lang="ts">
	import type { ServiceType } from '$lib/types';
	import { GitBranch, Package, FileCode, Stack, Cube, FileText, Database } from 'phosphor-svelte';

	export type WorkloadChoice = ServiceType | 'database';

	interface Props {
		selected: WorkloadChoice;
		onselect: (choice: WorkloadChoice) => void;
	}

	let { selected, onselect }: Props = $props();

	const options: {
		id: WorkloadChoice;
		title: string;
		description: string;
		badge?: string;
		icon: any;
	}[] = [
		{
			id: 'application',
			title: 'Application',
			description: 'Deploy from Git repository using Dockerfile or cloud-native buildpack',
			icon: GitBranch
		},
		{
			id: 'image',
			title: 'Container Image',
			description: 'Deploy an existing container image from any public or private registry',
			icon: Package
		},
		{
			id: 'compose',
			title: 'Compose',
			description: 'Declarative multi-container application defined by compose.yaml',
			icon: Stack
		},
		{
			id: 'pod',
			title: 'Podman Pod',
			description: 'Group co-located containers sharing localhost networking and namespaces',
			icon: Cube
		},
		{
			id: 'kubernetes',
			title: 'Kubernetes YAML',
			description: 'Direct manifest deployment powered by Podman kube play integration',
			icon: FileCode
		},
		{
			id: 'quadlet',
			title: 'Quadlet',
			description: 'Declarative systemd-managed Podman containers and pods (.container/.pod)',
			icon: FileText
		},
		{
			id: 'database',
			title: 'Database Preset',
			description: 'Pre-configured container workloads for PostgreSQL, Redis, MySQL, or MongoDB',
			badge: 'Preset',
			icon: Database
		}
	];
</script>

<div class="flex flex-col gap-2.5">
	<span class="text-xs font-medium text-[var(--text-secondary)]">
		Workload Type
	</span>

	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2.5">
		{#each options as opt}
			{@const Icon = opt.icon}
			{@const isSelected = selected === opt.id}
			<button
				type="button"
				onclick={() => onselect(opt.id)}
				class="relative flex flex-col text-left p-3.5 rounded-[var(--radius-card)] border transition-all cursor-pointer {isSelected
					? 'bg-[var(--bg-surface)] border-[var(--accent)]'
					: 'bg-[var(--bg-panel)] border-[var(--border)] hover:border-[var(--border-subtle)] hover:bg-[var(--bg-hover)]'}"
			>
				<div class="flex items-center justify-between mb-2">
					<div class="flex items-center gap-2">
						<span
							class="flex items-center justify-center w-7 h-7 rounded-[var(--radius-sm)] {isSelected
								? 'bg-[var(--accent)] text-white'
								: 'bg-[rgba(255,255,255,0.04)] text-[var(--text-secondary)]'}"
						>
							<Icon size={16} />
						</span>
						<span class="text-sm font-medium text-[var(--text-primary)]">{opt.title}</span>
					</div>
					{#if opt.badge}
						<span
							class="text-[9.5px] font-medium px-1.5 py-0.5 rounded bg-[rgba(105,115,168,0.12)] text-[var(--accent)] border border-[rgba(105,115,168,0.2)]"
						>
							{opt.badge}
						</span>
					{/if}
				</div>
				<p class="m-0 text-xs text-[var(--text-tertiary)] leading-relaxed">
					{opt.description}
				</p>
			</button>
		{/each}
	</div>
</div>
