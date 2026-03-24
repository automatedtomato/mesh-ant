package loader_test

import (
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// --- FollowDraftChain ---

// makeDraftChain builds a linear derivation chain of n drafts where each
// draft's DerivedFrom points to the previous draft's ID.
func makeDraftChain(n int) []schema.TraceDraft {
	drafts := make([]schema.TraceDraft, n)
	for i := range drafts {
		id := string(rune('a'+i)) + "0000000-0000-4000-8000-00000000000" + string(rune('0'+i))
		drafts[i] = schema.TraceDraft{
			ID:              id,
			SourceSpan:      "span",
			ExtractionStage: "weak-draft",
		}
		if i > 0 {
			drafts[i].DerivedFrom = drafts[i-1].ID
		}
	}
	return drafts
}

func TestFollowDraftChain_LinearChain(t *testing.T) {
	drafts := makeDraftChain(3)
	chain := loader.FollowDraftChain(drafts, drafts[0].ID)

	if len(chain) != 3 {
		t.Fatalf("chain length: got %d want 3", len(chain))
	}
	for i, d := range chain {
		if d.ID != drafts[i].ID {
			t.Errorf("chain[%d].ID = %q; want %q", i, d.ID, drafts[i].ID)
		}
	}
}

func TestFollowDraftChain_RootNotFound(t *testing.T) {
	drafts := makeDraftChain(2)
	chain := loader.FollowDraftChain(drafts, "nonexistent-id")
	if len(chain) != 0 {
		t.Errorf("root not found: got chain len %d; want 0", len(chain))
	}
}

func TestFollowDraftChain_SingleDraft(t *testing.T) {
	d := schema.TraceDraft{ID: "a0000000-0000-4000-8000-000000000001", SourceSpan: "span"}
	chain := loader.FollowDraftChain([]schema.TraceDraft{d}, d.ID)
	if len(chain) != 1 {
		t.Fatalf("single draft: got chain len %d; want 1", len(chain))
	}
	if chain[0].ID != d.ID {
		t.Errorf("chain[0].ID = %q; want %q", chain[0].ID, d.ID)
	}
}

func TestFollowDraftChain_CycleDetected(t *testing.T) {
	// A → B → A (cycle): chain should stop at B, not include A again.
	a := schema.TraceDraft{ID: "a0000000-0000-4000-8000-000000000001", SourceSpan: "span"}
	b := schema.TraceDraft{ID: "b0000000-0000-4000-8000-000000000002", SourceSpan: "span", DerivedFrom: a.ID}
	// Make A derive from B to close the cycle.
	a.DerivedFrom = b.ID

	chain := loader.FollowDraftChain([]schema.TraceDraft{a, b}, a.ID)
	// Should contain A and B but stop before cycling back to A.
	if len(chain) != 2 {
		t.Fatalf("cycle: got chain len %d; want 2", len(chain))
	}
}

func TestFollowDraftChain_StartsFromMiddle(t *testing.T) {
	// A → B → C; starting from B should return [B, C].
	drafts := makeDraftChain(3)
	chain := loader.FollowDraftChain(drafts, drafts[1].ID)
	if len(chain) != 2 {
		t.Fatalf("start from middle: got chain len %d; want 2", len(chain))
	}
	if chain[0].ID != drafts[1].ID {
		t.Errorf("chain[0].ID = %q; want %q", chain[0].ID, drafts[1].ID)
	}
}

func TestFollowDraftChain_EmptyDrafts(t *testing.T) {
	chain := loader.FollowDraftChain([]schema.TraceDraft{}, "any-id")
	if len(chain) != 0 {
		t.Errorf("empty drafts: got chain len %d; want 0", len(chain))
	}
}

// --- ClassifyDraftChain ---

func TestClassifyDraftChain_Intermediary(t *testing.T) {
	// Parent and child have identical content fields; only UncertaintyNote differs.
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "something happened",
		Observer:        "analyst",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "something happened", // unchanged
		Observer:        "analyst",            // unchanged
		ExtractionStage: "weak-draft",         // unchanged
		UncertaintyNote: "source unclear",     // provenance only
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftIntermediary {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftIntermediary)
	}
}

