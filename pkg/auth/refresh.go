package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var tokenURL = "https://platform.claude.com/v1/oauth/token"

// tokenResponse is the JSON body returned by the token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// RefreshAccessToken uses the refresh token to obtain a new access/refresh token pair.
// It acquires the file lock, refreshes, stores the new tokens, and releases the lock.
// If the lock cannot be acquired, it re-reads stored tokens (another process may have refreshed).
func RefreshAccessToken(refreshToken string) (*OAuthTokens, error) {
	lock, err := AcquireLock()
	if err != nil {
		// Another process may have refreshed. Re-read tokens.
		tokens, readErr := ReadTokens()
		if readErr == nil && !tokens.IsExpired() {
			return tokens, nil
		}
		return nil, fmt.Errorf("acquire lock: %w (re-read also failed: %v)", err, readErr)
	}
	defer lock.Release()

	// Re-read tokens after acquiring lock — another process may have refreshed while we waited.
	if tokens, readErr := ReadTokens(); readErr == nil && !tokens.IsExpired() {
		return tokens, nil
	}

	// Perform the token refresh.
	resp, err := http.PostForm(tokenURL, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {oauthClientID},
	})
	if err != nil {
		return nil, fmt.Errorf("token refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokResp tokenResponse
	if err := json.Unmarshal(body, &tokResp); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokResp.ExpiresIn) * time.Second)

	tokens := &OAuthTokens{
		AccessToken:  tokResp.AccessToken,
		RefreshToken: tokResp.RefreshToken,
		ExpiresAt:    expiresAt.UnixMilli(),
		Scopes:       strings.Fields(tokResp.Scope),
	}

	// Store the new tokens.
	if err := WriteTokens(tokens); err != nil {
		return nil, fmt.Errorf("store refreshed tokens: %w", err)
	}

	// Best-effort keychain update.
	_ = WriteKeychain(tokens)

	return tokens, nil
}
