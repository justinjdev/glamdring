package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain stubs keychain operations so tests never trigger macOS Keychain prompts.
func TestMain(m *testing.M) {
	readKeychainFn = func() (*OAuthTokens, error) {
		return nil, fmt.Errorf("keychain disabled in tests")
	}
	writeKeychainFn = func(_ *OAuthTokens) error {
		return nil
	}
	removeKeychainFn = func() (bool, error) {
		return false, nil
	}
	os.Exit(m.Run())
}

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
		ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
		Scopes:       []string{"org:read", "user:read"},
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
	if len(got.Scopes) != len(tok.Scopes) {
		t.Errorf("Scopes length = %d, want %d", len(got.Scopes), len(tok.Scopes))
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
		ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
		Scopes:       []string{"org:read"},
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
		ExpiresAt: time.Now().Add(-time.Hour).UnixMilli(),
	}
	if !tok.IsExpired() {
		t.Error("token with ExpiresAt in the past should be expired")
	}
}

func TestIsExpired_WithinFiveMinutes(t *testing.T) {
	tok := &OAuthTokens{
		ExpiresAt: time.Now().Add(3 * time.Minute).UnixMilli(),
	}
	if !tok.IsExpired() {
		t.Error("token expiring within 5 minutes should be treated as expired")
	}
}

func TestIsExpired_FutureToken(t *testing.T) {
	tok := &OAuthTokens{
		ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}
	if tok.IsExpired() {
		t.Error("token expiring in 1 hour should not be expired")
	}
}

// ---------------------------------------------------------------------------
// 5. ScopesString
// ---------------------------------------------------------------------------

func TestScopesString_Multiple(t *testing.T) {
	tok := &OAuthTokens{Scopes: []string{"user:read", "org:write", "admin"}}
	got := tok.ScopesString()
	want := "user:read org:write admin"
	if got != want {
		t.Errorf("ScopesString() = %q, want %q", got, want)
	}
}

func TestScopesString_Single(t *testing.T) {
	tok := &OAuthTokens{Scopes: []string{"user:read"}}
	got := tok.ScopesString()
	if got != "user:read" {
		t.Errorf("ScopesString() = %q, want %q", got, "user:read")
	}
}

func TestScopesString_Empty(t *testing.T) {
	tok := &OAuthTokens{Scopes: nil}
	got := tok.ScopesString()
	if got != "" {
		t.Errorf("ScopesString() = %q, want empty string", got)
	}
}

// ---------------------------------------------------------------------------
// 6. APIKeyCredentials
// ---------------------------------------------------------------------------

func TestAPIKeyCredentials_SetAuthHeaders(t *testing.T) {
	cred := &APIKeyCredentials{Key: "sk-test-123"}
	req, _ := http.NewRequest("GET", "https://example.com", nil)

	if err := cred.SetAuthHeaders(req); err != nil {
		t.Fatalf("SetAuthHeaders() error: %v", err)
	}

	got := req.Header.Get("x-api-key")
	if got != "sk-test-123" {
		t.Errorf("x-api-key header = %q, want %q", got, "sk-test-123")
	}
}

func TestAPIKeyCredentials_IsOAuth(t *testing.T) {
	cred := &APIKeyCredentials{Key: "sk-test"}
	if cred.IsOAuth() {
		t.Error("APIKeyCredentials.IsOAuth() = true, want false")
	}
}

// ---------------------------------------------------------------------------
// 7. OAuthCredentials
// ---------------------------------------------------------------------------

func TestOAuthCredentials_IsOAuth(t *testing.T) {
	cred := NewOAuthCredentials(sampleTokens())
	if !cred.IsOAuth() {
		t.Error("OAuthCredentials.IsOAuth() = false, want true")
	}
}

func TestOAuthCredentials_SetAuthHeaders_ValidToken(t *testing.T) {
	tok := &OAuthTokens{
		AccessToken:  "valid-token-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
		Scopes:       []string{"user:read"},
	}
	cred := NewOAuthCredentials(tok)

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	if err := cred.SetAuthHeaders(req); err != nil {
		t.Fatalf("SetAuthHeaders() error: %v", err)
	}

	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer valid-token-abc" {
		t.Errorf("Authorization = %q, want %q", authHeader, "Bearer valid-token-abc")
	}

	betaHeader := req.Header.Get("anthropic-beta")
	if betaHeader != "oauth-2025-04-20" {
		t.Errorf("anthropic-beta = %q, want %q", betaHeader, "oauth-2025-04-20")
	}
}

