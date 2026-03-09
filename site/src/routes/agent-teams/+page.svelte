<script>
	import { base } from '$app/paths';
	import CopyButton from '$lib/components/CopyButton.svelte';
</script>

<svelte:head>
	<title>Agent Teams - glamdring</title>
</svelte:head>

<div class="container page">
	<h1>Using Agent Teams</h1>

	<p class="lead">
		Agent teams let glamdring coordinate multiple AI agents working in parallel — each with a
		defined role, restricted tools, and a clear handoff point. This page explains the concept from
		scratch and walks you through your first team task.
	</p>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="the-problem">The Problem with One Agent</h2>

		<p>
			Give glamdring a large, open-ended task — implement a feature, refactor a module, audit a
			codebase — and it will work through it sequentially: read, think, write, repeat. That works
			well for focused tasks. For bigger ones, it breaks down.
		</p>

		<p>
			A single agent working alone can lose the thread. It might write code before fully
			understanding the codebase, make a decision in step 3 that conflicts with what it discovers in
			step 10, or simply grind through things serially when parallel would be faster. This isn't a
			skill problem — it's structural. One agent trying to plan, implement, and verify at the same
			time is doing too many things at once.
		</p>

		<p>
			Agent teams solve this by giving each phase of work its own agent, its own tools, and its own
			scope. A research agent can only read. An implementation agent can write. A verification agent
			checks the work. Each one stays focused.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="how-it-works">How Agent Teams Work</h2>

		<p>
			When you ask glamdring to use agent teams, it spawns a set of subagents and coordinates them
			through a <strong>workflow</strong> — a sequence of phases, each with its own job.
		</p>

		<div class="concept-list">
			<div class="concept">
				<h3>Phases</h3>
				<p>
					A workflow is divided into phases: research, plan, implement, verify — or whatever
					sequence makes sense for the task. Each phase runs one at a time, in order. An agent
					cannot skip ahead.
				</p>
			</div>

			<div class="concept">
				<h3>Tool access per phase</h3>
				<p>
					Each phase restricts what tools its agent can use. A research phase might only allow
					reading files. An implementation phase unlocks writing and editing. This keeps agents
					honest — a researcher can't accidentally modify code, and an implementer can't wander
					into tasks that belong to a later phase.
				</p>
			</div>

			<div class="concept">
				<h3>Phase gates</h3>
				<p>
					Before moving from one phase to the next, an agent must pass a gate. Most gates are
					automatic. But the built-in workflows use a <strong>leader gate</strong> on the planning
					phase — meaning glamdring pauses and asks <em>you</em> to approve the plan before any
					code gets written. You stay in control of the important decisions.
				</p>
			</div>

			<div class="concept">
				<h3>Tasks and coordination</h3>
				<p>
					Inside a team, agents communicate via a task board and message passing. They can claim
					tasks, mark them complete, and send each other messages. You don't need to manage this
					directly — it happens automatically — but you can observe it as tool calls appear in the
					output.
				</p>
			</div>
		</div>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="first-task">Your First Team Task</h2>

		<p>
			You don't configure agent teams manually. You just ask glamdring to use them. Here's an
			example prompt for implementing a new feature using the <code>rpiv</code> workflow (Research →
			Plan → Implement → Verify):
		</p>

		<div class="code-block">
			<CopyButton
				text="Use agent teams with the rpiv workflow to add dark mode support to the settings page."
			/>
			<pre><code>Use agent teams with the rpiv workflow to add dark mode support to the settings page.</code></pre>
		</div>

		<p>Here's what happens next, step by step:</p>

		<ol class="steps">
			<li>
				<strong>Research phase.</strong> A research agent reads the codebase — the settings page,
				existing styles, theme infrastructure. It cannot write anything. It builds a picture of what
				exists and what needs to change.
			</li>
			<li>
				<strong>Plan phase.</strong> A planning agent takes the research findings and produces a
				written implementation plan: what files to change, what approach to take, what edge cases to
				handle. When the plan is ready, <strong>glamdring pauses and shows it to you.</strong> You
				approve it (or ask for changes) before any code is written.
			</li>
			<li>
				<strong>Implement phase.</strong> After your approval, an implementation agent executes the
				plan — creating and editing files, running commands, staying within the scope defined by the
				plan.
			</li>
			<li>
				<strong>Verify phase.</strong> A verification agent reviews the implementation — checking that
				the feature works, that tests pass, that nothing was missed. It can read and run commands, but
				not edit.
			</li>
		</ol>

		<p>
			Throughout all of this, you'll see tool calls scrolling by in the TUI as the agents work. You
			can watch, ask questions, or just wait. The only moment you need to act is the plan approval
			gate.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="workflows">Choosing a Workflow</h2>

		<p>
			glamdring ships with four built-in workflows. Pick based on the size and risk of the task:
		</p>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Workflow</th>
						<th>Phases</th>
						<th>Use when</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><code>rpiv</code></td>
						<td>Research → Plan → Implement → Verify</td>
						<td>Non-trivial features, anything with real risk. Start here.</td>
					</tr>
					<tr>
						<td><code>plan-implement</code></td>
						<td>Plan → Implement</td>
						<td>Smaller tasks where research isn't needed but you still want a plan gate.</td>
					</tr>
					<tr>
						<td><code>scoped</code></td>
						<td>Work (single phase)</td>
						<td>Parallel independent tasks with file-scope enforcement, no phase gates.</td>
					</tr>
					<tr>
						<td><code>none</code></td>
						<td>(no phases)</td>
						<td>Full agent autonomy, no restrictions. Use only if you know what you're doing.</td>
					</tr>
				</tbody>
			</table>
		</div>

		<p>
			If you're not sure which to use, say <code>rpiv</code>. The plan approval gate means you'll
			catch problems before they turn into code.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="observability">Watching What's Happening</h2>

		<p>
			Agent teams can feel like a black box when you first use them. They're not. At any point, you
			can ask glamdring what the agents are doing:
		</p>

		<div class="code-block">
			<CopyButton text="What are the agents doing right now?" />
			<pre><code>What are the agents doing right now?</code></pre>
		</div>

		<p>
			glamdring will call the <code>TeamStatus</code> tool and show you a snapshot: which agents are
			active, what phase each one is in, what tasks are pending or in progress, and which files are
			locked. You can also ask it to summarize what's been done so far, or to explain why it's
			waiting.
		</p>

		<p>
			The TUI output itself is also informative. Tool calls from subagents appear inline — you can
			see when an agent reads a file, runs a test, or sends a message to another agent. Nothing
			happens silently.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="enabling">Enabling Agent Teams</h2>

		<p>Agent teams are experimental and must be explicitly enabled. Via flag:</p>

		<div class="code-block">
			<CopyButton text="glamdring --experimental-teams" />
			<pre><code>glamdring --experimental-teams</code></pre>
		</div>

		<p>Or permanently via your settings file:</p>

		<div class="code-block">
			<CopyButton
				text={`{
  "experimental": {
    "teams": true
  }
}`}
			/>
			<pre><code>{`{
  "experimental": {
    "teams": true
  }
}`}</code></pre>
		</div>

		<p>
			Once enabled, the agent has access to team coordination tools automatically. You don't need to
			configure anything else to get started.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="advanced">Going Further</h2>

		<p>
			Once you're comfortable with the built-in workflows, you can define custom ones in your
			settings — controlling exactly which tools each phase allows, which model each phase uses, and
			what kind of gate guards each transition. You can also configure file-level scoping to prevent
			agents from touching files outside their designated area.
		</p>

		<p>
			These are covered in the technical reference below. But for most tasks, <code>rpiv</code> is
			all you need.
		</p>

		<div class="code-block">
			<CopyButton
				text={`{
  "workflows": {
    "my-workflow": {
      "phases": [
        {
          "name": "research",
          "tools": ["Read", "Glob", "Grep"],
          "model": "claude-haiku-4-5-20251001"
        },
        {
          "name": "implement",
          "tools": ["Read", "Write", "Edit", "Bash", "Glob", "Grep"],
          "gate": "leader"
        }
      ]
    }
  }
}`}
			/>
			<pre><code>{`{
  "workflows": {
    "my-workflow": {
      "phases": [
        {
          "name": "research",
          "tools": ["Read", "Glob", "Grep"],
          "model": "claude-haiku-4-5-20251001"
        },
        {
          "name": "implement",
          "tools": ["Read", "Write", "Edit", "Bash", "Glob", "Grep"],
          "gate": "leader"
        }
      ]
    }
  }
}`}</code></pre>
		</div>

		<h3>Technical Reference</h3>

		<p>For the complete reference — phase gates, task dependencies, file locking, inter-agent messaging, context compaction, and team observability — see the sections below.</p>

		<div class="ref-grid">
			<a href="#phase-gates" class="ref-card">
				<span class="ref-title">Phase Gates</span>
				<span class="ref-desc">auto, leader, condition gate types and configuration</span>
			</a>
			<a href="#task-management" class="ref-card">
				<span class="ref-title">Task Management</span>
				<span class="ref-desc">status transitions and dependency resolution</span>
			</a>
			<a href="#file-locking" class="ref-card">
				<span class="ref-title">File Locking</span>
				<span class="ref-desc">per-task locks with automatic release</span>
			</a>
			<a href="#messaging" class="ref-card">
				<span class="ref-title">Messaging</span>
				<span class="ref-desc">inter-agent communication and ordering</span>
			</a>
		</div>
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
						<td>Advances immediately when the phase completes</td>
					</tr>
					<tr>
						<td><code>leader</code></td>
						<td>Pauses and sends an approval request to the team leader — by default, you. Work stops until approved or rejected.</td>
					</tr>
					<tr>
						<td><code>condition</code></td>
						<td>Runs a shell command. Advances only if the command exits with code 0.</td>
					</tr>
				</tbody>
			</table>
		</div>

		<p>
			The <code>rpiv</code> and <code>plan-implement</code> workflows set
			<code>gate: "leader"</code> on their plan phase. Leader resolution: phase-level
			<code>gate_config.leader</code>, then team-level <code>leader</code> field, then
			alphabetically first member.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="task-management">Task Management</h2>

		<h3>Status Transitions</h3>

		<ul class="transition-list">
			<li><code>pending</code> &rarr; <code>in_progress</code> or <code>deleted</code></li>
			<li><code>in_progress</code> &rarr; <code>pending</code>, <code>completed</code>, or <code>deleted</code></li>
			<li><code>completed</code> &rarr; <code>deleted</code></li>
			<li><code>deleted</code> is terminal</li>
		</ul>

		<h3>Dependencies</h3>

		<p>
			Agents cannot claim a task with unresolved <code>blocked_by</code> dependencies. Completing a
			task automatically unblocks any tasks that depended on it.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="file-locking">File Locking</h2>

		<p>
			File locks are scoped to the active task. When a task is marked completed via
			<code>TaskUpdate</code>, all locks acquired during that task are automatically released.
			Prevents agents from stepping on each other's work.
		</p>
	</section>

	<div class="divider"><div class="divider-ring"></div></div>

	<section>
		<h2 id="messaging">Inter-Agent Messaging</h2>

		<p>
			Agents communicate via the <code>SendMessage</code> tool. Messages carry monotonic sequence
			numbers and timestamps for reliable ordering. Shutdown and approval messages are prioritized
			over regular messages.
		</p>
	</section>
