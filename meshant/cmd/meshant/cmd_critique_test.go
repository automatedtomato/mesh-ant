// cmd_critique_test.go tests the cmdCritique CLI handler (package main, white-box).
//
// All LLM calls are intercepted by a mock client. No real API calls are made.
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

// critiqueMockClient implements llm.LLMClient for cmdCritique tests.
type critiqueMockClient struct {
	responses []string
	calls     int
	err       error
}

func (m *critiqueMockClient) Complete(_ context.Context, _, _ string) (string, error) {
	if m.err != nil {
		m.calls++
		return "", m.err
	}
	if len(m.responses) == 0 {
		m.calls++
		return `{"source_span":"fallback span"}`, nil
	}
	idx := m.calls
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.calls++
	return m.responses[idx], nil
}

// writeDraftsFile writes a TraceDraft JSON array to a temp file and returns its path.
func writeDraftsFile(t *testing.T, drafts []schema.TraceDraft) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "drafts.json")
	data, err := json.Marshal(drafts)
	if err != nil {
		t.Fatalf("writeDraftsFile: marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeDraftsFile: write: %v", err)
	}
	return path
}

// writeCritiquePromptTemplate writes a minimal prompt template and returns its path.
func writeCritiquePromptTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "critique_prompt.md")
	if err := os.WriteFile(path, []byte("Produce one critique draft."), 0o644); err != nil {
		t.Fatalf("writeCritiquePromptTemplate: %v", err)
	}
	return path
}

// minimalDraft returns a minimal TraceDraft suitable for critique tests.
func minimalDraft(id, sourceSpan string) schema.TraceDraft {
	return schema.TraceDraft{
		ID:              id,
		SourceSpan:      sourceSpan,
		WhatChanged:     "something changed",
		ExtractionStage: "weak-draft",
		ExtractedBy:     "claude-sonnet-4-6",
	}
}

// --- Group: cmdCritique ---

// TestCmdCritique_HappyPath verifies cmdCritique reads a drafts file, calls
// the LLM, writes output with correct provenance fields (ExtractionStage,
// DerivedFrom, UncertaintyNote).
func TestCmdCritique_HappyPath(t *testing.T) {
	orig := minimalDraft("orig-001", "The API failed.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{orig})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")

	response := `{"source_span":"The API failed.","what_changed":"a condition was recorded"}`
	client := &critiqueMockClient{responses: []string{response}}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
	})
	if err != nil {
		t.Fatalf("cmdCritique: unexpected error: %v", err)
	}

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
	if d.ExtractionStage != "critiqued" {
		t.Errorf("ExtractionStage: want %q, got %q", "critiqued", d.ExtractionStage)
	}
	if d.DerivedFrom != orig.ID {
		t.Errorf("DerivedFrom: want %q, got %q", orig.ID, d.DerivedFrom)
	}
	if !strings.Contains(d.UncertaintyNote, "LLM-produced candidate") {
		t.Errorf("UncertaintyNote: framework note not present, got %q", d.UncertaintyNote)
	}
	if d.ExtractedBy != "test-model" {
		t.Errorf("ExtractedBy: want %q, got %q", "test-model", d.ExtractedBy)
	}
}

// TestCmdCritique_MissingInput verifies that omitting --input returns an error.
func TestCmdCritique_MissingInput(t *testing.T) {
	var w bytes.Buffer
	err := cmdCritique(&w, nil, []string{})
	if err == nil {
		t.Fatal("want error when --input is omitted, got nil")
	}
}

// TestCmdCritique_MissingInputFile verifies that a non-existent input file
// returns an error.
func TestCmdCritique_MissingInputFile(t *testing.T) {
	var w bytes.Buffer
	err := cmdCritique(&w, nil, []string{
		"--input", "/nonexistent/drafts.json",
	})
	if err == nil {
		t.Fatal("want error for missing input file, got nil")
	}
}

// TestCmdCritique_SessionFileWritten verifies that a session JSON file is
// written alongside the output when --session-output is provided.
func TestCmdCritique_SessionFileWritten(t *testing.T) {
	orig := minimalDraft("orig-002", "The service restarted.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{orig})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")
	sessPath := filepath.Join(outDir, "session.json")

	response := `{"source_span":"The service restarted.","what_changed":"a restart was recorded"}`
	client := &critiqueMockClient{responses: []string{response}}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--session-output", sessPath,
	})
	if err != nil {
		t.Fatalf("cmdCritique: unexpected error: %v", err)
	}

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
	if rec.Command != "critique" {
		t.Errorf("Command: want %q, got %q", "critique", rec.Command)
	}
}

