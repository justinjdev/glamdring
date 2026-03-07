# Glamdring Documentation Site Design

## Overview

A SvelteKit static documentation site deployed to GitHub Pages at `justinjdev.github.io/glamdring`. Based on the fellowship site's structure and aesthetic, adapted with glamdring's color palette and content.

## Tech Stack

- SvelteKit 5 with adapter-static
- Pure CSS with custom properties (no Tailwind)
- Pagefind for static search
- TypeScript
- GitHub Actions for deployment

## Structure

```
site/
├── src/
│   ├── lib/
│   │   ├── components/
│   │   │   ├── Nav.svelte          # Top nav with search, theme toggle, mobile menu
│   │   │   ├── Footer.svelte       # Site footer
│   │   │   ├── ThemeToggle.svelte   # Dark/light mode toggle
│   │   │   ├── CopyButton.svelte   # Code block copy button
│   │   │   ├── Sidebar.svelte      # Section nav from headings (sticky, collapsible)
│   │   │   └── Search.svelte       # Pagefind search UI
│   │   ├── styles/
│   │   │   ├── global.css          # Layout, utilities, animations
│   │   │   └── theme.css           # Color palette CSS custom properties
│   │   └── index.ts
│   └── routes/
│       ├── +layout.svelte          # Root layout (nav, optional sidebar, footer)
│       ├── +layout.js              # prerender: true
│       ├── +page.svelte            # Home: hero, features, install snippet
│       ├── getting-started/
│       │   └── +page.svelte        # Install, API key, first run
│       ├── features/
│       │   └── +page.svelte        # Tools, streaming, permissions, themes, etc.
│       ├── configuration/
│       │   └── +page.svelte        # settings.json, permissions, hooks, commands
│       ├── agent-teams/
│       │   └── +page.svelte        # Workflows, phases, tasks, messaging
│       ├── mcp-servers/
│       │   └── +page.svelte        # External tool servers
│       ├── architecture/
│       │   └── +page.svelte        # Package structure, design patterns
│       └── changelog/
│           └── +page.svelte        # Version history
├── static/
├── svelte.config.js                # adapter-static, base: '/glamdring'
├── package.json
├── vite.config.ts
└── tsconfig.json
```

## Color Palette

### Dark Mode (Default)

| Role | Color | Hex |
|------|-------|-----|
| Background | Dark charcoal | `#1a1a1f` |
| Surface | Elevated dark | `#22222a` |
| Card bg | Slightly raised | `#1e1e26` |
| Text primary | Warm cream | `#e8dcc8` |
| Text secondary | Muted slate | `#b8c4d0` |
| Accent / Headings | Ice blue | `#7daea3` |
| Accent hover | Light ice | `#9bc4b8` |
| Border | Dark slate | `#3a3a4a` |
| Code bg | Deepest dark | `#16161c` |
| Error | Deep red | `#8b2500` |

### Light Mode

| Role | Color | Hex |
|------|-------|-----|
| Background | Warm cream | `#f4eee1` |
| Text primary | Dark charcoal | `#2a2a2f` |
| Accent | Deep teal | `#4a7a70` |
| Border | Light gray | `#d0ccc4` |

## Typography

- **Headings:** Cinzel (serif, classical)
- **Body:** Crimson Pro (serif, warm, readable)
- **Code:** JetBrains Mono (monospace)
- Base size: 1.125rem, line-height: 1.7

## Components

### Nav

Sticky top navigation. Logo/title, main section links, search trigger, theme toggle. Hamburger menu on mobile (<900px).

### Sidebar

Left sidebar for long content pages (features, configuration, agent-teams, architecture). Auto-generated from h2/h3 headings on the page. Sticky on desktop, collapsible on mobile. Not shown on home, getting-started, mcp-servers, or changelog.

### Search

Pagefind integration. Indexed at build time from static output. Search UI styled to match theme (dark input, ice blue highlights). Triggered from nav icon/shortcut.

### ThemeToggle

Dark/light mode. Persisted to localStorage. Respects prefers-color-scheme on first visit.

### CopyButton

Appears on code blocks. Copies content to clipboard with visual feedback.

## Content Sources

All content derived from:
- `README.md` (primary -- features, install, config, keybindings, architecture)
- `docs/plans/*.md` (theme system details, design decisions)
- `openspec/specs/` (detailed specs for agent-teams, task management, etc.)

## Deployment

GitHub Actions workflow at `.github/workflows/deploy-site.yml`:
- Trigger: push to `main` when `site/**` changes
- Steps: checkout, setup Node, npm ci, npm run build, pagefind index, deploy to GitHub Pages
- Published at `justinjdev.github.io/glamdring`

## Approach

Copy the fellowship site (`~/git/fellowship/site`) into `site/`, then modify:
1. Update color palette in theme.css (forest green + gold -> dark charcoal + ice blue)
2. Update base path from `/fellowship` to `/glamdring`
3. Replace all content with glamdring documentation
4. Update Nav links to match glamdring sections
5. Add Sidebar component for section navigation
6. Add Pagefind for search
7. Add GitHub Actions workflow

## Audience

Both end users (install, usage, config) and developers (architecture, package structure, extension points).
