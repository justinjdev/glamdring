## Why

Glamdring currently requires `ANTHROPIC_API_KEY` to be set manually, which means users need a paid API account. Most Claude Code users authenticate via OAuth with their Claude Pro/Max subscription — glamdring should support the same flow so it works out of the box for anyone with an existing Claude subscription, without needing a separate API key.

## What Changes

- Add a `pkg/auth` package implementing OAuth 2.0 Authorization Code with PKCE
- Read existing credentials from `~/.claude.json` (shared with Claude Code) and macOS Keychain
- Implement automatic token refresh when the access token expires
- Implement full OAuth login flow (open browser, PKCE challenge, exchange code for tokens, store tokens)
- Add `glamdring login` and `glamdring logout` subcommands
- Modify `pkg/api` client to support both `x-api-key` and `Authorization: Bearer` auth modes
- Update `main.go` to resolve credentials automatically: existing OAuth tokens → env var → prompt to login
- **BREAKING**: `ANTHROPIC_API_KEY` is no longer required if OAuth credentials are available

## Capabilities

### New Capabilities
- `oauth-login`: Full OAuth 2.0 PKCE flow — browser-based login, token exchange, credential storage
- `credential-resolution`: Multi-source credential resolution (stored OAuth tokens, keychain, env var, Claude Code shared credentials) with automatic token refresh
- `api-auth-modes`: API client support for both API key (`x-api-key`) and OAuth Bearer token auth headers

### Modified Capabilities
<!-- No existing specs to modify -->

## Impact

- **New package**: `pkg/auth/` — OAuth flow, token storage, credential resolution, token refresh
- **Modified**: `pkg/api/client.go` — accept auth mode (API key vs Bearer token), add `anthropic-beta: oauth-2025-04-20` header for OAuth
- **Modified**: `cmd/glamdring/main.go` — replace hard `ANTHROPIC_API_KEY` requirement with credential resolution; add `login`/`logout` subcommands
- **Dependencies**: None new (uses stdlib `net/http`, `crypto/sha256`, `encoding/base64`, `os/exec` for keychain/browser)
- **External services**: `https://claude.ai/oauth/authorize`, `https://platform.claude.com/v1/oauth/token`
- **Storage**: Writes to `~/.claude.json` (shared format with Claude Code) and macOS Keychain (`Claude Code-credentials`)
