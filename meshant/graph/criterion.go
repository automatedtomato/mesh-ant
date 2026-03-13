// criterion.go defines EquivalenceCriterion — the interpretive object that
// declares the conditions under which a chain reading is being conducted.
//
// An equivalence criterion is NOT a computation rule. It is an analytical
// declaration: the analyst states what they are treating as preserved,
// altered, or irrelevant before they read the chain. The criterion governs
// any future comparison function — the function must not define the criterion.
//
// This file implements Layers 1–2 of a three-layer design:
//   - Layer 1: Declaration — the interpretive grounds (what counts as equivalent)
//   - Layer 2: Preserve/Ignore — aspects explicitly named as continuity-bearing
//     or excluded from relevance
//   - Layer 3: Comparison function — deferred to a future milestone
//
// Layer order is strictly enforced: Layer 2 fields require a Layer 1
// Declaration. The Validate() method checks this at the type boundary.
package graph

import "errors"

// EquivalenceCriterion declares the conditions under which a chain reading
// is being conducted. It is an interpretive object, not a computational one.
// The criterion governs any future comparison function — the function must
// not define the criterion.
//
// The act of declaring a criterion is itself a cut that may eventually be
// recorded as a trace (reflexive criterion-tracing, deferred).
//
// A single criterion applied uniformly across all steps in a chain is a
// known simplification. This design is intentionally pre-translation-aware:
// translation steps cross regime boundaries, and a criterion that spans such
// a crossing may not be coherent. A future version may allow per-step or
// per-regime criteria.
type EquivalenceCriterion struct {
	// Name is a short identifier (handle) for this criterion.
	// Optional — a declaration-only criterion (no name) is valid and
	// encouraged as the primary mode. Name is a convenience for repeated
	// use, not an identity. Two criteria with different names may declare
	// the same grounds; two criteria with the same name may drift in their
	// declarations over time.
	Name string

	// Declaration is the interpretive criterion in natural language.
	// This is the primary layer (Layer 1) — the grounds for the reading.
	// Without it, Preserve and Ignore have no grounding.
	//
	// A name-only criterion (no Declaration) is structurally valid but
	// analytically weak — it is a handle without grounds. Code comments
	// and the decision record make this explicit: name-only criteria are
	// accepted as transport handles, but the conceptually strong usage
	// always includes a Declaration.
	Declaration string

	// Preserve is an optional list of aspects treated as
	// continuity-bearing under this criterion (Layer 2).
	// Values are free-text human vocabulary, not schema field names.
	// Requires Declaration to be non-empty (layer ordering).
	Preserve []string

	// Ignore is an optional list of aspects treated as irrelevant to
	// equivalence under this criterion (Layer 2).
	// Values are free-text human vocabulary, not schema field names.
	// Requires Declaration to be non-empty (layer ordering).
	//
	// These aspects are not absent — they are declared irrelevant under
	// this criterion. A different criterion might treat them as decisive.
	// This is a second-order shadow: what the reading conditions exclude
	// from relevance. Future milestones may extend the shadow apparatus
	// to surface criterion-excluded aspects explicitly.
	Ignore []string
}

// IsZero reports whether the criterion is entirely unset.
// Treats nil and empty slices as equivalent (both are zero for this purpose).
// A zero criterion means: no interpretive conditions declared; fall back to
// v1 heuristics (implicit criterion: trust the trace author's mediation
// annotation).
func (c EquivalenceCriterion) IsZero() bool {
	return c.Name == "" &&
		c.Declaration == "" &&
		len(c.Preserve) == 0 &&
		len(c.Ignore) == 0
}

// Validate checks layer ordering: Preserve and Ignore require Declaration.
// Returns an error if either Layer 2 field is populated without a Layer 1
// Declaration, since the aspects would have no interpretive grounding.
//
// Validate does not reject name-only criteria — they are structurally valid,
// even if analytically incomplete. The CLI's --criterion-file loader applies
// stricter rules (zero-value after unmarshal → hard error).
func (c EquivalenceCriterion) Validate() error {
	if c.Declaration == "" && (len(c.Preserve) > 0 || len(c.Ignore) > 0) {
		return errors.New("equivalence criterion: Preserve and Ignore require Declaration (layer ordering: Layer 2 has no meaning without Layer 1 grounds)")
	}
	return nil
}
