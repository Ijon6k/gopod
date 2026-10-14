<script lang="ts">
	import { page } from '$app/state';
	import Sidebar from './Sidebar.svelte';
	import Topbar from './Topbar.svelte';
	import { Sidebar as KumoSidebar, Toasty } from 'kumo-svelte';
	import { ui } from '$lib/stores/ui.svelte';
	import { onMount, type Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	let isFullBleed = $derived(page.url.pathname === '/topology');
	let isAuthPage = $derived(page.url.pathname === '/login' || page.url.pathname === '/setup');

	onMount(() => {
		ui.init();
	});
</script>

<Toasty>
	{#if isAuthPage}
		<!-- Standalone Fullscreen View for Setup & Login -->
		<div
			class="box-border flex h-screen max-h-screen w-screen max-w-full flex-col items-center justify-center overflow-y-auto bg-[var(--bg-canvas)] p-4 md:p-6"
		>
			{@render children()}
		</div>
	{:else}
		<!-- Cloudflare Kumo Docked Edge-to-Edge Architecture -->
		<KumoSidebar.Provider
			defaultOpen={true}
			collapsible="icon"
			defaultWidth={240}
			class="h-screen max-h-screen w-screen max-w-full overflow-hidden bg-[var(--bg-canvas)] text-[var(--text-primary)]"
		>
			<!-- Level 2: Left Docked Navigation (Handles desktop rail & mobile drawer) -->
			<Sidebar />

			<!-- Main Workspace (Edge-to-Edge Natural Flow, No Wrapper Box Clutter) -->
			<div class="flex h-screen min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
				<!-- Level 1: Global Context - Sticky Docked Topbar -->
				<Topbar />

				<!-- Level 4: Action & Content Canvas (Docked Edge-to-Edge) -->
				<main
					class="min-h-0 w-full flex-1 {isFullBleed
						? 'overflow-hidden p-0'
						: 'overflow-x-hidden overflow-y-auto px-4 py-5 md:px-6 md:py-6'}"
				>
					{@render children()}
				</main>
			</div>
		</KumoSidebar.Provider>
	{/if}
</Toasty>