func TestOAuthCredentials_SetAuthHeaders_ExpiredToken_RefreshSucceeds(t *testing.T) {
	setupTempHome(t)

	// Set up a mock token endpoint that returns new tokens.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"scope":         "user:read user:write",
		})
	}))
	defer srv.Close()

	// Override the token URL.
	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	tok := &OAuthTokens{
		AccessToken:  "expired-token",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    time.Now().Add(-time.Hour).UnixMilli(), // expired
		Scopes:       []string{"user:read"},
	}
	cred := NewOAuthCredentials(tok)

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	if err := cred.SetAuthHeaders(req); err != nil {
		t.Fatalf("SetAuthHeaders() error: %v", err)
	}

	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer new-access-token" {
		t.Errorf("Authorization = %q, want %q", authHeader, "Bearer new-access-token")
	}
}

func TestOAuthCredentials_SetAuthHeaders_ExpiredToken_RefreshFails(t *testing.T) {
	setupTempHome(t)

	// Set up a mock token endpoint that returns an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	tok := &OAuthTokens{
		AccessToken:  "expired-token",
		RefreshToken: "bad-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour).UnixMilli(), // expired
		Scopes:       []string{"user:read"},
	}
	cred := NewOAuthCredentials(tok)

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	err := cred.SetAuthHeaders(req)
	if err == nil {
		t.Fatal("SetAuthHeaders() should return error when refresh fails")
	}
	if !strings.Contains(err.Error(), "refresh token") {
		t.Errorf("error = %q, want it to contain 'refresh token'", err.Error())
	}
}

func TestOAuthCredentials_Refresh(t *testing.T) {
	setupTempHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-access",
			"refresh_token": "refreshed-refresh",
			"expires_in":    7200,
			"token_type":    "Bearer",
			"scope":         "user:read",
		})
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	tok := &OAuthTokens{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour).UnixMilli(),
		Scopes:       []string{"user:read"},
	}
	cred := NewOAuthCredentials(tok)

	if err := cred.Refresh(); err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	// Verify that the new token is used in subsequent calls.
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	if err := cred.SetAuthHeaders(req); err != nil {
		t.Fatalf("SetAuthHeaders() after Refresh() error: %v", err)
	}

	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer refreshed-access" {
		t.Errorf("Authorization = %q, want %q", authHeader, "Bearer refreshed-access")
	}
}

func TestOAuthCredentials_Refresh_Failure(t *testing.T) {
	setupTempHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	tok := &OAuthTokens{
		AccessToken:  "old-access",
		RefreshToken: "bad-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour).UnixMilli(),
		Scopes:       []string{"user:read"},
	}
	cred := NewOAuthCredentials(tok)

	err := cred.Refresh()
	if err == nil {
		t.Fatal("Refresh() should return error when server returns error")
	}
}

// ---------------------------------------------------------------------------
// 8. NoCredentialsError
// ---------------------------------------------------------------------------

func TestNoCredentialsError_Message(t *testing.T) {
	err := &NoCredentialsError{}
	msg := err.Error()
	if !strings.Contains(msg, "glamdring login") {
		t.Errorf("error message = %q, should mention 'glamdring login'", msg)
	}
	if !strings.Contains(msg, "ANTHROPIC_API_KEY") {
		t.Errorf("error message = %q, should mention 'ANTHROPIC_API_KEY'", msg)
	}
}

// ---------------------------------------------------------------------------
// 9. ReadTokens Error Paths
// ---------------------------------------------------------------------------

func TestReadTokens_FileNotExist(t *testing.T) {
	setupTempHome(t) // empty HOME
	_, err := ReadTokens()
	if err == nil {
		t.Fatal("ReadTokens() should error when file does not exist")
	}
}

