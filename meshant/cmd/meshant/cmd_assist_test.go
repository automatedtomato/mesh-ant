// cmd_assist_test.go tests the cmdAssist CLI handler (package main, white-box).
//
// All LLM calls are intercepted by a mock client. No real API calls are made.
// Interactive I/O is injected via strings.NewReader and bytes.Buffer.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// assistMockClientCmd implements llm.LLMClient for cmdAssist tests.
// It returns a preset response on every Complete call.
type assistMockClientCmd struct {
	responses []string
	calls     int
	err       error
}

func (m *assistMockClientCmd) Complete(_ context.Context, _, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if len(m.responses) == 0 {
		return `[{"source_span":"fallback"}]`, nil
	}
	idx := m.calls
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.calls++
	return m.responses[idx], nil
}

// writeAssistSpansFile writes a newline-separated spans file and returns its path.
func writeAssistSpansFile(t *testing.T, spans []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.txt")
	content := strings.Join(spans, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeAssistSpansFile: %v", err)
	}
	return path
}

// writeAssistPromptTemplate writes a minimal prompt template and returns its path.
func writeAssistPromptTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "assist_prompt.md")
	if err := os.WriteFile(path, []byte("Produce one TraceDraft."), 0o644); err != nil {
		t.Fatalf("writeAssistPromptTemplate: %v", err)
	}
	return path
}

// --- Group: cmdAssist ---

// TestCmdAssist_HappyPath verifies that cmdAssist accepts a spans file,
// calls the LLM, presents the draft, accepts the user's "a" response, and
// writes output to a file.
func TestCmdAssist_HappyPath(t *testing.T) {
	spansPath := writeAssistSpansFile(t, []string{"The service went down at 09:00."})
	promptPath := writeAssistPromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")

	client := &assistMockClientCmd{responses: []string{`[{"source_span":"The service went down at 09:00."}]`}}
	var w bytes.Buffer
	// user accepts the draft
	in := strings.NewReader("a\n")
	err := cmdAssist(&w, client, in, []string{
		"--spans-file", spansPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
	})
	if err != nil {
		t.Fatalf("cmdAssist: unexpected error: %v", err)
	}

	// Output file must be valid JSON containing at least one TraceDraft with
	// the correct provenance fields surviving the JSON round-trip.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("parse output JSON: %v", err)
	}
	if len(drafts) == 0 {
		t.Fatal("output must contain at least one draft")
	}
	d := drafts[0]
	if d.ExtractionStage != "weak-draft" {
		t.Errorf("ExtractionStage: want %q, got %q", "weak-draft", d.ExtractionStage)
	}
	if d.ExtractedBy != "test-model" {
		t.Errorf("ExtractedBy: want %q, got %q", "test-model", d.ExtractedBy)
	}
	if d.UncertaintyNote == "" {
		t.Error("UncertaintyNote must not be empty (framework note always appended)")
	}
}

// TestCmdAssist_NoSpansFlag verifies that omitting --spans-file returns an error.
func TestCmdAssist_NoSpansFlag(t *testing.T) {
	var w bytes.Buffer
	err := cmdAssist(&w, nil, strings.NewReader(""), []string{})
	if err == nil {
		t.Fatal("want error when --spans-file is omitted, got nil")
	}
}

// TestCmdAssist_MissingSpansFile verifies that a non-existent spans file
// returns an error.
func TestCmdAssist_MissingSpansFile(t *testing.T) {
	var w bytes.Buffer
	err := cmdAssist(&w, nil, strings.NewReader(""), []string{
		"--spans-file", "/nonexistent/spans.txt",
	})
	if err == nil {
		t.Fatal("want error for missing spans file, got nil")
	}
}

// TestCmdAssist_SessionFileWritten verifies that a session JSON file is written
// alongside the output when --session-output is provided.
func TestCmdAssist_SessionFileWritten(t *testing.T) {
	spansPath := writeAssistSpansFile(t, []string{"session test span"})
	promptPath := writeAssistPromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")
	sessPath := filepath.Join(outDir, "session.json")

	client := &assistMockClientCmd{responses: []string{`[{"source_span":"session test span"}]`}}
	in := strings.NewReader("a\n")
	var w bytes.Buffer
	err := cmdAssist(&w, client, in, []string{
		"--spans-file", spansPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--session-output", sessPath,
	})
	if err != nil {
		t.Fatalf("cmdAssist: unexpected error: %v", err)
	}

	// Session file must exist and decode as a SessionRecord.
	data, err := os.ReadFile(sessPath)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("parse session JSON: %v", err)
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must not be empty")
	}
	if rec.Command != "assist" {
		t.Errorf("Command: want %q, got %q", "assist", rec.Command)
	}
}

// TestCmdAssist_QuitOutputsPartialResults verifies that quitting the session
// immediately writes 0 drafts to the output file (partial results acceptable).
func TestCmdAssist_QuitOutputsPartialResults(t *testing.T) {
	spansPath := writeAssistSpansFile(t, []string{"span-quit-1", "span-quit-2"})
	promptPath := writeAssistPromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")

	client := &assistMockClientCmd{
		responses: []string{
			`[{"source_span":"span-quit-1"}]`,
			`[{"source_span":"span-quit-2"}]`,
		},
	}
	// quit immediately before accepting anything
	in := strings.NewReader("q\n")
	var w bytes.Buffer
	err := cmdAssist(&w, client, in, []string{
		"--spans-file", spansPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
	})
	if err != nil {
		t.Fatalf("cmdAssist: unexpected error on quit: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("parse output JSON: %v", err)
	}
	// Quit before accepting anything: 0 drafts in output.
	// The LLM is called for the first span before the prompt; quitting on the
	// first prompt means no accepted/edited/skipped drafts — just the quit.
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts after immediate quit, got %d", len(drafts))
	}
}

// TestCmdAssist_LLMError verifies that an LLM error causes cmdAssist to return
// a non-nil error. The session record is still written even on LLM error.
func TestCmdAssist_LLMError(t *testing.T) {
	spansPath := writeAssistSpansFile(t, []string{"error span"})
	promptPath := writeAssistPromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")
	sessPath := filepath.Join(outDir, "session.json")

	client := &assistMockClientCmd{err: errors.New("simulated API failure")}
	in := strings.NewReader("a\n")
	var w bytes.Buffer
	err := cmdAssist(&w, client, in, []string{
		"--spans-file", spansPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--session-output", sessPath,
	})
	if err == nil {
		t.Fatal("want error from LLM failure, got nil")
	}
	// Session file must still be written (always-write-on-error convention).
	if _, statErr := os.Stat(sessPath); statErr != nil {
		t.Errorf("session file not written on LLM error: %v", statErr)
	}
}