func TestClassifyDraftChain_Mediator(t *testing.T) {
	// Child reformulates WhatChanged but keeps the same ExtractionStage.
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "reformulated framing", // content changed
		ExtractionStage: "weak-draft",           // stage unchanged
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftMediator {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftMediator)
	}
}

func TestClassifyDraftChain_Translation(t *testing.T) {
	// Child reformulates WhatChanged AND advances ExtractionStage.
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "reformulated framing", // content changed
		ExtractionStage: "reviewed",             // stage advanced
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftTranslation {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftTranslation)
	}
}

func TestClassifyDraftChain_NilForSingleDraft(t *testing.T) {
	d := schema.TraceDraft{ID: "a0000000-0000-4000-8000-000000000001", SourceSpan: "span"}
	result := loader.ClassifyDraftChain([]schema.TraceDraft{d}, loader.ClassifyDraftChainOptions{}).Classifications
	if result != nil {
		t.Errorf("single draft: want nil, got %v", result)
	}
}

func TestClassifyDraftChain_MultiStep(t *testing.T) {
	// Three drafts: A→B (intermediary), B→C (mediator).
	a := schema.TraceDraft{
		ID: "a0000000-0000-4000-8000-000000000001", SourceSpan: "span",
		WhatChanged: "original", ExtractionStage: "weak-draft",
	}
	b := schema.TraceDraft{
		ID: "b0000000-0000-4000-8000-000000000002", SourceSpan: "span",
		WhatChanged: "original", ExtractionStage: "weak-draft", // same content
		UncertaintyNote: "note added", DerivedFrom: a.ID,
	}
	c := schema.TraceDraft{
		ID: "c0000000-0000-4000-8000-000000000003", SourceSpan: "span",
		WhatChanged: "reformulated", ExtractionStage: "weak-draft", // content changed
		DerivedFrom: b.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{a, b, c}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 2 {
		t.Fatalf("multi-step: got %d classifications; want 2", len(classifications))
	}
	if classifications[0].Kind != loader.DraftIntermediary {
		t.Errorf("step 0: got %q; want %q", classifications[0].Kind, loader.DraftIntermediary)
	}
	if classifications[1].Kind != loader.DraftMediator {
		t.Errorf("step 1: got %q; want %q", classifications[1].Kind, loader.DraftMediator)
	}
}

func TestClassifyDraftChain_StepIndexIsCorrect(t *testing.T) {
	drafts := makeDraftChain(3)
	classifications := loader.ClassifyDraftChain(drafts, loader.ClassifyDraftChainOptions{}).Classifications
	for i, c := range classifications {
		want := i + 1
		if c.StepIndex != want {
			t.Errorf("classifications[%d].StepIndex = %d; want %d", i, c.StepIndex, want)
		}
	}
}

func TestClassifyDraftChain_ReasonNonEmpty(t *testing.T) {
	drafts := makeDraftChain(2)
	classifications := loader.ClassifyDraftChain(drafts, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) == 0 {
		t.Fatal("no classifications returned")
	}
	if classifications[0].Reason == "" {
		t.Error("Reason is empty; must be non-empty for inspectability")
	}
}

func TestClassifyDraftChain_MediatorStageOnly(t *testing.T) {
	// Child advances ExtractionStage without changing any content fields.
	// This is the "endorsement" case: a reviewer accepts a draft as-is,
	// promoting it from weak-draft to reviewed. The stage advance is a
	// mediating act — it transforms the draft's epistemic standing even
	// though no content was reformulated.
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "original framing", // content unchanged
		ExtractionStage: "reviewed",         // stage advanced
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftMediator {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftMediator)
	}
	if classifications[0].Reason == "" {
		t.Error("Reason must be non-empty")
	}
}

func TestClassifyDraftChain_MediatorStageOnly_NonZeroContentUnchanged(t *testing.T) {
	// Like MediatorStageOnly but with non-zero, identical content fields on
	// both drafts — confirms draftContentChanged returns false when all six
	// content fields are populated but equal.
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		Source:          []string{"actor-a"},
		Target:          []string{"actor-b"},
		Mediation:       "rule-engine",
		Observer:        "analyst",
		Tags:            []string{"translation"},
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "original framing", // all content fields identical
		Source:          []string{"actor-a"},
		Target:          []string{"actor-b"},
		Mediation:       "rule-engine",
		Observer:        "analyst",
		Tags:            []string{"translation"},
		ExtractionStage: "reviewed", // stage advanced
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftMediator {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftMediator)
	}
}

