<script lang="ts">
	import { page } from '$app/state';
	import { projects } from '$lib/data';
	import CreateServiceModal from '$lib/components/features/services/create/CreateServiceModal.svelte';

	let projectId = $derived(page.params.projectId ?? '');
	let project = $derived(projects.find((p) => p.id === projectId));
</script>

<svelte:head>
	<title>New Service — {project?.name ?? 'Project'} — GOPOD</title>
</svelte:head>

<CreateServiceModal {projectId} inline={true} />
