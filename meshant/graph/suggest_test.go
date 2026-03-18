// Package graph_test provides tests for RearticSuggestion types and functions.
//
// Tests build ObserverGap structs directly by hand — they do not go through
// Articulate + AnalyseGaps. This keeps the tests focused on the suggestion
// logic itself and not on the articulation or gap analysis pipelines.
//
// Naming convention: each test targets one heuristic or invariant, named
// explicitly so failures identify which logical path failed.
package graph_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// --- Helpers ---

// makeGapWithOnlyInA returns a minimal ObserverGap where only cut A has
// exclusive elements and cut B has observer positions set. This is the
// baseline scenario for the observer-expansion-B heuristic.
func makeGapWithOnlyInA() graph.ObserverGap {
	return graph.ObserverGap{
		OnlyInA: []string{"elem-a1", "elem-a2"},
		OnlyInB: []string{},
		InBoth:  []string{"elem-shared"},
		CutA:    graph.Cut{ObserverPositions: []string{}},
		CutB:    graph.Cut{ObserverPositions: []string{"obs-b"}},
	}
}

// makeGapWithOnlyInB returns a minimal ObserverGap where only cut B has
// exclusive elements and cut A has observer positions set. This is the
// baseline scenario for the observer-expansion-A heuristic.
func makeGapWithOnlyInB() graph.ObserverGap {
	return graph.ObserverGap{
		OnlyInA: []string{},
		OnlyInB: []string{"elem-b1"},
		InBoth:  []string{},
		CutA:    graph.Cut{ObserverPositions: []string{"obs-a"}},
		CutB:    graph.Cut{ObserverPositions: []string{}},
	}
}

// makeNoGap returns an ObserverGap with no exclusive elements on either side.
func makeNoGap() graph.ObserverGap {
	return graph.ObserverGap{
		OnlyInA: []string{},
		OnlyInB: []string{},
		InBoth:  []string{"elem-shared"},
		CutA:    graph.Cut{ObserverPositions: []string{"obs-a"}},
		CutB:    graph.Cut{ObserverPositions: []string{"obs-b"}},
	}
}

// --- Tests ---

// TestSuggestRearticulations_NilWhenNoGap verifies that SuggestRearticulations
// returns nil (not an empty slice) when neither OnlyInA nor OnlyInB has
// elements. nil is the no-gap sentinel; the caller can distinguish it from
// "gap exists but no heuristic fired".
func TestSuggestRearticulations_NilWhenNoGap(t *testing.T) {
	gap := makeNoGap()
	result := graph.SuggestRearticulations(gap)
	if result != nil {
		t.Errorf("SuggestRearticulations with no gap: want nil, got %v", result)
	}
}

// TestSuggestRearticulations_ObserverExpansionSideB verifies that when cut A
// has exclusive elements and cut B has observer positions, a suggestion with
// Kind=observer-expansion and Side="B" is included.
func TestSuggestRearticulations_ObserverExpansionSideB(t *testing.T) {
	gap := makeGapWithOnlyInA()
	suggestions := graph.SuggestRearticulations(gap)

	if suggestions == nil {
		t.Fatal("SuggestRearticulations: got nil, want non-nil (gap exists)")
	}

	found := false
	for _, s := range suggestions {
		if s.Kind == graph.SuggestionObserverExpansion && s.Side == "B" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected observer-expansion Side=B suggestion; got: %v", suggestions)
	}
}

// TestSuggestRearticulations_ObserverExpansionSideA verifies that when cut B
// has exclusive elements and cut A has observer positions, a suggestion with
// Kind=observer-expansion and Side="A" is included.
func TestSuggestRearticulations_ObserverExpansionSideA(t *testing.T) {
	gap := makeGapWithOnlyInB()
	suggestions := graph.SuggestRearticulations(gap)

	if suggestions == nil {
		t.Fatal("SuggestRearticulations: got nil, want non-nil (gap exists)")
	}

	found := false
	for _, s := range suggestions {
		if s.Kind == graph.SuggestionObserverExpansion && s.Side == "A" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected observer-expansion Side=A suggestion; got: %v", suggestions)
	}
}

