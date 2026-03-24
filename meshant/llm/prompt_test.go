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

// --- HashPromptTemplate ---

// expectedHashFor computes the SHA-256 of content independently — it does NOT
// call HashPromptTemplate so tests are not circular.
func expectedHashFor(t *testing.T, content string) string {
	t.Helper()
	// Pre-computed via: python3 -c "import hashlib; print(hashlib.sha256(b'...').hexdigest()[:16])"
	// Used inline in tests that pin the exact hex value; this helper is for
	// tests that only need to check structural properties (length, hex chars).
	return content // unused body — tests use the precomputed constant directly
}

// knownTemplateContent is the string written by writePromptTemplate in extract_test.go.
// Pre-computed SHA-256: python3 -c "import hashlib; print(hashlib.sha256(b'Extract trace drafts.').hexdigest()[:16])"
// Result: 77b3b25b097865b4
const knownTemplateContent = "Extract trace drafts."
const knownTemplateHash = "77b3b25b097865b4"

// TestHashPromptTemplate_KnownContent verifies the hash of the canonical test
// template content used across the test suite.
func TestHashPromptTemplate_KnownContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(path, []byte(knownTemplateContent), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := llm.HashPromptTemplate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != knownTemplateHash {
		t.Errorf("want %q, got %q", knownTemplateHash, got)
	}
}

// TestHashPromptTemplate_EmptyPath verifies that an empty path returns ("", nil) —
// valid for assist sessions that have no prompt template file.
func TestHashPromptTemplate_EmptyPath(t *testing.T) {
	got, err := llm.HashPromptTemplate("")
	if err != nil {
		t.Fatalf("want nil error for empty path, got: %v", err)
	}
	if got != "" {
		t.Errorf("want empty string for empty path, got %q", got)
	}
}

// TestHashPromptTemplate_Missing verifies that a non-existent path returns an error.
func TestHashPromptTemplate_Missing(t *testing.T) {
	_, err := llm.HashPromptTemplate("/nonexistent/path/prompt.md")
	if err == nil {
		t.Fatal("want error for missing file, got nil")
	}
}

// TestHashPromptTemplate_TooLarge verifies that a file exceeding maxSourceBytes
// returns an error, matching the behavior of LoadPromptTemplate.
func TestHashPromptTemplate_TooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.md")
	// One byte beyond the 1 MiB cap.
	large := make([]byte, 1*1024*1024+1)
	for i := range large {
		large[i] = 'x'
	}
	if err := os.WriteFile(path, large, 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	_, err := llm.HashPromptTemplate(path)
	if err == nil {
		t.Fatal("want error for oversized file, got nil")
	}
}

// TestHashPromptTemplate_EmptyFile verifies that an empty (0-byte) file returns
// the 16-char SHA-256 of the empty byte string — not an error.
func TestHashPromptTemplate_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := llm.HashPromptTemplate(path)
	if err != nil {
		t.Fatalf("empty file should not error, got: %v", err)
	}
	if len(got) != 16 {
		t.Errorf("want 16-char hex for empty file, got %q (len=%d)", got, len(got))
	}
	// All characters must be valid lowercase hex digits.
	for _, c := range got {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character %q in hash %q", c, got)
		}
	}
}

// TestHashPromptTemplate_DifferentContent_DifferentHash verifies that two files
// with different contents produce different hashes.
func TestHashPromptTemplate_DifferentContent_DifferentHash(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.md")
	p2 := filepath.Join(dir, "b.md")
	if err := os.WriteFile(p1, []byte("content alpha"), 0o644); err != nil {
		t.Fatalf("write p1: %v", err)
	}
	if err := os.WriteFile(p2, []byte("content beta"), 0o644); err != nil {
		t.Fatalf("write p2: %v", err)
	}

	h1, err := llm.HashPromptTemplate(p1)
	if err != nil {
		t.Fatalf("hash p1: %v", err)
	}
	h2, err := llm.HashPromptTemplate(p2)
	if err != nil {
		t.Fatalf("hash p2: %v", err)
	}
	if h1 == h2 {
		t.Errorf("different contents produced the same hash %q", h1)
	}
}

// TestHashPromptTemplate_SameContent_SameHash verifies that two files with
// identical contents always produce the same hash.
func TestHashPromptTemplate_SameContent_SameHash(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "copy1.md")
	p2 := filepath.Join(dir, "copy2.md")
	content := []byte("same content for both files")
	if err := os.WriteFile(p1, content, 0o644); err != nil {
		t.Fatalf("write p1: %v", err)
	}
	if err := os.WriteFile(p2, content, 0o644); err != nil {
		t.Fatalf("write p2: %v", err)
	}

	h1, err := llm.HashPromptTemplate(p1)
	if err != nil {
		t.Fatalf("hash p1: %v", err)
	}
	h2, err := llm.HashPromptTemplate(p2)
	if err != nil {
		t.Fatalf("hash p2: %v", err)
	}
	if h1 != h2 {
		t.Errorf("same content produced different hashes: %q vs %q", h1, h2)
	}
}
