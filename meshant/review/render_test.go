package review_test

import (
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/review"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// TestRenderDraft_ContainsIndex verifies that the rendered output contains the
// index/total position indicator (e.g. "1/3") so the reviewer knows where
// they are in the queue.
func TestRenderDraft_ContainsIndex(t *testing.T) {
	d := schema.TraceDraft{WhatChanged: "something shifted"}
	out := review.RenderDraft(d, 1, 3)
	if !strings.Contains(out, "1/3") {
		t.Errorf("expected output to contain '1/3', got:\n%s", out)
	}
}

// TestRenderDraft_ContainsWhatChanged verifies that the rendered output
// contains the "what_changed:" label and the field's value.
func TestRenderDraft_ContainsWhatChanged(t *testing.T) {
	d := schema.TraceDraft{WhatChanged: "consensus collapsed"}
	out := review.RenderDraft(d, 1, 1)
	if !strings.Contains(out, "what_changed:") {
		t.Errorf("expected output to contain 'what_changed:', got:\n%s", out)
	}
	if !strings.Contains(out, "consensus collapsed") {
		t.Errorf("expected output to contain 'consensus collapsed', got:\n%s", out)
	}
}

// TestRenderDraft_ContainsExtractionStage verifies that the rendered output
// contains the "extraction_stage:" label and the field's value.
func TestRenderDraft_ContainsExtractionStage(t *testing.T) {
	d := schema.TraceDraft{ExtractionStage: "weak-draft"}
	out := review.RenderDraft(d, 2, 5)
	if !strings.Contains(out, "extraction_stage:") {
		t.Errorf("expected output to contain 'extraction_stage:', got:\n%s", out)
	}
	if !strings.Contains(out, "weak-draft") {
		t.Errorf("expected output to contain 'weak-draft', got:\n%s", out)
	}
}

// TestRenderDraft_EmptyFieldsRendered verifies that a zero-value draft renders
// without panic, and that field labels are still present in the output.
// Empty values should appear as "(empty)" rather than being omitted.
func TestRenderDraft_EmptyFieldsRendered(t *testing.T) {
	d := schema.TraceDraft{}
	// Should not panic.
	out := review.RenderDraft(d, 1, 1)
	expectedLabels := []string{
		"what_changed:",
		"source:",
		"target:",
		"mediation:",
		"observer:",
		"tags:",
		"extraction_stage:",
		"extracted_by:",
		"uncertainty_note:",
		"intentionally_blank:",
		"derived_from:",
		"criterion_ref:",
	}
	for _, label := range expectedLabels {
		if !strings.Contains(out, label) {
			t.Errorf("expected output to contain label %q, got:\n%s", label, out)
		}
	}
}

// TestRenderDraft_SliceFieldsPopulated verifies that non-empty slice fields
// (Source, Tags) are rendered as comma-joined values in the output.
// This exercises the non-empty branch of sliceOrEmpty.
func TestRenderDraft_SliceFieldsPopulated(t *testing.T) {
	d := schema.TraceDraft{
		Source: []string{"actor-a", "actor-b"},
		Tags:   []string{"policy", "consent"},
	}
	out := review.RenderDraft(d, 1, 1)
	if !strings.Contains(out, "actor-a, actor-b") {
		t.Errorf("expected comma-joined source 'actor-a, actor-b' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "policy, consent") {
		t.Errorf("expected comma-joined tags 'policy, consent' in output, got:\n%s", out)
	}
}

// TestRenderDraft_EmptyPlaceholder verifies that blank fields render as
// "(empty)" rather than being omitted from the output.
func TestRenderDraft_EmptyPlaceholder(t *testing.T) {
	d := schema.TraceDraft{}
	out := review.RenderDraft(d, 1, 1)
	if !strings.Contains(out, "(empty)") {
		t.Errorf("expected '(empty)' placeholder for blank fields, got:\n%s", out)
	}
}

// TestRenderAmbiguities_EmptySlice verifies that a non-nil empty slice also
// renders "(none)" — same contract as nil.
func TestRenderAmbiguities_EmptySlice(t *testing.T) {
	out := review.RenderAmbiguities([]review.AmbiguityWarning{})
	if !strings.Contains(strings.ToLower(out), "none") {
		t.Errorf("expected '(none)' for empty non-nil slice, got: %q", out)
	}
}

// TestRenderAmbiguities_NoWarnings verifies that an empty warnings slice
// renders a clear "(none)" (or similar) indicator rather than blank output.
func TestRenderAmbiguities_NoWarnings(t *testing.T) {
	out := review.RenderAmbiguities(nil)
	lowerOut := strings.ToLower(out)
	if !strings.Contains(lowerOut, "none") {
		t.Errorf("expected '(none)' or similar for empty warnings, got: %q", out)
	}
}

// TestRenderAmbiguities_ContainsWarningMessage verifies that when warnings are
// present, the warning's Message text appears in the rendered output.
func TestRenderAmbiguities_ContainsWarningMessage(t *testing.T) {
	warnings := []review.AmbiguityWarning{
		{
			Field:   "what_changed",
			Message: "what_changed is unregistered from this position — the nature of the change is in shadow",
		},
	}
	out := review.RenderAmbiguities(warnings)
	if !strings.Contains(out, "what_changed is unregistered from this position") {
		t.Errorf("expected warning message in output, got:\n%s", out)
	}
}

// TestRenderChain_EmptyChain verifies that a nil chain returns a non-empty
// string containing a "no derivation chain" notice rather than panicking.
func TestRenderChain_EmptyChain(t *testing.T) {
	out := review.RenderChain(nil, nil)
	if out == "" {
		t.Fatal("expected non-empty output for nil chain, got empty string")
	}
	if !strings.Contains(strings.ToLower(out), "no derivation chain") {
		t.Errorf("expected 'no derivation chain' notice, got:\n%s", out)
	}
}

// TestRenderChain_SingleDraft verifies that a one-element chain renders the
// truncated ID and marks the draft as "current". No classification lines
// should appear because there are no derivation steps between drafts.
func TestRenderChain_SingleDraft(t *testing.T) {
	chain := []schema.TraceDraft{
		{ID: "abc12345-0000-0000-0000-000000000000", WhatChanged: "policy shifted"},
	}
	out := review.RenderChain(chain, nil)
	if !strings.Contains(out, "abc12345") {
		t.Errorf("expected truncated ID 'abc12345' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "current") {
		t.Errorf("expected 'current' marker for single draft, got:\n%s", out)
	}
	lower := strings.ToLower(out)
	for _, word := range []string{"intermediary", "mediator", "translation"} {
		if strings.Contains(lower, word) {
			t.Errorf("expected no classification lines for single draft, but found %q in output:\n%s", word, out)
		}
	}
}

// TestRenderChain_TwoDrafts verifies that a two-element chain with one
// mediator classification renders the kind and reason text, and marks the
// last draft as "current".
func TestRenderChain_TwoDrafts(t *testing.T) {
	chain := []schema.TraceDraft{
		{ID: "aaaa0000-0000-0000-0000-000000000000", WhatChanged: "initial"},
		{ID: "bbbb1111-0000-0000-0000-000000000000", WhatChanged: "reformulated"},
	}
	classifications := []loader.DraftStepClassification{
		{StepIndex: 1, Kind: loader.DraftMediator, Reason: "content fields reformulated"},
	}
	out := review.RenderChain(chain, classifications)
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "mediator") {
		t.Errorf("expected 'mediator' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "content fields reformulated") {
		t.Errorf("expected reason 'content fields reformulated' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "current") {
		t.Errorf("expected 'current' marker on last draft, got:\n%s", out)
	}
}

// TestRenderChain_MultiStep verifies that a four-element chain with three
// classifications renders all three kind strings and places the "current"
// marker exactly once on the final draft.
func TestRenderChain_MultiStep(t *testing.T) {
	chain := []schema.TraceDraft{
		{ID: "d0000000-0000-0000-0000-000000000000"},
		{ID: "d1111111-0000-0000-0000-000000000000"},
		{ID: "d2222222-0000-0000-0000-000000000000"},
		{ID: "d3333333-0000-0000-0000-000000000000"},
	}
	classifications := []loader.DraftStepClassification{
		{StepIndex: 1, Kind: loader.DraftIntermediary, Reason: "no content fields changed"},
		{StepIndex: 2, Kind: loader.DraftMediator, Reason: "content fields reformulated"},
		{StepIndex: 3, Kind: loader.DraftTranslation, Reason: "content and stage changed"},
	}
	out := review.RenderChain(chain, classifications)
	lower := strings.ToLower(out)
	for _, kind := range []string{"intermediary", "mediator", "translation"} {
		if !strings.Contains(lower, kind) {
			t.Errorf("expected kind %q in output, got:\n%s", kind, out)
		}
	}
	// "current" must appear exactly once (on the last draft only).
	count := strings.Count(out, "current")
	if count != 1 {
		t.Errorf("expected exactly 1 'current' marker, got %d in output:\n%s", count, out)
	}
}

// TestRenderChain_TruncatesWhatChanged verifies that a what_changed value
// longer than the truncation limit is shortened and marked with "...".
func TestRenderChain_TruncatesWhatChanged(t *testing.T) {
	long := strings.Repeat("x", 80)
	chain := []schema.TraceDraft{
		{ID: "e0000000-0000-0000-0000-000000000000", WhatChanged: long},
	}
	out := review.RenderChain(chain, nil)
	if !strings.Contains(out, "...") {
		t.Errorf("expected truncation marker '...' in output, got:\n%s", out)
	}
	if strings.Contains(out, long) {
		t.Errorf("expected full 80-char string to be absent (truncated), but found it in output:\n%s", out)
	}
}

// TestRenderChain_OutOfRangeStepIndex verifies that a classification with a
// StepIndex that does not correspond to any chain position (e.g. 0 or 99) is
// silently omitted. The output must not panic and must still render all drafts.
func TestRenderChain_OutOfRangeStepIndex(t *testing.T) {
	chain := []schema.TraceDraft{
		{ID: "f0000000-0000-0000-0000-000000000000", WhatChanged: "initial"},
		{ID: "f1111111-0000-0000-0000-000000000000", WhatChanged: "derived"},
	}
	// StepIndex 0 and 99 are both out of range for this 2-element chain.
	// The valid step is StepIndex 1; both invalid entries must be silently dropped.
	classifications := []loader.DraftStepClassification{
		{StepIndex: 0, Kind: loader.DraftMediator, Reason: "should not appear (StepIndex 0)"},
		{StepIndex: 99, Kind: loader.DraftTranslation, Reason: "should not appear (StepIndex 99)"},
	}
	out := review.RenderChain(chain, classifications)
	if strings.Contains(out, "should not appear") {
		t.Errorf("expected out-of-range classification to be silently omitted, but found it in output:\n%s", out)
	}
	// Both drafts must still render.
	if !strings.Contains(out, "f0000000") {
		t.Errorf("expected first draft 'f0000000' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "f1111111") {
		t.Errorf("expected second draft 'f1111111' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "current") {
		t.Errorf("expected 'current' marker on last draft, got:\n%s", out)
	}
}

// TestRenderChain_PartialClassifications verifies that when a non-nil
// classifications slice omits a middle step, the present steps still render
// and the absent step produces no classification line.
func TestRenderChain_PartialClassifications(t *testing.T) {
	chain := []schema.TraceDraft{
		{ID: "g0000000-0000-0000-0000-000000000000"},
		{ID: "g1111111-0000-0000-0000-000000000000"},
		{ID: "g2222222-0000-0000-0000-000000000000"},
	}
	// Only StepIndex 2 is provided; StepIndex 1 (chain[0]→chain[1]) is absent.
	classifications := []loader.DraftStepClassification{
		{StepIndex: 2, Kind: loader.DraftTranslation, Reason: "stage advanced"},
	}
	out := review.RenderChain(chain, classifications)
	// Step 2 classification must appear.
	if !strings.Contains(out, "stage advanced") {
		t.Errorf("expected 'stage advanced' from StepIndex 2, got:\n%s", out)
	}
	// All three drafts must render.
	if !strings.Contains(out, "g0000000") || !strings.Contains(out, "g1111111") || !strings.Contains(out, "g2222222") {
		t.Errorf("expected all three draft IDs in output, got:\n%s", out)
	}
}

// TestRenderChain_TruncatesID verifies that only the first 8 characters of an
// ID appear in the output — not a longer prefix.
func TestRenderChain_TruncatesID(t *testing.T) {
	chain := []schema.TraceDraft{
		{ID: "abcdefgh-1234-5678-9012-345678901234"},
	}
	out := review.RenderChain(chain, nil)
	if !strings.Contains(out, "abcdefgh") {
		t.Errorf("expected first 8 chars 'abcdefgh' in output, got:\n%s", out)
	}
	if strings.Contains(out, "abcdefgh-1234") {
		t.Errorf("expected ID to be truncated to 8 chars, but found longer prefix 'abcdefgh-1234' in output:\n%s", out)
	}
}