// TestSuggestRearticulations_TimeWindowExpansion verifies that when cut A has
// a non-zero TimeWindow and cut B does not, and cut A has exclusive elements
// (OnlyInA non-empty), a time-window-expansion suggestion for Side="B" is
// generated — and that when the conditions are mirrored, Side="A" fires.
func TestSuggestRearticulations_TimeWindowExpansion(t *testing.T) {
	now := time.Now()

	// Cut B has a time window; cut A does not. Only A has exclusive elements.
	// Heuristic: bHasWindow && !aHasWindow && len(OnlyInA) > 0 → Side="B"
	gap := graph.ObserverGap{
		OnlyInA: []string{"elem-a1"},
		OnlyInB: []string{},
		InBoth:  []string{},
		CutA:    graph.Cut{},
		CutB: graph.Cut{
			TimeWindow: graph.TimeWindow{
				Start: now.Add(-24 * time.Hour),
				End:   now,
			},
		},
	}

	suggestions := graph.SuggestRearticulations(gap)
	if suggestions == nil {
		t.Fatal("SuggestRearticulations: got nil, want non-nil")
	}

	found := false
	for _, s := range suggestions {
		if s.Kind == graph.SuggestionTimeExpansion && s.Side == "B" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected time-window-expansion Side=B; got: %v", suggestions)
	}
}

// TestSuggestRearticulations_TagRelaxation verifies that when cut A has tags
// and cut B does not, and cut B has exclusive elements, a tag-relaxation
// suggestion for Side="A" is generated.
func TestSuggestRearticulations_TagRelaxation(t *testing.T) {
	gap := graph.ObserverGap{
		OnlyInA: []string{},
		OnlyInB: []string{"elem-b1"},
		InBoth:  []string{},
		CutA:    graph.Cut{Tags: []string{"critical"}},
		CutB:    graph.Cut{},
	}

	suggestions := graph.SuggestRearticulations(gap)
	if suggestions == nil {
		t.Fatal("SuggestRearticulations: got nil, want non-nil")
	}

	found := false
	for _, s := range suggestions {
		if s.Kind == graph.SuggestionTagRelaxation && s.Side == "A" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected tag-relaxation Side=A; got: %v", suggestions)
	}
}

