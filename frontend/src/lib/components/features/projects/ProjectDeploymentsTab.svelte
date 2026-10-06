<script lang="ts">
	import { goto } from '$app/navigation';
	import { DataTable, StatusBadge } from '$lib/components/ui';
	import type { Project, Deployment } from '$lib/types';

	interface Props {
		project: Project;
		deployments: Deployment[];
		title?: string;
		limit?: number;
	}

	let { project, deployments, title = 'All Project Deployments', limit }: Props = $props();

	let displayedDeployments = $derived(limit ? deployments.slice(0, limit) : deployments);
</script>

<section class="flex flex-col gap-3">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium text-[var(--text-primary)] m-0">{title}</h2>
		<span class="text-xs text-[var(--text-tertiary)] tabular-nums">{deployments.length} total</span>
	</div>

	<DataTable
		columns={[
			{ key: 'serviceName', label: 'Service' },
			{ key: 'version', label: 'Version', mono: true },
			{ key: 'commit', label: 'Commit', mono: true },
			{ key: 'status', label: 'Status', render: depStatusSnippet },
			{ key: 'duration', label: 'Duration' },
			{ key: 'timeAgo', label: 'When' }
		]}
		rows={displayedDeployments}
		keyExtractor={(d) => d.id}
		onRowClick={(d) => goto(`/projects/${project.id}/services/${d.serviceId}`)}
	/>
</section>

{#snippet depStatusSnippet(row: Deployment)}
	<StatusBadge status={row.status} size="sm" />
{/snippet}
