// split_test.go tests RunSplit and parseSplitResponse in black-box style.
//
// All tests use the llm_test package (black-box) to verify public behaviour
// without depending on internal implementation details.
package llm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
)

// --- Helper ---

// splitOpts returns a minimal SplitOptions for test use. The source doc path
// must point to a real file; tests that need one should create it via
// writeSourceDoc (defined in extract_test.go, same package).
func splitOpts(t *testing.T, sourcePath, promptPath string) llm.SplitOptions {
	t.Helper()
	return llm.SplitOptions{
		ModelID:            "claude-sonnet-4-6",
		InputPath:          sourcePath,
		PromptTemplatePath: promptPath,
		SourceDocRef:       "test-split-doc",
	}
}

// threeSpansJSON is a valid LLM response returning three observation spans.
const threeSpansJSON = `["Span one: the system was healthy.", "Span two: an alert fired.", "Span three: the incident was resolved."]`

// --- Group: RunSplit ---

// TestRunSplit_success verifies that a valid JSON array response produces
// the expected spans, DraftCount = len(spans), DraftIDs = nil, Command = "split",
// and no ErrorNote.
func TestRunSplit_success(t *testing.T) {
	src := writeSourceDoc(t, "Some source document content.")
	prompt := writePromptTemplate(t)
	client := newMockClient(threeSpansJSON)

	spans, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("RunSplit() want no error, got: %v", err)
	}
	if len(spans) != 3 {
		t.Errorf("want 3 spans, got %d", len(spans))
	}
	if rec.DraftCount != 3 {
		t.Errorf("SessionRecord.DraftCount: want 3, got %d", rec.DraftCount)
	}
	if rec.DraftIDs != nil {
		t.Errorf("SessionRecord.DraftIDs: want nil (spans are not TraceDraft records), got %v", rec.DraftIDs)
	}
	if rec.Command != "split" {
		t.Errorf("SessionRecord.Command: want %q, got %q", "split", rec.Command)
	}
	if rec.ErrorNote != "" {
		t.Errorf("SessionRecord.ErrorNote: want empty on success, got %q", rec.ErrorNote)
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty")
	}
}

// TestRunSplit_clientError verifies that when the LLM client returns an error,
// RunSplit propagates the error, sets ErrorNote on the SessionRecord, and still
// returns a non-nil SessionRecord.
func TestRunSplit_clientError(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)
	clientErr := errors.New("network timeout")
	client := newErrClient(clientErr)

	spans, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err == nil {
		t.Fatal("RunSplit() with client error: want error, got nil")
	}
	if spans != nil {
		t.Errorf("want nil spans on error, got %v", spans)
	}
	// SessionRecord must always be returned.
	if rec.ID == "" {
		t.Error("SessionRecord must be non-nil (non-empty ID) even on client error")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on client error")
	}
	if rec.Command != "split" {
		t.Errorf("SessionRecord.Command: want %q, got %q", "split", rec.Command)
	}
}

// TestRunSplit_refusal verifies that a refusal response produces ErrLLMRefusal
// and still returns a non-nil SessionRecord with ErrorNote set.
func TestRunSplit_refusal(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)
	client := newMockClient("I cannot do this.")

	_, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err == nil {
		t.Fatal("RunSplit() with refusal: want ErrLLMRefusal, got nil")
	}
	var refusalErr *llm.ErrLLMRefusal
	if !errors.As(err, &refusalErr) {
		t.Errorf("want *ErrLLMRefusal in error chain, got %T: %v", err, err)
	}
	if rec.ID == "" {
		t.Error("SessionRecord must be returned even on refusal")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on refusal")
	}
}

