// critique_test.go tests RunCritique (package llm_test, black-box).
//
// All LLM calls are intercepted by the mockClient defined in extract_test.go.
// No real API calls are made.
package llm_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// makeDraft returns a minimal TraceDraft with a known ID and source span.
// Used across critique tests as input to RunCritique.
func makeDraft(id, sourceSpan, whatChanged string) schema.TraceDraft {
	return schema.TraceDraft{
		ID:              id,
		SourceSpan:      sourceSpan,
		WhatChanged:     whatChanged,
		ExtractionStage: "weak-draft",
		ExtractedBy:     "claude-sonnet-4-6",
		Timestamp:       time.Now(),
	}
}

// critiqueJSON returns a single-draft JSON object that is a faithful critique
// of the given source span. The LLM returns this as a JSON object (not array).
func critiqueJSON(sourceSpan string) string {
	return `{"source_span":"` + sourceSpan + `","what_changed":"a condition was described","observer":"security-lead"}`
}

// baseCritiqueOpts returns CritiqueOptions with a prompt template file.
func baseCritiqueOpts(t *testing.T) llm.CritiqueOptions {
	t.Helper()
	return llm.CritiqueOptions{
		ModelID:            "claude-sonnet-4-6",
		PromptTemplatePath: writePromptTemplate(t),
		SourceDocRef:       "test-doc",
	}
}

// --- Group: RunCritique ---

// TestRunCritique_HappyPath verifies the core case: one input draft is
// critiqued; output draft has correct provenance fields.
func TestRunCritique_HappyPath(t *testing.T) {
	orig := makeDraft("draft-001", "The API went down.", "service interrupted")
	opts := baseCritiqueOpts(t)
	client := newMockClient(critiqueJSON("The API went down."))

	drafts, rec, err := llm.RunCritique(context.Background(), client, []schema.TraceDraft{orig}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 critique draft, got %d", len(drafts))
	}
	d := drafts[0]

	// ExtractionStage must be "critiqued" (Decision D4, F.1).
	if d.ExtractionStage != "critiqued" {
		t.Errorf("ExtractionStage: want %q, got %q", "critiqued", d.ExtractionStage)
	}
	// ExtractedBy must be the model ID string (Decision D2, F.1).
	if d.ExtractedBy != opts.ModelID {
		t.Errorf("ExtractedBy: want %q, got %q", opts.ModelID, d.ExtractedBy)
	}
	// DerivedFrom must link back to the original draft (F.4 design constraint).
	if d.DerivedFrom != orig.ID {
		t.Errorf("DerivedFrom: want %q, got %q", orig.ID, d.DerivedFrom)
	}
	// SessionRef must match the session (Decision D6, F.1).
	if d.SessionRef == "" {
		t.Error("SessionRef must not be empty")
	}
	if d.SessionRef != rec.ID {
		t.Errorf("SessionRef: want %q (session ID), got %q", rec.ID, d.SessionRef)
	}
	// Framework uncertainty note must be appended (Decision D3, F.1).
	if !strings.Contains(d.UncertaintyNote, "LLM-produced candidate") {
		t.Errorf("UncertaintyNote: framework note not present, got %q", d.UncertaintyNote)
	}
	// SourceSpan must be preserved from the original.
	if d.SourceSpan != orig.SourceSpan {
		t.Errorf("SourceSpan: want %q, got %q", orig.SourceSpan, d.SourceSpan)
	}

	// SessionRecord basics.
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty")
	}
	if rec.Command != "critique" {
		t.Errorf("Command: want %q, got %q", "critique", rec.Command)
	}
	if rec.DraftCount != 1 {
		t.Errorf("DraftCount: want 1, got %d", rec.DraftCount)
	}
}

