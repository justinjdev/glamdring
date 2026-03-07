<script lang="ts">
	import { browser } from '$app/environment';

	type Heading = { id: string; text: string; level: number };

	let { selector = 'h2, h3' }: { selector?: string } = $props();
	let headings = $state<Heading[]>([]);
	let activeId = $state('');
	let sidebarOpen = $state(false);

	$effect(() => {
		if (!browser) return;
		const main = document.getElementById('main-content');
		if (!main) return;

		const els = main.querySelectorAll(selector);
		headings = Array.from(els).map((el) => ({
			id: el.id,
			text: el.textContent ?? '',
			level: parseInt(el.tagName[1])
		})).filter((h) => h.id);

		const observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) {
						activeId = entry.target.id;
						break;
					}
				}
			},
			{ rootMargin: '-80px 0px -70% 0px' }
		);

		els.forEach((el) => { if (el.id) observer.observe(el); });
		return () => observer.disconnect();
	});
</script>

{#if headings.length > 0}
	<button
		class="sidebar-toggle"
		onclick={() => sidebarOpen = !sidebarOpen}
		aria-label="{sidebarOpen ? 'Close' : 'Open'} section navigation"
		aria-expanded={sidebarOpen}
	>
		<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<path d="M3 12h18M3 6h18M3 18h18"/>
		</svg>
		<span class="sr-only">Sections</span>
	</button>

	<aside class="sidebar" class:open={sidebarOpen}>
		<nav aria-label="Page sections">
			<ul>
				{#each headings as heading (heading.id)}
					<li class:nested={heading.level > 2}>
						<a
							href="#{heading.id}"
							class:active={activeId === heading.id}
							onclick={() => sidebarOpen = false}
						>
							{heading.text}
						</a>
					</li>
				{/each}
			</ul>
		</nav>
	</aside>
{/if}

<style>
	.sidebar {
		position: sticky;
		top: calc(var(--nav-height) + var(--space-lg));
		max-height: calc(100vh - var(--nav-height) - var(--space-2xl));
		overflow-y: auto;
		width: 14rem;
		flex-shrink: 0;
		padding-right: var(--space-md);
		border-right: 1px solid var(--color-border);
	}
	ul {
		list-style: none;
		padding: 0;
	}
	li { margin-bottom: var(--space-xs); }
	li.nested { padding-left: var(--space-md); }
	a {
		display: block;
		font-size: 0.85rem;
		color: var(--color-text-secondary);
		padding: var(--space-xs) var(--space-sm);
		border-radius: 4px;
		text-decoration: none;
		transition: color var(--transition-fast);
		line-height: 1.4;
	}
	a:hover { color: var(--color-accent); }
	a.active {
		color: var(--color-accent);
		font-weight: 600;
		border-left: 2px solid var(--color-accent);
		padding-left: calc(var(--space-sm) - 2px);
	}
	.sidebar-toggle {
		display: none;
		position: fixed;
		bottom: var(--space-md);
		right: var(--space-md);
		z-index: 50;
		background: var(--color-bg-elevated);
		border: 1px solid var(--color-border);
		color: var(--color-text);
		padding: var(--space-sm);
		border-radius: 8px;
		cursor: pointer;
	}
	@media (max-width: 900px) {
		.sidebar-toggle { display: flex; }
		.sidebar {
			display: none;
			position: fixed;
			bottom: calc(var(--space-md) + 3rem);
			right: var(--space-md);
			z-index: 50;
			background: var(--color-bg-elevated);
			border: 1px solid var(--color-border);
			border-radius: 8px;
			padding: var(--space-md);
			width: auto;
			max-width: 16rem;
			top: auto;
			max-height: 50vh;
			border-right: none;
		}
		.sidebar.open { display: block; }
	}
</style>