// TestRunSplit_malformedOutput verifies that a non-JSON response produces
// ErrMalformedOutput and still returns a non-nil SessionRecord with ErrorNote.
func TestRunSplit_malformedOutput(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)
	client := newMockClient("this is not json at all {broken}")

	_, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err == nil {
		t.Fatal("RunSplit() with malformed output: want error, got nil")
	}
	var malformedErr *llm.ErrMalformedOutput
	if !errors.As(err, &malformedErr) {
		t.Errorf("want *ErrMalformedOutput in error chain, got %T: %v", err, err)
	}
	if rec.ID == "" {
		t.Error("SessionRecord must be returned even on malformed output")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on malformed output")
	}
}

// TestRunSplit_emptySpanArray verifies that an empty JSON array response produces
// an error (0 spans is not valid output — it signals a genuine LLM failure).
func TestRunSplit_emptySpanArray(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)
	client := newMockClient("[]")

	_, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err == nil {
		t.Fatal("RunSplit() with empty array: want error (0 spans not valid), got nil")
	}
	if rec.ID == "" {
		t.Error("SessionRecord must be returned even when span array is empty")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set when 0 spans produced")
	}
}

// TestRunSplit_blankSpansFiltered verifies that blank strings in the JSON array
// are filtered out; the result contains only non-blank spans.
func TestRunSplit_blankSpansFiltered(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)
	// Two real spans, two blank.
	client := newMockClient(`["span1", "", "  ", "span2"]`)

	spans, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("RunSplit() with blank spans: want no error, got: %v", err)
	}
	if len(spans) != 2 {
		t.Errorf("want 2 non-blank spans, got %d: %v", len(spans), spans)
	}
	if rec.DraftCount != 2 {
		t.Errorf("SessionRecord.DraftCount: want 2, got %d", rec.DraftCount)
	}
}

// TestRunSplit_preamble verifies that text before the JSON array opening bracket
// is tolerated (LLM sometimes prefixes with prose before the JSON).
func TestRunSplit_preamble(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)
	client := newMockClient(`Here are the spans:
["span1","span2"]`)

	spans, _, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("RunSplit() with preamble: want no error, got: %v", err)
	}
	if len(spans) != 2 {
		t.Errorf("want 2 spans after preamble stripped, got %d", len(spans))
	}
}

// TestRunSplit_sessionRecordAlwaysReturned verifies that SessionRecord is returned
// on all error paths: client error, refusal, malformed output, and empty span array.
func TestRunSplit_sessionRecordAlwaysReturned(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)

	cases := []struct {
		name   string
		client llm.LLMClient
	}{
		{"client error", newErrClient(errors.New("connection refused"))},
		{"refusal", newMockClient("I cannot do this.")},
		{"malformed", newMockClient("not json at all")},
		{"empty array", newMockClient("[]")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, rec, err := llm.RunSplit(context.Background(), tc.client, splitOpts(t, src, prompt))
			if err == nil {
				t.Fatal("want error on this path, got nil")
			}
			if rec.Command != "split" {
				t.Errorf("SessionRecord.Command: want %q even on error, got %q", "split", rec.Command)
			}
			if rec.ID == "" {
				t.Error("SessionRecord.ID must be non-empty even on error")
			}
			if rec.ErrorNote == "" {
				t.Error("SessionRecord.ErrorNote must be set on error")
			}
		})
	}
}

// TestRunSplit_missingSourceDoc verifies that RunSplit returns an error and a
// non-empty SessionRecord when the source document path does not exist.
func TestRunSplit_missingSourceDoc(t *testing.T) {
	prompt := writePromptTemplate(t)
	client := newMockClient(threeSpansJSON)

	_, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, "/nonexistent/source.md", prompt))
	if err == nil {
		t.Fatal("RunSplit() with missing source doc: want error, got nil")
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even when source doc is missing")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set when source doc is unreadable")
	}
}

// TestRunSplit_missingPromptTemplate verifies that RunSplit returns an error and
// a non-empty SessionRecord when the prompt template path does not exist.
func TestRunSplit_missingPromptTemplate(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	client := newMockClient(threeSpansJSON)

	_, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, "/nonexistent/prompt.md"))
	if err == nil {
		t.Fatal("RunSplit() with missing prompt template: want error, got nil")
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even when prompt template is missing")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set when prompt template is unreadable")
	}
}

