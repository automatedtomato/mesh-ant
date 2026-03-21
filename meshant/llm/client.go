// client.go defines the LLMClient interface and AnthropicClient implementation.
//
// The interface has a single method — Complete — that sends system instructions
// and a user prompt and returns the LLM's text response. The single-method
// design makes the LLM's analytical boundary explicit: what enters, what exits.
// The LLM's internal transformations are not visible through this boundary
// (T2 in docs/decisions/llm-as-mediator-v1.md).
//
// AnthropicClient implements LLMClient using the Anthropic Messages API via
// net/http. No external SDK is used; the project maintains zero external
// dependencies. The API key is consumed at construction time and never exposed
// through any exported field or method.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LLMClient is the interface for LLM completion calls. Implementing this
// interface is sufficient for all llm package operations; tests inject a
// mock, production code uses AnthropicClient.
type LLMClient interface {
	Complete(ctx context.Context, system, prompt string) (string, error)
}

// httpTimeout is the end-to-end timeout for a single Anthropic API call.
// Large extraction responses (max_tokens: 4096) can be slow; 180 s is generous
// without allowing a network hang to block indefinitely.
const httpTimeout = 180 * time.Second

// maxResponseBytes caps the response body read from the Anthropic API.
// 8 MiB comfortably exceeds the maximum expected response size for a
// 4096-token completion while bounding memory use if the server misbehaves.
const maxResponseBytes = 8 * 1024 * 1024

// AnthropicClient implements LLMClient using the Anthropic Messages API.
// The API key is held in an unexported field and is never serialised,
// logged, or returned through any exported method.
type AnthropicClient struct {
	apiKey     string       // unexported: never in SessionRecord or any output
	model      string
	baseURL    string
	httpClient *http.Client // private client with explicit timeout; never http.DefaultClient
}

// NewAnthropicClient constructs a client ready to call the Anthropic API.
// It reads the API key from the environment: MESHANT_LLM_API_KEY is checked
// first; ANTHROPIC_API_KEY is used as a fallback. Returns a descriptive
// error if both are absent or empty.
func NewAnthropicClient(model string) (*AnthropicClient, error) {
	key := os.Getenv("MESHANT_LLM_API_KEY")
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("llm: API key required — set MESHANT_LLM_API_KEY (or ANTHROPIC_API_KEY as fallback)")
	}
	return &AnthropicClient{
		apiKey:     key,
		model:      model,
		baseURL:    "https://api.anthropic.com",
		httpClient: &http.Client{Timeout: httpTimeout},
	}, nil
}

// Complete sends a Messages API request to Anthropic and returns the text
// content of the first content block in the response. It uses the model
// specified at construction time.
//
// The system parameter is sent as the system prompt (extraction instructions);
// the prompt parameter is sent as the single user message (source document).
func (c *AnthropicClient) Complete(ctx context.Context, system, prompt string) (string, error) {
	reqBody, err := json.Marshal(map[string]any{
		"model":      c.model,
		"max_tokens": 4096,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("llm: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: send request: %w", err)
	}
	defer resp.Body.Close()

	// Cap the response body to prevent unbounded memory use if the server
	// sends an unexpectedly large or malformed response.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("llm: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Do not include the response body for authentication failures: the
		// body may echo sensitive request details. For other error codes the
		// truncated body is useful diagnostic text.
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("llm: API authentication error %d (check MESHANT_LLM_API_KEY)", resp.StatusCode)
		}
		return "", fmt.Errorf("llm: API error %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	// Parse the Messages API response envelope.
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("llm: parse API response: %w", err)
	}
	for _, block := range result.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", nil
}

// truncate returns s truncated to at most n bytes, with "..." appended if
// truncation occurred. Used to limit error message lengths.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}
