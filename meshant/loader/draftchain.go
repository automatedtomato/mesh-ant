// draftchain.go provides derivation chain traversal and classification for
// TraceDraft records — Layer 1 analytical operation on the ingestion pipeline.
//
// A derivation chain follows DerivedFrom links through a set of TraceDraft
// records: each draft that names another as its DerivedFrom is a step in the
// chain. This mirrors FollowTranslation in the graph package, but operates on
// drafts rather than graph edges.
//
// The ingestion pipeline (span → LLM draft → critique → revision → canonical
// trace) is itself a translation chain. FollowDraftChain makes it traversable
// in MeshAnt's own vocabulary. ClassifyDraftChain judges each derivation step:
// did the critique merely relay content (intermediary), transform it (mediator),
// or shift the interpretive register and stage (translation)?
//
// Classification heuristics are provisional (v1) — the same disposition as
// ClassifyChain in the graph package. Judgments are contestable.
package loader

import (
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// DraftStepKind is the classification of a single derivation step.
type DraftStepKind string

const (
	// DraftIntermediary means the derivation step did not reformulate any
	// candidate content fields. The draft relayed the source span without
	// recorded transformation — it may have added provenance annotations
	// (UncertaintyNote, ExtractedBy) but the interpretive content is unchanged.
	// This is "intermediary-like", not a definitive claim.
	DraftIntermediary DraftStepKind = "intermediary"

	// DraftMediator means at least one candidate content field changed between
	// the parent and the derived draft. The derivation step transformed the
	// interpretation — it reformulated what_changed, source, target, mediation,
	// observer, or tags. A mediator changes what passes through it.
	DraftMediator DraftStepKind = "mediator"

	// DraftTranslation means candidate content fields changed AND the
	// extraction_stage changed — a regime boundary was crossed. The derivation
	// step reformulated the interpretive frame and advanced the pipeline stage
	// (e.g., from "weak-draft" to "reviewed").
	DraftTranslation DraftStepKind = "translation"
)

// DraftStepClassification records the classification of one derivation step.
// A step is the passage from chain[i-1] to chain[i].
type DraftStepClassification struct {
	// StepIndex is the index of the destination draft in the chain slice
	// (i.e., the draft that was derived). StepIndex 1 means the first
	// derivation step: from chain[0] to chain[1].
	StepIndex int

	// Kind is the classification: intermediary, mediator, or translation.
	Kind DraftStepKind

	// Reason is a human-readable justification. Always non-empty. Makes the
	// judgment inspectable and contestable.
	Reason string
}

// FollowDraftChain traverses DerivedFrom links through drafts starting from
// the draft with ID from. It returns the drafts in derivation order — the root
// draft first, then each successive derivation.
//
// When a parent has more than one child (a fork in the derivation tree),
// FollowDraftChain follows the first child by encounter order in the drafts
// slice. Sibling branches are not traversed. The result is always a single
// linear path, not a tree. Callers that need the full tree should use cmdLineage
// or build their own traversal from the DerivedFrom links.
//
// Cycle detection is performed via a visited set: if a draft's DerivedFrom
// points to an already-visited ID the traversal stops (the cycle-closing draft
// is NOT included, consistent with the traversal stopping before re-entry).
//
// Returns an empty slice if from is not found in drafts.
// Returns a single-element slice if the root has no derived drafts.
func FollowDraftChain(drafts []schema.TraceDraft, from string) []schema.TraceDraft {
	// Index drafts by ID for O(1) lookup.
	byID := make(map[string]schema.TraceDraft, len(drafts))
	for _, d := range drafts {
		if d.ID != "" {
			byID[d.ID] = d
		}
	}

	// Build reverse index: parentID → []child to traverse forward from root.
	// DerivedFrom is the parent link; we want to walk parent → child.
	children := make(map[string][]string) // parentID → []childID
	for _, d := range drafts {
		if d.DerivedFrom != "" {
			children[d.DerivedFrom] = append(children[d.DerivedFrom], d.ID)
		}
	}

	root, ok := byID[from]
	if !ok {
		return []schema.TraceDraft{}
	}

	chain := []schema.TraceDraft{root}
	visited := map[string]bool{from: true}
	current := from

	for {
		kids, ok := children[current]
		if !ok || len(kids) == 0 {
			// No further derivations from current node.
			break
		}

		// First-match: follow the first child by encounter order.
		next := kids[0]
		if visited[next] {
			// Cycle detected — stop before re-entry.
			break
		}

		nextDraft, ok := byID[next]
		if !ok {
			// Child ID references a draft not in the set — stop.
			break
		}

		chain = append(chain, nextDraft)
		visited[next] = true
		current = next
	}

	return chain
}

// ClassifyDraftChain classifies each derivation step in chain. It returns one
// DraftStepClassification per step (len(chain)-1 entries). Returns nil if
// chain has fewer than two drafts (no steps to classify).
//
// v1 heuristics (provisional):
//   - DraftTranslation: content fields changed AND extraction_stage changed
//   - DraftMediator (content): content fields changed, extraction_stage unchanged
//   - DraftMediator (endorsement): extraction_stage advanced, no content fields changed
//   - DraftIntermediary: no content fields changed and extraction_stage unchanged
//
// Content fields are: what_changed, source, target, mediation, observer, tags.
// Provenance fields (uncertainty_note, extracted_by, intentionally_blank) are
// not content — they do not constitute mediation on their own.
func ClassifyDraftChain(chain []schema.TraceDraft) []DraftStepClassification {
	if len(chain) < 2 {
		return nil
	}

	result := make([]DraftStepClassification, len(chain)-1)
	for i := 1; i < len(chain); i++ {
		prev := chain[i-1]
		curr := chain[i]
		kind, reason := classifyDraftStep(prev, curr)
		result[i-1] = DraftStepClassification{
			StepIndex: i,
			Kind:      kind,
			Reason:    reason,
		}
	}
	return result
}

// classifyDraftStep applies the v1 heuristic to a single derivation step.
func classifyDraftStep(prev, curr schema.TraceDraft) (DraftStepKind, string) {
	content := draftContentChanged(prev, curr)
	stage := draftStageChanged(prev, curr)

	switch {
	case content && stage:
		return DraftTranslation,
			"content fields reformulated and extraction_stage advanced — interpretive frame shifted"
	case content:
		return DraftMediator,
			"content fields reformulated — interpretation transformed in derivation"
	case stage:
		return DraftMediator,
			"extraction_stage advanced without content change — endorsement transformed standing, not content"
	default:
		return DraftIntermediary,
			"no content fields changed — draft relayed without recorded transformation"
	}
}

// draftContentChanged reports whether any candidate content field differs
// between prev and curr. Content fields are: what_changed, source, target,
// mediation, observer, tags. Provenance fields are excluded.
func draftContentChanged(prev, curr schema.TraceDraft) bool {
	if prev.WhatChanged != curr.WhatChanged {
		return true
	}
	if !stringSlicesEqual(prev.Source, curr.Source) {
		return true
	}
	if !stringSlicesEqual(prev.Target, curr.Target) {
		return true
	}
	if prev.Mediation != curr.Mediation {
		return true
	}
	if prev.Observer != curr.Observer {
		return true
	}
	if !stringSlicesEqual(prev.Tags, curr.Tags) {
		return true
	}
	return false
}

// draftStageChanged reports whether extraction_stage changed between prev and
// curr. A non-empty curr.ExtractionStage that differs from prev is a stage
// change. An empty curr.ExtractionStage is not counted as a stage change —
// it is more likely an unpopulated field than a deliberate advancement.
func draftStageChanged(prev, curr schema.TraceDraft) bool {
	return curr.ExtractionStage != "" && curr.ExtractionStage != prev.ExtractionStage
}

// stringSlicesEqual reports whether two string slices have the same length and
// the same elements in the same order. Nil and empty are considered equal.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
