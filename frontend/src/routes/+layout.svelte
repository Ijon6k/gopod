<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { AppShell } from '$lib/components/layout';
	import { dataStore } from '$lib/stores/data.svelte';

	let { children } = $props();

	onMount(() => {
		dataStore.fetchLiveStats();
		const interval = setInterval(() => {
			dataStore.fetchLiveStats();
		}, 4000);
		return () => clearInterval(interval);
	});
</script>

<svelte:head>
	<title>GOPOD — Infrastructure Dashboard</title>
	<meta name="description" content="Self-hosted container management dashboard powered by Podman" />
</svelte:head>

<AppShell>
	{@render children()}
</AppShell>