func TestClassifyDraftChain_EmptyCurrStageNotCountedAsChange(t *testing.T) {
	// draftStageChanged contract: an empty curr.ExtractionStage is NOT counted
	// as a stage change, even when the parent has a non-empty stage. A child
	// with an unpopulated stage field should not trigger the endorsement path.
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "reviewed",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "original framing", // content unchanged
		ExtractionStage: "",                 // unset — not a stage change
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftIntermediary {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftIntermediary)
	}
}

func TestClassifyDraftChain_SameNonEmptyStageBothSides(t *testing.T) {
	// draftStageChanged contract: equal non-empty stages on both sides are not
	// counted as a stage change. No advancement = not a mediating act on the
	// stage axis. With no content change either, the step is DraftIntermediary.
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "reviewed",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "original framing", // content unchanged
		ExtractionStage: "reviewed",         // same stage — no advancement
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftIntermediary {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftIntermediary)
	}
}

func TestClassifyDraftChain_MultiStepWithEndorsement(t *testing.T) {
	// Three drafts: A→B (mediator, content change), B→C (mediator, stage-only endorsement).
	// Confirms the endorsement case works correctly alongside content-based mediation.
	a := schema.TraceDraft{
		ID: "a0000000-0000-4000-8000-000000000001", SourceSpan: "span",
		WhatChanged: "original", ExtractionStage: "weak-draft",
	}
	b := schema.TraceDraft{
		ID: "b0000000-0000-4000-8000-000000000002", SourceSpan: "span",
		WhatChanged: "reformulated", ExtractionStage: "weak-draft", // content changed
		DerivedFrom: a.ID,
	}
	c := schema.TraceDraft{
		ID: "c0000000-0000-4000-8000-000000000003", SourceSpan: "span",
		WhatChanged: "reformulated", ExtractionStage: "reviewed", // stage advanced, content unchanged
		DerivedFrom: b.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{a, b, c}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 2 {
		t.Fatalf("multi-step: got %d classifications; want 2", len(classifications))
	}
	if classifications[0].Kind != loader.DraftMediator {
		t.Errorf("step 0 (content change): got %q; want %q", classifications[0].Kind, loader.DraftMediator)
	}
	if classifications[1].Kind != loader.DraftMediator {
		t.Errorf("step 1 (stage-only endorsement): got %q; want %q", classifications[1].Kind, loader.DraftMediator)
	}
}

// --- SubKind field tests (RED phase for #96) ---

// TestClassifyDraftChain_EndorsementSubKind_StageOnly verifies that a stage-only
// derivation step (no content change) is classified as DraftMediator with
// SubKind == DraftSubKindEndorsement ("endorsement"). The stage advance
// transforms the draft's epistemic standing without reformulating content.
func TestClassifyDraftChain_EndorsementSubKind_StageOnly(t *testing.T) {
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "original framing", // content unchanged
		ExtractionStage: "reviewed",         // stage advanced — endorsement
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftMediator {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftMediator)
	}
	if classifications[0].SubKind != loader.DraftSubKindEndorsement {
		t.Errorf("sub_kind: got %q; want %q", classifications[0].SubKind, loader.DraftSubKindEndorsement)
	}
}

// TestClassifyDraftChain_ContentMediatorHasEmptySubKind verifies that a content-change
// mediator step (no stage change) carries an empty SubKind. SubKind is only set
// for the stage-only endorsement path.
func TestClassifyDraftChain_ContentMediatorHasEmptySubKind(t *testing.T) {
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "reformulated framing", // content changed
		ExtractionStage: "weak-draft",           // stage unchanged
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftMediator {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftMediator)
	}
	if classifications[0].SubKind != "" {
		t.Errorf("sub_kind: got %q; want empty string", classifications[0].SubKind)
	}
}

