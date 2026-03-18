// suggest.go provides re-articulation suggestion logic for observer gaps.
//
// When two articulations reveal a gap — elements visible from one position
// but not another — SuggestRearticulations generates heuristic provocations
// for how the articulation parameters might be adjusted to investigate the
// asymmetry. These suggestions are not prescriptions: they cannot know what
// a re-articulation would reveal, whether the gap is significant, or whether
// expanding a cut would produce signal or noise.
//
// PrintRearticSuggestions formats suggestions alongside a gap summary for
// display. The footer note encodes the epistemic constraint: a suggestion is
// a provocation, not a prescription.
package graph

import (
	"fmt"
	"io"
)

// SuggestionKind names the category of re-articulation change being suggested.
// Each kind points to a different dimension of the cut that might be adjusted.
type SuggestionKind string

const (
	// SuggestionObserverExpansion suggests broadening the observer filter on
	// one side to include positions that the other side's cut already covers.
	SuggestionObserverExpansion SuggestionKind = "observer-expansion"

	// SuggestionTimeExpansion suggests widening the time window on one side
	// to span a range that the other side's cut already covers.
	SuggestionTimeExpansion SuggestionKind = "time-window-expansion"

	// SuggestionTagRelaxation suggests relaxing or removing the tag filter
	// on one side that the other side's cut does not apply.
	SuggestionTagRelaxation SuggestionKind = "tag-relaxation"
)

// RearticSuggestion is a heuristic provocation for how one side of a gap
// might be re-articulated to investigate the asymmetry. It names what it
// cannot know: whether the suggested change would reveal structurally
// significant elements or merely produce noise.
//
// A RearticSuggestion is not a recommendation. It is an analytical prompt
// generated from observable structural asymmetry between two cuts.
type RearticSuggestion struct {
	// Kind names the category of change being suggested.
	Kind SuggestionKind

	// Side is "A" or "B" — the articulation side being suggested to change.
	Side string

	// Rationale names what this suggestion cannot know. Always non-empty.
	// Uses hedged language: "cannot know", "might", "does not guarantee".
	Rationale string

	// SuggestedParams is a plain-language description of the suggested change.
	// Always non-empty.
	SuggestedParams string
}

