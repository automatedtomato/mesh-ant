package graph_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// makeShadowGraph builds a MeshGraph with a controlled shadow for testing.
// It articulates from "observer-a" over two traces: one observed by "observer-a"
// (included) and one by "observer-b" (excluded → shadow).
func makeShadowGraph() graph.MeshGraph {
	traces := []schema.Trace{
		{
			ID:          "a0000000-0000-4000-8000-000000000001",
			WhatChanged: "A sees B",
			Source:      []string{"element-a"},
			Target:      []string{"element-b"},
			Observer:    "observer-a",
		},
		{
			ID:          "b0000000-0000-4000-8000-000000000002",
			WhatChanged: "C sees D",
			Source:      []string{"element-c"},
			Target:      []string{"element-d"},
			Observer:    "observer-b",
		},
	}
	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"observer-a"},
	}
	return graph.Articulate(traces, opts)
}

// --- SummariseShadow ---

func TestSummariseShadow_TotalShadowed(t *testing.T) {
	g := makeShadowGraph()
	s := graph.SummariseShadow(g)

	// observer-b's trace contributes element-c and element-d to the shadow.
	if s.TotalShadowed != 2 {
		t.Errorf("TotalShadowed: got %d want 2", s.TotalShadowed)
	}
}

func TestSummariseShadow_ByReason_Observer(t *testing.T) {
	g := makeShadowGraph()
	s := graph.SummariseShadow(g)

	if s.ByReason["observer"] != 2 {
		t.Errorf("ByReason[observer]: got %d want 2", s.ByReason["observer"])
	}
}

func TestSummariseShadow_SeenFromCounts(t *testing.T) {
	g := makeShadowGraph()
	s := graph.SummariseShadow(g)

	// Both shadow elements are seen from "observer-b".
	if s.SeenFromCounts["observer-b"] != 2 {
		t.Errorf("SeenFromCounts[observer-b]: got %d want 2", s.SeenFromCounts["observer-b"])
	}
}

func TestSummariseShadow_Elements_Sorted(t *testing.T) {
	g := makeShadowGraph()
	s := graph.SummariseShadow(g)

	if len(s.Elements) != 2 {
		t.Fatalf("Elements length: got %d want 2", len(s.Elements))
	}
	// Elements should be sorted alphabetically (element-c < element-d).
	if s.Elements[0].Name > s.Elements[1].Name {
		t.Errorf("Elements not sorted: %q > %q", s.Elements[0].Name, s.Elements[1].Name)
	}
}

func TestSummariseShadow_CutPreserved(t *testing.T) {
	g := makeShadowGraph()
	s := graph.SummariseShadow(g)

	if len(s.Cut.ObserverPositions) == 0 || s.Cut.ObserverPositions[0] != "observer-a" {
		t.Errorf("Cut.ObserverPositions: got %v want [observer-a]", s.Cut.ObserverPositions)
	}
}

func TestSummariseShadow_NoShadow(t *testing.T) {
	// Single-trace graph with no observer filter: no shadow.
	traces := []schema.Trace{
		{
			ID:          "a0000000-0000-4000-8000-000000000001",
			WhatChanged: "x", Source: []string{"a"}, Target: []string{"b"}, Observer: "obs",
		},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	s := graph.SummariseShadow(g)

	if s.TotalShadowed != 0 {
		t.Errorf("TotalShadowed: got %d want 0", s.TotalShadowed)
	}
}

func TestSummariseShadow_ElementsCopied(t *testing.T) {
	// Mutating the returned Elements slice must not affect the original graph.
	g := makeShadowGraph()
	s := graph.SummariseShadow(g)

	originalLen := len(g.Cut.ShadowElements)
	s.Elements = append(s.Elements, graph.ShadowElement{Name: "injected"})

	if len(g.Cut.ShadowElements) != originalLen {
		t.Error("mutating ShadowSummary.Elements affected original MeshGraph.Cut.ShadowElements")
	}
}

// --- PrintShadowSummary ---

func TestPrintShadowSummary_ContainsExpectedContent(t *testing.T) {
	g := makeShadowGraph()
	s := graph.SummariseShadow(g)

	var buf bytes.Buffer
	if err := graph.PrintShadowSummary(&buf, s); err != nil {
		t.Fatalf("PrintShadowSummary: unexpected error: %v", err)
	}

	out := buf.String()
	checks := []string{
		"Shadow Summary",
		"observer-a",
		"Shadow elements: 2",
		"observer",
		"element-c",
		"element-d",
		"observer-b",
		"cut decision",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q;\noutput:\n%s", want, out)
		}
	}
}

func TestPrintShadowSummary_NoShadowMessage(t *testing.T) {
	traces := []schema.Trace{
		{
			ID:          "a0000000-0000-4000-8000-000000000001",
			WhatChanged: "x", Source: []string{"a"}, Target: []string{"b"}, Observer: "obs",
		},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	s := graph.SummariseShadow(g)

	var buf bytes.Buffer
	if err := graph.PrintShadowSummary(&buf, s); err != nil {
		t.Fatalf("PrintShadowSummary: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No shadow") {
		t.Errorf("expected 'No shadow' message; output:\n%s", out)
	}
}

func TestPrintShadowSummary_SeenFromOrdering(t *testing.T) {
	// Build a graph where observer-b has 2 shadow elements and observer-c has 1.
	// observer-b should appear first in the "visible from" section.
	traces := []schema.Trace{
		{ID: "1", WhatChanged: "a", Source: []string{"x"}, Target: []string{"y"}, Observer: "obs-a"},
		{ID: "2", WhatChanged: "b", Source: []string{"p"}, Target: []string{"q"}, Observer: "obs-b"},
		{ID: "3", WhatChanged: "c", Source: []string{"p"}, Target: []string{"r"}, Observer: "obs-b"},
		{ID: "4", WhatChanged: "d", Source: []string{"s"}, Target: []string{"t"}, Observer: "obs-c"},
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"obs-a"},
	})
	s := graph.SummariseShadow(g)

	var buf bytes.Buffer
	if err := graph.PrintShadowSummary(&buf, s); err != nil {
		t.Fatalf("PrintShadowSummary: %v", err)
	}
	out := buf.String()

	// obs-b should appear before obs-c since it covers more shadow elements.
	posB := strings.Index(out, "obs-b")
	posC := strings.Index(out, "obs-c")
	if posB == -1 || posC == -1 {
		t.Fatalf("expected both obs-b and obs-c in output; output:\n%s", out)
	}
	if posB > posC {
		t.Errorf("obs-b (higher count) should appear before obs-c in output")
	}
}
