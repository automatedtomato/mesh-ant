package graph_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// makeGapGraphs returns two articulations of the same trace set from different
// observer positions, for use in gap tests.
//
//   - obsA sees: element-a, element-b (from trace 1)
//   - obsB sees: element-c, element-d (from trace 2)
//   - trace 3 is seen by both: element-shared-src, element-shared-tgt
func makeGapGraphs() (graph.MeshGraph, graph.MeshGraph) {
	traces := []schema.Trace{
		{
			ID:          "a0000000-0000-4000-8000-000000000001",
			WhatChanged: "A sees B",
			Source:      []string{"element-a"},
			Target:      []string{"element-b"},
			Observer:    "obs-a",
		},
		{
			ID:          "b0000000-0000-4000-8000-000000000002",
			WhatChanged: "C sees D",
			Source:      []string{"element-c"},
			Target:      []string{"element-d"},
			Observer:    "obs-b",
		},
		{
			ID:          "c0000000-0000-4000-8000-000000000003",
			WhatChanged: "shared",
			Source:      []string{"element-shared-src"},
			Target:      []string{"element-shared-tgt"},
			Observer:    "obs-a",
		},
	}
	gA := graph.Articulate(traces, graph.ArticulationOptions{ObserverPositions: []string{"obs-a"}})
	gB := graph.Articulate(traces, graph.ArticulationOptions{ObserverPositions: []string{"obs-b"}})
	return gA, gB
}

// --- AnalyseGaps ---

func TestAnalyseGaps_OnlyInA(t *testing.T) {
	gA, gB := makeGapGraphs()
	gap := graph.AnalyseGaps(gA, gB)

	// obs-a sees element-a, element-b, element-shared-src, element-shared-tgt
	// obs-b sees element-c, element-d
	// Only in A: element-a, element-b, element-shared-src, element-shared-tgt
	if len(gap.OnlyInA) != 4 {
		t.Errorf("OnlyInA: got %d want 4; elements: %v", len(gap.OnlyInA), gap.OnlyInA)
	}
}

func TestAnalyseGaps_OnlyInB(t *testing.T) {
	gA, gB := makeGapGraphs()
	gap := graph.AnalyseGaps(gA, gB)

	// Only in B: element-c, element-d
	if len(gap.OnlyInB) != 2 {
		t.Errorf("OnlyInB: got %d want 2; elements: %v", len(gap.OnlyInB), gap.OnlyInB)
	}
}

func TestAnalyseGaps_InBoth(t *testing.T) {
	// Both observers see the same trace.
	traces := []schema.Trace{
		{
			ID: "1", WhatChanged: "shared",
			Source: []string{"shared-src"}, Target: []string{"shared-tgt"},
			Observer: "obs-a",
		},
		{
			ID: "2", WhatChanged: "shared",
			Source: []string{"shared-src"}, Target: []string{"shared-tgt"},
			Observer: "obs-b",
		},
	}
	gA := graph.Articulate(traces, graph.ArticulationOptions{ObserverPositions: []string{"obs-a"}})
	gB := graph.Articulate(traces, graph.ArticulationOptions{ObserverPositions: []string{"obs-b"}})
	gap := graph.AnalyseGaps(gA, gB)

	if len(gap.InBoth) != 2 {
		t.Errorf("InBoth: got %d want 2; elements: %v", len(gap.InBoth), gap.InBoth)
	}
	if len(gap.OnlyInA) != 0 || len(gap.OnlyInB) != 0 {
		t.Errorf("expected no exclusive elements; OnlyInA=%v OnlyInB=%v", gap.OnlyInA, gap.OnlyInB)
	}
}

func TestAnalyseGaps_SortedAlphabetically(t *testing.T) {
	gA, gB := makeGapGraphs()
	gap := graph.AnalyseGaps(gA, gB)

	checkSorted := func(label string, ss []string) {
		for i := 1; i < len(ss); i++ {
			if ss[i-1] > ss[i] {
				t.Errorf("%s not sorted: %q > %q", label, ss[i-1], ss[i])
			}
		}
	}
	checkSorted("OnlyInA", gap.OnlyInA)
	checkSorted("OnlyInB", gap.OnlyInB)
	checkSorted("InBoth", gap.InBoth)
}

func TestAnalyseGaps_CutsPreserved(t *testing.T) {
	gA, gB := makeGapGraphs()
	gap := graph.AnalyseGaps(gA, gB)

	if len(gap.CutA.ObserverPositions) == 0 || gap.CutA.ObserverPositions[0] != "obs-a" {
		t.Errorf("CutA: got %v want [obs-a]", gap.CutA.ObserverPositions)
	}
	if len(gap.CutB.ObserverPositions) == 0 || gap.CutB.ObserverPositions[0] != "obs-b" {
		t.Errorf("CutB: got %v want [obs-b]", gap.CutB.ObserverPositions)
	}
}

func TestAnalyseGaps_NoGap_IdenticalGraphs(t *testing.T) {
	traces := []schema.Trace{
		{ID: "1", WhatChanged: "x", Source: []string{"a"}, Target: []string{"b"}, Observer: "obs"},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	gap := graph.AnalyseGaps(g, g)

	if len(gap.OnlyInA) != 0 || len(gap.OnlyInB) != 0 {
		t.Errorf("identical graphs: expected no gap; OnlyInA=%v OnlyInB=%v", gap.OnlyInA, gap.OnlyInB)
	}
	if len(gap.InBoth) != 2 { // element a and b
		t.Errorf("InBoth: got %d want 2", len(gap.InBoth))
	}
}

// --- PrintObserverGap ---

func TestPrintObserverGap_ContainsExpectedContent(t *testing.T) {
	gA, gB := makeGapGraphs()
	gap := graph.AnalyseGaps(gA, gB)

	var buf bytes.Buffer
	if err := graph.PrintObserverGap(&buf, gap); err != nil {
		t.Fatalf("PrintObserverGap: %v", err)
	}
	out := buf.String()

	checks := []string{
		"Observer Gap",
		"obs-a",
		"obs-b",
		"Only in A",
		"Only in B",
		"element-a",
		"element-c",
		"authoritative",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q;\noutput:\n%s", want, out)
		}
	}
}

func TestPrintObserverGap_NoGapMessage(t *testing.T) {
	traces := []schema.Trace{
		{ID: "1", WhatChanged: "x", Source: []string{"a"}, Target: []string{"b"}, Observer: "obs"},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	gap := graph.AnalyseGaps(g, g)

	var buf bytes.Buffer
	if err := graph.PrintObserverGap(&buf, gap); err != nil {
		t.Fatalf("PrintObserverGap: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "No gap") {
		t.Errorf("expected 'No gap' message; output:\n%s", out)
	}
}
