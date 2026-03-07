<script>
	import { base } from '$app/paths';
	import CopyButton from '$lib/components/CopyButton.svelte';

	const installCmd = 'curl -fsSL https://raw.githubusercontent.com/justinjdev/glamdring/main/install.sh | sh';
	const buildCmds = `git clone https://github.com/justinjdev/glamdring.git
cd glamdring
go build -o glamdring .`;

	const features = [
		{
			title: 'Agentic Loop',
			description: 'Streaming responses, multi-turn conversations with persistent session memory, extended thinking, prompt caching, and per-model cost tracking.'
		},
		{
			title: 'Built-in Tools',
			description: 'Read, Write, Edit, Bash, Glob, Grep -- all with safety guards. Plus shire code indexer integration for intelligent code search.'
		},
		{
			title: 'Permission System',
			description: 'Three-tier model with always-allow, prompt, and block rules. Path and command-scoped presets. YOLO mode for full auto-approval.'
		},
		{
			title: 'Themes',
			description: 'Five LOTR-inspired color themes with runtime switching via /theme. High contrast mode and custom user-defined themes.'
		},
		{
			title: 'Agent Teams',
			description: 'Coordinated multi-agent workflows with phase-gated tool access, inter-agent messaging, file locking, and task dependencies.'
		},
		{
			title: 'MCP Support',
			description: 'Connect external tool servers via stdio transport. Health monitoring, per-tool enable/disable, and runtime management.'
		}
	];
</script>

<svelte:head>
	<title>glamdring</title>
</svelte:head>

<section class="hero">
	<div class="container">
		<h1 class="hero-title animate-in">glamdring</h1>
		<p class="hero-tagline animate-in" style="animation-delay: 100ms">
			A fast, native TUI for agentic coding with Claude
		</p>
		<p class="hero-description animate-in" style="animation-delay: 200ms">
			Built in Go with Charm libraries. Native binary, streaming responses, multi-turn
			conversations, built-in tools, and full MCP support.
		</p>
		<div class="hero-install animate-in" style="animation-delay: 300ms">
			<div class="code-block-wrapper">
				<pre><code>{installCmd}</code></pre>
				<div class="copy-btn-pos">
					<CopyButton text={installCmd} />
				</div>
			</div>
		</div>
	</div>
</section>

<section class="features">
	<div class="container">
		<h2 class="section-heading animate-in" style="animation-delay: 400ms">Features</h2>
		<div class="feature-grid">
			{#each features as feature, i (feature.title)}
				<div class="feature-card animate-in" style="animation-delay: {500 + i * 100}ms">
					<h3>{feature.title}</h3>
					<p>{feature.description}</p>
				</div>
			{/each}
		</div>
	</div>
</section>

<section class="install-section">
	<div class="container">
		<h2 class="section-heading animate-in" style="animation-delay: 1100ms">Install</h2>

		<div class="install-method animate-in" style="animation-delay: 1200ms">
			<h3>Quick install</h3>
			<div class="code-block-wrapper">
				<pre><code>{installCmd}</code></pre>
				<div class="copy-btn-pos">
					<CopyButton text={installCmd} />
				</div>
			</div>
		</div>

		<div class="install-method animate-in" style="animation-delay: 1300ms">
			<h3>Build from source</h3>
			<div class="code-block-wrapper">
				<pre><code>{buildCmds}</code></pre>
				<div class="copy-btn-pos">
					<CopyButton text={buildCmds} />
				</div>
			</div>
		</div>
	</div>
</section>

<section class="cta">
	<div class="container animate-in" style="animation-delay: 1400ms">
		<a href="{base}/getting-started" class="cta-link">Get started &rarr;</a>
	</div>
</section>

<style>
	.hero {
		padding-top: var(--space-2xl);
		padding-bottom: var(--space-xl);
		text-align: center;
	}

	.hero-title {
		font-size: 4rem;
		color: var(--color-accent);
		letter-spacing: 0.06em;
		margin-bottom: var(--space-md);
	}

	.hero-tagline {
		font-size: 1.5rem;
		color: var(--color-text);
		margin-bottom: var(--space-md);
		max-width: 100%;
	}

	.hero-description {
		font-size: 1.125rem;
		color: var(--color-text-secondary);
		max-width: 40rem;
		margin: 0 auto var(--space-lg);
	}

	.hero-install {
		max-width: 48rem;
		margin: 0 auto;
		text-align: left;
	}

	.code-block-wrapper {
		position: relative;
	}

	.copy-btn-pos {
		position: absolute;
		top: var(--space-sm);
		right: var(--space-sm);
	}

	.section-heading {
		margin-bottom: var(--space-lg);
	}

	.features {
		margin-top: var(--space-2xl);
	}

	.feature-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: var(--space-lg);
	}

	.feature-card {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: var(--space-lg);
		transition: transform var(--transition-fast), border-color var(--transition-fast);
	}

	.feature-card:hover {
		transform: translateY(-2px);
		border-color: var(--color-accent);
	}

	.feature-card h3 {
		margin-bottom: var(--space-sm);
	}

	.feature-card p {
		color: var(--color-text-secondary);
		line-height: 1.6;
	}

	.install-section {
		margin-top: var(--space-2xl);
	}

	.install-method {
		margin-bottom: var(--space-lg);
	}

	.install-method h3 {
		margin-bottom: var(--space-sm);
	}

	.cta {
		margin-top: var(--space-2xl);
		margin-bottom: var(--space-2xl);
		text-align: center;
	}

	.cta-link {
		font-family: var(--font-heading);
		font-size: 1.5rem;
		color: var(--color-accent);
		border: 2px solid var(--color-accent);
		padding: var(--space-md) var(--space-xl);
		border-radius: 8px;
		display: inline-block;
		transition: background-color var(--transition-fast), color var(--transition-fast);
	}

	.cta-link:hover {
		background-color: var(--color-accent);
		color: var(--color-bg);
	}

	@media (max-width: 768px) {
		.hero-title {
			font-size: 2.5rem;
		}

		.hero-tagline {
			font-size: 1.25rem;
		}

		.feature-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
