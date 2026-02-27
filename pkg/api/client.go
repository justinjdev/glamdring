package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/justin/glamdring/pkg/auth"
)

const (
	defaultEndpoint   = "https://api.anthropic.com/v1/messages"
	anthropicVersion  = "2023-06-01"
)

// Client is the HTTP client for the Claude Messages API.
type Client struct {
	creds      auth.Credentials
	model      string
	httpClient *http.Client
	endpoint   string
}

// NewClient creates a new API client for the given credentials and model.
func NewClient(creds auth.Credentials, model string) *Client {
	return &Client{
		creds:      creds,
		model:      model,
		httpClient: &http.Client{},
		endpoint:   defaultEndpoint,
	}
}

// Model returns the model this client is configured to use.
func (c *Client) Model() string {
	return c.model
}

// SetEndpoint overrides the API endpoint URL. This is intended for testing
// with httptest servers.
func (c *Client) SetEndpoint(url string) {
	c.endpoint = url
}

// supportsThinking returns true if the model supports adaptive thinking.
func (c *Client) supportsThinking() bool {
	m := strings.ToLower(c.model)
	return strings.Contains(m, "opus-4") || strings.Contains(m, "sonnet-4")
}

// Stream sends a MessageRequest to the API with streaming enabled and returns
// a channel of StreamEvents. The channel is closed when the stream completes
// or an error occurs. Cancelling ctx will abort the request.
func (c *Client) Stream(ctx context.Context, req *MessageRequest) (<-chan StreamEvent, error) {
	// Force streaming and set model.
	req.Stream = true
	req.Model = c.model

	// Enable adaptive thinking for supported models.
	if c.supportsThinking() && req.Thinking == nil {
		req.Thinking = &ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 10000,
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, body)
	if err != nil {
		return nil, err
	}

	// Set up a done channel that closes when the context is cancelled.
	// This signals the SSE parser goroutine to stop.
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
		// Close the response body to unblock the scanner.
		resp.Body.Close()
	}()

	events := parseSSE(resp.Body, done)

	// Wrap in a goroutine that ensures the body is closed when the stream ends.
	out := make(chan StreamEvent, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		for ev := range events {
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// doWithRetry executes the HTTP request with retry logic for retryable errors.
func (c *Client) doWithRetry(ctx context.Context, body []byte) (*http.Response, error) {
	var lastErr error

	for attempt := range maxRetries + 1 {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if err := c.creds.SetAuthHeaders(req); err != nil {
			return nil, fmt.Errorf("set auth headers: %w", err)
		}
		req.Header.Set("anthropic-version", anthropicVersion)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network errors are retryable.
			lastErr = fmt.Errorf("http request: %w", err)
			if attempt < maxRetries {
				if sleepErr := sleepWithContext(ctx, backoffDelay(attempt)); sleepErr != nil {
					return nil, sleepErr
				}
			}
			continue
		}

		// Success.
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		// Read the error body for the error message.
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// OAuth 401 handling: refresh token and retry exactly once.
		if resp.StatusCode == http.StatusUnauthorized && c.creds.IsOAuth() {
			oauthCreds := c.creds.(*auth.OAuthCredentials)
			if err := oauthCreds.Refresh(); err != nil {
				return nil, parseAPIError(resp.StatusCode, errBody)
			}

			// Retry the request with refreshed credentials.
			retryReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
			if err != nil {
				return nil, fmt.Errorf("create retry request: %w", err)
			}
			retryReq.Header.Set("Content-Type", "application/json")
			if err := c.creds.SetAuthHeaders(retryReq); err != nil {
				return nil, fmt.Errorf("set auth headers on retry: %w", err)
			}
			retryReq.Header.Set("anthropic-version", anthropicVersion)

			retryResp, err := c.httpClient.Do(retryReq)
			if err != nil {
				return nil, fmt.Errorf("http request after token refresh: %w", err)
			}
			if retryResp.StatusCode == http.StatusOK {
				return retryResp, nil
			}
			retryBody, _ := io.ReadAll(retryResp.Body)
			retryResp.Body.Close()
			return nil, parseAPIError(retryResp.StatusCode, retryBody)
		}

		// Non-retryable error — return immediately.
		if !shouldRetry(resp.StatusCode) {
			apiErr := parseAPIError(resp.StatusCode, errBody)
			return nil, apiErr
		}

		// Retryable error.
		lastErr = parseAPIError(resp.StatusCode, errBody)
		if attempt < maxRetries {
			delay := backoffDelay(attempt)
			// Prefer retry-after header for 429s.
			if resp.StatusCode == http.StatusTooManyRequests {
				if ra := retryAfterDelay(resp); ra > 0 {
					delay = ra
				}
			}
			if sleepErr := sleepWithContext(ctx, delay); sleepErr != nil {
				return nil, sleepErr
			}
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// parseAPIError extracts a structured error from the API response body.
func parseAPIError(statusCode int, body []byte) *APIError {
	var parsed struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		return &APIError{
			StatusCode: statusCode,
			Type:       parsed.Error.Type,
			Message:    parsed.Error.Message,
		}
	}
	return &APIError{
		StatusCode: statusCode,
		Type:       "unknown",
		Message:    fmt.Sprintf("API error %d: %s", statusCode, string(body)),
	}
}
