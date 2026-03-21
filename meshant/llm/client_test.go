package llm_test

import (
	"os"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
)

// clearLLMEnv unsets both API key env vars and returns a restore function.
func clearLLMEnv(t *testing.T) func() {
	t.Helper()
	prev1 := os.Getenv("MESHANT_LLM_API_KEY")
	prev2 := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("MESHANT_LLM_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	return func() {
		os.Setenv("MESHANT_LLM_API_KEY", prev1)
		os.Setenv("ANTHROPIC_API_KEY", prev2)
	}
}

func TestNewAnthropicClient_MissingKey(t *testing.T) {
	restore := clearLLMEnv(t)
	defer restore()

	_, err := llm.NewAnthropicClient("claude-sonnet-4-6")
	if err == nil {
		t.Fatal("want error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "MESHANT_LLM_API_KEY") {
		t.Errorf("error should name MESHANT_LLM_API_KEY, got: %v", err)
	}
}

func TestNewAnthropicClient_EmptyKey(t *testing.T) {
	restore := clearLLMEnv(t)
	defer restore()
	os.Setenv("MESHANT_LLM_API_KEY", "")

	_, err := llm.NewAnthropicClient("claude-sonnet-4-6")
	if err == nil {
		t.Fatal("want error for empty API key, got nil")
	}
	if !strings.Contains(err.Error(), "MESHANT_LLM_API_KEY") {
		t.Errorf("error should name MESHANT_LLM_API_KEY, got: %v", err)
	}
}

func TestNewAnthropicClient_PrimaryKey(t *testing.T) {
	restore := clearLLMEnv(t)
	defer restore()
	os.Setenv("MESHANT_LLM_API_KEY", "test-key-primary")

	c, err := llm.NewAnthropicClient("claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("want no error with primary key, got: %v", err)
	}
	if c == nil {
		t.Fatal("want non-nil client")
	}
}

func TestNewAnthropicClient_FallbackKey(t *testing.T) {
	restore := clearLLMEnv(t)
	defer restore()
	// Primary key absent; fallback should be used.
	os.Setenv("ANTHROPIC_API_KEY", "test-key-fallback")

	c, err := llm.NewAnthropicClient("claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("want no error with fallback key, got: %v", err)
	}
	if c == nil {
		t.Fatal("want non-nil client")
	}
}

func TestNewAnthropicClient_PrimaryTakesPrecedence(t *testing.T) {
	restore := clearLLMEnv(t)
	defer restore()
	os.Setenv("MESHANT_LLM_API_KEY", "primary-key")
	os.Setenv("ANTHROPIC_API_KEY", "fallback-key")

	// Both set — should succeed (primary wins, but we can only observe
	// that construction succeeds, not which key was used).
	_, err := llm.NewAnthropicClient("claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("want no error when both keys set, got: %v", err)
	}
}