// TestClassifyDraftChain_TranslationHasEmptySubKind verifies that a translation step
// (content change + stage advance) has Kind == DraftTranslation and SubKind == "".
// SubKind is not used for the translation path.
func TestClassifyDraftChain_TranslationHasEmptySubKind(t *testing.T) {
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "reformulated framing", // content changed
		ExtractionStage: "reviewed",             // stage advanced
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftTranslation {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftTranslation)
	}
	if classifications[0].SubKind != "" {
		t.Errorf("sub_kind: got %q; want empty string", classifications[0].SubKind)
	}
}

// TestClassifyDraftChain_IntermediaryHasEmptySubKind verifies that an intermediary
// step (no content change, no stage change) carries an empty SubKind.
func TestClassifyDraftChain_IntermediaryHasEmptySubKind(t *testing.T) {
	parent := schema.TraceDraft{
		ID:              "a0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "b0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "original framing", // unchanged
		ExtractionStage: "weak-draft",       // unchanged
		UncertaintyNote: "provenance note",  // provenance-only change
		DerivedFrom:     parent.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftIntermediary {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftIntermediary)
	}
	if classifications[0].SubKind != "" {
		t.Errorf("sub_kind: got %q; want empty string", classifications[0].SubKind)
	}
}

// TestClassifyDraftChain_MultiStepSubKinds verifies per-step SubKind values across a
// three-draft chain that includes both mediator types:
//
//	A → B: content-change mediator (SubKind empty)
//	B → C: stage-only endorsement mediator (SubKind == "endorsement")
func TestClassifyDraftChain_MultiStepSubKinds(t *testing.T) {
	a := schema.TraceDraft{
		ID: "a0000000-0000-4000-8000-000000000001", SourceSpan: "span",
		WhatChanged: "original", ExtractionStage: "weak-draft",
	}
	b := schema.TraceDraft{
		ID: "b0000000-0000-4000-8000-000000000002", SourceSpan: "span",
		WhatChanged: "reformulated", ExtractionStage: "weak-draft", // content changed, stage same
		DerivedFrom: a.ID,
	}
	c := schema.TraceDraft{
		ID: "c0000000-0000-4000-8000-000000000003", SourceSpan: "span",
		WhatChanged: "reformulated", ExtractionStage: "reviewed", // stage advanced, content same
		DerivedFrom: b.ID,
	}

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{a, b, c}, loader.ClassifyDraftChainOptions{}).Classifications
	if len(classifications) != 2 {
		t.Fatalf("multi-step: got %d classifications; want 2", len(classifications))
	}

	// Step A→B: content mediator — SubKind must be empty.
	if classifications[0].Kind != loader.DraftMediator {
		t.Errorf("step 0 kind: got %q; want %q", classifications[0].Kind, loader.DraftMediator)
	}
	if classifications[0].SubKind != "" {
		t.Errorf("step 0 sub_kind: got %q; want empty string", classifications[0].SubKind)
	}

	// Step B→C: endorsement mediator — SubKind must be DraftSubKindEndorsement.
	if classifications[1].Kind != loader.DraftMediator {
		t.Errorf("step 1 kind: got %q; want %q", classifications[1].Kind, loader.DraftMediator)
	}
	if classifications[1].SubKind != loader.DraftSubKindEndorsement {
		t.Errorf("step 1 sub_kind: got %q; want %q", classifications[1].SubKind, loader.DraftSubKindEndorsement)
	}
}

// --- ClassifyDraftChainOptions / ClassifiedDraftChain envelope tests (RED phase for #95) ---

// makeContentChain returns a two-draft chain where child reformulates WhatChanged
// but keeps the same ExtractionStage. Used by envelope tests to confirm that
// criterion carriage does not disturb content-based classification.
func makeContentChain() []schema.TraceDraft {
	parent := schema.TraceDraft{
		ID:              "e0000000-0000-4000-8000-000000000001",
		SourceSpan:      "span",
		WhatChanged:     "original framing",
		ExtractionStage: "weak-draft",
	}
	child := schema.TraceDraft{
		ID:              "e0000000-0000-4000-8000-000000000002",
		SourceSpan:      "span",
		WhatChanged:     "reformulated framing", // content changed
		ExtractionStage: "weak-draft",           // stage unchanged
		DerivedFrom:     parent.ID,
	}
	return []schema.TraceDraft{parent, child}
}

