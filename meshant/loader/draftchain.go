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

// DraftStepClassification records the classification of one derivation step (chain[i-1]→chain[i]).
type DraftStepClassification struct {
	// StepIndex is the destination draft's index in the chain (1 = first step).
	StepIndex int

	// Kind is the classification: intermediary, mediator, or translation.
	Kind DraftStepKind

	// Reason is a human-readable justification. Always non-empty.
	Reason string
}

// FollowDraftChain traverses DerivedFrom links starting from from, returning drafts
// in derivation order (root first). Follows first child on forks (first-match).
// Stops on cycle detection; cycle-closing draft excluded.
// Returns empty slice if from not found; single-element slice if no children.
func FollowDraftChain(drafts []schema.TraceDraft, from string) []schema.TraceDraft {
	byID := make(map[string]schema.TraceDraft, len(drafts))
	for _, d := range drafts {
		if d.ID != "" {
			byID[d.ID] = d
		}
	}

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
			break
		}

		next := kids[0]
		if visited[next] {
			break // cycle — stop before re-entry
		}

		nextDraft, ok := byID[next]
		if !ok {
			break // child not in set
		}

		chain = append(chain, nextDraft)
		visited[next] = true
		current = next
	}

	return chain
}

// ClassifyDraftChain classifies each derivation step in chain (len(chain)-1 entries).
// Returns nil for chains shorter than 2. v1 heuristics: content+stage → translation;
// content only → mediator; stage only → mediator (endorsement); neither → intermediary.
// Content fields: what_changed, source, target, mediation, observer, tags.
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

// draftContentChanged reports whether any content field differs between prev and curr.
// Provenance fields (uncertainty_note, extracted_by, intentionally_blank) are excluded.
func draftContentChanged(prev, curr schema.TraceDraft) bool {
	if prev.WhatChanged != curr.WhatChanged {
		return true
	}
	if !stringSlicesEqualOrdered(prev.Source, curr.Source) {
		return true
	}
	if !stringSlicesEqualOrdered(prev.Target, curr.Target) {
		return true
	}
	if prev.Mediation != curr.Mediation {
		return true
	}
	if prev.Observer != curr.Observer {
		return true
	}
	if !stringSlicesEqualOrdered(prev.Tags, curr.Tags) {
		return true
	}
	return false
}

// draftStageChanged reports whether extraction_stage changed. Empty curr stage
// is not counted — more likely an unpopulated field than a deliberate advancement.
func draftStageChanged(prev, curr schema.TraceDraft) bool {
	return curr.ExtractionStage != "" && curr.ExtractionStage != prev.ExtractionStage
}

// stringSlicesEqualOrdered reports element-wise (order-sensitive) equality;
// nil and empty are equal. Use stringSlicesEqualUnordered (extractiongap.go)
// when element order does not matter.
func stringSlicesEqualOrdered(a, b []string) bool {
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
