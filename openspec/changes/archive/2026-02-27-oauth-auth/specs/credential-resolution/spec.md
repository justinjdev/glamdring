## ADDED Requirements

### Requirement: Multi-source credential resolution
The system SHALL resolve credentials in priority order: (1) `ANTHROPIC_API_KEY` environment variable, (2) OAuth tokens from `~/.claude.json`, (3) OAuth tokens from macOS Keychain, (4) no credentials found.

#### Scenario: API key environment variable set
- **WHEN** `ANTHROPIC_API_KEY` is set in the environment
- **THEN** system uses API key auth mode regardless of any stored OAuth tokens

#### Scenario: OAuth tokens in claude.json
- **WHEN** `ANTHROPIC_API_KEY` is not set AND `~/.claude.json` contains valid OAuth tokens
- **THEN** system uses OAuth Bearer auth mode with the stored access token

#### Scenario: OAuth tokens in keychain only
- **WHEN** `ANTHROPIC_API_KEY` is not set AND `~/.claude.json` has no OAuth tokens AND macOS Keychain has `Claude Code-credentials`
- **THEN** system uses OAuth Bearer auth mode with the keychain token

#### Scenario: No credentials found
- **WHEN** no credentials are available from any source
- **THEN** system prints a message suggesting `glamdring login` or setting `ANTHROPIC_API_KEY`, and exits with non-zero status

### Requirement: Automatic token refresh
The system SHALL automatically refresh the OAuth access token when it is expired or within 5 minutes of expiry, using the stored refresh token.

#### Scenario: Token expired before API call
- **WHEN** the stored access token has expired AND a valid refresh token exists
- **THEN** system exchanges the refresh token for a new access/refresh token pair at `https://platform.claude.com/v1/oauth/token`, stores the new tokens, and proceeds with the new access token

#### Scenario: Token expires during a session
- **WHEN** an API call returns HTTP 401 during an active session
- **THEN** system attempts a token refresh and retries the failed request with the new token

#### Scenario: Refresh token is invalid
- **WHEN** the refresh token has been revoked or is invalid
- **THEN** system prints a message suggesting `glamdring login` to re-authenticate and exits with non-zero status

### Requirement: Concurrent refresh safety
The system SHALL use a file lock at `~/.claude/.auth-lock` during token refresh to prevent concurrent refresh by multiple processes.

#### Scenario: Lock acquired successfully
- **WHEN** a token refresh is needed AND no other process holds the lock
- **THEN** system acquires the lock, refreshes the token, stores new tokens, and releases the lock

#### Scenario: Lock held by another process
- **WHEN** a token refresh is needed AND another process holds the lock
- **THEN** system retries up to 5 times with 1-2 second random backoff, then re-reads stored tokens (which the other process may have refreshed)

#### Scenario: Lock holder refreshed successfully
- **WHEN** a process fails to acquire the lock and then re-reads stored tokens
- **THEN** if the re-read tokens are valid (not expired), the system uses them without refreshing