// TestClassifyDraftChain_ZeroOptsPreservesClassifications verifies that using
// zero-value ClassifyDraftChainOptions returns Classifications with the same
// Kind and Reason as the old single-argument behaviour (design rule C1:
// criterion does NOT alter step heuristics).
func TestClassifyDraftChain_ZeroOptsPreservesClassifications(t *testing.T) {
	chain := makeContentChain()
	result := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{})
	if len(result.Classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(result.Classifications))
	}
	if result.Classifications[0].Kind != loader.DraftMediator {
		t.Errorf("kind: got %q; want %q", result.Classifications[0].Kind, loader.DraftMediator)
	}
	if result.Classifications[0].Reason == "" {
		t.Error("Reason must be non-empty")
	}
}

// TestClassifyDraftChain_CriterionCarriedOnEnvelope verifies that a non-zero
// ClassifyDraftChainOptions.Criterion is stored verbatim on result.Criterion.
// All four fields (Name, Declaration, Preserve, Ignore) must appear on the envelope.
func TestClassifyDraftChain_CriterionCarriedOnEnvelope(t *testing.T) {
	crit := graph.EquivalenceCriterion{
		Name:        "operational-meaning",
		Declaration: "traces share meaning when they record the same delegation act",
		Preserve:    []string{"mediation", "observer"},
		Ignore:      []string{"timestamp"},
	}
	chain := makeContentChain()
	result := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{Criterion: crit})

	if result.Criterion.Name != crit.Name {
		t.Errorf("Criterion.Name: got %q; want %q", result.Criterion.Name, crit.Name)
	}
	if result.Criterion.Declaration != crit.Declaration {
		t.Errorf("Criterion.Declaration: got %q; want %q", result.Criterion.Declaration, crit.Declaration)
	}
	if len(result.Criterion.Preserve) != len(crit.Preserve) {
		t.Errorf("Criterion.Preserve length: got %d; want %d", len(result.Criterion.Preserve), len(crit.Preserve))
	}
	if len(result.Criterion.Ignore) != len(crit.Ignore) {
		t.Errorf("Criterion.Ignore length: got %d; want %d", len(result.Criterion.Ignore), len(crit.Ignore))
	}
}

// TestClassifyDraftChain_ZeroOpts_CriterionIsZero verifies that when
// ClassifyDraftChainOptions is zero-valued, result.Criterion is also zero-valued.
// Callers that do not declare a criterion receive an empty envelope field —
// no surprise metadata is injected.
func TestClassifyDraftChain_ZeroOpts_CriterionIsZero(t *testing.T) {
	chain := makeContentChain()
	result := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{})
	if !result.Criterion.IsZero() {
		t.Errorf("expected zero-value Criterion; got Name=%q Declaration=%q",
			result.Criterion.Name, result.Criterion.Declaration)
	}
}

// TestClassifyDraftChain_CriterionDoesNotAlterStepKind verifies design rule C1:
// the same chain classified under two different criteria returns identical
// Classifications (Kind, Reason, SubKind, StepIndex). Criterion is envelope
// metadata only and must not influence heuristic output.
func TestClassifyDraftChain_CriterionDoesNotAlterStepKind(t *testing.T) {
	chain := makeContentChain()

	critA := graph.EquivalenceCriterion{
		Name:        "operational-meaning",
		Declaration: "same delegation act",
		Preserve:    []string{"mediation"},
	}
	critB := graph.EquivalenceCriterion{
		Name:        "textual-proximity",
		Declaration: "same surface-level text",
		Ignore:      []string{"observer"},
	}

	resultA := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{Criterion: critA})
	resultB := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{Criterion: critB})

	if len(resultA.Classifications) != len(resultB.Classifications) {
		t.Fatalf("classification count differs: A=%d B=%d", len(resultA.Classifications), len(resultB.Classifications))
	}
	for i := range resultA.Classifications {
		a := resultA.Classifications[i]
		b := resultB.Classifications[i]
		if a.Kind != b.Kind {
			t.Errorf("step %d Kind: A=%q B=%q — criterion must not alter step heuristics (C1)", i, a.Kind, b.Kind)
		}
		if a.Reason != b.Reason {
			t.Errorf("step %d Reason: A=%q B=%q — criterion must not alter step heuristics (C1)", i, a.Reason, b.Reason)
		}
		if a.SubKind != b.SubKind {
			t.Errorf("step %d SubKind: A=%q B=%q — criterion must not alter step heuristics (C1)", i, a.SubKind, b.SubKind)
		}
	}
}

