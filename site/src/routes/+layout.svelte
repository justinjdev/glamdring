<script>
	import '$lib/styles/global.css';
	import Nav from '$lib/components/Nav.svelte';
	import Footer from '$lib/components/Footer.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import { page } from '$app/state';
	import { base } from '$app/paths';

	let { children } = $props();

	const sidebarPages = ['/features', '/configuration', '/agent-teams', '/architecture'];

	function hasSidebar(): boolean {
		const path = page.url.pathname;
		return sidebarPages.some((p) => path === `${base}${p}`);
	}
</script>

<svelte:head>
	<meta name="description" content="glamdring - A fast, native TUI for agentic coding with Claude" />
</svelte:head>

<a href="#main-content" class="skip-link">Skip to content</a>
<Nav />
<main id="main-content">
	{#if hasSidebar()}
		<div class="container layout-with-sidebar">
			<Sidebar />
			<div class="content">
				{@render children()}
			</div>
		</div>
	{:else}
		{@render children()}
	{/if}
</main>
<Footer />

<style>
	.layout-with-sidebar {
		display: flex;
		gap: var(--space-lg);
		padding-top: var(--space-lg);
	}
	.content {
		flex: 1;
		min-width: 0;
	}
	@media (max-width: 900px) {
		.layout-with-sidebar {
			display: block;
		}
	}
</style>
