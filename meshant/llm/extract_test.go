package llm_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
)

// mockClient is a test double for LLMClient. It returns a canned response or
// error on each call to Complete, advancing through responses in order.
type mockClient struct {
	responses []string
	errs      []error
	calls     int
}

func (m *mockClient) Complete(_ context.Context, _, _ string) (string, error) {
	i := m.calls
	m.calls++
	if i < len(m.errs) && m.errs[i] != nil {
		return "", m.errs[i]
	}
	if i < len(m.responses) {
		return m.responses[i], nil
	}
	return "[]", nil
}

// newMockClient returns a mock that returns the given response on every call.
func newMockClient(response string) *mockClient {
	return &mockClient{responses: []string{response}}
}

// newErrClient returns a mock that returns the given error on every call.
func newErrClient(err error) *mockClient {
	return &mockClient{errs: []error{err}}
}

// writeSourceDoc writes content to a temp file and returns its path.
func writeSourceDoc(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "source.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write source doc: %v", err)
	}
	return path
}

// writePromptTemplate writes a minimal prompt template and returns its path.
func writePromptTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(path, []byte("Extract trace drafts."), 0o644); err != nil {
		t.Fatalf("write prompt template: %v", err)
	}
	return path
}

// baseOpts returns a valid ExtractionOptions pointing at real temp files.
func baseOpts(t *testing.T, sourcePath, promptPath string) llm.ExtractionOptions {
	t.Helper()
	return llm.ExtractionOptions{
		ModelID:            "claude-sonnet-4-6",
		InputPath:          sourcePath,
		PromptTemplatePath: promptPath,
		SourceDocRef:       "test-doc",
	}
}

const validDraftsJSON = `[
  {
    "source_span": "The system failed at 14:00.",
    "what_changed": "system failure observed",
    "observer": "ops-engineer"
  },
  {
    "source_span": "Recovery completed by 14:45.",
    "what_changed": "recovery completed"
  }
]`

// --- Tests ---

func TestRunExtraction_HappyPath(t *testing.T) {
	src := writeSourceDoc(t, "The system failed at 14:00. Recovery completed by 14:45.")
	prompt := writePromptTemplate(t)
	client := newMockClient(validDraftsJSON)

	drafts, rec, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error, got: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("want 2 drafts, got %d", len(drafts))
	}

	// SessionRecord invariants.
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty")
	}
	if rec.Command != "extract" {
		t.Errorf("SessionRecord.Command: want %q, got %q", "extract", rec.Command)
	}
	if rec.DraftCount != 2 {
		t.Errorf("SessionRecord.DraftCount: want 2, got %d", rec.DraftCount)
	}
	if rec.ErrorNote != "" {
		t.Errorf("SessionRecord.ErrorNote: want empty on success, got %q", rec.ErrorNote)
	}
	if len(rec.DraftIDs) != 2 {
		t.Errorf("SessionRecord.DraftIDs: want 2, got %d", len(rec.DraftIDs))
	}

	// Per-draft provenance invariants.
	for i, d := range drafts {
		if d.ID == "" {
			t.Errorf("draft[%d].ID: must be non-empty", i)
		}
		if d.ExtractedBy != "claude-sonnet-4-6" {
			t.Errorf("draft[%d].ExtractedBy: want %q, got %q", i, "claude-sonnet-4-6", d.ExtractedBy)
		}
		if d.ExtractionStage != "weak-draft" {
			t.Errorf("draft[%d].ExtractionStage: want %q, got %q", i, "weak-draft", d.ExtractionStage)
		}
		if d.SessionRef != rec.ID {
			t.Errorf("draft[%d].SessionRef: want %q, got %q", i, rec.ID, d.SessionRef)
		}
		if !strings.Contains(d.UncertaintyNote, "LLM-produced candidate; unverified by human review") {
			t.Errorf("draft[%d].UncertaintyNote missing framework suffix: %q", i, d.UncertaintyNote)
		}
		if d.SourceDocRef != "test-doc" {
			t.Errorf("draft[%d].SourceDocRef: want %q, got %q", i, "test-doc", d.SourceDocRef)
		}
		if err := d.Validate(); err != nil {
			t.Errorf("draft[%d].Validate(): %v", i, err)
		}
		// Draft IDs must appear in SessionRecord.DraftIDs.
		found := false
		for _, id := range rec.DraftIDs {
			if id == d.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("draft[%d].ID %q not found in SessionRecord.DraftIDs", i, d.ID)
		}
	}
}