func TestReadTokens_InvalidJSON(t *testing.T) {
	tmp := setupTempHome(t)
	path := filepath.Join(tmp, ".claude.json")
	os.WriteFile(path, []byte("not valid json{{{"), 0600)

	_, err := ReadTokens()
	if err == nil {
		t.Fatal("ReadTokens() should error on invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse claude.json") {
		t.Errorf("error = %q, want it to mention 'parse claude.json'", err.Error())
	}
}

func TestReadTokens_MissingOAuthKey(t *testing.T) {
	tmp := setupTempHome(t)
	path := filepath.Join(tmp, ".claude.json")
	data, _ := json.Marshal(map[string]any{"otherKey": "value"})
	os.WriteFile(path, data, 0600)

	_, err := ReadTokens()
	if err == nil {
		t.Fatal("ReadTokens() should error when claudeAiOauth key is missing")
	}
	if !strings.Contains(err.Error(), "no claudeAiOauth") {
		t.Errorf("error = %q, want it to mention 'no claudeAiOauth'", err.Error())
	}
}

func TestReadTokens_EmptyAccessToken(t *testing.T) {
	tmp := setupTempHome(t)
	path := filepath.Join(tmp, ".claude.json")
	tok := map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":  "",
			"refreshToken": "some-refresh",
			"expiresAt":    time.Now().Add(time.Hour).UnixMilli(),
		},
	}
	data, _ := json.Marshal(tok)
	os.WriteFile(path, data, 0600)

	_, err := ReadTokens()
	if err == nil {
		t.Fatal("ReadTokens() should error when access token is empty")
	}
	if !strings.Contains(err.Error(), "empty access token") {
		t.Errorf("error = %q, want it to mention 'empty access token'", err.Error())
	}
}

func TestReadTokens_BadOAuthValue(t *testing.T) {
	tmp := setupTempHome(t)
	path := filepath.Join(tmp, ".claude.json")
	// claudeAiOauth is not a valid JSON object for OAuthTokens.
	data := []byte(`{"claudeAiOauth": "not-an-object"}`)
	os.WriteFile(path, data, 0600)

	_, err := ReadTokens()
	if err == nil {
		t.Fatal("ReadTokens() should error when claudeAiOauth is not a valid object")
	}
	if !strings.Contains(err.Error(), "parse claudeAiOauth") {
		t.Errorf("error = %q, want it to mention 'parse claudeAiOauth'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 10. WriteTokens Edge Cases
// ---------------------------------------------------------------------------

func TestWriteTokens_BadExistingJSON(t *testing.T) {
	tmp := setupTempHome(t)
	path := filepath.Join(tmp, ".claude.json")
	// Write invalid JSON. WriteTokens should start fresh.
	os.WriteFile(path, []byte("not json!!!"), 0600)

	if err := WriteTokens(sampleTokens()); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	// Should still be able to read back.
	got, err := ReadTokens()
	if err != nil {
		t.Fatalf("ReadTokens() after WriteTokens() on bad JSON: %v", err)
	}
	if got.AccessToken != "access-abc" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "access-abc")
	}
}

func TestWriteTokens_CreatesFile(t *testing.T) {
	setupTempHome(t)

	// No .claude.json exists yet.
	if err := WriteTokens(sampleTokens()); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	got, err := ReadTokens()
	if err != nil {
		t.Fatalf("ReadTokens() error: %v", err)
	}
	if got.AccessToken != "access-abc" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "access-abc")
	}
}

// ---------------------------------------------------------------------------
// 11. RemoveTokens Edge Cases
// ---------------------------------------------------------------------------

func TestRemoveTokens_NoFile(t *testing.T) {
	setupTempHome(t) // empty home, no .claude.json

	removed, err := RemoveTokens()
	if err != nil {
		t.Fatalf("RemoveTokens() error: %v", err)
	}
	if removed {
		t.Error("RemoveTokens() returned true, want false when file does not exist")
	}
}

func TestRemoveTokens_NoOAuthKey(t *testing.T) {
	tmp := setupTempHome(t)
	path := filepath.Join(tmp, ".claude.json")
	data, _ := json.Marshal(map[string]any{"otherKey": "value"})
	os.WriteFile(path, data, 0600)

	removed, err := RemoveTokens()
	if err != nil {
		t.Fatalf("RemoveTokens() error: %v", err)
	}
	if removed {
		t.Error("RemoveTokens() returned true, want false when no claudeAiOauth key")
	}
}