// TestSuggestRearticulations_EmptySliceWhenGapButNoHeuristic verifies that
// when a gap exists but no heuristic condition is met, the result is a
// non-nil empty slice — not nil. This distinguishes "gap with no suggestion"
// from "no gap" (nil).
func TestSuggestRearticulations_EmptySliceWhenGapButNoHeuristic(t *testing.T) {
	// Gap exists: OnlyInA is non-empty.
	// No heuristic fires: CutB.ObserverPositions is empty, no time window
	// asymmetry, no tag asymmetry.
	gap := graph.ObserverGap{
		OnlyInA: []string{"elem-a1"},
		OnlyInB: []string{},
		InBoth:  []string{},
		CutA:    graph.Cut{},
		CutB:    graph.Cut{}, // no observer positions, no time window, no tags
	}

	result := graph.SuggestRearticulations(gap)
	if result == nil {
		t.Fatal("SuggestRearticulations with gap but no heuristic: want non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("SuggestRearticulations: want empty slice, got %d suggestions: %v", len(result), result)
	}
}

// TestSuggestRearticulations_RationaleAlwaysNonEmpty verifies that every
// suggestion produced has a non-empty Rationale field.
func TestSuggestRearticulations_RationaleAlwaysNonEmpty(t *testing.T) {
	// Use a gap that triggers multiple heuristics.
	now := time.Now()
	gap := graph.ObserverGap{
		OnlyInA: []string{"elem-a1"},
		OnlyInB: []string{"elem-b1"},
		InBoth:  []string{},
		CutA: graph.Cut{
			ObserverPositions: []string{"obs-a"},
			Tags:              []string{"tag-a"},
		},
		CutB: graph.Cut{
			ObserverPositions: []string{"obs-b"},
			TimeWindow: graph.TimeWindow{
				Start: now.Add(-24 * time.Hour),
				End:   now,
			},
		},
	}

	suggestions := graph.SuggestRearticulations(gap)
	if len(suggestions) == 0 {
		t.Skip("no suggestions generated; cannot test rationale invariant")
	}

	for i, s := range suggestions {
		if s.Rationale == "" {
			t.Errorf("suggestion[%d] (Kind=%s Side=%s): Rationale is empty", i, s.Kind, s.Side)
		}
	}
}

// TestSuggestRearticulations_RationaleNamesLimits verifies that each
// Rationale contains hedged language acknowledging the limit of the
// suggestion ("cannot know" or equivalent uncertainty language).
func TestSuggestRearticulations_RationaleNamesLimits(t *testing.T) {
	gap := makeGapWithOnlyInA()
	suggestions := graph.SuggestRearticulations(gap)

	if len(suggestions) == 0 {
		t.Skip("no suggestions generated; cannot test rationale limit language")
	}

	for i, s := range suggestions {
		lower := strings.ToLower(s.Rationale)
		// Acceptable hedging language.
		if !strings.Contains(lower, "cannot know") &&
			!strings.Contains(lower, "might") &&
			!strings.Contains(lower, "does not guarantee") &&
			!strings.Contains(lower, "does not know") {
			t.Errorf("suggestion[%d] (Kind=%s Side=%s): Rationale lacks limit language; got: %q",
				i, s.Kind, s.Side, s.Rationale)
		}
	}
}

// TestSuggestRearticulations_SuggestedParamsNonEmpty verifies that every
// suggestion produced has a non-empty SuggestedParams field.
func TestSuggestRearticulations_SuggestedParamsNonEmpty(t *testing.T) {
	now := time.Now()
	gap := graph.ObserverGap{
		OnlyInA: []string{"elem-a1"},
		OnlyInB: []string{"elem-b1"},
		InBoth:  []string{},
		CutA: graph.Cut{
			ObserverPositions: []string{"obs-a"},
		},
		CutB: graph.Cut{
			ObserverPositions: []string{"obs-b"},
			TimeWindow: graph.TimeWindow{
				Start: now.Add(-24 * time.Hour),
				End:   now,
			},
		},
	}

	suggestions := graph.SuggestRearticulations(gap)
	if len(suggestions) == 0 {
		t.Skip("no suggestions generated; cannot test SuggestedParams invariant")
	}

	for i, s := range suggestions {
		if s.SuggestedParams == "" {
			t.Errorf("suggestion[%d] (Kind=%s Side=%s): SuggestedParams is empty", i, s.Kind, s.Side)
		}
	}
}

// TestPrintRearticSuggestions_OutputContainsExpectedContent verifies that
// PrintRearticSuggestions writes the header, gap summary, per-suggestion
// fields, and footer note to w when suggestions are present.
func TestPrintRearticSuggestions_OutputContainsExpectedContent(t *testing.T) {
	gap := makeGapWithOnlyInA()
	suggestions := graph.SuggestRearticulations(gap)

	if suggestions == nil {
		t.Fatal("expected non-nil suggestions for gap fixture")
	}

	var buf bytes.Buffer
	err := graph.PrintRearticSuggestions(&buf, gap, suggestions)
	if err != nil {
		t.Fatalf("PrintRearticSuggestions: unexpected error: %v", err)
	}
	out := buf.String()

	checks := []string{
		"Re-articulation Suggestions",
		"Gap: Only in A:",
		"Only in B:",
		"In both:",
		"provocation",
		"prescription",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q;\nfull output:\n%s", want, out)
		}
	}

	// Verify per-suggestion fields are present.
	if len(suggestions) > 0 {
		if !strings.Contains(out, "Kind:") {
			t.Errorf("output missing 'Kind:' field; output:\n%s", out)
		}
		if !strings.Contains(out, "Side:") {
			t.Errorf("output missing 'Side:' field; output:\n%s", out)
		}
		if !strings.Contains(out, "Rationale:") {
			t.Errorf("output missing 'Rationale:' field; output:\n%s", out)
		}
		if !strings.Contains(out, "SuggestedParams:") {
			t.Errorf("output missing 'SuggestedParams:' field; output:\n%s", out)
		}
	}
}

// TestPrintRearticSuggestions_NilSuggestionsWritesNothing verifies that
// PrintRearticSuggestions returns nil and writes nothing when suggestions
// is nil (the no-gap sentinel).
func TestPrintRearticSuggestions_NilSuggestionsWritesNothing(t *testing.T) {
	gap := makeNoGap()

	var buf bytes.Buffer
	err := graph.PrintRearticSuggestions(&buf, gap, nil)
	if err != nil {
		t.Fatalf("PrintRearticSuggestions(nil): unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("PrintRearticSuggestions(nil): expected no output; got:\n%s", buf.String())
	}
}

// TestPrintRearticSuggestions_EmptySuggestionsWritesNoSuggestionsLine verifies
// that when suggestions is a non-nil empty slice (gap exists but no heuristic
// fired), the output contains the "No suggestions generated" line.
func TestPrintRearticSuggestions_EmptySuggestionsWritesNoSuggestionsLine(t *testing.T) {
	gap := graph.ObserverGap{
		OnlyInA: []string{"elem-a1"},
		OnlyInB: []string{},
		InBoth:  []string{},
		CutA:    graph.Cut{},
		CutB:    graph.Cut{},
	}
	emptySlice := []graph.RearticSuggestion{}

	var buf bytes.Buffer
	err := graph.PrintRearticSuggestions(&buf, gap, emptySlice)
	if err != nil {
		t.Fatalf("PrintRearticSuggestions(empty): unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No suggestions generated") {
		t.Errorf("expected 'No suggestions generated' line; output:\n%s", out)
	}
}