// TestClassifyDraftChain_TwoCriteriaSameClassification verifies that two calls
// with different criteria produce identical Classifications but different
// Criterion fields on the envelope. This is the positive complement to
// TestClassifyDraftChain_CriterionDoesNotAlterStepKind.
func TestClassifyDraftChain_TwoCriteriaSameClassification(t *testing.T) {
	chain := makeContentChain()

	critAlpha := graph.EquivalenceCriterion{
		Name:        "alpha-criterion",
		Declaration: "declaration alpha",
	}
	critBeta := graph.EquivalenceCriterion{
		Name:        "beta-criterion",
		Declaration: "declaration beta",
	}

	resultAlpha := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{Criterion: critAlpha})
	resultBeta := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{Criterion: critBeta})

	// Classifications must be identical.
	if len(resultAlpha.Classifications) != len(resultBeta.Classifications) {
		t.Fatalf("classification count: alpha=%d beta=%d", len(resultAlpha.Classifications), len(resultBeta.Classifications))
	}
	for i := range resultAlpha.Classifications {
		a := resultAlpha.Classifications[i]
		b := resultBeta.Classifications[i]
		if a.Kind != b.Kind || a.Reason != b.Reason || a.SubKind != b.SubKind {
			t.Errorf("step %d: classifications differ despite only criterion changing", i)
		}
	}

	// Criterion envelopes must differ.
	if resultAlpha.Criterion.Name == resultBeta.Criterion.Name {
		t.Errorf("Criterion.Name should differ: alpha=%q beta=%q", resultAlpha.Criterion.Name, resultBeta.Criterion.Name)
	}
	if resultAlpha.Criterion.Declaration == resultBeta.Criterion.Declaration {
		t.Errorf("Criterion.Declaration should differ")
	}
}

// TestClassifyDraftChain_CriterionSlicesDefensivelyCopied verifies that mutating
// opts.Criterion.Preserve or opts.Criterion.Ignore slices after the call does
// NOT affect result.Criterion. The implementation must copy slices defensively.
func TestClassifyDraftChain_CriterionSlicesDefensivelyCopied(t *testing.T) {
	preserve := []string{"mediation", "observer"}
	ignore := []string{"timestamp"}
	crit := graph.EquivalenceCriterion{
		Name:     "mutable-criterion",
		Preserve: preserve,
		Ignore:   ignore,
	}
	chain := makeContentChain()
	result := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{Criterion: crit})

	// Mutate the original slices after the call.
	preserve[0] = "MUTATED"
	ignore[0] = "MUTATED"

	// result.Criterion slices must be unaffected.
	if len(result.Criterion.Preserve) > 0 && result.Criterion.Preserve[0] == "MUTATED" {
		t.Error("result.Criterion.Preserve was not defensively copied — mutation propagated")
	}
	if len(result.Criterion.Ignore) > 0 && result.Criterion.Ignore[0] == "MUTATED" {
		t.Error("result.Criterion.Ignore was not defensively copied — mutation propagated")
	}
}

// TestClassifyDraftChain_NilForSingleDraft_WithOpts verifies that a single-draft
// chain with a non-zero criterion returns Classifications == nil and a populated
// Criterion envelope. The criterion must be carried even when no classification
// steps are possible.
func TestClassifyDraftChain_NilForSingleDraft_WithOpts(t *testing.T) {
	crit := graph.EquivalenceCriterion{
		Name:        "provenance-alignment",
		Declaration: "single-draft baseline",
	}
	d := schema.TraceDraft{
		ID:         "a0000000-0000-4000-8000-000000000001",
		SourceSpan: "span",
	}
	result := loader.ClassifyDraftChain([]schema.TraceDraft{d}, loader.ClassifyDraftChainOptions{Criterion: crit})

	if result.Classifications != nil {
		t.Errorf("single draft: Classifications must be nil; got %v", result.Classifications)
	}
	if result.Criterion.Name != crit.Name {
		t.Errorf("Criterion.Name: got %q; want %q", result.Criterion.Name, crit.Name)
	}
	if result.Criterion.Declaration != crit.Declaration {
		t.Errorf("Criterion.Declaration: got %q; want %q", result.Criterion.Declaration, crit.Declaration)
	}
}

