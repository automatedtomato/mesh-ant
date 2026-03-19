package graph_test

// bottleneck_test.go verifies IdentifyBottlenecks and PrintBottleneckNotes.
//
// Tests follow the TDD-first approach: all tests were written before the
// implementation. The helper makeBottleneckGraph is modelled after makeShadowGraph
// in shadow_test.go — articulate a real graph so tests exercise the full stack.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// --- helpers ---

// makeBottleneckGraph builds a MeshGraph suitable for bottleneck tests.
// It contains:
//   - "hub" element: appears in 3 traces (AC=3), is a mediator in 2 edges, SC=0
//   - "bridge" element: appears in 2 traces (AC=2), mediator in 0, SC=1
//   - "leaf" element: appears in 1 trace (AC=1), mediator in 0, SC=0  (should be excluded)
//   - "cross" element: appears in 1 trace (AC=1), mediator in 1, SC=0 (included by MC)
//
// All traces belong to "observer-a", so no shadow for most elements.
// One extra trace belongs to "observer-b" — "bridge" appears there (SC=1).
func makeBottleneckGraph() graph.MeshGraph {
	traces := []schema.Trace{
		// hub → leaf (hub is mediator)
		{
			ID:          "b1000000-0000-4000-8000-000000000001",
			WhatChanged: "hub mediates leaf",
			Source:      []string{"hub"},
			Target:      []string{"leaf"},
			Mediation:   "hub",
			Observer:    "observer-a",
		},
		// bridge → hub (hub is mediator again)
		{
			ID:          "b1000000-0000-4000-8000-000000000002",
			WhatChanged: "hub mediates bridge",
			Source:      []string{"bridge"},
			Target:      []string{"hub"},
			Mediation:   "hub",
			Observer:    "observer-a",
		},
		// hub → cross (cross is mediator)
		{
			ID:          "b1000000-0000-4000-8000-000000000003",
			WhatChanged: "cross mediates something",
			Source:      []string{"hub"},
			Target:      []string{"cross"},
			Mediation:   "cross",
			Observer:    "observer-a",
		},
		// bridge appears in excluded trace → ShadowCount=1 for bridge
		{
			ID:          "b1000000-0000-4000-8000-000000000004",
			WhatChanged: "bridge seen from b",
			Source:      []string{"bridge"},
			Target:      []string{"shadow-only"},
			Observer:    "observer-b",
		},
	}
	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"observer-a"},
	}
	return graph.Articulate(traces, opts)
}

// --- TestIdentifyBottlenecks_EmptyGraph ---

// TestIdentifyBottlenecks_EmptyGraph verifies that an empty graph returns a
// non-nil empty slice, not nil.
func TestIdentifyBottlenecks_EmptyGraph(t *testing.T) {
	g := graph.Articulate(nil, graph.ArticulationOptions{})

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	if result == nil {
		t.Error("IdentifyBottlenecks(empty): got nil, want non-nil empty slice")
	}
	if len(result) != 0 {
		t.Errorf("IdentifyBottlenecks(empty): got len=%d, want 0", len(result))
	}
}

// --- TestIdentifyBottlenecks_ExcludedWhenAllZero ---

