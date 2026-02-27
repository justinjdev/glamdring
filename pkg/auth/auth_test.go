package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// 1. PKCE Generation
// ---------------------------------------------------------------------------

func TestGeneratePKCE_VerifierLength(t *testing.T) {
	p, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error: %v", err)
	}
	// 32 bytes base64url-encoded without padding = 43 characters.
	if len(p.Verifier) < 43 {
		t.Errorf("verifier length = %d, want >= 43", len(p.Verifier))
	}
}

func TestGeneratePKCE_ChallengeIsSHA256(t *testing.T) {
	p, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error: %v", err)
	}

	hash := sha256.Sum256([]byte(p.Verifier))
	want := base64.RawURLEncoding.EncodeToString(hash[:])

	if p.Challenge != want {
		t.Errorf("challenge mismatch\n  got:  %s\n  want: %s", p.Challenge, want)
	}
}

func TestGeneratePKCE_Randomness(t *testing.T) {
	a, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("first GeneratePKCE() error: %v", err)
	}
	b, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("second GeneratePKCE() error: %v", err)
	}
	if a.Verifier == b.Verifier {
		t.Error("two GeneratePKCE calls returned identical verifiers")
	}
}

// ---------------------------------------------------------------------------
// 2. Token Store Round-Trip
// ---------------------------------------------------------------------------

// setupTempHome points HOME at a temp directory and creates the .claude.json
// parent so the store functions can write to it.
func setupTempHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	return tmp
}

func sampleTokens() *OAuthTokens {
	return &OAuthTokens{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
		Scopes:       "org:read user:read",
	}
}

func TestWriteReadTokens_RoundTrip(t *testing.T) {
	setupTempHome(t)

	tok := sampleTokens()
	if err := WriteTokens(tok); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	got, err := ReadTokens()
	if err != nil {
		t.Fatalf("ReadTokens() error: %v", err)
	}

	if got.AccessToken != tok.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, tok.AccessToken)
	}
	if got.RefreshToken != tok.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, tok.RefreshToken)
	}
	if got.Scopes != tok.Scopes {
		t.Errorf("Scopes = %q, want %q", got.Scopes, tok.Scopes)
	}
}

func TestWriteTokens_PreservesExistingKeys(t *testing.T) {
	tmp := setupTempHome(t)

	// Seed claude.json with an extra key.
	path := filepath.Join(tmp, ".claude.json")
	seed := map[string]any{"someOtherKey": "preserve-me"}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("seed write error: %v", err)
	}

	if err := WriteTokens(sampleTokens()); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back error: %v", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := doc["someOtherKey"]; !ok {
		t.Error("existing key 'someOtherKey' was not preserved")
	}
	if _, ok := doc["claudeAiOauth"]; !ok {
		t.Error("claudeAiOauth key missing after WriteTokens")
	}
}

func TestRemoveTokens_RemovesOAuthPreservesOthers(t *testing.T) {
	tmp := setupTempHome(t)

	// Write a file with both claudeAiOauth and another key.
	path := filepath.Join(tmp, ".claude.json")
	seed := map[string]any{
		"someOtherKey": "keep-this",
	}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("seed write error: %v", err)
	}

	// Add tokens via WriteTokens so the key exists.
	if err := WriteTokens(sampleTokens()); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	removed, err := RemoveTokens()
	if err != nil {
		t.Fatalf("RemoveTokens() error: %v", err)
	}
	if !removed {
		t.Error("RemoveTokens() returned false, want true")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back error: %v", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := doc["claudeAiOauth"]; ok {
		t.Error("claudeAiOauth key still present after RemoveTokens")
	}
	if _, ok := doc["someOtherKey"]; !ok {
		t.Error("existing key 'someOtherKey' was not preserved after RemoveTokens")
	}
}

// ---------------------------------------------------------------------------
// 3. Credential Resolution Priority
// ---------------------------------------------------------------------------

func TestResolve_APIKeyFromEnv(t *testing.T) {
	setupTempHome(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-key-123")

	cred, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	apiKey, ok := cred.(*APIKeyCredentials)
	if !ok {
		t.Fatalf("got type %T, want *APIKeyCredentials", cred)
	}
	if apiKey.Key != "sk-test-key-123" {
		t.Errorf("Key = %q, want %q", apiKey.Key, "sk-test-key-123")
	}
}

func TestResolve_OAuthFromClaudeJSON(t *testing.T) {
	setupTempHome(t)
	t.Setenv("ANTHROPIC_API_KEY", "") // ensure no API key

	tok := &OAuthTokens{
		AccessToken:  "oauth-access",
		RefreshToken: "oauth-refresh",
		ExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
		Scopes:       "org:read",
	}
	if err := WriteTokens(tok); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	cred, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	if _, ok := cred.(*OAuthCredentials); !ok {
		t.Fatalf("got type %T, want *OAuthCredentials", cred)
	}
}

func TestResolve_NoCredentials(t *testing.T) {
	setupTempHome(t) // empty HOME — no .claude.json
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := Resolve()
	if err == nil {
		t.Fatal("Resolve() returned nil error, want NoCredentialsError")
	}

	var nce *NoCredentialsError
	if !errors.As(err, &nce) {
		t.Errorf("error type = %T, want *NoCredentialsError", err)
	}
}

// ---------------------------------------------------------------------------
// 4. OAuthTokens.IsExpired
// ---------------------------------------------------------------------------

func TestIsExpired_PastToken(t *testing.T) {
	tok := &OAuthTokens{
		ExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
	}
	if !tok.IsExpired() {
		t.Error("token with ExpiresAt in the past should be expired")
	}
}

func TestIsExpired_WithinFiveMinutes(t *testing.T) {
	tok := &OAuthTokens{
		ExpiresAt: time.Now().Add(3 * time.Minute).Format(time.RFC3339),
	}
	if !tok.IsExpired() {
		t.Error("token expiring within 5 minutes should be treated as expired")
	}
}

func TestIsExpired_FutureToken(t *testing.T) {
	tok := &OAuthTokens{
		ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if tok.IsExpired() {
		t.Error("token expiring in 1 hour should not be expired")
	}
}