// TestCmdCritique_LLMError verifies that an LLM error causes cmdCritique to
// return a non-nil error, and the session file is still written.
func TestCmdCritique_LLMError(t *testing.T) {
	orig := minimalDraft("orig-003", "Error span.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{orig})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")
	sessPath := filepath.Join(outDir, "session.json")

	client := &critiqueMockClient{err: errors.New("simulated API failure")}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--session-output", sessPath,
	})
	// RunCritique returns nil on per-draft errors (partial results valid).
	// cmdCritique should return an error when DraftCount == 0 and input was non-empty.
	if err == nil {
		t.Fatal("want error from LLM failure with zero produced drafts, got nil")
	}
	if !strings.Contains(err.Error(), "no critique drafts produced") {
		t.Errorf("error message: want substring %q, got %q", "no critique drafts produced", err.Error())
	}
	// Session file must still be written.
	if _, statErr := os.Stat(sessPath); statErr != nil {
		t.Errorf("session file not written on LLM error: %v", statErr)
	}
}

// TestCmdCritique_MalformedInputJSON verifies that a non-JSON input file
// returns a parse error.
func TestCmdCritique_MalformedInputJSON(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte("not json at all"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	var w bytes.Buffer
	err := cmdCritique(&w, nil, []string{
		"--input", badPath,
	})
	if err == nil {
		t.Fatal("want error for malformed JSON input, got nil")
	}
	if !strings.Contains(err.Error(), "parse input") {
		t.Errorf("error message: want substring %q, got %q", "parse input", err.Error())
	}
}

// TestCmdCritique_EmptyInputArray verifies that a valid but empty JSON array
// input produces no error and no output drafts.
func TestCmdCritique_EmptyInputArray(t *testing.T) {
	inputPath := writeDraftsFile(t, []schema.TraceDraft{})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")

	client := &critiqueMockClient{}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
	})
	if err != nil {
		t.Fatalf("cmdCritique: unexpected error for empty input: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("parse output JSON: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts for empty input, got %d", len(drafts))
	}
	if client.calls != 0 {
		t.Errorf("LLM calls: want 0 for empty input, got %d", client.calls)
	}
}

// TestCmdCritique_StdoutMode verifies that cmdCritique writes JSON to w when
// --output is not provided. No session file is written in stdout mode unless
// --session-output is explicitly set.
func TestCmdCritique_StdoutMode(t *testing.T) {
	orig := minimalDraft("stdout-001", "Stdout span.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{orig})
	promptPath := writeCritiquePromptTemplate(t)

	response := `{"source_span":"Stdout span.","what_changed":"a condition"}`
	client := &critiqueMockClient{responses: []string{response}}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		// no --output: output goes to w
	})
	if err != nil {
		t.Fatalf("cmdCritique stdout mode: unexpected error: %v", err)
	}
	// w should contain a valid JSON array with at least one draft.
	// Unmarshal to verify structure, not just string presence.
	var jsonStart int
	rawOutput := w.Bytes()
	for jsonStart = 0; jsonStart < len(rawOutput); jsonStart++ {
		if rawOutput[jsonStart] == '[' {
			break
		}
	}
	if jsonStart >= len(rawOutput) {
		t.Fatalf("stdout output: want JSON array, got no '[' in output: %q", w.String())
	}
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(rawOutput[jsonStart:], &drafts); err != nil {
		// The encoder may write the JSON followed by other text; try just the JSON portion.
		dec := json.NewDecoder(bytes.NewReader(rawOutput[jsonStart:]))
		if decErr := dec.Decode(&drafts); decErr != nil {
			t.Fatalf("stdout output: not valid JSON array: %v; output was %q", decErr, w.String())
		}
	}
	if len(drafts) == 0 {
		t.Errorf("stdout output: want at least one draft in JSON array, got 0")
	}
	if drafts[0].SourceSpan == "" {
		t.Errorf("stdout output: draft[0].SourceSpan must not be empty")
	}
}

// TestCmdCritique_IDFilterNotFound verifies that --id with a non-existent
// draft ID causes cmdCritique to return a non-nil error.
func TestCmdCritique_IDFilterNotFound(t *testing.T) {
	d1 := minimalDraft("real-id", "Span A.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{d1})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")

	client := &critiqueMockClient{}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--id", "nonexistent-id",
	})
	if err == nil {
		t.Fatal("want error when --id does not match any draft, got nil")
	}
}

// TestCmdCritique_SessionOutputConfirmation verifies that the session record
// path is printed to the writer after a successful run.
func TestCmdCritique_SessionOutputConfirmation(t *testing.T) {
	orig := minimalDraft("confirm-001", "Confirm span.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{orig})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")
	sessPath := filepath.Join(outDir, "session.json")

	response := `{"source_span":"Confirm span.","what_changed":"a condition"}`
	client := &critiqueMockClient{responses: []string{response}}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--session-output", sessPath,
	})
	if err != nil {
		t.Fatalf("cmdCritique: unexpected error: %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "wrote session record") {
		t.Errorf("stdout: want session confirmation message, got %q", output)
	}
}

