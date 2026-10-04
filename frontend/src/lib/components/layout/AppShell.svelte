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

	onMount(() => {
		ui.init();
	});
</script>

<div
	class="h-screen max-h-screen w-screen max-w-full overflow-hidden bg-[var(--bg-shell)] md:bg-[var(--bg-outer)] flex items-stretch p-0 md:p-[12px] md:px-3.5 md:gap-3 transition-colors duration-200 box-border"
>
	<!-- Sidebar — Desktop (Seamlessly transitions between expanded and minimized rail) -->
	<div class="hidden md:flex shrink-0 h-full max-h-full overflow-hidden">
		<Sidebar />
	</div>

	<!-- Mobile sidebar overlay drawer -->
	{#if ui.sidebarMobileOpen}
		<button
			class="fixed inset-0 bg-black/70 backdrop-blur-xs z-50 md:hidden border-0 cursor-default"
			onclick={() => ui.closeMobileSidebar()}
			aria-label="Close navigation"
		></button>
		<div class="fixed top-0 left-0 bottom-0 z-50 md:hidden w-[260px] bg-[var(--bg-outer)] shadow-2xl">
			<Sidebar onclose={() => ui.closeMobileSidebar()} />
		</div>
	{/if}

	<!-- Main content shell: seamless edge-to-edge on mobile, framed on desktop -->
	<div
		class="flex-1 min-w-0 h-full max-h-full bg-[var(--bg-shell)] rounded-none md:rounded-[var(--radius-shell)] border-0 md:border md:border-[var(--border)] flex flex-col overflow-hidden transition-colors duration-200 shadow-none md:shadow-sm"
	>
		<Topbar />
		<main class="flex-1 min-h-0 {isFullBleed ? 'overflow-hidden p-0' : 'overflow-y-auto overflow-x-hidden px-3.5 md:px-8 pb-4 md:pb-7'}">
			<div class="w-full flex-1 flex flex-col {isFullBleed ? 'h-full pt-0' : 'pt-4 md:pt-7'}">
				{@render children()}
			</div>
		</main>
	</div>
</div>
