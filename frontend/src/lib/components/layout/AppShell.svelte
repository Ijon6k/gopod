<script lang="ts">
	import { page } from '$app/state';
	import Sidebar from './Sidebar.svelte';
	import Topbar from './Topbar.svelte';
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

{#if isAuthPage}
	<!-- Standalone Fullscreen View for Setup & Login -->
	<div
		class="box-border flex h-screen max-h-screen w-screen max-w-full flex-col items-center justify-center overflow-y-auto bg-[var(--bg-outer)] p-4 transition-colors duration-200 md:p-6"
	>
		{@render children()}
	</div>
{:else}
	<div
		class="box-border flex h-screen max-h-screen w-screen max-w-full items-stretch overflow-hidden bg-[var(--bg-shell)] p-0 transition-colors duration-200 md:gap-3 md:bg-[var(--bg-outer)] md:p-[12px] md:px-3.5"
	>
		<!-- Sidebar — Desktop (Seamlessly transitions between expanded and minimized rail) -->
		<div class="hidden h-full max-h-full shrink-0 overflow-hidden md:flex">
			<Sidebar />
		</div>

		<!-- Mobile sidebar overlay drawer -->
		{#if ui.sidebarMobileOpen}
			<button
				class="fixed inset-0 z-50 cursor-default border-0 bg-black/70 backdrop-blur-xs md:hidden"
				onclick={() => ui.closeMobileSidebar()}
				aria-label="Close navigation"
			></button>
			<div
				class="fixed top-0 bottom-0 left-0 z-50 w-[260px] bg-[var(--bg-outer)] shadow-2xl md:hidden"
			>
				<Sidebar onclose={() => ui.closeMobileSidebar()} />
			</div>
		{/if}

		<!-- Main content shell: seamless edge-to-edge on mobile, framed on desktop -->
		<div
			class="flex h-full max-h-full min-w-0 flex-1 flex-col overflow-hidden rounded-none border-0 bg-[var(--bg-shell)] shadow-none transition-colors duration-200 md:rounded-[var(--radius-shell)] md:border md:border-[var(--border)] md:shadow-sm"
		>
			<Topbar />
			<main
				class="min-h-0 flex-1 {isFullBleed
					? 'overflow-hidden p-0'
					: 'overflow-x-hidden overflow-y-auto px-3.5 pb-4 md:px-8 md:pb-7'}"
			>
				<div class="flex w-full flex-1 flex-col {isFullBleed ? 'h-full pt-0' : 'pt-4 md:pt-7'}">
					{@render children()}
				</div>
			</main>
		</div>
	</div>
{/if}