func TestRunExtraction_EmptyResponse(t *testing.T) {
	src := writeSourceDoc(t, "Some source text.")
	prompt := writePromptTemplate(t)
	client := newMockClient("[]")

	drafts, rec, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error for empty response, got: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts, got %d", len(drafts))
	}
	if rec.DraftCount != 0 {
		t.Errorf("SessionRecord.DraftCount: want 0, got %d", rec.DraftCount)
	}
	// SessionRecord must still be non-zero and valid.
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even for empty response")
	}
}

func TestRunExtraction_MalformedResponse(t *testing.T) {
	src := writeSourceDoc(t, "Some source text.")
	prompt := writePromptTemplate(t)
	client := newMockClient("not json at all {{")

	_, rec, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err == nil {
		t.Fatal("want error for malformed response, got nil")
	}

	var malformed *llm.ErrMalformedOutput
	if !errors.As(err, &malformed) {
		t.Errorf("want ErrMalformedOutput, got %T: %v", err, err)
	}
	if malformed != nil && !strings.Contains(malformed.RawResponse, "not json") {
		t.Errorf("ErrMalformedOutput.RawResponse should contain original text, got: %q", malformed.RawResponse)
	}
	// SessionRecord must still be returned with ErrorNote.
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even on error")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on malformed output error")
	}
}

func TestRunExtraction_Refusal(t *testing.T) {
	src := writeSourceDoc(t, "Some source text.")
	prompt := writePromptTemplate(t)
	client := newMockClient("I cannot assist with this request.")

	_, rec, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err == nil {
		t.Fatal("want error for refusal, got nil")
	}

	var refusal *llm.ErrLLMRefusal
	if !errors.As(err, &refusal) {
		t.Errorf("want ErrLLMRefusal, got %T: %v", err, err)
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even on refusal")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on refusal")
	}
}

func TestRunExtraction_UncertaintyNote_FrameworkAppended(t *testing.T) {
	// LLM returns a draft with its own uncertainty note already set.
	const draftsWithNote = `[
  {
    "source_span": "The deployment failed.",
    "uncertainty_note": "attribution unclear from this span"
  }
]`
	src := writeSourceDoc(t, "The deployment failed.")
	prompt := writePromptTemplate(t)
	client := newMockClient(draftsWithNote)

	drafts, _, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error, got: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}

	note := drafts[0].UncertaintyNote
	// Both the LLM's note and the framework suffix must be present.
	if !strings.Contains(note, "attribution unclear from this span") {
		t.Errorf("LLM uncertainty note must be preserved: %q", note)
	}
	if !strings.Contains(note, "LLM-produced candidate; unverified by human review") {
		t.Errorf("framework uncertainty suffix must be appended: %q", note)
	}
}

func TestRunExtraction_UncertaintyNote_EmptyLLMNote(t *testing.T) {
	// LLM returns draft with no uncertainty note — framework must still append.
	const draftsNoNote = `[{"source_span": "The cache expired."}]`
	src := writeSourceDoc(t, "The cache expired.")
	prompt := writePromptTemplate(t)
	client := newMockClient(draftsNoNote)

	drafts, _, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error, got: %v", err)
	}
	if len(drafts) == 0 {
		t.Fatal("want at least 1 draft")
	}
	if !strings.Contains(drafts[0].UncertaintyNote, "LLM-produced candidate; unverified by human review") {
		t.Errorf("framework suffix must be set even when LLM note is empty: %q", drafts[0].UncertaintyNote)
	}
}