func TestRemoveTokens_BadJSON(t *testing.T) {
	tmp := setupTempHome(t)
	path := filepath.Join(tmp, ".claude.json")
	os.WriteFile(path, []byte("not json"), 0600)

	_, err := RemoveTokens()
	if err == nil {
		t.Fatal("RemoveTokens() should error on invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// 12. Lock Acquire / Release
// ---------------------------------------------------------------------------

func TestAcquireLock_Success(t *testing.T) {
	setupTempHome(t)

	lock, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock() error: %v", err)
	}
	defer lock.Release()

	// Lock file should exist.
	path := lockPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("lock file should exist after AcquireLock()")
	}
}

func TestAcquireLock_ReleaseRemovesFile(t *testing.T) {
	setupTempHome(t)

	lock, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock() error: %v", err)
	}

	path := lockPath()
	lock.Release()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("lock file should not exist after Release()")
	}
}

func TestAcquireLock_ReacquireAfterRelease(t *testing.T) {
	setupTempHome(t)

	lock1, err := AcquireLock()
	if err != nil {
		t.Fatalf("first AcquireLock() error: %v", err)
	}
	lock1.Release()

	lock2, err := AcquireLock()
	if err != nil {
		t.Fatalf("second AcquireLock() error: %v", err)
	}
	lock2.Release()
}

func TestAcquireLock_StaleLockRemoved(t *testing.T) {
	tmp := setupTempHome(t)

	// Manually create a stale lock file (modified 60 seconds ago).
	lockDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(lockDir, 0700)
	path := filepath.Join(lockDir, ".auth-lock")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create stale lock: %v", err)
	}
	f.Close()

	// Set modification time to 60 seconds ago (past the 30s stale threshold).
	staleTime := time.Now().Add(-60 * time.Second)
	os.Chtimes(path, staleTime, staleTime)

	// Should be able to acquire despite the stale lock.
	lock, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock() with stale lock error: %v", err)
	}
	lock.Release()
}

// ---------------------------------------------------------------------------
// 13. RefreshAccessToken
// ---------------------------------------------------------------------------

func TestRefreshAccessToken_Success(t *testing.T) {
	setupTempHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate the request body.
		r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want %q", r.FormValue("grant_type"), "refresh_token")
		}
		if r.FormValue("refresh_token") != "my-refresh-token" {
			t.Errorf("refresh_token = %q, want %q", r.FormValue("refresh_token"), "my-refresh-token")
		}
		if r.FormValue("client_id") != oauthClientID {
			t.Errorf("client_id = %q, want %q", r.FormValue("client_id"), oauthClientID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"scope":         "user:read user:write",
		})
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	tokens, err := RefreshAccessToken("my-refresh-token")
	if err != nil {
		t.Fatalf("RefreshAccessToken() error: %v", err)
	}

	if tokens.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want %q", tokens.AccessToken, "new-access")
	}
	if tokens.RefreshToken != "new-refresh" {
		t.Errorf("RefreshToken = %q, want %q", tokens.RefreshToken, "new-refresh")
	}
	if len(tokens.Scopes) != 2 {
		t.Errorf("Scopes length = %d, want 2", len(tokens.Scopes))
	}

	// Verify that the tokens were persisted to ~/.claude.json.
	stored, err := ReadTokens()
	if err != nil {
		t.Fatalf("ReadTokens() after refresh: %v", err)
	}
	if stored.AccessToken != "new-access" {
		t.Errorf("stored AccessToken = %q, want %q", stored.AccessToken, "new-access")
	}
}

func TestRefreshAccessToken_ServerError(t *testing.T) {
	setupTempHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server_error"}`))
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	_, err := RefreshAccessToken("my-refresh-token")
	if err == nil {
		t.Fatal("RefreshAccessToken() should error on server error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error = %q, want it to mention 'HTTP 500'", err.Error())
	}
}

func TestRefreshAccessToken_BadJSON(t *testing.T) {
	setupTempHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	_, err := RefreshAccessToken("my-refresh-token")
	if err == nil {
		t.Fatal("RefreshAccessToken() should error on bad JSON response")
	}
	if !strings.Contains(err.Error(), "parse refresh response") {
		t.Errorf("error = %q, want it to mention 'parse refresh response'", err.Error())
	}
}