</div>

<style>
	.page {
		padding-top: var(--space-xl);
		padding-bottom: var(--space-2xl);
	}

	h1 {
		margin-bottom: var(--space-md);
	}

	.lead {
		font-size: 1.1rem;
		max-width: 48rem;
		color: var(--color-text-muted);
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

	.concept-list {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
		gap: var(--space-md);
		margin: var(--space-lg) 0;
	}

	.concept {
		padding: var(--space-md);
		background: var(--color-bg-elevated);
		border: 1px solid var(--color-border);
		border-radius: 6px;
	}

	.concept h3 {
		margin-top: 0;
		margin-bottom: var(--space-xs);
		font-size: 0.95rem;
		letter-spacing: 0.03em;
	}

	.concept p {
		margin: 0;
		font-size: 0.9rem;
		color: var(--color-text-muted);
	}

	.steps {
		padding-left: var(--space-lg);
		max-width: 42rem;
		margin-bottom: var(--space-lg);
	}

	.steps li {
		margin-bottom: var(--space-md);
		line-height: 1.6;
	}

	.steps li strong {
		color: var(--color-heading);
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

	.ref-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: var(--space-sm);
		margin: var(--space-lg) 0;
	}

	.ref-card {
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
		padding: var(--space-md);
		background: var(--color-bg-elevated);
		border: 1px solid var(--color-border);
		border-radius: 6px;
		text-decoration: none;
		transition: border-color 0.15s, background 0.15s;
	}

	.ref-card:hover {
		border-color: var(--color-accent);
		background: var(--color-surface);
	}

	.ref-title {
		font-family: var(--font-heading);
		font-size: 0.9rem;
		color: var(--color-heading);
		letter-spacing: 0.02em;
	}

	.ref-desc {
		font-size: 0.82rem;
		color: var(--color-text-muted);
		line-height: 1.4;
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