// TestIdentifyBottlenecks_ExcludedWhenAllZero verifies that an element with
// AC=1, MC=0, SC=0 is not included in the bottleneck notes.
func TestIdentifyBottlenecks_ExcludedWhenAllZero(t *testing.T) {
	traces := []schema.Trace{
		{
			ID:          "b2000000-0000-4000-8000-000000000001",
			WhatChanged: "a sees b",
			Source:      []string{"element-once"},
			Target:      []string{"other"},
			Observer:    "obs",
		},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	for _, note := range result {
		if note.Element == "element-once" {
			t.Errorf("element-once (AC=1, MC=0, SC=0) should not be included; got %+v", note)
		}
	}
}

// --- TestIdentifyBottlenecks_IncludedByMediationCount ---

// TestIdentifyBottlenecks_IncludedByMediationCount verifies that an element
// that serves as Edge.Mediation is included and has the correct MediationCount.
func TestIdentifyBottlenecks_IncludedByMediationCount(t *testing.T) {
	g := makeBottleneckGraph()

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	var hubNote *graph.BottleneckNote
	for i, note := range result {
		if note.Element == "hub" {
			hubNote = &result[i]
			break
		}
	}
	if hubNote == nil {
		t.Fatal("hub should be in bottleneck notes (MC=2) but was not found")
	}
	if hubNote.MediationCount != 2 {
		t.Errorf("hub.MediationCount: got %d want 2", hubNote.MediationCount)
	}
}

// --- TestIdentifyBottlenecks_IncludedByAppearanceCount ---

// TestIdentifyBottlenecks_IncludedByAppearanceCount verifies that an element
// with AppearanceCount >= 2 is included, even with MC=0.
func TestIdentifyBottlenecks_IncludedByAppearanceCount(t *testing.T) {
	traces := []schema.Trace{
		{
			ID:          "b3000000-0000-4000-8000-000000000001",
			WhatChanged: "first mention",
			Source:      []string{"repeat-element"},
			Target:      []string{"x"},
			Observer:    "obs",
		},
		{
			ID:          "b3000000-0000-4000-8000-000000000002",
			WhatChanged: "second mention",
			Source:      []string{"repeat-element"},
			Target:      []string{"y"},
			Observer:    "obs",
		},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	found := false
	for _, note := range result {
		if note.Element == "repeat-element" {
			found = true
			if note.AppearanceCount < 2 {
				t.Errorf("repeat-element.AppearanceCount: got %d want >=2", note.AppearanceCount)
			}
		}
	}
	if !found {
		t.Error("repeat-element (AC>=2) should be in bottleneck notes but was not found")
	}
}

// --- TestIdentifyBottlenecks_IncludedByShadowCount ---

// TestIdentifyBottlenecks_IncludedByShadowCount verifies that an element with
// ShadowCount > 0 is included in bottleneck notes.
func TestIdentifyBottlenecks_IncludedByShadowCount(t *testing.T) {
	g := makeBottleneckGraph()

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	// "bridge" has AC=2 (appears in trace 2 as source and in trace 1 as... wait)
	// Actually bridge appears in included traces: trace2 (source) and trace3 (target).
	// bridge also appears in excluded trace → SC=1.
	var bridgeNote *graph.BottleneckNote
	for i, note := range result {
		if note.Element == "bridge" {
			bridgeNote = &result[i]
			break
		}
	}
	if bridgeNote == nil {
		t.Fatal("bridge should be in bottleneck notes (SC=1 and/or AC>=2) but was not found")
	}
	if bridgeNote.ShadowCount < 1 {
		t.Errorf("bridge.ShadowCount: got %d want >=1", bridgeNote.ShadowCount)
	}
}

// --- TestIdentifyBottlenecks_HeuristicExclusion ---

// TestIdentifyBottlenecks_HeuristicExclusion verifies that an element with
// AC=1, MC=0, SC=0 is explicitly excluded by the v1 heuristic.
func TestIdentifyBottlenecks_HeuristicExclusion(t *testing.T) {
	traces := []schema.Trace{
		{
			ID:          "b4000000-0000-4000-8000-000000000001",
			WhatChanged: "a does b",
			Source:      []string{"single-appearance"},
			Target:      []string{"something"},
			Observer:    "obs",
		},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	for _, note := range result {
		if note.Element == "single-appearance" {
			t.Errorf("single-appearance (AC=1, MC=0, SC=0) must be excluded by heuristic; got %+v", note)
		}
	}
}

// --- TestIdentifyBottlenecks_SortOrder_MediationDescPrimary ---

// TestIdentifyBottlenecks_SortOrder_MediationDescPrimary verifies that elements
// with higher MediationCount appear before those with lower MediationCount.
func TestIdentifyBottlenecks_SortOrder_MediationDescPrimary(t *testing.T) {
	// Build a graph where "heavy-mediator" has MC=3 and "light-mediator" has MC=1.
	// Each mediator must also appear as a source or target in at least one trace
	// so that it is present in g.Nodes (IdentifyBottlenecks only considers Nodes).
	traces := []schema.Trace{
		// heavy-mediator appears as mediator in 3 edges, and as source in trace s5
		{ID: "s1", WhatChanged: "m1", Source: []string{"a"}, Target: []string{"b"}, Mediation: "heavy-mediator", Observer: "obs"},
		{ID: "s2", WhatChanged: "m2", Source: []string{"a"}, Target: []string{"c"}, Mediation: "heavy-mediator", Observer: "obs"},
		{ID: "s3", WhatChanged: "m3", Source: []string{"a"}, Target: []string{"d"}, Mediation: "heavy-mediator", Observer: "obs"},
		{ID: "s5", WhatChanged: "hm-node", Source: []string{"heavy-mediator"}, Target: []string{"z"}, Observer: "obs"},
		// light-mediator appears as mediator in 1 edge, and as source in trace s6
		{ID: "s4", WhatChanged: "m4", Source: []string{"b"}, Target: []string{"e"}, Mediation: "light-mediator", Observer: "obs"},
		{ID: "s6", WhatChanged: "lm-node", Source: []string{"light-mediator"}, Target: []string{"z"}, Observer: "obs"},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	heavyIdx, lightIdx := -1, -1
	for i, note := range result {
		switch note.Element {
		case "heavy-mediator":
			heavyIdx = i
		case "light-mediator":
			lightIdx = i
		}
	}
	if heavyIdx == -1 || lightIdx == -1 {
		t.Fatalf("expected both heavy-mediator and light-mediator in result; got %+v", result)
	}
	if heavyIdx > lightIdx {
		t.Errorf("heavy-mediator (MC=3) should sort before light-mediator (MC=1); got positions %d and %d", heavyIdx, lightIdx)
	}
}

// --- TestIdentifyBottlenecks_SortOrder_AppearanceDescSecondary ---

// TestIdentifyBottlenecks_SortOrder_AppearanceDescSecondary verifies that when
// two elements have equal MediationCount, higher AppearanceCount sorts first.
func TestIdentifyBottlenecks_SortOrder_AppearanceDescSecondary(t *testing.T) {
	// "high-ac" and "low-ac" both have MC=0 but different AC.
	// high-ac appears in 3 traces (AC=3), low-ac in 2 (AC=2).
	traces := []schema.Trace{
		{ID: "t1", WhatChanged: "x", Source: []string{"high-ac"}, Target: []string{"z"}, Observer: "obs"},
		{ID: "t2", WhatChanged: "x", Source: []string{"high-ac"}, Target: []string{"z"}, Observer: "obs"},
		{ID: "t3", WhatChanged: "x", Source: []string{"high-ac"}, Target: []string{"z"}, Observer: "obs"},
		{ID: "t4", WhatChanged: "x", Source: []string{"low-ac"}, Target: []string{"z"}, Observer: "obs"},
		{ID: "t5", WhatChanged: "x", Source: []string{"low-ac"}, Target: []string{"z"}, Observer: "obs"},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	highIdx, lowIdx := -1, -1
	for i, note := range result {
		switch note.Element {
		case "high-ac":
			highIdx = i
		case "low-ac":
			lowIdx = i
		}
	}
	if highIdx == -1 || lowIdx == -1 {
		t.Fatalf("expected high-ac and low-ac in result; got %+v", result)
	}
	if highIdx > lowIdx {
		t.Errorf("high-ac (AC=3) should sort before low-ac (AC=2) when MC equal; got positions %d and %d", highIdx, lowIdx)
	}
}

// --- TestIdentifyBottlenecks_SortOrder_NameAlphaTertiary ---

// TestIdentifyBottlenecks_SortOrder_NameAlphaTertiary verifies that when MC
// and AC are equal, elements sort alphabetically by name.
func TestIdentifyBottlenecks_SortOrder_NameAlphaTertiary(t *testing.T) {
	// "aardvark" and "zebra" both have MC=0 and AC=2.
	traces := []schema.Trace{
		{ID: "n1", WhatChanged: "x", Source: []string{"aardvark"}, Target: []string{"z"}, Observer: "obs"},
		{ID: "n2", WhatChanged: "x", Source: []string{"aardvark"}, Target: []string{"z"}, Observer: "obs"},
		{ID: "n3", WhatChanged: "x", Source: []string{"zebra"}, Target: []string{"z"}, Observer: "obs"},
		{ID: "n4", WhatChanged: "x", Source: []string{"zebra"}, Target: []string{"z"}, Observer: "obs"},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	aardIdx, zebraIdx := -1, -1
	for i, note := range result {
		switch note.Element {
		case "aardvark":
			aardIdx = i
		case "zebra":
			zebraIdx = i
		}
	}
	if aardIdx == -1 || zebraIdx == -1 {
		t.Fatalf("expected aardvark and zebra in result; got %+v", result)
	}
	if aardIdx > zebraIdx {
		t.Errorf("aardvark should sort before zebra (alphabetical tiebreak); got positions %d and %d", aardIdx, zebraIdx)
	}
}

// --- TestIdentifyBottlenecks_ReasonNonEmpty ---

// TestIdentifyBottlenecks_ReasonNonEmpty verifies that every returned note has
// a non-empty Reason string.
func TestIdentifyBottlenecks_ReasonNonEmpty(t *testing.T) {
	g := makeBottleneckGraph()

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	if len(result) == 0 {
		t.Fatal("expected at least one note from makeBottleneckGraph; got none")
	}
	for _, note := range result {
		if note.Reason == "" {
			t.Errorf("note for element %q has empty Reason", note.Element)
		}
	}
}

// --- TestIdentifyBottlenecks_ReasonProvisionalLanguage ---

// TestIdentifyBottlenecks_ReasonProvisionalLanguage verifies that every Reason
// contains "appears" or "from this cut" — provisional, not definitive language.
func TestIdentifyBottlenecks_ReasonProvisionalLanguage(t *testing.T) {
	g := makeBottleneckGraph()

	result := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	if len(result) == 0 {
		t.Fatal("expected at least one note from makeBottleneckGraph; got none")
	}
	for _, note := range result {
		hasAppears := strings.Contains(note.Reason, "appears")
		hasCut := strings.Contains(note.Reason, "from this cut")
		if !hasAppears && !hasCut {
			t.Errorf("note for element %q has non-provisional Reason %q (want 'appears' or 'from this cut')",
				note.Element, note.Reason)
		}
	}
}

// --- TestPrintBottleneckNotes_CutLabelHeader ---

// TestPrintBottleneckNotes_CutLabelHeader verifies that PrintBottleneckNotes
// output contains the observer position and the word "provisional".
func TestPrintBottleneckNotes_CutLabelHeader(t *testing.T) {
	g := makeBottleneckGraph()
	notes := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	var buf bytes.Buffer
	err := graph.PrintBottleneckNotes(&buf, g, notes)
	if err != nil {
		t.Fatalf("PrintBottleneckNotes: unexpected error: %v", err)
	}

	out := buf.String()
	checks := []string{
		"provisional",
		"observer-a",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("PrintBottleneckNotes: output missing %q;\noutput:\n%s", want, out)
		}
	}
}
