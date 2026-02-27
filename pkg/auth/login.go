package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	oauthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	authURL       = "https://claude.ai/oauth/authorize"
	oauthScopes   = "user:profile user:inference user:sessions:claude_code user:mcp_servers"
)

// Login runs the OAuth 2.0 PKCE login flow: opens browser, prompts for code,
// exchanges it for tokens, and stores them.
func Login() error {
	pkce, err := GeneratePKCE()
	if err != nil {
		return fmt.Errorf("generate PKCE: %w", err)
	}

	// Build the authorization URL.
	params := url.Values{
		"client_id":             {oauthClientID},
		"response_type":        {"code"},
		"redirect_uri":         {"https://platform.claude.com/oauth/code/callback"},
		"scope":                {oauthScopes},
		"code_challenge":       {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}

	authorizationURL := authURL + "?" + params.Encode()

	// Open the browser.
	fmt.Println("Opening browser to authenticate with Claude...")
	if err := openBrowser(authorizationURL); err != nil {
		fmt.Printf("Could not open browser. Please visit:\n%s\n\n", authorizationURL)
	}

	// Prompt for the authorization code.
	fmt.Print("Paste the authorization code here: ")
	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return fmt.Errorf("read authorization code: %w", err)
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("empty authorization code")
	}

	// Exchange the code for tokens.
	tokens, err := exchangeCode(code, pkce.Verifier)
	if err != nil {
		return err
	}

	// Store tokens in ~/.claude.json.
	if err := WriteTokens(tokens); err != nil {
		return fmt.Errorf("store tokens: %w", err)
	}

	// Best-effort keychain storage.
	_ = WriteKeychain(tokens)

	fmt.Println("Logged in successfully.")
	return nil
}

// exchangeCode exchanges an authorization code for OAuth tokens.
func exchangeCode(code, verifier string) (*OAuthTokens, error) {
	resp, err := http.PostForm(tokenURL, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {oauthClientID},
		"code_verifier": {verifier},
		"redirect_uri":  {"https://platform.claude.com/oauth/code/callback"},
	})
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read exchange response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokResp tokenResponse
	if err := json.Unmarshal(body, &tokResp); err != nil {
		return nil, fmt.Errorf("parse exchange response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokResp.ExpiresIn) * time.Second)

	return &OAuthTokens{
		AccessToken:  tokResp.AccessToken,
		RefreshToken: tokResp.RefreshToken,
		ExpiresAt:    expiresAt.UnixMilli(),
		Scopes:       strings.Fields(tokResp.Scope),
	}, nil
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
