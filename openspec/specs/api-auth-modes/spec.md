## ADDED Requirements

### Requirement: API key auth mode
The system SHALL support authenticating API requests with an API key sent as the `x-api-key` header.

#### Scenario: API key auth headers
- **WHEN** credentials are resolved as API key mode
- **THEN** HTTP requests to the Messages API include `x-api-key: <key>` header and `anthropic-version: 2023-06-01` header

### Requirement: OAuth Bearer auth mode
The system SHALL support authenticating API requests with an OAuth access token sent as `Authorization: Bearer` header, with the OAuth beta header.

#### Scenario: OAuth auth headers
- **WHEN** credentials are resolved as OAuth mode
- **THEN** HTTP requests to the Messages API include `Authorization: Bearer <token>` header, `anthropic-version: 2023-06-01` header, and `anthropic-beta: oauth-2025-04-20` header

### Requirement: Transparent auth in API client
The API client SHALL accept a credentials provider that sets the appropriate auth headers without the client needing to know which auth mode is active.

#### Scenario: Client uses API key credentials
- **WHEN** the API client is configured with API key credentials
- **THEN** all requests use `x-api-key` header

#### Scenario: Client uses OAuth credentials
- **WHEN** the API client is configured with OAuth credentials
- **THEN** all requests use `Authorization: Bearer` header with beta header

### Requirement: OAuth 401 retry
The API client SHALL detect HTTP 401 responses when using OAuth auth and attempt a token refresh before retrying the request once.

#### Scenario: 401 triggers refresh and retry
- **WHEN** an API request returns HTTP 401 AND auth mode is OAuth
- **THEN** system refreshes the access token and retries the request exactly once with the new token

#### Scenario: 401 after refresh still fails
- **WHEN** a retried request after token refresh returns HTTP 401 again
- **THEN** system returns the error without further retry
