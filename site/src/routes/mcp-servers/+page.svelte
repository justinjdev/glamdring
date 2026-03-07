<script>
	import CopyButton from '$lib/components/CopyButton.svelte';
</script>

<svelte:head>
	<title>MCP Servers - glamdring</title>
</svelte:head>

<div class="container page">
	<h1>MCP Servers</h1>

	<p>
		Connect external tool servers via stdio transport with health monitoring.
	</p>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="configuration">Configuration</h2>

		<div class="code-block">
			<CopyButton text={`{
  "mcp_servers": {
    "myserver": {
      "command": "node",
      "args": ["server.js"],
      "env": { "API_KEY": "secret123" },
      "tools": { "enabled": ["read", "write"] }
    }
  }
}`} />
			<pre><code>{`{
  "mcp_servers": {
    "myserver": {
      "command": "node",
      "args": ["server.js"],
      "env": { "API_KEY": "secret123" },
      "tools": { "enabled": ["read", "write"] }
    }
  }
}`}</code></pre>
		</div>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Field</th>
						<th>Description</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>command</code></td>
						<td>Server binary to launch</td>
					</tr>
					<tr>
						<td><code>args</code></td>
						<td>Command-line arguments</td>
					</tr>
					<tr>
						<td><code>env</code></td>
						<td>Environment variables passed to the server process</td>
					</tr>
					<tr>
						<td><code>tools.enabled</code></td>
						<td>Allowlist: only register these tools (takes precedence)</td>
					</tr>
					<tr>
						<td><code>tools.disabled</code></td>
						<td>Denylist: register all tools except these</td>
					</tr>
				</tbody>
			</table>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="runtime-management">Runtime Management</h2>

		<p>Use <code>/mcp</code> commands to manage servers during a session:</p>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Command</th>
						<th>Description</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>/mcp</code></td>
						<td>List all servers with status and tool count</td>
					</tr>
					<tr>
						<td><code>/mcp restart &lt;name&gt;</code></td>
						<td>Restart a server</td>
					</tr>
					<tr>
						<td><code>/mcp disconnect &lt;name&gt;</code></td>
						<td>Stop and remove a server</td>
					</tr>
					<tr>
						<td><code>/mcp tools &lt;name&gt;</code></td>
						<td>List tools on a server with enabled/disabled status</td>
					</tr>
					<tr>
						<td><code>/mcp enable &lt;server&gt; &lt;tool&gt;</code></td>
						<td>Re-enable a disabled tool (session-only)</td>
					</tr>
					<tr>
						<td><code>/mcp disable &lt;server&gt; &lt;tool&gt;</code></td>
						<td>Disable a tool (session-only)</td>
					</tr>
				</tbody>
			</table>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="status-bar">Status Bar</h2>

		<p>
			The status bar shows <code>mcp: N</code> when servers are connected, or
			<code>mcp: N/M</code> if some have died. Server deaths are surfaced inline
			in output.
		</p>
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
</style>