func TestRunExtraction_SessionRef_OnEveryDraft(t *testing.T) {
	const threeDrafts = `[
  {"source_span": "span one"},
  {"source_span": "span two"},
  {"source_span": "span three"}
]`
	src := writeSourceDoc(t, "span one\nspan two\nspan three")
	prompt := writePromptTemplate(t)
	client := newMockClient(threeDrafts)

	drafts, rec, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error, got: %v", err)
	}
	if len(drafts) != 3 {
		t.Fatalf("want 3 drafts, got %d", len(drafts))
	}
	for i, d := range drafts {
		if d.SessionRef == "" {
			t.Errorf("draft[%d].SessionRef must be non-empty", i)
		}
		if d.SessionRef != rec.ID {
			t.Errorf("draft[%d].SessionRef %q != SessionRecord.ID %q", i, d.SessionRef, rec.ID)
		}
	}
}

func TestRunExtraction_IntentionallyBlank_Valid(t *testing.T) {
	const draftsWithBlank = `[
  {
    "source_span": "The service degraded.",
    "intentionally_blank": ["source", "tags"]
  }
]`
	src := writeSourceDoc(t, "The service degraded.")
	prompt := writePromptTemplate(t)
	client := newMockClient(draftsWithBlank)

	drafts, _, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error for valid intentionally_blank, got: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	blank := drafts[0].IntentionallyBlank
	if len(blank) != 2 || blank[0] != "source" || blank[1] != "tags" {
		t.Errorf("IntentionallyBlank not preserved: %v", blank)
	}
}

func TestRunExtraction_IntentionallyBlank_Invalid(t *testing.T) {
	// "extracted_by" is a provenance field — not valid in intentionally_blank.
	const draftsWithBadBlank = `[
  {
    "source_span": "The service degraded.",
    "intentionally_blank": ["extracted_by"]
  }
]`
	src := writeSourceDoc(t, "The service degraded.")
	prompt := writePromptTemplate(t)
	client := newMockClient(draftsWithBadBlank)

	_, _, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err == nil {
		t.Fatal("want error for invalid intentionally_blank field name, got nil")
	}
}

