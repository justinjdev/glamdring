<script lang="ts">
	import CopyButton from '$lib/components/CopyButton.svelte';
</script>

<svelte:head>
	<title>Getting Started - glamdring</title>
</svelte:head>

<div class="container page">
	<h1>Getting Started</h1>

	<section>
		<h2 id="install">Install</h2>

		<p>Quick install via curl:</p>
		<div class="code-block">
			<CopyButton text="curl -fsSL https://raw.githubusercontent.com/justinjdev/glamdring/main/install.sh | sh" />
			<pre><code>curl -fsSL https://raw.githubusercontent.com/justinjdev/glamdring/main/install.sh | sh</code></pre>
		</div>

		<p>Pin a specific version:</p>
		<div class="code-block">
			<CopyButton text="GLAMDRING_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/justinjdev/glamdring/main/install.sh | sh" />
			<pre><code>GLAMDRING_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/justinjdev/glamdring/main/install.sh | sh</code></pre>
		</div>

		<p>Override install directory (default: <code>~/.local/bin</code>):</p>
		<div class="code-block">
			<CopyButton text="GLAMDRING_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/justinjdev/glamdring/main/install.sh | sudo sh" />
			<pre><code>GLAMDRING_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/justinjdev/glamdring/main/install.sh | sudo sh</code></pre>
		</div>

		<p>Build from source:</p>
		<div class="code-block">
			<CopyButton text={"git clone https://github.com/justinjdev/glamdring.git\ncd glamdring\ngo build -ldflags \"-X main.version=$(git describe --tags)\" -o glamdring ./cmd/glamdring"} />
			<pre><code>git clone https://github.com/justinjdev/glamdring.git
cd glamdring
go build -ldflags "-X main.version=$(git describe --tags)" -o glamdring ./cmd/glamdring</code></pre>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="setup">Setup</h2>

		<p>Set your Anthropic API key and launch glamdring:</p>
		<div class="code-block">
			<CopyButton text={"export ANTHROPIC_API_KEY=sk-ant-...\nglamdring"} />
			<pre><code>export ANTHROPIC_API_KEY=sk-ant-...
glamdring</code></pre>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="subcommands">Subcommands</h2>

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
						<td><code>glamdring login</code></td>
						<td>Authenticate with Claude account</td>
					</tr>
					<tr>
						<td><code>glamdring logout</code></td>
						<td>Remove credentials</td>
					</tr>
					<tr>
						<td><code>glamdring version</code></td>
						<td>Print version</td>
					</tr>
				</tbody>
			</table>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="flags">Flags</h2>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Flag</th>
						<th>Description</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>--cwd &lt;path&gt;</code></td>
						<td>Set working directory (defaults to current)</td>
					</tr>
					<tr>
						<td><code>--model &lt;id&gt;</code></td>
						<td>Override model (default: <code>claude-opus-4-6</code>)</td>
					</tr>
					<tr>
						<td><code>--yolo</code></td>
						<td>Auto-approve all tool permissions (no prompts)</td>
					</tr>
					<tr>
						<td><code>--experimental-teams</code></td>
						<td>Enable agent teams support</td>
					</tr>
					<tr>
						<td><code>--version</code></td>
						<td>Print version and exit</td>
					</tr>
				</tbody>
			</table>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="keybindings">Keybindings</h2>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Key</th>
						<th>Action</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>Enter</code></td>
						<td>Submit prompt</td>
					</tr>
					<tr>
						<td><code>Alt+Enter</code></td>
						<td>Insert newline</td>
					</tr>
					<tr>
						<td><code>Up</code> / <code>Down</code></td>
						<td>Cycle input history (on first/last line)</td>
					</tr>
					<tr>
						<td><code>Ctrl+r</code></td>
						<td>Reverse search input history</td>
					</tr>
					<tr>
						<td><code>j</code> / <code>k</code></td>
						<td>Scroll line up/down</td>
					</tr>
					<tr>
						<td><code>Ctrl+u</code> / <code>Ctrl+d</code></td>
						<td>Scroll half page</td>
					</tr>
					<tr>
						<td><code>G</code> / <code>g</code></td>
						<td>Jump to bottom/top</td>
					</tr>
					<tr>
						<td><code>e</code></td>
						<td>Expand/collapse last tool result (while agent is running)</td>
					</tr>
					<tr>
						<td><code>y</code> / <code>n</code> / <code>a</code></td>
						<td>Permission: yes / no / always</td>
					</tr>
					<tr>
						<td><code>y</code> / <code>n</code></td>
						<td>Checkpoint prompt: load / skip</td>
					</tr>
					<tr>
						<td><code>Ctrl+v</code></td>
						<td>Paste image from clipboard (or text if no image)</td>
					</tr>
					<tr>
						<td><code>Tab</code></td>
						<td>Complete slash command</td>
					</tr>
					<tr>
						<td><code>Shift+Tab</code></td>
						<td>Toggle YOLO mode (auto-approve all tools)</td>
					</tr>
					<tr>
						<td><code>Ctrl+c</code></td>
						<td>Interrupt agent turn (double-press to quit)</td>
					</tr>
					<tr>
						<td><code>Esc</code></td>
						<td>Deny permission request</td>
					</tr>
				</tbody>
			</table>
		</div>
	</section>
</div>

<style>
	.page {
		padding-top: var(--space-xl);
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
