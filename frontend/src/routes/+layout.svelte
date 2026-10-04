<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { AppShell } from '$lib/components/layout';
	import { dataStore } from '$lib/stores/data.svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';

	let { children } = $props();
	let initialCheckDone = $state(false);

	function routeGuard(initialized: boolean | null, authenticated: boolean, pathname: string) {
		if (initialized === null) return;

		// Case 1: Fresh installation / no users exist -> redirect to /setup
		if (!initialized) {
			if (pathname !== '/setup') {
				goto('/setup', { replaceState: true });
			}
			return;
		}

		// Case 2: Cluster initialized but user is not authenticated
		if (!authenticated) {
			if (pathname !== '/login') {
				goto('/login', { replaceState: true });
			}
			return;
		}

		// Case 3: Fully authenticated user trying to access /login or /setup
		if (pathname === '/login' || pathname === '/setup') {
			goto('/', { replaceState: true });
		}
	}

	onMount(async () => {
		try {
			await authStore.checkStatus();
		} catch (e) {
			console.error('Failed to verify authentication status:', e);
		} finally {
			initialCheckDone = true;
		}

		// Check route upon mounting
		routeGuard(authStore.isInitialized, authStore.isAuthenticated, page.url.pathname);

		// Polling stats only when authenticated
		if (authStore.isAuthenticated) {
			dataStore.fetchLiveStats();
		}
		const interval = setInterval(() => {
			if (authStore.isAuthenticated) {
				dataStore.fetchLiveStats();
			}
		}, 4000);
		return () => clearInterval(interval);
	});

	$effect(() => {
		if (initialCheckDone) {
			routeGuard(authStore.isInitialized, authStore.isAuthenticated, page.url.pathname);
		}
	});
</script>

<svelte:head>
	<title>GOPOD — Infrastructure Dashboard</title>
	<meta name="description" content="Self-hosted container management dashboard powered by Podman" />
</svelte:head>

{#if !initialCheckDone && page.url.pathname !== '/login' && page.url.pathname !== '/setup'}
	<div class="h-screen w-screen bg-[var(--bg-outer)] flex items-center justify-center">
		<div class="flex flex-col items-center gap-3">
			<div class="w-8 h-8 rounded-full border-2 border-[var(--accent)] border-t-transparent animate-spin"></div>
			<span class="text-xs font-mono text-[var(--text-tertiary)]">Verifying cluster session…</span>
		</div>
	</div>
{:else}
	<AppShell>
		{@render children()}
	</AppShell>
{/if}