// TestRunCritique_SourceSpanMismatch verifies that a draft whose SourceSpan
// differs from the original is rejected. The session continues; DraftCount = 0.
func TestRunCritique_SourceSpanMismatch(t *testing.T) {
	orig := makeDraft("draft-002", "The API went down.", "service interrupted")
	opts := baseCritiqueOpts(t)
	// LLM returns a different source_span.
	client := newMockClient(`{"source_span":"DIFFERENT SPAN","what_changed":"something"}`)

	drafts, rec, err := llm.RunCritique(context.Background(), client, []schema.TraceDraft{orig}, opts)
	if err != nil {
		t.Fatalf("want nil error (partial results), got: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts (mismatch rejected), got %d", len(drafts))
	}
	if rec.DraftCount != 0 {
		t.Errorf("DraftCount: want 0, got %d", rec.DraftCount)
	}
	if rec.ErrorNote == "" {
		t.Error("ErrorNote must record the mismatch reason")
	}
}

// TestRunCritique_MalformedResponse verifies that a parse error on one draft
// does not abort the session; remaining drafts are processed.
func TestRunCritique_MalformedResponse(t *testing.T) {
	orig1 := makeDraft("draft-bad", "Span bad.", "bad")
	orig2 := makeDraft("draft-ok", "Span ok.", "ok")
	opts := baseCritiqueOpts(t)
	// First call returns unparseable text; second returns valid JSON.
	client := &mockClient{
		responses: []string{
			"not json at all",
			critiqueJSON("Span ok."),
		},
	}

	drafts, rec, err := llm.RunCritique(
		context.Background(), client,
		[]schema.TraceDraft{orig1, orig2},
		opts,
	)
	if err != nil {
		t.Fatalf("want nil error (partial results), got: %v", err)
	}
	// Only the second draft should be in the output.
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft (second processed ok), got %d", len(drafts))
	}
	if drafts[0].DerivedFrom != orig2.ID {
		t.Errorf("DerivedFrom: want %q, got %q", orig2.ID, drafts[0].DerivedFrom)
	}
	if rec.DraftCount != 1 {
		t.Errorf("DraftCount: want 1, got %d", rec.DraftCount)
	}
	if rec.ErrorNote == "" {
		t.Error("ErrorNote must record the parse failure on first draft")
	}
}

// TestRunCritique_SessionRecordAlwaysReturned verifies that even if all drafts
// fail, RunCritique returns a non-nil SessionRecord with correct metadata.
func TestRunCritique_SessionRecordAlwaysReturned(t *testing.T) {
	orig := makeDraft("draft-fail", "Span fail.", "fail")
	opts := baseCritiqueOpts(t)
	// LLM returns error on every call.
	client := newErrClient(errors.New("simulated API failure"))

	drafts, rec, err := llm.RunCritique(context.Background(), client, []schema.TraceDraft{orig}, opts)
	if err != nil {
		t.Fatalf("want nil error (partial results), got: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts, got %d", len(drafts))
	}
	// SessionRecord must always be non-nil and carry identifying fields.
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even on total failure")
	}
	if rec.Command != "critique" {
		t.Errorf("Command: want %q, got %q", "critique", rec.Command)
	}
	if rec.DraftCount != 0 {
		t.Errorf("DraftCount: want 0, got %d", rec.DraftCount)
	}
	if rec.ErrorNote == "" {
		t.Error("ErrorNote must be set on total failure")
	}
}

