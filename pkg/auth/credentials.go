package auth

import (
	"fmt"
	"net/http"
	"sync"
)

// Credentials sets authentication headers on an HTTP request.
type Credentials interface {
	SetAuthHeaders(req *http.Request) error
	// IsOAuth returns true if these are OAuth credentials (for beta header / 401 retry).
	IsOAuth() bool
}

// APIKeyCredentials authenticates using an API key via x-api-key header.
type APIKeyCredentials struct {
	Key string
}

func (c *APIKeyCredentials) SetAuthHeaders(req *http.Request) error {
	req.Header.Set("x-api-key", c.Key)
	return nil
}

func (c *APIKeyCredentials) IsOAuth() bool { return false }

// OAuthCredentials authenticates using an OAuth Bearer token.
// It auto-refreshes expired tokens before setting headers.
type OAuthCredentials struct {
	mu     sync.Mutex
	tokens *OAuthTokens
}

func NewOAuthCredentials(tokens *OAuthTokens) *OAuthCredentials {
	return &OAuthCredentials{tokens: tokens}
}

func (c *OAuthCredentials) SetAuthHeaders(req *http.Request) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Refresh if expired or about to expire.
	if c.tokens.IsExpired() {
		refreshed, err := RefreshAccessToken(c.tokens.RefreshToken)
		if err != nil {
			return fmt.Errorf("refresh token: %w", err)
		}
		c.tokens = refreshed
	}

	req.Header.Set("Authorization", "Bearer "+c.tokens.AccessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	return nil
}

func (c *OAuthCredentials) IsOAuth() bool { return true }

// Refresh forces a token refresh and updates the stored tokens.
// Used by the API client after receiving a 401.
func (c *OAuthCredentials) Refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	refreshed, err := RefreshAccessToken(c.tokens.RefreshToken)
	if err != nil {
		return err
	}
	c.tokens = refreshed
	return nil
}