// --- Group: cmdCritique — CritiqueConditions bifurcation (#151) ---

// TestCmdCritique_SessionFile_HasCritiqueConditions verifies that after a
// successful cmdCritique run with --session-output, the written session JSON
// file contains a "critique_conditions" key (not just "conditions").
func TestCmdCritique_SessionFile_HasCritiqueConditions(t *testing.T) {
	orig := minimalDraft("cc-cmd-001", "The deployment failed.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{orig})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")
	sessPath := filepath.Join(outDir, "session.json")

	response := `{"source_span":"The deployment failed.","what_changed":"a deployment failure was recorded"}`
	client := &critiqueMockClient{responses: []string{response}}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--session-output", sessPath,
	})
	if err != nil {
		t.Fatalf("cmdCritique: unexpected error: %v", err)
	}

	data, err := os.ReadFile(sessPath)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}

	// The session JSON must contain a "critique_conditions" key (new format).
	if !strings.Contains(string(data), `"critique_conditions"`) {
		t.Errorf("session JSON: want key %q, got: %s", "critique_conditions", string(data))
	}

	// Decode and verify the CritiqueConditions field is populated.
	var rec llm.SessionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("parse session JSON: %v", err)
	}
	if rec.CritiqueConditions == nil {
		t.Fatal("SessionRecord.CritiqueConditions: want non-nil for critique session, got nil")
	}
	if rec.CritiqueConditions.ModelID != "test-model" {
		t.Errorf("CritiqueConditions.ModelID: want %q, got %q", "test-model", rec.CritiqueConditions.ModelID)
	}
}

// TestCmdCritique_SessionFile_ConditionsFieldZero verifies that after a
// successful cmdCritique run, the written session JSON file has "conditions"
// with zero/empty values — Conditions must not be populated for critique sessions.
func TestCmdCritique_SessionFile_ConditionsFieldZero(t *testing.T) {
	orig := minimalDraft("cc-cmd-002", "The rollback completed.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{orig})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")
	sessPath := filepath.Join(outDir, "session.json")

	response := `{"source_span":"The rollback completed.","what_changed":"a rollback was recorded"}`
	client := &critiqueMockClient{responses: []string{response}}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--session-output", sessPath,
	})
	if err != nil {
		t.Fatalf("cmdCritique: unexpected error: %v", err)
	}

	data, err := os.ReadFile(sessPath)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}

	var rec llm.SessionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("parse session JSON: %v", err)
	}

	// rec.Conditions must be the zero value — critique sessions use CritiqueConditions.
	// Slices cannot be compared with ==, so check the string fields individually.
	if rec.Conditions.ModelID != "" {
		t.Errorf("Conditions.ModelID: want empty for critique session, got %q", rec.Conditions.ModelID)
	}
	if rec.Conditions.SystemInstructions != "" {
		t.Errorf("Conditions.SystemInstructions: want empty for critique session, got %q", rec.Conditions.SystemInstructions)
	}
	if len(rec.Conditions.SourceDocRefs) != 0 {
		t.Errorf("Conditions.SourceDocRefs: want empty for critique session, got %v", rec.Conditions.SourceDocRefs)
	}
}

// TestCmdCritique_IDFilter verifies that --id filters to a single draft.
func TestCmdCritique_IDFilter(t *testing.T) {
	d1 := minimalDraft("filter-A", "Span A.")
	d2 := minimalDraft("filter-B", "Span B.")
	d3 := minimalDraft("filter-C", "Span C.")
	inputPath := writeDraftsFile(t, []schema.TraceDraft{d1, d2, d3})
	promptPath := writeCritiquePromptTemplate(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.json")

	// Mock returns a critique for Span B (only one call expected).
	response := `{"source_span":"Span B.","what_changed":"b condition"}`
	client := &critiqueMockClient{responses: []string{response}}
	var w bytes.Buffer
	err := cmdCritique(&w, client, []string{
		"--input", inputPath,
		"--prompt-template", promptPath,
		"--model", "test-model",
		"--output", outPath,
		"--id", "filter-B",
	})
	if err != nil {
		t.Fatalf("cmdCritique: unexpected error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("parse output JSON: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft (id filter), got %d", len(drafts))
	}
	if drafts[0].DerivedFrom != "filter-B" {
		t.Errorf("DerivedFrom: want %q, got %q", "filter-B", drafts[0].DerivedFrom)
	}
	if client.calls != 1 {
		t.Errorf("LLM calls: want 1, got %d", client.calls)
	}
}