// SuggestRearticulations generates heuristic re-articulation suggestions from
// an ObserverGap. It examines structural asymmetries between the two cuts and
// produces suggestions for how one side's parameters might be adjusted to
// investigate the gap.
//
// Returns nil when there is no gap (both OnlyInA and OnlyInB are empty) —
// nil signals "no gap, no suggestions applicable".
//
// Returns a non-nil empty slice when a gap exists but no heuristic fires —
// this distinction preserves the caller's ability to differentiate "no gap"
// from "gap but no automated suggestion available".
//
// v1 heuristics:
//   - Observer expansion: if A has exclusive elements and B has observer
//     positions set, suggest expanding B's observers (and vice versa).
//   - Time-window expansion: if one cut has a non-zero TimeWindow and the
//     other does not, and the windowless side has exclusive elements, suggest
//     expanding its time window.
//   - Tag relaxation: if one cut has tags and the other does not, and the
//     tagless side has exclusive elements, suggest relaxing the tagged side's
//     filter.
//
// SuggestRearticulations takes only an ObserverGap — not traces or a
// MeshGraph — so it cannot inspect what a re-articulation would actually
// reveal. The Rationale field names this limit explicitly.
func SuggestRearticulations(gap ObserverGap) []RearticSuggestion {
	// nil means no gap — return nil to signal "no suggestions applicable".
	if len(gap.OnlyInA) == 0 && len(gap.OnlyInB) == 0 {
		return nil
	}

	// Gap exists: initialise a non-nil result so callers can distinguish
	// "no gap" (nil) from "gap but no heuristic fired" (empty non-nil slice).
	result := []RearticSuggestion{}

	// Heuristic 1: observer expansion.
	// If A has exclusive elements and B has observer positions set, suggest
	// expanding B to include A's positions. The suggestion cannot know whether
	// those positions would reveal the same elements or unrelated ones.
	if len(gap.OnlyInA) > 0 && len(gap.CutB.ObserverPositions) > 0 {
		result = append(result, RearticSuggestion{
			Kind: SuggestionObserverExpansion,
			Side: "B",
			Rationale: "this suggestion cannot know whether expanding the observer set " +
				"would reveal structurally significant elements or produce noise",
			SuggestedParams: "consider expanding the observer positions for cut B to " +
				"include the positions used in cut A, then re-run the gap analysis",
		})
	}

	// If B has exclusive elements and A has observer positions set, suggest
	// expanding A's observer filter.
	if len(gap.OnlyInB) > 0 && len(gap.CutA.ObserverPositions) > 0 {
		result = append(result, RearticSuggestion{
			Kind: SuggestionObserverExpansion,
			Side: "A",
			Rationale: "this suggestion cannot know whether expanding the observer set " +
				"would reveal structurally significant elements or produce noise",
			SuggestedParams: "consider expanding the observer positions for cut A to " +
				"include the positions used in cut B, then re-run the gap analysis",
		})
	}

	// Heuristic 2: time-window expansion.
	// If one cut has a non-zero TimeWindow and the other does not, and the
	// windowed side has exclusive elements, suggest widening that window to
	// match the full temporal scope of the other cut.
	aHasWindow := !gap.CutA.TimeWindow.IsZero()
	bHasWindow := !gap.CutB.TimeWindow.IsZero()

	if aHasWindow && !bHasWindow && len(gap.OnlyInB) > 0 {
		// B has no time window (full temporal cut) and has exclusive elements.
		// Suggest expanding A's time window to investigate what B can see.
		result = append(result, RearticSuggestion{
			Kind: SuggestionTimeExpansion,
			Side: "A",
			Rationale: "this suggestion cannot know whether widening the time window " +
				"would surface the same elements or unrelated ones from a different period",
			SuggestedParams: "consider widening or removing the time window for cut A " +
				"to align with the full temporal scope used by cut B",
		})
	}

	if bHasWindow && !aHasWindow && len(gap.OnlyInA) > 0 {
		// A has no time window (full temporal cut) and has exclusive elements.
		// Suggest expanding B's time window to investigate what A can see.
		result = append(result, RearticSuggestion{
			Kind: SuggestionTimeExpansion,
			Side: "B",
			Rationale: "this suggestion cannot know whether widening the time window " +
				"would surface the same elements or unrelated ones from a different period",
			SuggestedParams: "consider widening or removing the time window for cut B " +
				"to align with the full temporal scope used by cut A",
		})
	}

	// Heuristic 3: tag relaxation.
	// If one cut has tags and the other does not, and the tagless side has
	// exclusive elements, suggest relaxing the tagged side's filter.
	aHasTags := len(gap.CutA.Tags) > 0
	bHasTags := len(gap.CutB.Tags) > 0

	if aHasTags && !bHasTags && len(gap.OnlyInB) > 0 {
		// B has no tag filter and has exclusive elements.
		// Suggest relaxing A's tag filter to investigate what B can see.
		result = append(result, RearticSuggestion{
			Kind: SuggestionTagRelaxation,
			Side: "A",
			Rationale: "this suggestion cannot know whether relaxing the tag filter " +
				"would reveal the same elements or elements from an unrelated domain",
			SuggestedParams: "consider removing or relaxing the tag filter on cut A " +
				"to broaden it toward the full tag cut used by cut B",
		})
	}

	if bHasTags && !aHasTags && len(gap.OnlyInA) > 0 {
		// A has no tag filter and has exclusive elements.
		// Suggest relaxing B's tag filter to investigate what A can see.
		result = append(result, RearticSuggestion{
			Kind: SuggestionTagRelaxation,
			Side: "B",
			Rationale: "this suggestion cannot know whether relaxing the tag filter " +
				"would reveal the same elements or elements from an unrelated domain",
			SuggestedParams: "consider removing or relaxing the tag filter on cut B " +
				"to broaden it toward the full tag cut used by cut A",
		})
	}

	return result
}

// PrintRearticSuggestions writes a re-articulation suggestions report to w.
//
// If suggestions is nil (signalling no gap), PrintRearticSuggestions returns
// nil immediately without writing anything — nil is the no-gap sentinel.
//
// The report includes:
//   - A section header
//   - A gap summary (counts for OnlyInA, OnlyInB, InBoth)
//   - A "no suggestions" line if the slice is non-nil but empty
//   - Per-suggestion blocks (Kind, Side, Rationale, SuggestedParams)
//   - A footer note encoding the epistemic constraint
//
// Uses its own fmt.Fprintln loop — does NOT delegate to writeLines (which
// carries a hardcoded "PrintShadowSummary" error prefix from shadow.go).
//
// Returns the first write error encountered, if any.
func PrintRearticSuggestions(w io.Writer, gap ObserverGap, suggestions []RearticSuggestion) error {
	// nil means no gap — print nothing.
	if suggestions == nil {
		return nil
	}

	lines := []string{
		"=== Re-articulation Suggestions ===",
		"",
		fmt.Sprintf("Gap: Only in A: %d | Only in B: %d | In both: %d",
			len(gap.OnlyInA), len(gap.OnlyInB), len(gap.InBoth)),
		"",
	}

	if len(suggestions) == 0 {
		lines = append(lines, "No suggestions generated from this gap.")
	} else {
		for i, s := range suggestions {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines,
				fmt.Sprintf("Kind:            %s", s.Kind),
				fmt.Sprintf("Side:            %s", s.Side),
				fmt.Sprintf("Rationale:       %s", s.Rationale),
				fmt.Sprintf("SuggestedParams: %s", s.SuggestedParams),
			)
		}
	}

	lines = append(lines,
		"",
		"---",
		"Note: a suggestion is a provocation, not a prescription. " +
			"It does not know what a re-articulation would reveal. " +
			"It can only suggest changes to observer, time, and tag parameters — " +
			"not to element boundaries, equivalence criteria, or the trace dataset itself.",
	)

	// Own Fprintln loop — not writeLines from shadow.go (wrong error prefix).
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("graph: PrintRearticSuggestions: %w", err)
		}
	}
	return nil
}