func TestRefreshAccessToken_SkipsIfFreshTokensExist(t *testing.T) {
	setupTempHome(t)

	// Write fresh tokens to the store first.
	fresh := &OAuthTokens{
		AccessToken:  "fresh-access",
		RefreshToken: "fresh-refresh",
		ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
		Scopes:       []string{"user:read"},
	}
	if err := WriteTokens(fresh); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	// The server should never be called because stored tokens are fresh.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	tokens, err := RefreshAccessToken("old-refresh")
	if err != nil {
		t.Fatalf("RefreshAccessToken() error: %v", err)
	}

	if called {
		t.Error("token endpoint was called even though stored tokens are fresh")
	}
	if tokens.AccessToken != "fresh-access" {
		t.Errorf("AccessToken = %q, want %q", tokens.AccessToken, "fresh-access")
	}
}

// ---------------------------------------------------------------------------
// 14. exchangeCode (via Login internals)
// ---------------------------------------------------------------------------

func TestExchangeCode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q, want %q", r.FormValue("grant_type"), "authorization_code")
		}
		if r.FormValue("code") != "test-auth-code" {
			t.Errorf("code = %q, want %q", r.FormValue("code"), "test-auth-code")
		}
		if r.FormValue("code_verifier") != "test-verifier" {
			t.Errorf("code_verifier = %q, want %q", r.FormValue("code_verifier"), "test-verifier")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "exchanged-access",
			"refresh_token": "exchanged-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"scope":         "user:read org:write",
		})
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	tokens, err := exchangeCode("test-auth-code", "test-verifier")
	if err != nil {
		t.Fatalf("exchangeCode() error: %v", err)
	}

	if tokens.AccessToken != "exchanged-access" {
		t.Errorf("AccessToken = %q, want %q", tokens.AccessToken, "exchanged-access")
	}
	if tokens.RefreshToken != "exchanged-refresh" {
		t.Errorf("RefreshToken = %q, want %q", tokens.RefreshToken, "exchanged-refresh")
	}
	if len(tokens.Scopes) != 2 {
		t.Errorf("Scopes = %v, want 2 elements", tokens.Scopes)
	}
}

func TestExchangeCode_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_code"}`))
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	_, err := exchangeCode("bad-code", "verifier")
	if err == nil {
		t.Fatal("exchangeCode() should error on HTTP error")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("error = %q, want it to contain 'HTTP 400'", err.Error())
	}
}

