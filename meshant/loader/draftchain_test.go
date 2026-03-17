package loader_test

import (
	"testing"

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

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child})
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

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child})
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

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{parent, child})
	if len(classifications) != 1 {
		t.Fatalf("classification count: got %d; want 1", len(classifications))
	}
	if classifications[0].Kind != loader.DraftTranslation {
		t.Errorf("kind: got %q; want %q", classifications[0].Kind, loader.DraftTranslation)
	}
}

func TestClassifyDraftChain_NilForSingleDraft(t *testing.T) {
	d := schema.TraceDraft{ID: "a0000000-0000-4000-8000-000000000001", SourceSpan: "span"}
	result := loader.ClassifyDraftChain([]schema.TraceDraft{d})
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

	classifications := loader.ClassifyDraftChain([]schema.TraceDraft{a, b, c})
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
	classifications := loader.ClassifyDraftChain(drafts)
	for i, c := range classifications {
		want := i + 1
		if c.StepIndex != want {
			t.Errorf("classifications[%d].StepIndex = %d; want %d", i, c.StepIndex, want)
		}
	}
}

func TestClassifyDraftChain_ReasonNonEmpty(t *testing.T) {
	drafts := makeDraftChain(2)
	classifications := loader.ClassifyDraftChain(drafts)
	if len(classifications) == 0 {
		t.Fatal("no classifications returned")
	}
	if classifications[0].Reason == "" {
		t.Error("Reason is empty; must be non-empty for inspectability")
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