// TestRunCritique_UncertaintyNoteAppended verifies that the framework always
// appends its uncertainty note, even when the LLM also sets one.
func TestRunCritique_UncertaintyNoteAppended(t *testing.T) {
	orig := makeDraft("draft-003", "Service call failed.", "call failure")
	opts := baseCritiqueOpts(t)
	// LLM response includes its own uncertainty note.
	client := newMockClient(`{
		"source_span": "Service call failed.",
		"what_changed": "a call failure was recorded",
		"uncertainty_note": "LLM expressed uncertainty"
	}`)

	drafts, _, err := llm.RunCritique(context.Background(), client, []schema.TraceDraft{orig}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	note := drafts[0].UncertaintyNote
	// LLM note preserved.
	if !strings.Contains(note, "LLM expressed uncertainty") {
		t.Errorf("LLM uncertainty note not preserved, got %q", note)
	}
	// Framework note appended.
	if !strings.Contains(note, "LLM-produced candidate") {
		t.Errorf("framework uncertainty note not appended, got %q", note)
	}
}

// TestRunCritique_IntentionallyBlankPreserved verifies that IntentionallyBlank
// set by the LLM is preserved in the output draft.
func TestRunCritique_IntentionallyBlankPreserved(t *testing.T) {
	orig := makeDraft("draft-004", "Span with blank source.", "no actor named")
	opts := baseCritiqueOpts(t)
	client := newMockClient(`{
		"source_span": "Span with blank source.",
		"what_changed": "a condition was described",
		"intentionally_blank": ["source"],
		"uncertainty_note": "span names no actor"
	}`)

	drafts, _, err := llm.RunCritique(context.Background(), client, []schema.TraceDraft{orig}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	d := drafts[0]
	if len(d.IntentionallyBlank) != 1 || d.IntentionallyBlank[0] != "source" {
		t.Errorf("IntentionallyBlank: want [source], got %v", d.IntentionallyBlank)
	}
}

// TestRunCritique_IDFilter verifies that when DraftID is set, only the matching
// draft is critiqued; others are skipped.
func TestRunCritique_IDFilter(t *testing.T) {
	d1 := makeDraft("draft-A", "Span A.", "change A")
	d2 := makeDraft("draft-B", "Span B.", "change B")
	d3 := makeDraft("draft-C", "Span C.", "change C")
	opts := baseCritiqueOpts(t)
	opts.DraftID = "draft-B"
	// Mock returns a critique for Span B only (only one call expected).
	client := newMockClient(critiqueJSON("Span B."))

	drafts, rec, err := llm.RunCritique(
		context.Background(), client,
		[]schema.TraceDraft{d1, d2, d3},
		opts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft (only draft-B critiqued), got %d", len(drafts))
	}
	if drafts[0].DerivedFrom != "draft-B" {
		t.Errorf("DerivedFrom: want %q, got %q", "draft-B", drafts[0].DerivedFrom)
	}
	if rec.DraftCount != 1 {
		t.Errorf("DraftCount: want 1, got %d", rec.DraftCount)
	}
	// LLM must be called exactly once.
	if client.calls != 1 {
		t.Errorf("LLM calls: want 1, got %d", client.calls)
	}
}

// TestRunCritique_SessionRefOnDraft verifies that every produced draft has a
// non-empty SessionRef matching the SessionRecord ID.
func TestRunCritique_SessionRefOnDraft(t *testing.T) {
	d1 := makeDraft("ref-1", "Span one.", "change one")
	d2 := makeDraft("ref-2", "Span two.", "change two")
	opts := baseCritiqueOpts(t)
	client := &mockClient{
		responses: []string{
			critiqueJSON("Span one."),
			critiqueJSON("Span two."),
		},
	}

	drafts, rec, err := llm.RunCritique(
		context.Background(), client,
		[]schema.TraceDraft{d1, d2},
		opts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("want 2 drafts, got %d", len(drafts))
	}
	for i, d := range drafts {
		if d.SessionRef == "" {
			t.Errorf("draft %d: SessionRef must not be empty", i)
		}
		if d.SessionRef != rec.ID {
			t.Errorf("draft %d: SessionRef %q != SessionRecord.ID %q", i, d.SessionRef, rec.ID)
		}
	}
}

// TestRunCritique_DerivedFromInjectionGuard verifies that a LLM-supplied
// derived_from value is zeroed and then overwritten by the framework with the
// original draft's ID. The LLM must not be able to inject false derivation
// chain links.
func TestRunCritique_DerivedFromInjectionGuard(t *testing.T) {
	orig := makeDraft("original-id-xyz", "A span.", "something")
	opts := baseCritiqueOpts(t)
	// LLM response contains a derived_from value it should not set.
	client := newMockClient(`{"source_span":"A span.","what_changed":"something","derived_from":"injected-id"}`)

	drafts, _, err := llm.RunCritique(context.Background(), client, []schema.TraceDraft{orig}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	// Framework must set DerivedFrom to the original ID, not the LLM-injected value.
	if drafts[0].DerivedFrom != "original-id-xyz" {
		t.Errorf("DerivedFrom: want %q (original), got %q", "original-id-xyz", drafts[0].DerivedFrom)
	}
}

// TestRunCritique_ExtractionStageInjectionGuard verifies that an LLM-supplied
// extraction_stage is discarded and replaced by the framework with "critiqued".
// The LLM does not control its own stage assignment.
func TestRunCritique_ExtractionStageInjectionGuard(t *testing.T) {
	orig := makeDraft("stage-guard-1", "Span one.", "something")
	opts := baseCritiqueOpts(t)
	// LLM response sets extraction_stage to a value it should not control.
	client := newMockClient(`{"source_span":"Span one.","what_changed":"something","extraction_stage":"reviewed"}`)

	drafts, _, err := llm.RunCritique(context.Background(), client, []schema.TraceDraft{orig}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	// Framework must override with "critiqued"; LLM-supplied "reviewed" must be discarded.
	if drafts[0].ExtractionStage != "critiqued" {
		t.Errorf("ExtractionStage: want %q (framework-assigned), got %q", "critiqued", drafts[0].ExtractionStage)
	}
}

// TestRunCritique_ArrayResponse verifies that parseCritiqueDraft correctly
// handles an LLM response wrapped in a JSON array — a common LLM behaviour
// even when a single object is requested.
func TestRunCritique_ArrayResponse(t *testing.T) {
	orig := makeDraft("arr-1", "Array span.", "something")
	opts := baseCritiqueOpts(t)
	// LLM wraps the response in an array with one element.
	client := newMockClient(`[{"source_span":"Array span.","what_changed":"a condition was described"}]`)

	drafts, rec, err := llm.RunCritique(context.Background(), client, []schema.TraceDraft{orig}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft from array response, got %d", len(drafts))
	}
	if drafts[0].DerivedFrom != orig.ID {
		t.Errorf("DerivedFrom: want %q, got %q", orig.ID, drafts[0].DerivedFrom)
	}
	if drafts[0].ExtractionStage != "critiqued" {
		t.Errorf("ExtractionStage: want %q, got %q", "critiqued", drafts[0].ExtractionStage)
	}
	if rec.DraftCount != 1 {
		t.Errorf("DraftCount: want 1, got %d", rec.DraftCount)
	}
}

// TestRunCritique_EmptyInput verifies that zero input drafts (both nil and
// empty slice) produces zero output drafts and a non-nil SessionRecord with
// DraftCount = 0 and zero LLM calls.
func TestRunCritique_EmptyInput(t *testing.T) {
	inputs := []struct {
		name   string
		drafts []schema.TraceDraft
	}{
		{"nil", nil},
		{"empty slice", []schema.TraceDraft{}},
	}
	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			opts := baseCritiqueOpts(t)
			client := newMockClient("")

			drafts, rec, err := llm.RunCritique(context.Background(), client, tc.drafts, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(drafts) != 0 {
				t.Errorf("want 0 drafts, got %d", len(drafts))
			}
			if rec.ID == "" {
				t.Error("SessionRecord.ID must be non-empty even with empty input")
			}
			if rec.DraftCount != 0 {
				t.Errorf("DraftCount: want 0, got %d", rec.DraftCount)
			}
			if client.calls != 0 {
				t.Errorf("LLM calls: want 0 for empty input, got %d", client.calls)
			}
		})
	}
}

// TestRunCritique_IDFilterNotFound verifies that when DraftID is set but no
// draft has that ID, an error is returned alongside a non-nil SessionRecord.
func TestRunCritique_IDFilterNotFound(t *testing.T) {
	d1 := makeDraft("draft-X", "Span X.", "change X")
	opts := baseCritiqueOpts(t)
	opts.DraftID = "nonexistent-id"

	_, rec, err := llm.RunCritique(context.Background(), nil, []schema.TraceDraft{d1}, opts)
	if err == nil {
		t.Fatal("want error when DraftID not found, got nil")
	}
	// SessionRecord must still be populated so the caller can inspect the failure.
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty even on ID-not-found error")
	}
	if rec.Command != "critique" {
		t.Errorf("Command: want %q, got %q", "critique", rec.Command)
	}
	if rec.ErrorNote == "" {
		t.Error("ErrorNote must record the ID-not-found reason")
	}
}
