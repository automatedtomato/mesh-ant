// client.go defines the LLMClient interface and AnthropicClient implementation.
//
// The single-method interface makes the LLM's analytical boundary explicit:
// what enters, what exits — internal transformations are not visible (T2 in
// docs/decisions/llm-as-mediator-v1.md). No external SDK; zero external dependencies.
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

// LLMClient is the interface for LLM completion calls. Tests inject a mock;
// production code uses AnthropicClient.
type LLMClient interface {
	Complete(ctx context.Context, system, prompt string) (string, error)
}

// httpTimeout is generous for large extraction responses while bounding network hangs.
const httpTimeout = 180 * time.Second

// maxResponseBytes caps the Anthropic API response body to bound memory use.
const maxResponseBytes = 8 * 1024 * 1024

// AnthropicClient implements LLMClient using the Anthropic Messages API.
// The API key is held unexported and never serialised, logged, or returned.
type AnthropicClient struct {
	apiKey     string       // unexported: never in SessionRecord or any output
	model      string
	baseURL    string
	httpClient *http.Client // private client with explicit timeout; never http.DefaultClient
}

// NewAnthropicClient constructs an AnthropicClient, reading the API key from
// MESHANT_LLM_API_KEY (falling back to ANTHROPIC_API_KEY). Errors if both are absent.
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

// Complete sends a Messages API request and returns the text of the first
// content block. system is the system prompt; prompt is the user message.
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

	// Cap the response body to bound memory use on malformed/oversized responses.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("llm: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Omit body on auth failures — it may echo sensitive request details.
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("llm: API authentication error %d (check MESHANT_LLM_API_KEY)", resp.StatusCode)
		}
		return "", fmt.Errorf("llm: API error %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

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

// truncate returns s truncated to at most n bytes with "..." appended on truncation.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}
