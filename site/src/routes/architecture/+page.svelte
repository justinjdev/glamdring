<script>
	import CopyButton from '$lib/components/CopyButton.svelte';
</script>

<svelte:head>
	<title>Architecture - glamdring</title>
</svelte:head>

<div class="page">
	<h1>Architecture</h1>

	<p>glamdring has a layered architecture. <code>pkg/</code> is the reusable engine. <code>internal/tui/</code> is the terminal frontend. The boundary is designed so alternative frontends can consume <code>pkg/</code> directly.</p>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="package-overview">Package Overview</h2>

		<div class="code-block">
			<CopyButton text={`pkg/
  agent/       Core agentic loop, Session, permission system
  api/         Claude Messages API client (HTTP + SSE, prompt caching, retry)
  tools/       Built-in tools + Task tool for subagents
  teams/       Agent teams coordination (members, tasks, messaging, phases)
  index/       Shire index Go bindings (read-only SQLite queries)
  mcp/         MCP client (stdio JSON-RPC)
  config/      Instructions discovery, system prompt, settings, paths
  hooks/       Event hook system
  commands/    Slash command discovery + expansion
  agents/      Custom agent definitions

internal/
  tui/         Bubbletea TUI (not part of library API)

cmd/
  glamdring/   Entry point`} />
			<pre><code>{`pkg/
  agent/       Core agentic loop, Session, permission system
  api/         Claude Messages API client (HTTP + SSE, prompt caching, retry)
  tools/       Built-in tools + Task tool for subagents
  teams/       Agent teams coordination (members, tasks, messaging, phases)
  index/       Shire index Go bindings (read-only SQLite queries)
  mcp/         MCP client (stdio JSON-RPC)
  config/      Instructions discovery, system prompt, settings, paths
  hooks/       Event hook system
  commands/    Slash command discovery + expansion
  agents/      Custom agent definitions

internal/
  tui/         Bubbletea TUI (not part of library API)

cmd/
  glamdring/   Entry point`}</code></pre>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="agent-loop">pkg/agent</h2>

		<p>Core agent orchestration. Manages multi-turn conversations with persistent session memory. Handles the streaming loop: send messages to Claude, receive streaming responses, process tool calls, execute tools, send results back. Permission system integrated at this layer.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="api-client">pkg/api</h2>

		<p>Claude Messages API client. HTTP + SSE streaming. Prompt caching support. Exponential backoff retry logic. Request/response type definitions.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="tools">pkg/tools</h2>

		<p>Built-in tool implementations. Each tool has its own file (<code>read.go</code>, <code>write.go</code>, <code>edit.go</code>, <code>bash.go</code>, <code>glob.go</code>, <code>grep.go</code>). Tool registry for registration and discovery. Task tool enables subagent spawning.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="teams">pkg/teams</h2>

		<p>Agent teams coordination (42+ files). Members, tasks, messaging, phases, decorators, file locking, mailbox system. Phase registry with built-in and custom workflows. Tool decorators for phase-gated access.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="config">pkg/config</h2>

		<p>Instructions file discovery at every directory level. Settings struct with theme fields. Permission rules. Config path resolution with primary/fallback namespaces. System prompt generation.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="mcp-client">pkg/mcp</h2>

		<p>MCP client implementation using stdio JSON-RPC transport. Health monitoring. Per-tool enable/disable.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="other-packages">Other Packages</h2>

		<ul class="package-list">
			<li><strong>pkg/auth</strong> -- Authentication and credential management</li>
			<li><strong>pkg/index</strong> -- Shire indexer Go bindings (read-only SQLite)</li>
			<li><strong>pkg/hooks</strong> -- Event hook system</li>
			<li><strong>pkg/commands</strong> -- Slash command discovery and expansion</li>
			<li><strong>pkg/agents</strong> -- Custom agent definitions</li>
		</ul>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="tui">internal/tui</h2>

		<p>Bubbletea TUI frontend. Root model, input textarea, output rendering with scrolling, status bar with context %, model info, and token counts. Not part of the library API -- consumers use <code>pkg/</code> directly.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="performance">Performance</h2>

		<ul class="perf-list">
			<li><strong>Render caching:</strong> Finalized output blocks cache their rendered markdown. Only the active (streaming) block is re-rendered on each update.</li>
			<li><strong>Tool result truncation:</strong> Tool results exceeding 50KB are truncated before being sent to the API to protect the context window. Full output still shown in TUI.</li>
			<li><strong>Bash output streaming:</strong> Line-by-line streaming to TUI as output arrives, rather than buffering until completion.</li>
		</ul>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="design-philosophy">Design Philosophy</h2>

		<ul class="philosophy-list">
			<li>Minimal external dependencies, heavy use of stdlib and Charm libraries</li>
			<li>Clear package boundary between reusable engine (<code>pkg/</code>) and frontend (<code>internal/tui/</code>)</li>
			<li>Single native binary (~27MB) with no runtime dependencies</li>
			<li>Performance-first: render caching, output streaming, result truncation</li>
		</ul>
	</section>
</div>

<style>
	.page {
		padding-bottom: var(--space-2xl);
	}

	h1 {
		margin-bottom: var(--space-xl);
	}

	section {
		margin-bottom: var(--space-md);
	}

	h2 {
		margin-bottom: var(--space-lg);
	}

	p {
		margin-bottom: var(--space-sm);
		max-width: 42rem;
	}

	.code-block {
		position: relative;
		margin-bottom: var(--space-lg);
	}

	.code-block :global(.copy-btn) {
		position: absolute;
		top: var(--space-sm);
		right: var(--space-sm);
		z-index: 1;
	}

	.package-list,
	.perf-list,
	.philosophy-list {
		margin-bottom: var(--space-md);
		padding-left: var(--space-lg);
		max-width: 42rem;
	}

	.package-list li,
	.perf-list li,
	.philosophy-list li {
		margin-bottom: var(--space-xs);
		line-height: 1.6;
	}
</style>
