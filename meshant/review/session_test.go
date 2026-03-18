// session_test.go tests the interactive review session for TraceDraft records.
//
// Tests use black-box style (package review_test) and inject io.Reader / io.Writer
// to avoid any interaction with the terminal. All inputs are provided via
// strings.NewReader; all output is captured in a bytes.Buffer so assertions
// can inspect what the session rendered.
package review_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/review"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// weakDraft returns a minimal TraceDraft with ExtractionStage "weak-draft".
func weakDraft(id, sourceSpan, whatChanged string) schema.TraceDraft {
	return schema.TraceDraft{
		ID:              id,
		SourceSpan:      sourceSpan,
		WhatChanged:     whatChanged,
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
	}
}

// TestRunReviewSession_AcceptOne presents 2 weak-draft drafts, accepts the
// first, then quits. Asserts that one result is returned and carries the
// correct provenance.
func TestRunReviewSession_AcceptOne(t *testing.T) {
	d1 := weakDraft("id-a", "span-a", "change-a")
	d2 := weakDraft("id-b", "span-b", "change-b")
	drafts := []schema.TraceDraft{d1, d2}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("a\nq\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.DerivedFrom != d1.ID {
		t.Errorf("DerivedFrom: want %q, got %q", d1.ID, r.DerivedFrom)
	}
	if r.ExtractionStage != "reviewed" {
		t.Errorf("ExtractionStage: want %q, got %q", "reviewed", r.ExtractionStage)
	}
	if r.ExtractedBy != "meshant-review" {
		t.Errorf("ExtractedBy: want %q, got %q", "meshant-review", r.ExtractedBy)
	}
	if r.ID == "" {
		t.Error("result ID must not be empty")
	}
	if r.ID == d1.ID {
		t.Errorf("result ID must differ from parent ID, both are %q", r.ID)
	}
}

// TestRunReviewSession_AcceptAll accepts both drafts in a two-draft session.
func TestRunReviewSession_AcceptAll(t *testing.T) {
	d1 := weakDraft("id-1", "span-1", "change-1")
	d2 := weakDraft("id-2", "span-2", "change-2")
	drafts := []schema.TraceDraft{d1, d2}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("a\na\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].DerivedFrom != d1.ID {
		t.Errorf("results[0].DerivedFrom: want %q, got %q", d1.ID, results[0].DerivedFrom)
	}
	if results[1].DerivedFrom != d2.ID {
		t.Errorf("results[1].DerivedFrom: want %q, got %q", d2.ID, results[1].DerivedFrom)
	}
}

