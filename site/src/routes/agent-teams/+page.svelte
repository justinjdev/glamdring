<script>
	import CopyButton from '$lib/components/CopyButton.svelte';
</script>

<svelte:head>
	<title>Agent Teams - glamdring</title>
</svelte:head>

<div class="page">
	<h1>Agent Teams</h1>

	<p>
		Experimental feature for coordinated multi-agent workflows. Enable with the
		<code>--experimental-teams</code> flag or via settings.
	</p>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="enabling-teams">Enabling Teams</h2>

		<p>Via flag:</p>
		<div class="code-block">
			<CopyButton text="glamdring --experimental-teams" />
			<pre><code>glamdring --experimental-teams</code></pre>
		</div>

		<p>Via settings:</p>
		<div class="code-block">
			<CopyButton text={`{\n  "experimental": {\n    "teams": true\n  }\n}`} />
			<pre><code>{`{
  "experimental": {
    "teams": true
  }
}`}</code></pre>
		</div>

		<p>
			When enabled, the agent gets team coordination tools:
			<code>TeamCreate</code>, <code>TeamDelete</code>, <code>TaskCreate</code>,
			<code>TaskList</code>, <code>TaskGet</code>, <code>TaskUpdate</code>,
			<code>SendMessage</code>, <code>AdvancePhase</code>, <code>TeamStatus</code>.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="built-in-workflows">Built-in Workflows</h2>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Workflow</th>
						<th>Phases</th>
						<th>Description</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>rpiv</code></td>
						<td>research, plan, implement, verify</td>
						<td>Full research-to-verification cycle</td>
					</tr>
					<tr>
						<td><code>plan-implement</code></td>
						<td>plan, implement</td>
						<td>Simpler two-phase workflow</td>
					</tr>
					<tr>
						<td><code>scoped</code></td>
						<td>work</td>
						<td>Single phase with file scope enforcement</td>
					</tr>
					<tr>
						<td><code>none</code></td>
						<td>(no phases)</td>
						<td>No workflow enforcement</td>
					</tr>
				</tbody>
			</table>
		</div>

		<p>Default workflow when none specified: <code>scoped</code>.</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="custom-workflows">Custom Workflows</h2>

		<div class="code-block">
			<CopyButton text={`{
  "workflows": {
    "my-workflow": {
      "phases": [
        {"name": "research", "tools": ["Read", "Glob", "Grep"], "model": "claude-sonnet-4-6"},
        {"name": "implement", "tools": ["Read", "Write", "Edit", "Bash", "Glob", "Grep"]}
      ]
    }
  }
}`} />
			<pre><code>{`{
  "workflows": {
    "my-workflow": {
      "phases": [
        {"name": "research", "tools": ["Read", "Glob", "Grep"], "model": "claude-sonnet-4-6"},
        {"name": "implement", "tools": ["Read", "Write", "Edit", "Bash", "Glob", "Grep"]}
      ]
    }
  }
}`}</code></pre>
		</div>

		<p>
			Each phase controls available tools and optionally overrides model.
			Custom workflows take precedence over built-in presets.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="phase-gates">Phase Gates</h2>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Gate</th>
						<th>Behavior</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>auto</code> (default)</td>
						<td>Advances immediately when requested</td>
					</tr>
					<tr>
						<td><code>leader</code></td>
						<td>Sends approval request to team leader; blocks until approved or rejected</td>
					</tr>
					<tr>
						<td><code>condition</code></td>
						<td>Runs a shell command; advances only if exit code is 0</td>
					</tr>
				</tbody>
			</table>
		</div>

		<p>
			The <code>rpiv</code> and <code>plan-implement</code> workflows set
			<code>gate: "leader"</code> on their plan phase by default.
		</p>

		<div class="code-block">
			<CopyButton text={`{
  "workflows": {
    "gated": {
      "phases": [
        {"name": "plan", "tools": ["Read", "Glob", "Grep"], "gate": "leader"},
        {"name": "test", "tools": ["Read", "Bash"], "gate": "condition", "gate_config": {"command": "make test"}},
        {"name": "implement", "tools": ["Read", "Write", "Edit", "Bash"]}
      ]
    }
  }
}`} />
			<pre><code>{`{
  "workflows": {
    "gated": {
      "phases": [
        {"name": "plan", "tools": ["Read", "Glob", "Grep"], "gate": "leader"},
        {"name": "test", "tools": ["Read", "Bash"], "gate": "condition", "gate_config": {"command": "make test"}},
        {"name": "implement", "tools": ["Read", "Write", "Edit", "Bash"]}
      ]
    }
  }
}`}</code></pre>
		</div>

		<p>
			Leader resolution priority: phase-level <code>gate_config.leader</code>,
			team-level <code>leader</code> field, then alphabetically first team member.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="file-locking">File Locking</h2>

		<p>
			File locks are scoped to the active task. When a task is completed via
			<code>TaskUpdate</code>, all locks acquired for that task are automatically
			released. Prevents lock leaks across task boundaries.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="task-management">Task Management</h2>

		<h3>Status Transitions</h3>

		<p>Validated against allowed transitions:</p>

		<ul class="transition-list">
			<li><code>pending</code> &rarr; <code>in_progress</code> or <code>deleted</code></li>
			<li><code>in_progress</code> &rarr; <code>pending</code>, <code>completed</code>, or <code>deleted</code></li>
			<li><code>completed</code> &rarr; <code>deleted</code></li>
			<li><code>deleted</code> is terminal</li>
		</ul>

		<p>Invalid transitions return an error.</p>

		<h3>Dependencies</h3>

		<p>
			Agents cannot claim a task that has unresolved <code>BlockedBy</code> dependencies.
			Clear blockers first, then claim.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="inter-agent-messaging">Inter-agent Messaging</h2>

		<p>
			All messages carry a monotonic sequence number and timestamp for reliable
			ordering. Use the <code>SendMessage</code> tool.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="force-shutdown">Force Shutdown</h2>

		<p>
			Send a shutdown request with <code>force: true</code> to terminate an agent
			immediately via context cancellation.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="team-observability">Team Observability</h2>

		<p>
			<code>TeamStatus</code> tool returns structured JSON with: member statuses,
			lock state, task summary (counts by status), and per-agent phase information.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="context-compaction">Context Compaction</h2>

		<p>
			Phase changes trigger compaction of conversation history when
			<code>PhaseTransitionCallback</code> is configured.
			<code>ArchivingCompactor</code> stores raw conversation history in context
			cache before compaction.
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

	h3 {
		margin-bottom: var(--space-sm);
		margin-top: var(--space-lg);
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

	.transition-list {
		margin-bottom: var(--space-md);
		padding-left: var(--space-lg);
		max-width: 42rem;
	}

	.transition-list li {
		margin-bottom: var(--space-xs);
		line-height: 1.6;
	}
</style>