func TestRunExtraction_SourceDocTooLarge(t *testing.T) {
	// Write a file larger than maxSourceBytes (1 MiB).
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.md")
	large := make([]byte, 1*1024*1024+1)
	for i := range large {
		large[i] = 'x'
	}
	if err := os.WriteFile(largePath, large, 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	prompt := writePromptTemplate(t)
	client := newMockClient("[]")

	opts := llm.ExtractionOptions{
		ModelID:            "claude-sonnet-4-6",
		InputPath:          largePath,
		PromptTemplatePath: prompt,
	}
	_, _, err := llm.RunExtraction(context.Background(), client, opts)
	if err == nil {
		t.Fatal("want error for oversized source doc, got nil")
	}
}

func TestRunExtraction_ClientError(t *testing.T) {
	src := writeSourceDoc(t, "Some text.")
	prompt := writePromptTemplate(t)
	clientErr := errors.New("network timeout")
	client := newErrClient(clientErr)

	_, rec, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err == nil {
		t.Fatal("want error when client fails, got nil")
	}
	if rec.ID == "" {
		t.Error("SessionRecord must be returned even on client error")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on client error")
	}
}

func TestRunExtraction_SessionRecord_AlwaysReturned(t *testing.T) {
	// Covers multiple error paths to verify SessionRecord is never zero-valued.
	cases := []struct {
		name     string
		response string
		wantErr  bool
	}{
		{"success", validDraftsJSON, false},
		{"empty", "[]", false},
		{"malformed", "{bad json", true},
		{"refusal", "I'm sorry, I cannot help.", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := writeSourceDoc(t, "source text")
			prompt := writePromptTemplate(t)
			client := newMockClient(tc.response)

			_, rec, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
			if tc.wantErr && err == nil {
				t.Errorf("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("want no error, got: %v", err)
			}
			// SessionRecord must always have a non-empty ID.
			if rec.ID == "" {
				t.Errorf("SessionRecord.ID must be non-empty (case: %s)", tc.name)
			}
			if rec.Command != "extract" {
				t.Errorf("SessionRecord.Command: want %q, got %q", "extract", rec.Command)
			}
		})
	}
}

func TestRunExtraction_MissingSourceDoc(t *testing.T) {
	prompt := writePromptTemplate(t)
	client := newMockClient("[]")

	opts := llm.ExtractionOptions{
		ModelID:            "claude-sonnet-4-6",
		InputPath:          "/nonexistent/source.md",
		PromptTemplatePath: prompt,
	}
	_, _, err := llm.RunExtraction(context.Background(), client, opts)
	if err == nil {
		t.Fatal("want error for missing source doc, got nil")
	}
}

func TestRunExtraction_MissingPromptTemplate(t *testing.T) {
	src := writeSourceDoc(t, "Some source text.")
	client := newMockClient("[]")

	opts := llm.ExtractionOptions{
		ModelID:            "claude-sonnet-4-6",
		InputPath:          src,
		PromptTemplatePath: "/nonexistent/prompt.md",
	}
	_, rec, err := llm.RunExtraction(context.Background(), client, opts)
	if err == nil {
		t.Fatal("want error for missing prompt template, got nil")
	}
	// SessionRecord must still be returned with ErrorNote.
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even when prompt template is missing")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set when prompt template is missing")
	}
}

func TestRunExtraction_EmptyStringResponse(t *testing.T) {
	// An empty-string LLM response is not a refusal — it is malformed output.
	// The isRefusal function explicitly returns false for the empty-string case
	// ("empty response → malformed, not refusal").
	src := writeSourceDoc(t, "Some source text.")
	prompt := writePromptTemplate(t)
	client := newMockClient("")

	_, _, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err == nil {
		t.Fatal("want error for empty-string response, got nil")
	}
	var malformed *llm.ErrMalformedOutput
	if !errors.As(err, &malformed) {
		t.Errorf("empty-string response must produce ErrMalformedOutput, got %T: %v", err, err)
	}
}

func TestRunExtraction_PreambleBeforeJSON(t *testing.T) {
	// Some LLMs prefix their JSON output with a sentence. parseResponse strips
	// the preamble by finding the first '[' and last ']'.
	const responseWithPreamble = `Here are the candidate trace drafts:
[
  {"source_span": "The router went offline at 02:00.", "what_changed": "router offline"}
]`
	src := writeSourceDoc(t, "The router went offline at 02:00.")
	prompt := writePromptTemplate(t)
	client := newMockClient(responseWithPreamble)

	drafts, _, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error for response with preamble, got: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft parsed despite preamble, got %d", len(drafts))
	}
	if drafts[0].SourceSpan != "The router went offline at 02:00." {
		t.Errorf("source_span not preserved: %q", drafts[0].SourceSpan)
	}
}

func TestRunExtraction_DraftValidationFailure(t *testing.T) {
	// A draft with no source_span fails schema validation. RunExtraction must
	// return an error and set SessionRecord.ErrorNote.
	const draftWithoutSourceSpan = `[{"what_changed": "something happened"}]`
	src := writeSourceDoc(t, "something happened")
	prompt := writePromptTemplate(t)
	client := newMockClient(draftWithoutSourceSpan)

	_, rec, err := llm.RunExtraction(context.Background(), client, baseOpts(t, src, prompt))
	if err == nil {
		t.Fatal("want error when draft has no source_span, got nil")
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even on draft validation failure")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on draft validation failure")
	}
}
