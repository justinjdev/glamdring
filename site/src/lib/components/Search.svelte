<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';

	type SearchResult = {
		url: string;
		meta: { title?: string };
		excerpt: string;
	};

	let query = $state('');
	let results = $state<SearchResult[]>([]);
	let open = $state(false);
	let pagefind: any = null;
	let inputEl: HTMLInputElement | undefined = $state();

	async function loadPagefind() {
		if (pagefind) return;
		try {
			pagefind = await import(/* @vite-ignore */ `${base}/pagefind/pagefind.js`);
			await pagefind.init();
		} catch {
			// Pagefind not available in dev mode
		}
	}

	async function search() {
		if (!pagefind || !query.trim()) {
			results = [];
			return;
		}
		const response = await pagefind.search(query);
		const loaded = await Promise.all(response.results.slice(0, 8).map((r: any) => r.data()));
		results = loaded;
	}

	function openSearch() {
		open = true;
		loadPagefind();
		setTimeout(() => inputEl?.focus(), 50);
	}

	function closeSearch() {
		open = false;
		query = '';
		results = [];
	}

	$effect(() => {
		if (!browser) return;
		function onKeydown(e: KeyboardEvent) {
			if (e.key === '/' && !open && !(e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement)) {
				e.preventDefault();
				openSearch();
			}
			if (e.key === 'Escape' && open) {
				closeSearch();
			}
		}
		window.addEventListener('keydown', onKeydown);
		return () => window.removeEventListener('keydown', onKeydown);
	});
</script>

<button class="search-trigger" onclick={openSearch} aria-label="Search (press /)">
	<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
		<circle cx="11" cy="11" r="8"/>
		<path d="M21 21l-4.35-4.35"/>
	</svg>
	<span class="search-hint">/</span>
</button>

{#if open}
	<div class="search-overlay" onclick={closeSearch} role="presentation">
		<div class="search-dialog" onclick={(e) => e.stopPropagation()} role="dialog" aria-label="Search documentation">
			<input
				bind:this={inputEl}
				bind:value={query}
				oninput={search}
				type="search"
				placeholder="Search documentation..."
				class="search-input"
				aria-label="Search"
			/>
			{#if results.length > 0}
				<ul class="search-results">
					{#each results as result (result.url)}
						<li>
							<a href={result.url} onclick={closeSearch}>
								<strong>{result.meta.title ?? 'Untitled'}</strong>
								<span class="search-excerpt">{@html result.excerpt}</span>
							</a>
						</li>
					{/each}
				</ul>
			{:else if query.trim()}
				<p class="search-empty">No results for "{query}"</p>
			{/if}
		</div>
	</div>
{/if}

<style>
	.search-trigger {
		background: none;
		border: 1px solid var(--color-border);
		color: var(--color-text-secondary);
		padding: var(--space-xs) var(--space-sm);
		border-radius: 6px;
		cursor: pointer;
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		transition: border-color var(--transition-fast), color var(--transition-fast);
	}
	.search-trigger:hover {
		border-color: var(--color-accent);
		color: var(--color-accent);
	}
	.search-hint {
		font-family: var(--font-code);
		font-size: 0.75rem;
		opacity: 0.6;
	}
	.search-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		z-index: 200;
		display: flex;
		justify-content: center;
		padding-top: 15vh;
	}
	.search-dialog {
		background: var(--color-bg-elevated);
		border: 1px solid var(--color-border);
		border-radius: 12px;
		width: 36rem;
		max-width: 90vw;
		max-height: 60vh;
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}
	.search-input {
		font-family: var(--font-body);
		font-size: 1.125rem;
		padding: var(--space-md);
		background: transparent;
		border: none;
		border-bottom: 1px solid var(--color-border);
		color: var(--color-text);
		outline: none;
		width: 100%;
	}
	.search-input::placeholder { color: var(--color-text-secondary); }
	.search-results {
		list-style: none;
		padding: var(--space-sm);
		overflow-y: auto;
	}
	.search-results li { margin-bottom: 2px; }
	.search-results a {
		display: block;
		padding: var(--space-sm) var(--space-md);
		border-radius: 6px;
		text-decoration: none;
		color: var(--color-text);
		transition: background var(--transition-fast);
	}
	.search-results a:hover { background: var(--color-bg-card); }
	.search-results strong {
		display: block;
		color: var(--color-accent);
		font-size: 0.95rem;
		margin-bottom: var(--space-xs);
	}
	.search-excerpt {
		font-size: 0.85rem;
		color: var(--color-text-secondary);
		line-height: 1.5;
	}
	:global(.search-excerpt mark) {
		background: rgba(125, 174, 163, 0.3);
		color: var(--color-accent);
		border-radius: 2px;
		padding: 0 2px;
	}
	.search-empty {
		padding: var(--space-lg);
		text-align: center;
		color: var(--color-text-secondary);
		font-size: 0.95rem;
	}
</style>