func TestExchangeCode_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{invalid"))
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	_, err := exchangeCode("code", "verifier")
	if err == nil {
		t.Fatal("exchangeCode() should error on bad JSON response")
	}
	if !strings.Contains(err.Error(), "parse exchange response") {
		t.Errorf("error = %q, want it to contain 'parse exchange response'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 15. Resolve edge cases
// ---------------------------------------------------------------------------

func TestResolve_APIKeyTakesPriority(t *testing.T) {
	setupTempHome(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-priority")

	// Also write OAuth tokens -- API key should still win.
	WriteTokens(sampleTokens())

	cred, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	apiKey, ok := cred.(*APIKeyCredentials)
	if !ok {
		t.Fatalf("got type %T, want *APIKeyCredentials", cred)
	}
	if apiKey.Key != "sk-priority" {
		t.Errorf("Key = %q, want %q", apiKey.Key, "sk-priority")
	}
}

// ---------------------------------------------------------------------------
// 16. NewOAuthCredentials
// ---------------------------------------------------------------------------

func TestNewOAuthCredentials(t *testing.T) {
	tok := sampleTokens()
	cred := NewOAuthCredentials(tok)
	if cred == nil {
		t.Fatal("NewOAuthCredentials() returned nil")
	}
	if !cred.IsOAuth() {
		t.Error("NewOAuthCredentials().IsOAuth() = false, want true")
	}
}

// ---------------------------------------------------------------------------
// 17. Lock Release idempotency
// ---------------------------------------------------------------------------

func TestAuthLock_Release_Idempotent(t *testing.T) {
	setupTempHome(t)

	lock, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock() error: %v", err)
	}

	// Release twice should not panic.
	lock.Release()
	lock.Release()
}

// ---------------------------------------------------------------------------
// 18. Integration: WriteTokens then RemoveTokens then ReadTokens fails
// ---------------------------------------------------------------------------

func TestWriteRemoveReadTokens_Integration(t *testing.T) {
	setupTempHome(t)

	if err := WriteTokens(sampleTokens()); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	removed, err := RemoveTokens()
	if err != nil {
		t.Fatalf("RemoveTokens() error: %v", err)
	}
	if !removed {
		t.Error("RemoveTokens() = false, want true")
	}

	_, err = ReadTokens()
	if err == nil {
		t.Fatal("ReadTokens() after RemoveTokens() should fail")
	}
}

// ---------------------------------------------------------------------------
// 19. Token expiry boundary
// ---------------------------------------------------------------------------

func TestIsExpired_ExactlyFiveMinutes(t *testing.T) {
	// Token that expires exactly 5 minutes from now.
	tok := &OAuthTokens{
		ExpiresAt: time.Now().Add(5 * time.Minute).UnixMilli(),
	}
	// At exactly the 5-minute boundary, time.Now() >= exp - 5m,
	// so this should be expired (or right at the boundary).
	if !tok.IsExpired() {
		t.Error("token expiring in exactly 5 minutes should be treated as expired")
	}
}

func TestIsExpired_JustOverFiveMinutes(t *testing.T) {
	tok := &OAuthTokens{
		ExpiresAt: time.Now().Add(5*time.Minute + 10*time.Second).UnixMilli(),
	}
	if tok.IsExpired() {
		t.Error("token expiring in 5m10s should not be expired")
	}
}

// ---------------------------------------------------------------------------
// 20. Logout
// ---------------------------------------------------------------------------

func TestLogout_WithTokens(t *testing.T) {
	setupTempHome(t)

	// Write tokens first.
	if err := WriteTokens(sampleTokens()); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	// Logout should succeed and remove the tokens.
	if err := Logout(); err != nil {
		t.Fatalf("Logout() error: %v", err)
	}

	// Tokens should be gone.
	_, err := ReadTokens()
	if err == nil {
		t.Error("ReadTokens() should fail after Logout()")
	}
}

func TestLogout_NoCredentials(t *testing.T) {
	setupTempHome(t) // empty HOME, no tokens

	// Logout should succeed (prints "No stored credentials found.").
	if err := Logout(); err != nil {
		t.Fatalf("Logout() error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 21. RefreshAccessToken connection error
// ---------------------------------------------------------------------------

func TestRefreshAccessToken_ConnectionError(t *testing.T) {
	setupTempHome(t)

	// Point to an address that will refuse connections.
	origURL := tokenURL
	tokenURL = "http://127.0.0.1:1" // port 1 should refuse
	t.Cleanup(func() { tokenURL = origURL })

	_, err := RefreshAccessToken("some-refresh")
	if err == nil {
		t.Fatal("RefreshAccessToken() should error on connection failure")
	}
	if !strings.Contains(err.Error(), "token refresh request") {
		t.Errorf("error = %q, want it to contain 'token refresh request'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 22. exchangeCode connection error
// ---------------------------------------------------------------------------

func TestExchangeCode_ConnectionError(t *testing.T) {
	origURL := tokenURL
	tokenURL = "http://127.0.0.1:1"
	t.Cleanup(func() { tokenURL = origURL })

	_, err := exchangeCode("code", "verifier")
	if err == nil {
		t.Fatal("exchangeCode() should error on connection failure")
	}
	if !strings.Contains(err.Error(), "token exchange request") {
		t.Errorf("error = %q, want it to contain 'token exchange request'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 23. OAuthTokens JSON marshaling round-trip
// ---------------------------------------------------------------------------

func TestOAuthTokens_JSONRoundTrip(t *testing.T) {
	tok := &OAuthTokens{
		AccessToken:  "at-123",
		RefreshToken: "rt-456",
		ExpiresAt:    1700000000000,
		Scopes:       []string{"a", "b", "c"},
	}

	data, err := json.Marshal(tok)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got OAuthTokens
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if got.AccessToken != tok.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, tok.AccessToken)
	}
	if got.RefreshToken != tok.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, tok.RefreshToken)
	}
	if got.ExpiresAt != tok.ExpiresAt {
		t.Errorf("ExpiresAt = %d, want %d", got.ExpiresAt, tok.ExpiresAt)
	}
	if len(got.Scopes) != len(tok.Scopes) {
		t.Errorf("Scopes length = %d, want %d", len(got.Scopes), len(tok.Scopes))
	}
}

// ---------------------------------------------------------------------------
// 24. Multiple WriteTokens overwrites previous
// ---------------------------------------------------------------------------

func TestWriteTokens_OverwritesPrevious(t *testing.T) {
	setupTempHome(t)

	tok1 := &OAuthTokens{
		AccessToken:  "first",
		RefreshToken: "r1",
		ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
		Scopes:       []string{"a"},
	}
	tok2 := &OAuthTokens{
		AccessToken:  "second",
		RefreshToken: "r2",
		ExpiresAt:    time.Now().Add(2 * time.Hour).UnixMilli(),
		Scopes:       []string{"b", "c"},
	}

	WriteTokens(tok1)
	WriteTokens(tok2)

	got, err := ReadTokens()
	if err != nil {
		t.Fatalf("ReadTokens() error: %v", err)
	}
	if got.AccessToken != "second" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "second")
	}
	if got.RefreshToken != "r2" {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, "r2")
	}
}

// ---------------------------------------------------------------------------
// 25. WriteTokens / RemoveTokens write-error paths
// ---------------------------------------------------------------------------

func TestWriteTokens_WriteError(t *testing.T) {
	tmp := setupTempHome(t)

	// Create .claude.json as a directory so the write fails.
	path := filepath.Join(tmp, ".claude.json")
	os.MkdirAll(path, 0700)

	err := WriteTokens(sampleTokens())
	if err == nil {
		t.Fatal("WriteTokens() should error when path is a directory")
	}
	if !strings.Contains(err.Error(), "write claude.json") {
		t.Errorf("error = %q, want it to contain 'write claude.json'", err.Error())
	}
}

func TestRemoveTokens_WriteError(t *testing.T) {
	tmp := setupTempHome(t)

	// Write valid tokens first.
	if err := WriteTokens(sampleTokens()); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	// Make the file read-only so the rewrite fails.
	path := filepath.Join(tmp, ".claude.json")
	os.Chmod(path, 0400)
	t.Cleanup(func() { os.Chmod(path, 0600) })

	_, err := RemoveTokens()
	if err == nil {
		t.Fatal("RemoveTokens() should error when file is read-only")
	}
}

// ---------------------------------------------------------------------------
// 26. ExchangeCode validates ExpiresAt is in the future
// ---------------------------------------------------------------------------

func TestExchangeCode_ExpiresAtInFuture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at",
			"refresh_token": "rt",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"scope":         "user:read",
		})
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	tokens, err := exchangeCode("code", "verifier")
	if err != nil {
		t.Fatalf("exchangeCode() error: %v", err)
	}

	// ExpiresAt should be roughly now + 3600 seconds.
	exp := time.UnixMilli(tokens.ExpiresAt)
	diff := time.Until(exp)
	if diff < 3500*time.Second || diff > 3700*time.Second {
		t.Errorf("ExpiresAt is %v from now, want ~3600s", diff)
	}
}

// ---------------------------------------------------------------------------
// 27. Resolve returns OAuthCredentials with correct type
// ---------------------------------------------------------------------------

func TestResolve_OAuthCredentials_InterfaceSatisfaction(t *testing.T) {
	setupTempHome(t)
	t.Setenv("ANTHROPIC_API_KEY", "")

	WriteTokens(sampleTokens())

	cred, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	// Verify it satisfies the Credentials interface.
	var _ Credentials = cred

	if !cred.IsOAuth() {
		t.Error("OAuth credentials should return IsOAuth() = true")
	}
}

// ---------------------------------------------------------------------------
// 28. RefreshAccessToken lock failure fallback: fresh tokens
// ---------------------------------------------------------------------------

func TestRefreshAccessToken_LockFail_FreshTokensFallback(t *testing.T) {
	tmp := setupTempHome(t)

	// Write fresh tokens so the fallback re-read succeeds.
	fresh := &OAuthTokens{
		AccessToken:  "fallback-access",
		RefreshToken: "fallback-refresh",
		ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
		Scopes:       []string{"user:read"},
	}
	if err := WriteTokens(fresh); err != nil {
		t.Fatalf("WriteTokens() error: %v", err)
	}

	// Make the .claude directory read-only so AcquireLock fails immediately
	// with EACCES (not os.IsExist), avoiding retry backoff.
	lockDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(lockDir, 0700)
	os.Chmod(lockDir, 0500)
	t.Cleanup(func() { os.Chmod(lockDir, 0700) })

	tokens, err := RefreshAccessToken("old-refresh")
	if err != nil {
		t.Fatalf("RefreshAccessToken() should fallback to fresh tokens, got error: %v", err)
	}
	if tokens.AccessToken != "fallback-access" {
		t.Errorf("AccessToken = %q, want %q", tokens.AccessToken, "fallback-access")
	}
}

// ---------------------------------------------------------------------------
// 29. RefreshAccessToken lock failure fallback: no fresh tokens
// ---------------------------------------------------------------------------

func TestRefreshAccessToken_LockFail_NoFreshTokens(t *testing.T) {
	tmp := setupTempHome(t)

	// Make the .claude directory read-only so AcquireLock fails immediately.
	lockDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(lockDir, 0700)
	os.Chmod(lockDir, 0500)
	t.Cleanup(func() { os.Chmod(lockDir, 0700) })

	// No tokens written -- fallback re-read will also fail.
	_, err := RefreshAccessToken("some-refresh")
	if err == nil {
		t.Fatal("RefreshAccessToken() should error when lock fails and no stored tokens")
	}
	if !strings.Contains(err.Error(), "acquire lock") {
		t.Errorf("error = %q, want it to contain 'acquire lock'", err.Error())
	}
	if !strings.Contains(err.Error(), "re-read also failed") {
		t.Errorf("error = %q, want it to contain 're-read also failed'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 30. AcquireLock non-EEXIST error
// ---------------------------------------------------------------------------

func TestAcquireLock_PermissionDenied(t *testing.T) {
	tmp := setupTempHome(t)

	// Create the .claude directory but make it unwritable so OpenFile fails
	// with EACCES, which is not os.IsExist -- this hits the non-exist error path.
	lockDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(lockDir, 0700)
	os.Chmod(lockDir, 0500)
	t.Cleanup(func() { os.Chmod(lockDir, 0700) })

	_, err := AcquireLock()
	if err == nil {
		t.Fatal("AcquireLock() should fail with permission denied")
	}
	if !strings.Contains(err.Error(), "open lock file") {
		t.Errorf("error = %q, want it to contain 'open lock file'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 31. RefreshAccessToken store failure
// ---------------------------------------------------------------------------

func TestRefreshAccessToken_StoreFailure(t *testing.T) {
	tmp := setupTempHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-at",
			"refresh_token": "new-rt",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"scope":         "user:read",
		})
	}))
	defer srv.Close()

	origURL := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = origURL })

	// Make .claude.json a directory so WriteTokens fails.
	path := filepath.Join(tmp, ".claude.json")
	os.MkdirAll(path, 0700)

	_, err := RefreshAccessToken("refresh-tok")
	if err == nil {
		t.Fatal("RefreshAccessToken() should error when WriteTokens fails")
	}
	if !strings.Contains(err.Error(), "store refreshed tokens") {
		t.Errorf("error = %q, want it to contain 'store refreshed tokens'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 32. Logout error when RemoveTokens fails
// ---------------------------------------------------------------------------

func TestLogout_RemoveTokensError(t *testing.T) {
	tmp := setupTempHome(t)

	// Write invalid JSON so RemoveTokens fails on parse.
	path := filepath.Join(tmp, ".claude.json")
	os.WriteFile(path, []byte("invalid json"), 0600)

	err := Logout()
	if err == nil {
		t.Fatal("Logout() should error when RemoveTokens fails")
	}
	if !strings.Contains(err.Error(), "remove tokens from file") {
		t.Errorf("error = %q, want it to contain 'remove tokens from file'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 33. AcquireLock MkdirAll failure
// ---------------------------------------------------------------------------

func TestAcquireLock_MkdirAllError(t *testing.T) {
	tmp := setupTempHome(t)

	// Create .claude as a file (not directory) so MkdirAll fails
	// when trying to create .claude/.auth-lock's parent.
	claudePath := filepath.Join(tmp, ".claude")
	os.WriteFile(claudePath, []byte("blocker"), 0600)

	_, err := AcquireLock()
	if err == nil {
		t.Fatal("AcquireLock() should fail when .claude is a file")
	}
	if !strings.Contains(err.Error(), "create lock dir") {
		t.Errorf("error = %q, want it to contain 'create lock dir'", err.Error())
	}
}

// Ensure fmt import is used.
var _ = fmt.Sprintf
