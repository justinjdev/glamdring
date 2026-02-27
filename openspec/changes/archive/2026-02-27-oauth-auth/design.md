## Context

Glamdring currently hardcodes `x-api-key` header auth and requires `ANTHROPIC_API_KEY` to be set. This means users need a separate paid API account even if they have a Claude Pro/Max subscription.

Claude Code authenticates via OAuth 2.0 Authorization Code with PKCE. It stores tokens in `~/.claude.json` and the macOS Keychain (`Claude Code-credentials`). The access token (`sk-ant-oat01-...`) is sent as `Authorization: Bearer` with an `anthropic-beta: oauth-2025-04-20` header. Tokens expire after 8 hours and are refreshed automatically using a stored refresh token (`sk-ant-ort01-...`).

Glamdring should support the same flow, sharing credentials with Claude Code so users don't need to authenticate twice.

## Goals / Non-Goals

**Goals:**
- Support OAuth 2.0 PKCE login flow (browser-based, same as Claude Code)
- Read and share credentials with Claude Code (`~/.claude.json` + Keychain)
- Automatic token refresh when access token expires
- `glamdring login` and `glamdring logout` subcommands
- Continue supporting `ANTHROPIC_API_KEY` as an override
- API client supports both auth modes transparently

**Non-Goals:**
- Bedrock/Vertex/Foundry auth providers
- Implementing a local HTTP callback server (Claude Code uses a redirect-based flow, not localhost callback)
- Encrypting tokens beyond what the OS keychain provides
- Supporting non-macOS keychain backends (Linux keyring, Windows credential manager) — macOS only for now

## Decisions

### 1. Credential resolution order

**Decision**: Resolve credentials in this order:
1. `ANTHROPIC_API_KEY` env var → API key mode
2. `~/.claude.json` OAuth tokens → OAuth mode (refresh if expired)
3. macOS Keychain `Claude Code-credentials` → OAuth mode (fallback)
4. No credentials found → print message suggesting `glamdring login`

**Rationale**: Env var takes precedence for CI/scripting. Shared file is the common case for interactive use. Keychain is a fallback if the JSON file is missing/corrupt.

### 2. Token storage format

**Decision**: Use the same `~/.claude.json` format as Claude Code. Store under `claudeAiOauth` key with fields: `accessToken`, `refreshToken`, `expiresAt`, `scopes`.

**Rationale**: Sharing the exact format means users who've already run `claude` can use glamdring immediately with zero setup. Glamdring login also stores tokens Claude Code can read.

### 3. OAuth parameters

**Decision**: Use the same OAuth client and endpoints as Claude Code:
- Client ID: `9d1c250a-e61b-44d9-88ed-5944d1962f5e`
- Auth URL: `https://claude.ai/oauth/authorize`
- Token URL: `https://platform.claude.com/v1/oauth/token`
- Scopes: `user:profile user:inference user:sessions:claude_code user:mcp_servers`

**Rationale**: Using the same client ID ensures tokens are interchangeable. The scopes match what Claude Code requests for Claude.ai accounts.

### 4. PKCE flow without local server

**Decision**: Use the same redirect-based flow as Claude Code. Open browser to auth URL with PKCE challenge. The browser redirects to `https://platform.claude.com/oauth/code/callback` which displays the authorization code. The user copies the code back to the terminal (or we parse it from the redirect URL if possible).

**Alternative considered**: Local HTTP server on localhost to catch the callback. Rejected because Claude Code doesn't use this pattern and it introduces port conflicts.

**Alternative considered**: Device code flow (RFC 8628). Not supported by Anthropic's OAuth server.

### 5. Subcommand handling

**Decision**: Use `flag` package with positional args. `glamdring login` and `glamdring logout` are handled before the TUI starts. If the first positional arg is `login` or `logout`, run that flow and exit. Otherwise, proceed to TUI.

**Alternative considered**: Using a CLI framework like cobra. Rejected — only two subcommands, not worth the dependency.

### 6. Token refresh with file locking

**Decision**: Use a simple file lock at `~/.claude/.auth-lock` during token refresh to prevent concurrent refresh from multiple glamdring/claude processes. Retry up to 5 times with random 1-2s backoff if lock is held.

**Rationale**: Matches Claude Code's behavior. Prevents race conditions where two processes refresh the same token simultaneously (which would invalidate one refresh token).

### 7. API client auth abstraction

**Decision**: Replace the `apiKey string` field in `api.Client` with a `Credentials` interface that sets auth headers. Two implementations: `APIKeyCredentials` (sets `x-api-key`) and `OAuthCredentials` (sets `Authorization: Bearer` + beta header). The `OAuthCredentials` implementation holds a reference to the token store so it can transparently refresh on 401.

**Rationale**: Clean separation. The API client doesn't need to know which auth mode is active.

## Risks / Trade-offs

- **Shared credentials with Claude Code** → If Anthropic changes the token format or storage location, glamdring breaks. Mitigation: the format has been stable since launch and we fall back to env var.
- **Using Claude Code's OAuth client ID** → Anthropic could restrict the client ID to Claude Code only. Mitigation: unlikely since it's embedded in a public binary, but we could register our own client ID later.
- **macOS-only keychain** → Linux/Windows users can't use keychain storage. Mitigation: `~/.claude.json` file storage works everywhere, keychain is just a bonus.
- **8-hour token expiry** → Long-running sessions need in-flight refresh. Mitigation: `OAuthCredentials` detects 401 and refreshes before retrying.
- **File lock contention** → Multiple processes refreshing simultaneously. Mitigation: lock with retry + backoff, matches Claude Code's approach.