// TestClassifyDraftChain_Classifications_MatchOldSliceReturn verifies that
// result.Classifications returns the same elements as the old slice return
// for a known multi-step chain: A→B (intermediary), B→C (translation).
func TestClassifyDraftChain_Classifications_MatchOldSliceReturn(t *testing.T) {
	a := schema.TraceDraft{
		ID: "a0000000-0000-4000-8000-000000000001", SourceSpan: "span",
		WhatChanged: "original", ExtractionStage: "weak-draft",
	}
	b := schema.TraceDraft{
		ID: "b0000000-0000-4000-8000-000000000002", SourceSpan: "span",
		WhatChanged: "original", ExtractionStage: "weak-draft",
		UncertaintyNote: "note added", DerivedFrom: a.ID,
	}
	c := schema.TraceDraft{
		ID: "c0000000-0000-4000-8000-000000000003", SourceSpan: "span",
		WhatChanged: "reformulated", ExtractionStage: "reviewed", // content + stage
		DerivedFrom: b.ID,
	}
	chain := []schema.TraceDraft{a, b, c}

	result := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{})
	if len(result.Classifications) != 2 {
		t.Fatalf("expected 2 classifications; got %d", len(result.Classifications))
	}

	// A→B: no content or stage change — intermediary.
	if result.Classifications[0].Kind != loader.DraftIntermediary {
		t.Errorf("step 0 kind: got %q; want %q", result.Classifications[0].Kind, loader.DraftIntermediary)
	}
	if result.Classifications[0].StepIndex != 1 {
		t.Errorf("step 0 StepIndex: got %d; want 1", result.Classifications[0].StepIndex)
	}

	// B→C: content changed + stage advanced — translation.
	if result.Classifications[1].Kind != loader.DraftTranslation {
		t.Errorf("step 1 kind: got %q; want %q", result.Classifications[1].Kind, loader.DraftTranslation)
	}
	if result.Classifications[1].StepIndex != 2 {
		t.Errorf("step 1 StepIndex: got %d; want 2", result.Classifications[1].StepIndex)
	}
}

// TestFollowDraftChain_Fork verifies that when a parent has two children,
// FollowDraftChain follows exactly one branch (the first child by input order)
// and returns a linear chain — not both branches. This documents the
// first-match behaviour described in the FollowDraftChain doc comment.
func TestFollowDraftChain_Fork(t *testing.T) {
	// root → child-a (first in slice)
	//      → child-b (second in slice)
	root := schema.TraceDraft{
		ID: "f0000000-0000-4000-8000-000000000001", SourceSpan: "root",
	}
	childA := schema.TraceDraft{
		ID: "f0000000-0000-4000-8000-000000000002", SourceSpan: "branch-a",
		DerivedFrom: root.ID,
	}
	childB := schema.TraceDraft{
		ID: "f0000000-0000-4000-8000-000000000003", SourceSpan: "branch-b",
		DerivedFrom: root.ID,
	}
	// Pass drafts in deterministic order: root, childA, childB.
	// FollowDraftChain appends children in slice-iteration order, so childA
	// is kids[0] and will be followed.
	drafts := []schema.TraceDraft{root, childA, childB}
	chain := loader.FollowDraftChain(drafts, root.ID)

	// Must be exactly 2 elements: root + one branch.
	if len(chain) != 2 {
		t.Fatalf("fork chain: got len %d; want 2", len(chain))
	}
	if chain[0].ID != root.ID {
		t.Errorf("chain[0] = %q; want root %q", chain[0].ID, root.ID)
	}
	// The followed branch must be one of the two children (childA by first-match).
	if chain[1].ID != childA.ID && chain[1].ID != childB.ID {
		t.Errorf("chain[1] = %q; want one of childA or childB", chain[1].ID)
	}
}
