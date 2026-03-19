// session_test.go tests the interactive review session for TraceDraft records.
//
// Tests use black-box style (package review_test) and inject io.Reader / io.Writer
// to avoid any interaction with the terminal. All inputs are provided via
// strings.NewReader; all output is captured in a bytes.Buffer so assertions
// can inspect what the session rendered.
package review_test

import (
	"bytes"
	"reflect"
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
// asserts that the derived result carries exact copies of all candidate content
// fields copied by deriveAccepted.
func TestRunReviewSession_ContentFieldsCopied(t *testing.T) {
	parent := schema.TraceDraft{
		ID:                 "id-full",
		SourceSpan:         "original span",
		SourceDocRef:       "doc-ref-1",
		WhatChanged:        "change description",
		Source:             []string{"actor-x", "actor-y"},
		Target:             []string{"actor-z"},
		Mediation:          "some mediation",
		Observer:           "analyst-1",
		Tags:               []string{"tag1", "tag2"},
		UncertaintyNote:    "somewhat uncertain",
		CriterionRef:       "c-001",
		IntentionallyBlank: []string{"observer"},
		ExtractionStage:    "weak-draft",
		ExtractedBy:        "llm-v1",
		Timestamp:          time.Now(),
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
	if !reflect.DeepEqual(r.Source, parent.Source) {
		t.Errorf("Source: want %v, got %v", parent.Source, r.Source)
	}
	if !reflect.DeepEqual(r.Target, parent.Target) {
		t.Errorf("Target: want %v, got %v", parent.Target, r.Target)
	}
	if r.Mediation != parent.Mediation {
		t.Errorf("Mediation: want %q, got %q", parent.Mediation, r.Mediation)
	}
	if r.Observer != parent.Observer {
		t.Errorf("Observer: want %q, got %q", parent.Observer, r.Observer)
	}
	if !reflect.DeepEqual(r.Tags, parent.Tags) {
		t.Errorf("Tags: want %v, got %v", parent.Tags, r.Tags)
	}
	if r.SourceSpan != parent.SourceSpan {
		t.Errorf("SourceSpan: want %q, got %q", parent.SourceSpan, r.SourceSpan)
	}
	if r.SourceDocRef != parent.SourceDocRef {
		t.Errorf("SourceDocRef: want %q, got %q", parent.SourceDocRef, r.SourceDocRef)
	}
	if r.UncertaintyNote != parent.UncertaintyNote {
		t.Errorf("UncertaintyNote: want %q, got %q", parent.UncertaintyNote, r.UncertaintyNote)
	}
	if r.CriterionRef != parent.CriterionRef {
		t.Errorf("CriterionRef: want %q, got %q", parent.CriterionRef, r.CriterionRef)
	}
	if !reflect.DeepEqual(r.IntentionallyBlank, parent.IntentionallyBlank) {
		t.Errorf("IntentionallyBlank: want %v, got %v", parent.IntentionallyBlank, r.IntentionallyBlank)
	}
}

// TestRunReviewSession_OriginalUnmodified verifies that mutating the result's
// slice fields (Source, Target, Tags) does not affect the parent draft —
// confirming deep-copy semantics for all cloned slice fields.
func TestRunReviewSession_OriginalUnmodified(t *testing.T) {
	parent := schema.TraceDraft{
		ID:              "id-orig",
		SourceSpan:      "span",
		Source:          []string{"actor-a"},
		Target:          []string{"actor-b"},
		Tags:            []string{"tag-orig"},
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

	// Mutate each slice field on the result and verify the parent is unchanged.
	results[0].Source[0] = "mutated-source"
	if parent.Source[0] != "actor-a" {
		t.Errorf("parent.Source[0] should remain %q, got %q", "actor-a", parent.Source[0])
	}
	results[0].Target[0] = "mutated-target"
	if parent.Target[0] != "actor-b" {
		t.Errorf("parent.Target[0] should remain %q, got %q", "actor-b", parent.Target[0])
	}
	results[0].Tags[0] = "mutated-tag"
	if parent.Tags[0] != "tag-orig" {
		t.Errorf("parent.Tags[0] should remain %q, got %q", "tag-orig", parent.Tags[0])
	}

	// IntentionallyBlank is also cloned — mutations must not reach the parent.
	parent2 := schema.TraceDraft{
		ID:                 "id-orig-2",
		SourceSpan:         "span2",
		IntentionallyBlank: []string{"field-x"},
		ExtractionStage:    "weak-draft",
		ExtractedBy:        "llm-v1",
	}
	drafts2 := []schema.TraceDraft{parent2}
	var out2 bytes.Buffer
	results2, err2 := review.RunReviewSession(drafts2, strings.NewReader("a\n"), &out2)
	if err2 != nil {
		t.Fatalf("unexpected error (IntentionallyBlank case): %v", err2)
	}
	if len(results2) != 1 {
		t.Fatalf("expected 1 result (IntentionallyBlank case), got %d", len(results2))
	}
	results2[0].IntentionallyBlank[0] = "mutated-blank"
	if parent2.IntentionallyBlank[0] != "field-x" {
		t.Errorf("parent2.IntentionallyBlank[0] should remain %q, got %q", "field-x", parent2.IntentionallyBlank[0])
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
	if !strings.Contains(out.String(), "<-- current") {
		t.Errorf("output should contain %q (RenderChain marker); got:\n%s", "<-- current", out.String())
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

// TestRunReviewSession_NoStageFallback verifies that when no draft has any
// ExtractionStage set, all drafts are presented (legacy dataset fallback).
// Accepting the only draft should yield 1 result derived from it.
func TestRunReviewSession_NoStageFallback(t *testing.T) {
	// No ExtractionStage set on any draft — filterReviewable should fall back
	// to presenting all drafts.
	d := schema.TraceDraft{
		ID:          "id-nostage",
		SourceSpan:  "span-nostage",
		WhatChanged: "no-stage change",
		// ExtractionStage intentionally omitted
	}
	drafts := []schema.TraceDraft{d}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("a\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (fallback presented all drafts), got %d", len(results))
	}
	if results[0].DerivedFrom != d.ID {
		t.Errorf("expected DerivedFrom %q, got %q", d.ID, results[0].DerivedFrom)
	}
}

// TestRunReviewSession_SelfDerivedNoPanic verifies that a draft whose DerivedFrom
// points to its own ID (a cycle of length 1) does not cause RunReviewSession to
// loop or panic. FollowDraftChain handles cycle detection; the session must not
// block.
func TestRunReviewSession_SelfDerivedNoPanic(t *testing.T) {
	d := schema.TraceDraft{
		ID:              "id-self",
		SourceSpan:      "span-self",
		WhatChanged:     "self-referential change",
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
		DerivedFrom:     "id-self", // points to itself
	}
	drafts := []schema.TraceDraft{d}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("s\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error on self-derived draft: %v", err)
	}
	// Skipping should yield 0 results — just confirm no panic occurred.
	if len(results) != 0 {
		t.Errorf("expected 0 results (skipped), got %d", len(results))
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

// TestRunReviewSession_EditProducesOneDerivedDraft enters edit mode, accepts
// all default values (8 Enter presses for 8 fields), and verifies that one
// derived draft is produced.
func TestRunReviewSession_EditProducesOneDerivedDraft(t *testing.T) {
	d := weakDraft("id-edit", "span-edit", "original change")
	drafts := []schema.TraceDraft{d}

	// "e\n" enters edit mode; 8 "\n" presses accept all field defaults.
	// No further input — session advances past the only draft and ends.
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n\n\n\n\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after edit, got %d", len(results))
	}
	if results[0].DerivedFrom != "id-edit" {
		t.Errorf("DerivedFrom: want %q, got %q", "id-edit", results[0].DerivedFrom)
	}
}

// TestRunReviewSession_EditDerivedProvenance verifies that an edited draft
// carries the correct provenance: DerivedFrom points to the parent, stage is
// "reviewed", ExtractedBy is "meshant-review", and a fresh ID was assigned.
func TestRunReviewSession_EditDerivedProvenance(t *testing.T) {
	d := weakDraft("parent-1", "span-p", "original")
	drafts := []schema.TraceDraft{d}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n\n\n\n\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.DerivedFrom != "parent-1" {
		t.Errorf("DerivedFrom: want %q, got %q", "parent-1", r.DerivedFrom)
	}
	if r.ExtractionStage != "reviewed" {
		t.Errorf("ExtractionStage: want %q, got %q", "reviewed", r.ExtractionStage)
	}
	if r.ExtractedBy != "meshant-review" {
		t.Errorf("ExtractedBy: want %q, got %q", "meshant-review", r.ExtractedBy)
	}
	if r.ID == "parent-1" || r.ID == "" {
		t.Errorf("result ID must be a fresh non-empty UUID distinct from parent, got %q", r.ID)
	}
}

// TestRunReviewSession_EditChangesWhatChanged enters edit mode and provides a
// new value for the what_changed field, leaving all others at their defaults.
func TestRunReviewSession_EditChangesWhatChanged(t *testing.T) {
	d := weakDraft("id-wc", "span-wc", "original")
	drafts := []schema.TraceDraft{d}

	// "e\n" = edit; "new description\n" = what_changed; remaining "\n" = default.
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\nnew description\n\n\n\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].WhatChanged != "new description" {
		t.Errorf("WhatChanged: want %q, got %q", "new description", results[0].WhatChanged)
	}
}

// TestRunReviewSession_EditKeepsUnchangedFields verifies that pressing Enter
// (empty input) for each field keeps the current value unchanged.
func TestRunReviewSession_EditKeepsUnchangedFields(t *testing.T) {
	d := schema.TraceDraft{
		ID:              "id-keep",
		SourceSpan:      "span-keep",
		WhatChanged:     "orig",
		Source:          []string{"a"},
		Mediation:       "m",
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
	}
	drafts := []schema.TraceDraft{d}

	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n\n\n\n\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.WhatChanged != "orig" {
		t.Errorf("WhatChanged: want %q, got %q", "orig", r.WhatChanged)
	}
	if len(r.Source) != 1 || r.Source[0] != "a" {
		t.Errorf("Source: want [a], got %v", r.Source)
	}
	if r.Mediation != "m" {
		t.Errorf("Mediation: want %q, got %q", "m", r.Mediation)
	}
}

// TestRunReviewSession_EditChangesSliceField verifies that a non-empty comma-
// separated input for a slice field (source) replaces the existing value.
func TestRunReviewSession_EditChangesSliceField(t *testing.T) {
	d := schema.TraceDraft{
		ID:              "id-slice",
		SourceSpan:      "span-slice",
		Source:          []string{"actor-x"},
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
	}
	drafts := []schema.TraceDraft{d}

	// Fields in order: what_changed, source, target, mediation, observer, tags,
	// uncertainty_note, criterion_ref.
	// Skip what_changed ("\n"), then set source to "actor-a, actor-b".
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n\nactor-a, actor-b\n\n\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	want := []string{"actor-a", "actor-b"}
	if !reflect.DeepEqual(results[0].Source, want) {
		t.Errorf("Source: want %v, got %v", want, results[0].Source)
	}
}

// TestRunReviewSession_EditContentFromEdited_ProvenanceFromParent verifies
// that after an edit the derived draft takes content fields from the edited
// version but provenance fields (SourceSpan, SourceDocRef, IntentionallyBlank)
// from the original parent.
func TestRunReviewSession_EditContentFromEdited_ProvenanceFromParent(t *testing.T) {
	d := schema.TraceDraft{
		ID:                 "id-prov",
		SourceSpan:         "sp",
		SourceDocRef:       "ref",
		IntentionallyBlank: []string{"x"},
		WhatChanged:        "original-what",
		ExtractionStage:    "weak-draft",
		ExtractedBy:        "llm-v1",
	}
	drafts := []schema.TraceDraft{d}

	// Edit: replace what_changed (field 1); leave the other 7 fields as defaults.
	// Field order: what_changed, source, target, mediation, observer, tags,
	// uncertainty_note, criterion_ref.
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\nedited-what\n\n\n\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]

	// Provenance fields must come from parent.
	if r.SourceSpan != "sp" {
		t.Errorf("SourceSpan: want %q, got %q", "sp", r.SourceSpan)
	}
	if r.SourceDocRef != "ref" {
		t.Errorf("SourceDocRef: want %q, got %q", "ref", r.SourceDocRef)
	}
	if !reflect.DeepEqual(r.IntentionallyBlank, []string{"x"}) {
		t.Errorf("IntentionallyBlank: want [x], got %v", r.IntentionallyBlank)
	}

	// Content fields must come from the edited values (not silently fall back to parent).
	if r.WhatChanged != "edited-what" {
		t.Errorf("WhatChanged: want %q (edited), got %q", "edited-what", r.WhatChanged)
	}
}

// TestRunReviewSession_EditThenEOFMidFlow verifies that an EOF that occurs
// inside the edit flow (before all fields are read) causes the session to
// return 0 results and nil error — no partial draft is appended.
func TestRunReviewSession_EditThenEOFMidFlow(t *testing.T) {
	d := weakDraft("id-mideof", "span-mideof", "change")
	drafts := []schema.TraceDraft{d}

	// "e\n" enters edit mode; no further input → EOF before first field.
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on mid-flow EOF, got %d", len(results))
	}
}

// TestRunReviewSession_PromptShowsEdit verifies that the session prompt
// includes the "[e]dit" option so reviewers know it is available.
func TestRunReviewSession_PromptShowsEdit(t *testing.T) {
	d := weakDraft("id-prompt", "span-prompt", "change-prompt")
	drafts := []schema.TraceDraft{d}

	var out bytes.Buffer
	_, err := review.RunReviewSession(drafts, strings.NewReader("q\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "[e]dit") {
		t.Errorf("prompt should contain %q; got:\n%s", "[e]dit", out.String())
	}
}

// TestRunReviewSession_EditChangesTargetField verifies that a non-empty comma-
// separated input for the target field (field 3) replaces the existing value.
func TestRunReviewSession_EditChangesTargetField(t *testing.T) {
	d := schema.TraceDraft{
		ID:              "id-target",
		SourceSpan:      "span-target",
		Target:          []string{"target-x"},
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
	}
	drafts := []schema.TraceDraft{d}

	// Fields: what_changed(\n), source(\n), target("target-a, target-b\n"), rest(\n*5).
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n\n\ntarget-a, target-b\n\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	want := []string{"target-a", "target-b"}
	if !reflect.DeepEqual(results[0].Target, want) {
		t.Errorf("Target: want %v, got %v", want, results[0].Target)
	}
}

// TestRunReviewSession_EditChangesTagsField verifies that a non-empty comma-
// separated input for the tags field (field 6) replaces the existing value.
func TestRunReviewSession_EditChangesTagsField(t *testing.T) {
	d := schema.TraceDraft{
		ID:              "id-tags",
		SourceSpan:      "span-tags",
		Tags:            []string{"tag-orig"},
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
	}
	drafts := []schema.TraceDraft{d}

	// Fields: what_changed(\n), source(\n), target(\n), mediation(\n), observer(\n),
	// tags("tag-new, tag-extra\n"), uncertainty_note(\n), criterion_ref(\n).
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n\n\n\n\n\ntag-new, tag-extra\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	want := []string{"tag-new", "tag-extra"}
	if !reflect.DeepEqual(results[0].Tags, want) {
		t.Errorf("Tags: want %v, got %v", want, results[0].Tags)
	}
}

// TestRunReviewSession_EditEOFAfterField2 verifies that an EOF after the second
// field (source) causes the session to return 0 results and nil error — no
// partial draft is appended regardless of how many fields were already read.
func TestRunReviewSession_EditEOFAfterField2(t *testing.T) {
	d := weakDraft("id-eof2", "span-eof2", "change")
	drafts := []schema.TraceDraft{d}

	// "e\n" = edit; "\n" = what_changed (keep); then EOF before source.
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on mid-edit EOF (after field 1), got %d", len(results))
	}
}

// TestRunReviewSession_EditEOFAfterField4 verifies that an EOF after the fourth
// field (mediation) causes the session to return 0 results and nil error.
func TestRunReviewSession_EditEOFAfterField4(t *testing.T) {
	d := weakDraft("id-eof4", "span-eof4", "change")
	drafts := []schema.TraceDraft{d}

	// "e\n" = edit; "\n"*4 = what_changed, source, target, mediation (all keep); then EOF.
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on mid-edit EOF (after field 4), got %d", len(results))
	}
}

// TestRunReviewSession_EditWhitespaceOnlyKeepsCurrent verifies that a
// whitespace-only input line (e.g. "   ") is treated as empty (keep current)
// rather than replacing the field with an empty string.
func TestRunReviewSession_EditWhitespaceOnlyKeepsCurrent(t *testing.T) {
	d := schema.TraceDraft{
		ID:              "id-ws",
		SourceSpan:      "span-ws",
		WhatChanged:     "original-ws",
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-v1",
	}
	drafts := []schema.TraceDraft{d}

	// what_changed receives "   " (whitespace only); rest are empty.
	var out bytes.Buffer
	results, err := review.RunReviewSession(drafts, strings.NewReader("e\n   \n\n\n\n\n\n\n\n"), &out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].WhatChanged != "original-ws" {
		t.Errorf("WhatChanged: whitespace-only input should keep %q, got %q", "original-ws", results[0].WhatChanged)
	}
}
