<script lang="ts">
	import { base } from '$app/paths';
	import CopyButton from '$lib/components/CopyButton.svelte';
</script>

<svelte:head>
	<title>Features - glamdring</title>
</svelte:head>

<div class="container page">
	<h1>Features</h1>

	<p>glamdring ships with a full-featured agentic coding environment out of the box. No plugins, no extensions -- just a single binary with everything built in.</p>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="agentic-loop">Agentic Loop</h2>

		<p>Streaming responses and multi-turn conversations with persistent session memory. The agent maintains context across turns, remembering what it has read, edited, and discussed.</p>

		<p>Extended thinking can be toggled at runtime with <code>/thinking</code>. Prompt caching is supported for efficient API usage, reducing costs on repeated context.</p>

		<p>Per-model cost tracking provides accurate pricing for Opus, Sonnet, and Haiku. The status bar shows live <code>ctx: N%</code> usage with color thresholds -- gold at 60%, red at 80% -- and the agent will suggest <code>/compact</code> inline when context pressure is high.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="agent-interrupt">Agent Interrupt</h2>

		<p><code>Ctrl+C</code> cancels the current turn instead of killing the program. Double-press to quit. A visual thinking spinner is displayed while the agent is processing, so you always know when it is working.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="built-in-tools">Built-in Tools</h2>

		<p>Six built-in tools cover file operations, search, and shell execution. Each tool is designed with safety guards and sensible defaults.</p>

		<h3>Read</h3>
		<p>Reads files with a 2000-line default limit. Long lines are truncated to prevent buffer overflows in the context window.</p>

		<h3>Write</h3>
		<p>Writes files with a read-before-write safety check. The agent must have read a file before it can overwrite it, preventing accidental data loss.</p>

		<h3>Edit</h3>
		<p>Applies targeted edits that preserve file permissions. No-op edits (edits that would not change anything) are rejected to keep the conversation honest.</p>

		<h3>Bash</h3>
		<p>Executes shell commands with timeout detection and a 1MB output limit. Supports background execution and real-time output streaming -- output is sent line-by-line to the TUI as it arrives.</p>

		<h3>Glob</h3>
		<p>Pattern-based file search with noise directory filtering. Directories like <code>node_modules</code>, <code>.git</code>, and <code>vendor</code> are ignored by default. Results are capped to prevent context flooding.</p>

		<h3>Grep</h3>
		<p>Content search with full ripgrep-style flags. Includes binary file detection and type filters for scoping searches to specific languages.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="shire-indexer">Shire Indexer</h2>

		<p>Code indexer integration for intelligent code search across your project. The indexer is auto-detected when <code>.shire/index.db</code> exists in the project root.</p>

		<p>The index is automatically rebuilt after file changes, keeping search results current. Indexer behavior is configurable via settings.</p>

		<p>On startup, if no index is found, glamdring prompts you to build one. Set <code>indexer.auto_build: true</code> to skip the prompt and build automatically.</p>

		<figure>
			<img src="{base}/screenshots/index-prompt.png" alt="index build prompt" />
			<figcaption>startup index build prompt</figcaption>
		</figure>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="permission-system">Permission System</h2>

		<p>A three-tier permission model controls tool access: <strong>always-allow</strong>, <strong>prompt</strong>, and <strong>block</strong>. Session-level overrides let you grant temporary permissions without changing your config. YOLO mode auto-approves all tool permissions for uninterrupted flow.</p>

		<p>For fine-grained control, configure path-scoped and command-scoped permission presets in <code>.glamdring/permissions.json</code>:</p>

		<div class="code-block">
			<CopyButton text={`{
  "allow": [
    {"tool": "Write", "path": "src/**"},
    {"tool": "Bash", "command": "go test*"}
  ],
  "deny": [
    {"tool": "Bash", "command": "rm -rf*"}
  ]
}`} />
			<pre><code>{`{
  "allow": [
    {"tool": "Write", "path": "src/**"},
    {"tool": "Bash", "command": "go test*"}
  ],
  "deny": [
    {"tool": "Bash", "command": "rm -rf*"}
  ]
}`}</code></pre>
		</div>

		<p>Deny rules are checked first and block outright. Allow rules skip the prompt. Both override the default prompt behavior.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="themes">Themes</h2>

		<p>Six LOTR-inspired color themes are built in:</p>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Theme</th>
						<th>Description</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>glamdring</code></td>
						<td>Cool steel-blue (default)</td>
					</tr>
					<tr>
						<td><code>rivendell</code></td>
						<td>Silver and starlight</td>
					</tr>
					<tr>
						<td><code>mithril</code></td>
						<td>Bright cyan-silver</td>
					</tr>
					<tr>
						<td><code>lothlorien</code></td>
						<td>Golden-amber</td>
					</tr>
					<tr>
						<td><code>shire</code></td>
						<td>Warm russet-earth</td>
					</tr>
					<tr>
						<td><code>anduin</code></td>
						<td>Colorblind-safe warm neutral (deuteranopia/protanopia)</td>
					</tr>
				</tbody>
			</table>
		</div>

		<div class="theme-gallery">
			<figure>
				<img src="{base}/screenshots/theme-glamdring.png" alt="glamdring theme" />
				<figcaption>glamdring (default)</figcaption>
			</figure>
			<figure>
				<img src="{base}/screenshots/theme-rivendell.png" alt="rivendell theme" />
				<figcaption>rivendell</figcaption>
			</figure>
			<figure>
				<img src="{base}/screenshots/theme-mithril.png" alt="mithril theme" />
				<figcaption>mithril</figcaption>
			</figure>
			<figure>
				<img src="{base}/screenshots/theme-lothlorien.png" alt="lothlorien theme" />
				<figcaption>lothlorien</figcaption>
			</figure>
			<figure>
				<img src="{base}/screenshots/theme-shire.png" alt="shire theme" />
				<figcaption>shire</figcaption>
			</figure>
			<figure>
				<img src="{base}/screenshots/theme-anduin.png" alt="anduin theme" />
				<figcaption>anduin (colorblind-safe)</figcaption>
			</figure>
		</div>

		<p>Switch themes at runtime with <code>/theme</code>. Running <code>/theme</code> alone lists all available themes. <code>/theme &lt;name&gt;</code> switches immediately.</p>

		<p>For high contrast displays, set <code>"high_contrast": true</code> in your config to boost text brightness and accent saturation.</p>

		<p>Custom themes can be defined in your config with 13 color slots. User-defined themes take precedence over built-in themes with the same name.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="image-paste">Image Paste</h2>

		<p><code>Ctrl+V</code> pastes clipboard images -- screenshots, copied images, or any image data -- directly into the prompt for Claude's vision API. Multiple images per message are supported.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="conversation-export">Conversation Export</h2>

		<p><code>/export</code> saves the current conversation as a markdown file. Use <code>/export --html</code> for a self-contained HTML document with syntax highlighting.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="input-history">Input History</h2>

		<p>Use <code>Up</code> and <code>Down</code> arrow keys to cycle through previous prompts. <code>Ctrl+R</code> opens reverse search for finding earlier inputs by substring.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="slash-commands">Slash Commands</h2>

		<p>Define custom prompt templates in <code>.glamdring/commands/*.md</code> (or <code>.claude/commands/*.md</code>). Commands are available via tab completion at the prompt.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="custom-agents">Custom Agents</h2>

		<p>Define specialized subagents in <code>.glamdring/agents/*.md</code> or <code>*.yaml</code> (also supports <code>.claude/agents/</code>). Each agent definition specifies its system prompt, available tools, and behavioral constraints.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="subagents">Subagents</h2>

		<p>The Task tool enables parallel task spawning for concurrent work. Multiple subagents can operate simultaneously on independent parts of a problem, reporting results back to the orchestrating agent.</p>
	</section>
</div>

<style>
	.page {
		padding-top: var(--space-xl);
		padding-bottom: var(--space-2xl);
	}

	h1 {
		margin-bottom: var(--space-lg);
	}

	section {
		margin-bottom: var(--space-md);
	}

	h2 {
		margin-bottom: var(--space-lg);
	}

	h3 {
		margin-top: var(--space-lg);
		margin-bottom: var(--space-sm);
		color: var(--color-accent);
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

	.table-wrap {
		overflow-x: auto;
		margin-bottom: var(--space-md);
	}

	table {
		width: 100%;
		border-collapse: collapse;
		border: 1px solid var(--color-border);
	}

	th, td {
		text-align: left;
		padding: var(--space-sm) var(--space-md);
		border: 1px solid var(--color-border);
	}

	th {
		background-color: var(--color-bg-elevated);
		font-family: var(--font-heading);
		font-size: 0.95rem;
		letter-spacing: 0.03em;
		color: var(--color-heading);
	}

	td:first-child {
		white-space: nowrap;
	}

	tr:nth-child(even) {
		background-color: var(--color-bg-elevated);
	}

	tr:hover {
		background-color: var(--color-surface);
	}

	.theme-gallery {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
		gap: var(--space-md);
		margin-bottom: var(--space-lg);
	}

	.theme-gallery figure {
		margin: 0;
	}

	.theme-gallery img {
		width: 100%;
		border-radius: 6px;
		border: 1px solid var(--color-border);
	}

	.theme-gallery figcaption {
		margin-top: var(--space-xs, 0.25rem);
		font-size: 0.85rem;
		color: var(--color-text-muted);
		text-align: center;
	}
</style>
