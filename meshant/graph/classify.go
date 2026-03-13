// classify.go provides chain step classification — judging each step in a
// TranslationChain as intermediary-like, mediator-like, or translation.
//
// Classification is an analytical judgment, not an intrinsic property of the
// underlying trace. The same edge, appearing in two different articulations
// (different observer positions or time windows), can receive different
// classifications. This is consistent with ANT: nothing has an essence; its
// role is constituted by the network that describes it.
//
// The v1 heuristics are provisional and intentionally simple. They use only
// information already present on the Edge (Mediation field and Tags). Future
// versions may introduce contextual heuristics via ClassifyOptions that
// consider adjacent steps or user-supplied rules.
package graph

// StepKind is the classification of a single chain step.
type StepKind string

const (
	// StepIntermediary means no mediation was observed on this edge — the
	// action appears to have been relayed without recorded transformation.
	// This is "intermediary-*like*", not a definitive claim. Absence of a
	// recorded mediator does not prove that no transformation occurred.
	StepIntermediary StepKind = "intermediary"

	// StepMediator means a mediation was observed — something transformed,
	// redirected, or displaced the action in passage. A mediator changes
	// what passes through it.
	StepMediator StepKind = "mediator"

	// StepTranslation means a mediation was observed AND the edge carries
	// the "translation" tag — a regime boundary was crossed. Translation
	// converts across operational registers (e.g., scientific → juridical).
	StepTranslation StepKind = "translation"
)

// StepClassification records the classification of one step in a chain.
type StepClassification struct {
	// StepIndex is the index into TranslationChain.Steps.
	StepIndex int

	// Kind is the classification: intermediary, mediator, or translation.
	Kind StepKind

	// Reason is a human-readable justification for the classification.
	// Always non-empty. Makes the judgment inspectable and contestable.
	Reason string
}

// ClassifiedChain pairs a TranslationChain with per-step classifications.
// Classification is a separate layer from traversal — it reads the chain
// and judges each step without modifying the chain itself.
type ClassifiedChain struct {
	// Chain is the translation chain that was classified.
	Chain TranslationChain

	// Classifications has one entry per step, in the same order as
	// Chain.Steps. Empty if the chain has no steps.
	Classifications []StepClassification
}

// ClassifyOptions parameterises classification heuristics.
// Empty for v1 — the struct exists as an extension point so the API can
// accept contextual heuristics or user-supplied rules in future versions
// without breaking callers.
type ClassifyOptions struct{}

// ClassifyChain classifies each step in chain as intermediary-like,
// mediator-like, or translation. Returns an immutable ClassifiedChain.
//
// v1 heuristics (provisional):
//   - Translation: non-empty Edge.Mediation AND "translation" tag present
//   - Mediator: non-empty Edge.Mediation, no "translation" tag
//   - Intermediary: empty Edge.Mediation — "no mediation observed"
//
// These heuristics are properties of the edge within a specific cut, not
// intrinsic properties of the underlying trace.
func ClassifyChain(chain TranslationChain, _ ClassifyOptions) ClassifiedChain {
	cc := ClassifiedChain{
		Chain:           chain,
		Classifications: make([]StepClassification, len(chain.Steps)),
	}

	for i, step := range chain.Steps {
		kind, reason := classifyStep(step)
		cc.Classifications[i] = StepClassification{
			StepIndex: i,
			Kind:      kind,
			Reason:    reason,
		}
	}

	return cc
}

// classifyStep applies the v1 heuristic to a single step.
func classifyStep(step ChainStep) (StepKind, string) {
	if step.Edge.Mediation == "" {
		return StepIntermediary, "no mediation observed — action relayed without recorded transformation"
	}

	if hasTag(step.Edge.Tags, "translation") {
		return StepTranslation, "mediation present with translation tag — regime boundary crossed"
	}

	return StepMediator, "mediation present — action transformed in passage"
}

// hasTag checks whether tags contains the given value.
func hasTag(tags []string, value string) bool {
	for _, t := range tags {
		if t == value {
			return true
		}
	}
	return false
}
