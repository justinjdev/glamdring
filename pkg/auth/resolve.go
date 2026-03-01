package auth

import "os"

// Resolve returns credentials using the priority order:
// 1. ANTHROPIC_API_KEY env var → API key mode
// 2. ~/.claude.json OAuth tokens → OAuth mode
// 3. macOS Keychain → OAuth mode
// 4. No credentials → error
func Resolve() (Credentials, error) {
	// 1. Environment variable.
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return &APIKeyCredentials{Key: key}, nil
	}

	// 2. OAuth tokens from ~/.claude.json.
	if tokens, err := ReadTokens(); err == nil {
		return NewOAuthCredentials(tokens), nil
	}

	// 3. OAuth tokens from macOS Keychain.
	if tokens, err := readKeychainFn(); err == nil {
		return NewOAuthCredentials(tokens), nil
	}

	// 4. No credentials found.
	return nil, &NoCredentialsError{}
}

// NoCredentialsError is returned when no credentials are found from any source.
type NoCredentialsError struct{}

func (e *NoCredentialsError) Error() string {
	return "no credentials found — run 'glamdring login' or set ANTHROPIC_API_KEY"
}
