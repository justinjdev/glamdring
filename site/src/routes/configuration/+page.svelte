<script lang="ts">
	import CopyButton from '$lib/components/CopyButton.svelte';
</script>

<svelte:head>
	<title>Configuration - glamdring</title>
</svelte:head>

<div class="page">
	<h1>Configuration</h1>

	<p>
		Glamdring uses <code>.glamdring/</code> as its primary config directory, with
		<code>.claude/</code> as a fallback for backward compatibility. When both exist,
		<code>.glamdring/</code> takes priority.
	</p>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="config-directory">Config Directory</h2>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Purpose</th>
						<th>Primary</th>
						<th>Fallback</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td>Instructions</td>
						<td><code>GLAMDRING.md</code>, <code>.glamdring/GLAMDRING.md</code>, <code>.glamdring/GLAMDRING.local.md</code></td>
						<td><code>CLAUDE.md</code>, <code>.claude/CLAUDE.md</code>, <code>.claude/CLAUDE.local.md</code></td>
					</tr>
					<tr>
						<td>Settings</td>
						<td><code>.glamdring/config.json</code></td>
						<td><code>.claude/settings.json</code></td>
					</tr>
					<tr>
						<td>Permissions</td>
						<td><code>.glamdring/permissions.json</code></td>
						<td><code>.claude/permissions.json</code></td>
					</tr>
					<tr>
						<td>Commands</td>
						<td><code>.glamdring/commands/*.md</code></td>
						<td><code>.claude/commands/*.md</code></td>
					</tr>
					<tr>
						<td>Agents</td>
						<td><code>.glamdring/agents/*.md</code> or <code>*.yaml</code></td>
						<td><code>.claude/agents/*.md</code> or <code>*.yaml</code></td>
					</tr>
					<tr>
						<td>Hooks</td>
						<td><code>hooks</code> array in <code>.glamdring/config.json</code></td>
						<td><code>hooks</code> array in <code>.claude/settings.json</code></td>
					</tr>
					<tr>
						<td>User config</td>
						<td><code>~/.config/glamdring/</code></td>
						<td><code>~/.claude/</code></td>
					</tr>
				</tbody>
			</table>
		</div>

		<p class="note">
			Instructions files are additive -- both loaded if present. All other config types use the first file found.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="instructions">Instructions</h2>

		<p>
			Glamdring discovers and loads instruction files at every directory level from the
			project root up to the user home directory. Primary files are
			<code>GLAMDRING.md</code> and <code>.glamdring/GLAMDRING.md</code>; fallback
			files are <code>CLAUDE.md</code> and <code>.claude/CLAUDE.md</code>.
		</p>
		<p>
			Use <code>.local.md</code> variants (e.g. <code>.glamdring/GLAMDRING.local.md</code>)
			for user-specific instructions that should not be committed to version control.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="settings">Settings</h2>

		<p>
			Project settings live in <code>.glamdring/config.json</code>, with fallback to
			<code>.claude/settings.json</code>. User-level settings go in
			<code>~/.config/glamdring/config.json</code> (fallback: <code>~/.claude/settings.json</code>).
		</p>

		<div class="code-block">
			<CopyButton text={`{
  "model": "claude-opus-4-6",
  "max_turns": 100,
  "theme": "glamdring",
  "high_contrast": false,
  "themes": {},
  "mcp_servers": {},
  "indexer": {},
  "experimental": { "teams": true },
  "workflows": {}
}`} />
			<pre><code>{`{
  "model": "claude-opus-4-6",
  "max_turns": 100,
  "theme": "glamdring",
  "high_contrast": false,
  "themes": {},
  "mcp_servers": {},
  "indexer": {},
  "experimental": { "teams": true },
  "workflows": {}
}`}</code></pre>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="permission-presets">Permission Presets</h2>

		<p>
			Configure tool permissions in <code>.glamdring/permissions.json</code>.
			Deny rules are checked first. Allow rules skip the approval prompt.
			Path rules use glob matching (<code>**</code> recursive, <code>*</code> prefix).
			Command rules match against the bash command string.
		</p>

		<div class="code-block">
			<CopyButton text={`{
  "allow": [
    {"tool": "Write", "path": "src/**"},
    {"tool": "Bash", "command": "go test*"},
    {"tool": "Bash", "command": "go build*"}
  ],
  "deny": [
    {"tool": "Bash", "command": "rm -rf*"},
    {"tool": "Write", "path": "/etc/**"}
  ]
}`} />
			<pre><code>{`{
  "allow": [
    {"tool": "Write", "path": "src/**"},
    {"tool": "Bash", "command": "go test*"},
    {"tool": "Bash", "command": "go build*"}
  ],
  "deny": [
    {"tool": "Bash", "command": "rm -rf*"},
    {"tool": "Write", "path": "/etc/**"}
  ]
}`}</code></pre>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="hooks">Hooks</h2>

		<p>
			Shell commands triggered by lifecycle events. Events include
			<code>SessionStart</code> (on launch), <code>SessionEnd</code> (on exit),
			and <code>ContextThreshold</code> (when context usage crosses a threshold).
		</p>

		<div class="code-block">
			<CopyButton text={`{
  "hooks": [
    {
      "event": "SessionStart",
      "command": "echo Starting session"
    }
  ]
}`} />
			<pre><code>{`{
  "hooks": [
    {
      "event": "SessionStart",
      "command": "echo Starting session"
    }
  ]
}`}</code></pre>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="indexer">Indexer Configuration</h2>

		<p>
			The shire code indexer is auto-detected by default. Configure it explicitly in your settings file:
		</p>

		<div class="code-block">
			<CopyButton text={`{
  "indexer": {
    "enabled": true,
    "command": "shire",
    "auto_rebuild": true
  }
}`} />
			<pre><code>{`{
  "indexer": {
    "enabled": true,
    "command": "shire",
    "auto_rebuild": true
  }
}`}</code></pre>
		</div>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Field</th>
						<th>Default</th>
						<th>Description</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>enabled</code></td>
						<td>auto-detect</td>
						<td><code>true</code> = force on, <code>false</code> = disable, omit = auto-detect <code>.shire/index.db</code></td>
					</tr>
					<tr>
						<td><code>command</code></td>
						<td><code>"shire"</code></td>
						<td>Binary name for the indexer</td>
					</tr>
					<tr>
						<td><code>auto_rebuild</code></td>
						<td><code>true</code></td>
						<td>Rebuild index after agent turns that modify files</td>
					</tr>
				</tbody>
			</table>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="slash-commands">Slash Commands</h2>

		<p>
			Define custom prompts as Markdown files in <code>.glamdring/commands/*.md</code>
			(fallback: <code>.claude/commands/*.md</code>). Each file becomes a slash command
			with tab completion in the TUI.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="custom-agents">Custom Agents</h2>

		<p>
			Define specialized subagents in <code>.glamdring/agents/*.md</code> or
			<code>*.yaml</code> (fallback: <code>.claude/agents/</code>). Agent definitions
			configure model, tools, instructions, and permissions for focused subtasks.
		</p>
	</section>
</div>

<style>
	.page {
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

	p {
		margin-bottom: var(--space-sm);
		max-width: 42rem;
	}

	p.note {
		font-size: 0.9rem;
		color: var(--color-text-secondary);
		margin-top: var(--space-md);
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
</style>
