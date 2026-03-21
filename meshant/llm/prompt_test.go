package llm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
)

func TestLoadPromptTemplate_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	content := "# Extraction Pass\n\nYou are a mediating instrument."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got, err := llm.LoadPromptTemplate(path)
	if err != nil {
		t.Fatalf("want no error, got: %v", err)
	}
	if got != content {
		t.Errorf("want %q, got %q", content, got)
	}
}

func TestLoadPromptTemplate_Missing(t *testing.T) {
	_, err := llm.LoadPromptTemplate("/nonexistent/path/prompt.md")
	if err == nil {
		t.Fatal("want error for missing file, got nil")
	}
}

func TestLoadPromptTemplate_TooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.md")
	// Write one byte beyond the 1 MiB cap.
	large := make([]byte, 1*1024*1024+1)
	for i := range large {
		large[i] = 'x'
	}
	if err := os.WriteFile(path, large, 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	_, err := llm.LoadPromptTemplate(path)
	if err == nil {
		t.Fatal("want error for oversized prompt template, got nil")
	}
}

func TestLoadPromptTemplate_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got, err := llm.LoadPromptTemplate(path)
	if err != nil {
		t.Fatalf("empty file should not error, got: %v", err)
	}
	if got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}
