package auth

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const keychainService = "Claude Code-credentials"
const keychainAccount = "default"

// ReadKeychain reads OAuth tokens from the macOS Keychain entry for Claude Code.
func ReadKeychain() (*OAuthTokens, error) {
	out, err := exec.Command(
		"security", "find-generic-password",
		"-s", keychainService,
		"-a", keychainAccount,
		"-w",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("keychain read: %w", err)
	}

	val := strings.TrimSpace(string(out))
	if val == "" {
		return nil, fmt.Errorf("empty keychain entry")
	}

	var tokens OAuthTokens
	if err := json.Unmarshal([]byte(val), &tokens); err != nil {
		return nil, fmt.Errorf("parse keychain entry: %w", err)
	}

	if tokens.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in keychain")
	}

	return &tokens, nil
}

// WriteKeychain writes OAuth tokens to the macOS Keychain. It deletes any existing
// entry first to avoid duplicate errors.
func WriteKeychain(tokens *OAuthTokens) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}

	// Delete existing entry (ignore errors — may not exist).
	_ = exec.Command(
		"security", "delete-generic-password",
		"-s", keychainService,
		"-a", keychainAccount,
	).Run()

	if err := exec.Command(
		"security", "add-generic-password",
		"-s", keychainService,
		"-a", keychainAccount,
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
		"-a", keychainAccount,
	).Run()
	if err != nil {
		// Exit code non-zero means entry didn't exist.
		return false, nil
	}
	return true, nil
}