// TestRunReviewSession_SkipThenQuit skips the first draft and quits on the
// second. No results should be returned.
func TestRunReviewSession_SkipThenQuit(t *testing.T) {
	d1 := weakDraft("id-1", "span-1", "change-1")
	d2 := weakDraft("id-2", "span-2", "change-2")
	drafts := []schema.TraceDraft{d1, d2}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("s\nq\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestRunReviewSession_QuitImmediately quits immediately on the first draft.
func TestRunReviewSession_QuitImmediately(t *testing.T) {
	d1 := weakDraft("id-1", "span-1", "change-1")
	drafts := []schema.TraceDraft{d1}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("q\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestRunReviewSession_UnknownInputReprompts sends two unknown inputs before
// accepting. The output must contain "unknown" to confirm the re-prompt
// message was written.
func TestRunReviewSession_UnknownInputReprompts(t *testing.T) {
	d1 := weakDraft("id-1", "span-1", "change-1")
	drafts := []schema.TraceDraft{d1}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("x\nz\na\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(out.String(), "unknown") {
		t.Errorf("output should contain %q for re-prompt; got:\n%s", "unknown", out.String())
	}
}

// TestRunReviewSession_EmptyDrafts passes an empty slice. The session should
// return (nil, nil) and write a "no drafts" message.
func TestRunReviewSession_EmptyDrafts(t *testing.T) {
	var out bytes.Buffer
	results, err := review.RunReviewSession([]schema.TraceDraft{}, strings.NewReader("q\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if !strings.Contains(strings.ToLower(out.String()), "no drafts") {
		t.Errorf("output should contain %q; got:\n%s", "no drafts", out.String())
	}
}

// TestRunReviewSession_AllAlreadyReviewed passes a draft whose stage is
// "reviewed". The session should treat it as not reviewable and emit
// "no drafts".
func TestRunReviewSession_AllAlreadyReviewed(t *testing.T) {
	d := schema.TraceDraft{
		ID:              "id-already",
		SourceSpan:      "span",
		WhatChanged:     "done",
		ExtractionStage: "reviewed",
		ExtractedBy:     "meshant-review",
	}
	drafts := []schema.TraceDraft{d}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("q\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if !strings.Contains(strings.ToLower(out.String()), "no drafts") {
		t.Errorf("output should contain %q; got:\n%s", "no drafts", out.String())
	}
}

// TestRunReviewSession_ContentFieldsCopied accepts a fully-populated draft and
// asserts that the derived result carries exact copies of all content fields.
func TestRunReviewSession_ContentFieldsCopied(t *testing.T) {
	parent := schema.TraceDraft{
		ID:              "id-full",
		SourceSpan:      "original span",
		WhatChanged:     "change description",
		Source:          []string{"actor-x", "actor-y"},
		Target:          []string{"actor-z"},
		Mediation:       "some mediation",
		Observer:        "analyst-1",
		Tags:            []string{"tag1"},
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
		Timestamp:       time.Now(),
	}
	drafts := []schema.TraceDraft{parent}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("a\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.WhatChanged != parent.WhatChanged {
		t.Errorf("WhatChanged: want %q, got %q", parent.WhatChanged, r.WhatChanged)
	}
	if len(r.Source) != len(parent.Source) || r.Source[0] != parent.Source[0] {
		t.Errorf("Source: want %v, got %v", parent.Source, r.Source)
	}
	if r.Mediation != parent.Mediation {
		t.Errorf("Mediation: want %q, got %q", parent.Mediation, r.Mediation)
	}
	if r.Observer != parent.Observer {
		t.Errorf("Observer: want %q, got %q", parent.Observer, r.Observer)
	}
	if r.SourceSpan != parent.SourceSpan {
		t.Errorf("SourceSpan: want %q, got %q", parent.SourceSpan, r.SourceSpan)
	}
}

// TestRunReviewSession_OriginalUnmodified verifies that mutating the result's
// Source slice does not affect the parent draft's Source slice — confirming
// deep-copy semantics.
func TestRunReviewSession_OriginalUnmodified(t *testing.T) {
	parent := schema.TraceDraft{
		ID:              "id-orig",
		SourceSpan:      "span",
		Source:          []string{"actor-a"},
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
	}
	drafts := []schema.TraceDraft{parent}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("a\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Mutate the result's Source and confirm the original draft is unchanged.
	results[0].Source[0] = "mutated"
	if parent.Source[0] != "actor-a" {
		t.Errorf("parent.Source[0] should remain %q after mutating result, got %q", "actor-a", parent.Source[0])
	}
}

// TestRunReviewSession_OutputContainsChain presents a draft that is part of a
// chain (another draft derives from it). The output should contain "current"
// because RenderChain marks the last draft with "<-- current".
func TestRunReviewSession_OutputContainsChain(t *testing.T) {
	// d1 is the root. d2 derives from d1 — so when d1 is reviewed, the chain
	// renders both d1 and d2, marking d2 as "current".
	d1 := weakDraft("id-chain-root", "span-root", "root change")
	d2 := schema.TraceDraft{
		ID:              "id-chain-child",
		SourceSpan:      "span-child",
		WhatChanged:     "child change",
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
		DerivedFrom:     "id-chain-root",
	}
	drafts := []schema.TraceDraft{d1, d2}

	var out bytes.Buffer
	// Skip d1 only — we're testing the output content on first presentation.
	_, err := review.RunReviewSession(drafts, strings.NewReader("s\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "current") {
		t.Errorf("output should contain %q for chain rendering; got:\n%s", "current", out.String())
	}
}

// TestRunReviewSession_OutputContainsAmbiguities presents a draft with an empty
// WhatChanged field (an ambiguity). The output should surface the field name
// "what_changed" in the ambiguity section.
func TestRunReviewSession_OutputContainsAmbiguities(t *testing.T) {
	d := schema.TraceDraft{
		ID:              "id-ambig",
		SourceSpan:      "span",
		WhatChanged:     "", // blank — should trigger ambiguity warning
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
	}
	drafts := []schema.TraceDraft{d}

	var out bytes.Buffer
	_, err := review.RunReviewSession(drafts, strings.NewReader("s\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "what_changed") {
		t.Errorf("output should contain %q for ambiguity warning; got:\n%s", "what_changed", out.String())
	}
}

// TestRunReviewSession_FilterWeakDraftOnly presents two drafts: one with
// ExtractionStage "weak-draft" and one with "span-harvest". Only the
// weak-draft should be presented. Accepting should yield 1 result.
func TestRunReviewSession_FilterWeakDraftOnly(t *testing.T) {
	d1 := weakDraft("id-weak", "span-weak", "change-weak")
	d2 := schema.TraceDraft{
		ID:              "id-harvest",
		SourceSpan:      "span-harvest",
		WhatChanged:     "harvest change",
		ExtractionStage: "span-harvest",
		ExtractedBy:     "harvester-v1",
	}
	drafts := []schema.TraceDraft{d1, d2}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("a\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only weak-draft presented), got %d", len(results))
	}
	if results[0].DerivedFrom != d1.ID {
		t.Errorf("expected result derived from weak-draft %q, got %q", d1.ID, results[0].DerivedFrom)
	}
}

// TestRunReviewSession_AcceptThenEOF verifies that when one draft is accepted
// and the reader hits EOF before a valid action is provided for the next draft,
// the accepted result is returned and no error is raised.
func TestRunReviewSession_AcceptThenEOF(t *testing.T) {
	d1 := weakDraft("id-eof1", "span-eof1", "first change")
	d2 := weakDraft("id-eof2", "span-eof2", "second change")
	drafts := []schema.TraceDraft{d1, d2}

	// "a\n" accepts the first draft; no further input → EOF on the second prompt.
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("a\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (first draft accepted, EOF on second), got %d", len(results))
	}
	if results[0].DerivedFrom != d1.ID {
		t.Errorf("expected result derived from %q, got %q", d1.ID, results[0].DerivedFrom)
	}
}

// TestRunReviewSession_EOFTreatedAsQuit passes an empty reader (immediate EOF).
// The session should return (nil, nil) without error — EOF is treated as quit.
func TestRunReviewSession_EOFTreatedAsQuit(t *testing.T) {
	d1 := weakDraft("id-eof", "span-eof", "change-eof")
	drafts := []schema.TraceDraft{d1}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader(""), &out)

	if err != nil {
		t.Fatalf("unexpected error on EOF: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on EOF, got %d", len(results))
	}
}
