<svelte:head>
	<title>Changelog - glamdring</title>
</svelte:head>

<div class="container changelog">
	<h1>Changelog</h1>

	<div class="timeline">
		<div class="timeline-month">
			<h2>March 2026</h2>
			<div class="timeline-entries">
				<div class="entry">
					<h3>LOTR Theme System</h3>
					<p>
						Five built-in themes (glamdring, rivendell, mithril, lothlorien, shire) with runtime
						switching, high contrast mode, and custom theme support.
					</p>
				</div>
				<div class="entry">
					<h3>Image Paste</h3>
					<p>Ctrl+V clipboard image support for Claude's vision API.</p>
				</div>
				<div class="entry">
					<h3>Documentation Site</h3>
					<p>This site.</p>
				</div>
			</div>
		</div>

		<div class="timeline-month">
			<h2>February 2026</h2>
			<div class="timeline-entries">
				<div class="entry">
					<h3>Agent Teams</h3>
					<p>
						Multi-agent coordination with phase-gated workflows, file locking, task dependencies,
						and inter-agent messaging.
					</p>
				</div>
				<div class="entry">
					<h3>Performance Optimizations</h3>
					<p>Render caching, tool result truncation, and bash output streaming.</p>
				</div>
			</div>
		</div>

		<div class="timeline-month">
			<h2>Earlier</h2>
			<div class="timeline-entries">
				<div class="entry">
					<h3>Core</h3>
					<ul>
						<li>Core agentic loop with streaming responses</li>
						<li>Built-in tools (Read, Write, Edit, Bash, Glob, Grep)</li>
						<li>Permission system with three-tier model</li>
						<li>MCP server support</li>
						<li>Instructions discovery</li>
						<li>Conversation export</li>
						<li>Input history and slash commands</li>
					</ul>
				</div>
			</div>
		</div>
	</div>
</div>

<style>
	.changelog {
		padding-top: var(--space-2xl);
		padding-bottom: var(--space-2xl);
		max-width: 48rem;
	}

	h1 {
		margin-bottom: var(--space-xl);
		color: var(--color-accent);
	}

	.timeline {
		display: flex;
		flex-direction: column;
		gap: var(--space-xl);
	}

	.timeline-month {
		position: relative;
		padding-left: var(--space-lg);
		border-left: 2px solid var(--color-accent);
	}

	.timeline-month h2 {
		position: relative;
		margin-bottom: var(--space-md);
		font-size: 1.25rem;
		color: var(--color-text);
	}

	.timeline-month h2::before {
		content: '';
		position: absolute;
		left: calc(-1 * var(--space-lg) - 5px);
		top: 50%;
		transform: translateY(-50%);
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: var(--color-accent);
	}

	.timeline-entries {
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
	}

	.entry {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: var(--space-lg);
		transition:
			transform var(--transition-fast),
			border-color var(--transition-fast);
	}

	.entry:hover {
		transform: translateY(-2px);
		border-color: var(--color-accent);
	}

	.entry h3 {
		margin-bottom: var(--space-sm);
		font-size: 1.1rem;
	}

	.entry p {
		color: var(--color-text-secondary);
		line-height: 1.6;
	}

	.entry ul {
		color: var(--color-text-secondary);
		line-height: 1.8;
		padding-left: var(--space-lg);
		margin: 0;
	}

	.entry li {
		padding: 2px 0;
	}

	@media (max-width: 768px) {
		.changelog {
			padding-top: var(--space-xl);
			padding-bottom: var(--space-xl);
		}
	}
</style>
