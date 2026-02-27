package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// OAuthTokens holds the OAuth token data stored in ~/.claude.json.
type OAuthTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
	Scopes       string `json:"scopes"`
}

// IsExpired returns true if the access token has expired or will expire within 5 minutes.
func (t *OAuthTokens) IsExpired() bool {
	exp, err := time.Parse(time.RFC3339, t.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().After(exp.Add(-5 * time.Minute))
}

// claudeJSONPath returns the path to ~/.claude.json.
func claudeJSONPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

// ReadTokens reads OAuth tokens from ~/.claude.json under the "claudeAiOauth" key.
func ReadTokens() (*OAuthTokens, error) {
	data, err := os.ReadFile(claudeJSONPath())
	if err != nil {
		return nil, fmt.Errorf("read claude.json: %w", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse claude.json: %w", err)
	}

	raw, ok := doc["claudeAiOauth"]
	if !ok {
		return nil, fmt.Errorf("no claudeAiOauth key in claude.json")
	}

	var tokens OAuthTokens
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return nil, fmt.Errorf("parse claudeAiOauth: %w", err)
	}

	if tokens.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in claude.json")
	}

	return &tokens, nil
}

// WriteTokens writes OAuth tokens to ~/.claude.json under the "claudeAiOauth" key,
// preserving all other keys in the file. The file is created with 0600 permissions.
func WriteTokens(tokens *OAuthTokens) error {
	path := claudeJSONPath()

	// Read existing content to preserve other keys.
	var doc map[string]json.RawMessage
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			doc = make(map[string]json.RawMessage)
		}
	} else {
		doc = make(map[string]json.RawMessage)
	}

	raw, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}
	doc["claudeAiOauth"] = raw

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal claude.json: %w", err)
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("write claude.json: %w", err)
	}

	return nil
}

// RemoveTokens removes the "claudeAiOauth" key from ~/.claude.json,
// preserving all other keys. Returns true if tokens were removed.
func RemoveTokens() (bool, error) {
	path := claudeJSONPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read claude.json: %w", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("parse claude.json: %w", err)
	}

	if _, ok := doc["claudeAiOauth"]; !ok {
		return false, nil
	}

	delete(doc, "claudeAiOauth")

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal claude.json: %w", err)
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return false, fmt.Errorf("write claude.json: %w", err)
	}

	return true, nil
}
