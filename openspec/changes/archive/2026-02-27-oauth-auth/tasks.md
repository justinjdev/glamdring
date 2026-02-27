## 1. Token Storage & File I/O

- [ ] 1.1 Create `pkg/auth/store.go` — read/write OAuth tokens from `~/.claude.json` (preserving existing keys), enforce 0600 permissions
- [ ] 1.2 Create `pkg/auth/keychain.go` — read/write macOS Keychain entry `Claude Code-credentials` via `security` CLI
- [ ] 1.3 Create `pkg/auth/lock.go` — file-based lock at `~/.claude/.auth-lock` with retry + random backoff

## 2. Token Refresh

- [ ] 2.1 Create `pkg/auth/refresh.go` — exchange refresh token for new access/refresh tokens via `POST https://platform.claude.com/v1/oauth/token`
- [ ] 2.2 Integrate file lock during refresh, re-read tokens after failed lock acquisition

## 3. Credential Resolution

- [ ] 3.1 Create `pkg/auth/credentials.go` — `Credentials` interface with `APIKeyCredentials` and `OAuthCredentials` implementations that set auth headers
- [ ] 3.2 Create `pkg/auth/resolve.go` — `Resolve()` function implementing priority order: env var → claude.json → keychain → error
- [ ] 3.3 Wire `OAuthCredentials` to auto-refresh expired tokens before returning headers

## 4. OAuth Login Flow

- [ ] 4.1 Create `pkg/auth/pkce.go` — generate PKCE code verifier (32 random bytes, base64url) and S256 challenge
- [ ] 4.2 Create `pkg/auth/login.go` — build authorization URL, open browser, prompt for code, exchange code for tokens, store
- [ ] 4.3 Create `pkg/auth/logout.go` — remove tokens from `~/.claude.json` and Keychain

## 5. API Client Auth Modes

- [ ] 5.1 Modify `pkg/api/client.go` — replace `apiKey string` with `Credentials` interface, delegate header setting
- [ ] 5.2 Add `anthropic-beta: oauth-2025-04-20` header for OAuth mode
- [ ] 5.3 Add 401 detection + token refresh + single retry for OAuth mode

## 6. CLI Subcommands & Main Wiring

- [ ] 6.1 Update `cmd/glamdring/main.go` — handle `login`/`logout` positional args before TUI startup
- [ ] 6.2 Replace `ANTHROPIC_API_KEY` requirement with `auth.Resolve()` — pass resolved credentials to API client
- [ ] 6.3 Update error message for missing credentials to suggest `glamdring login`

## 7. Tests

- [ ] 7.1 Test PKCE generation (verifier length, challenge is SHA-256 of verifier)
- [ ] 7.2 Test credential resolution priority order
- [ ] 7.3 Test token store read/write (round-trip, preserves existing keys, file permissions)
- [ ] 7.4 Test login/logout subcommand argument parsing
