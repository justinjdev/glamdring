## ADDED Requirements

### Requirement: OAuth login via browser
The system SHALL implement OAuth 2.0 Authorization Code with PKCE to authenticate users via their Claude account. Running `glamdring login` SHALL open the user's default browser to the Anthropic authorization URL with a PKCE code challenge.

#### Scenario: Successful login
- **WHEN** user runs `glamdring login`
- **THEN** system generates a PKCE code verifier and challenge, opens the browser to `https://claude.ai/oauth/authorize` with the challenge, prompts the user to paste the authorization code, exchanges the code for access and refresh tokens, stores tokens in `~/.claude.json` and macOS Keychain, and prints a success message

#### Scenario: Login when already authenticated
- **WHEN** user runs `glamdring login` and valid OAuth tokens already exist
- **THEN** system completes the login flow and overwrites the existing tokens with fresh ones

#### Scenario: Login with invalid authorization code
- **WHEN** user pastes an invalid or expired authorization code
- **THEN** system prints an error message indicating the code was rejected and exits with non-zero status

### Requirement: OAuth logout
The system SHALL support `glamdring logout` to remove stored OAuth credentials.

#### Scenario: Successful logout
- **WHEN** user runs `glamdring logout` and OAuth tokens are stored
- **THEN** system removes OAuth tokens from `~/.claude.json` and the macOS Keychain, and prints a confirmation message

#### Scenario: Logout when not authenticated
- **WHEN** user runs `glamdring logout` and no OAuth tokens are stored
- **THEN** system prints a message indicating no credentials were found and exits with zero status

### Requirement: PKCE security
The system SHALL use PKCE (RFC 7636) with S256 code challenge method. The code verifier SHALL be 32 random bytes, base64url-encoded. The code challenge SHALL be the SHA-256 hash of the verifier, base64url-encoded.

#### Scenario: PKCE parameters are generated correctly
- **WHEN** the login flow generates PKCE parameters
- **THEN** the code verifier is 43+ characters of base64url, the code challenge is the SHA-256 of the verifier base64url-encoded, and the challenge method is `S256`

### Requirement: Token storage
The system SHALL store OAuth tokens in `~/.claude.json` under the `claudeAiOauth` key with fields `accessToken`, `refreshToken`, `expiresAt`, and `scopes`. The system SHALL also store tokens in the macOS Keychain under service name `Claude Code-credentials`.

#### Scenario: Tokens are persisted after login
- **WHEN** the token exchange succeeds
- **THEN** `~/.claude.json` contains the access token, refresh token, expiration timestamp, and scopes under `claudeAiOauth`, AND the macOS Keychain entry `Claude Code-credentials` is created or updated

#### Scenario: Token file permissions
- **WHEN** tokens are written to `~/.claude.json`
- **THEN** the file permissions SHALL be 0600 (owner read/write only)
