package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const keychainService = "Claude Code-credentials"

// ReadKeychain reads OAuth tokens from the macOS Keychain entry for Claude Code.
// The Keychain entry is stored as {"claudeAiOauth": {...tokens...}}.
func ReadKeychain() (*OAuthTokens, error) {
	out, err := exec.Command(
		"security", "find-generic-password",
		"-s", keychainService,
		"-w",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("keychain read: %w", err)
	}

	val := strings.TrimSpace(string(out))
	if val == "" {
		return nil, fmt.Errorf("empty keychain entry")
	}

	// The keychain value is wrapped: {"claudeAiOauth": {...tokens...}}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(val), &envelope); err != nil {
		return nil, fmt.Errorf("parse keychain entry: %w", err)
	}

	raw, ok := envelope["claudeAiOauth"]
	if !ok {
		return nil, fmt.Errorf("no claudeAiOauth in keychain entry")
	}

	var tokens OAuthTokens
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return nil, fmt.Errorf("parse keychain tokens: %w", err)
	}

	if tokens.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in keychain")
	}

	return &tokens, nil
}

// WriteKeychain writes OAuth tokens to the macOS Keychain, wrapped in the
// {"claudeAiOauth": ...} envelope to match Claude Code's format.
func WriteKeychain(tokens *OAuthTokens) error {
	envelope := map[string]*OAuthTokens{"claudeAiOauth": tokens}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}

	user, _ := os.UserHomeDir()
	account := filepath.Base(user)

	// Delete existing entry (ignore errors — may not exist).
	_ = exec.Command(
		"security", "delete-generic-password",
		"-s", keychainService,
	).Run()

	if err := exec.Command(
		"security", "add-generic-password",
		"-s", keychainService,
		"-a", account,
		"-w", string(data),
		"-U",
	).Run(); err != nil {
		return fmt.Errorf("keychain write: %w", err)
	}

	return nil
}

// RemoveKeychain removes the OAuth tokens from the macOS Keychain.
// Returns true if an entry was removed.
func RemoveKeychain() (bool, error) {
	err := exec.Command(
		"security", "delete-generic-password",
		"-s", keychainService,
	).Run()
	if err != nil {
		// Exit code non-zero means entry didn't exist.
		return false, nil
	}
	return true, nil
}