// TestRunSplit_allBlankSpans verifies that an array of only blank strings
// (after filtering) produces a 0-spans error — distinct from the empty-array
// case where the LLM returned [] directly.
func TestRunSplit_allBlankSpans(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)
	client := newMockClient(`["", "  ", "\t"]`)

	_, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err == nil {
		t.Fatal("RunSplit() with all-blank spans: want error (0 non-blank spans), got nil")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set when all spans are blank")
	}
}

// TestRunSplit_emptyStringResponse verifies that an empty LLM response is
// treated as malformed output (not a refusal).
func TestRunSplit_emptyStringResponse(t *testing.T) {
	src := writeSourceDoc(t, "Document content.")
	prompt := writePromptTemplate(t)
	client := newMockClient("")

	_, _, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err == nil {
		t.Fatal("RunSplit() with empty response: want error, got nil")
	}
	var malformed *llm.ErrMalformedOutput
	if !errors.As(err, &malformed) {
		t.Errorf("empty response should produce ErrMalformedOutput, got %T: %v", err, err)
	}
}

// --- Group: parseSplitResponse (indirect coverage via RunSplit) ---

// parseSplitResponse is unexported. These tests verify its behaviour indirectly
// through RunSplit by injecting mock LLM responses and asserting on RunSplit's
// output. Direct unit coverage of parseSplitResponse is exercised by the
// RunSplit tests above (preamble, malformed, empty, all-blank).

// TestParseSplitResponse_valid exercises parseSplitResponse via RunSplit:
// a clean JSON array parses to the expected slice.
func TestParseSplitResponse_valid(t *testing.T) {
	src := writeSourceDoc(t, "source text")
	prompt := writePromptTemplate(t)
	client := newMockClient(`["alpha", "beta", "gamma"]`)

	spans, _, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error, got: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if len(spans) != len(want) {
		t.Fatalf("want %d spans, got %d", len(want), len(spans))
	}
	for i, w := range want {
		if spans[i] != w {
			t.Errorf("span[%d]: want %q, got %q", i, w, spans[i])
		}
	}
}

// TestParseSplitResponse_preamble exercises preamble tolerance via RunSplit.
func TestParseSplitResponse_preamble(t *testing.T) {
	src := writeSourceDoc(t, "source text")
	prompt := writePromptTemplate(t)
	client := newMockClient("Preamble text. More prose.\n[\"x\",\"y\"]")

	spans, _, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("want no error for preamble input, got: %v", err)
	}
	if len(spans) != 2 {
		t.Errorf("want 2 spans, got %d", len(spans))
	}
}

// TestParseSplitResponse_invalid exercises the malformed-output path via RunSplit.
func TestParseSplitResponse_invalid(t *testing.T) {
	src := writeSourceDoc(t, "source text")
	prompt := writePromptTemplate(t)
	client := newMockClient("{not a json array at all}")

	_, _, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err == nil {
		t.Fatal("want error for non-JSON array input, got nil")
	}
}

// --- PromptHash tests ---

// TestRunSplit_PromptHash_Set verifies that RunSplit populates
// Conditions.PromptHash with a 16-character hex string when a prompt template
// path is provided.
func TestRunSplit_PromptHash_Set(t *testing.T) {
	src := writeSourceDoc(t, "Some source document content.")
	prompt := writePromptTemplate(t)
	client := newMockClient(threeSpansJSON)

	_, rec, err := llm.RunSplit(context.Background(), client, splitOpts(t, src, prompt))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := rec.Conditions.PromptHash
	if len(h) != 16 {
		t.Errorf("Conditions.PromptHash: want 16-char hex, got %q (len=%d)", h, len(h))
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character %q in PromptHash %q", c, h)
		}
	}
}
