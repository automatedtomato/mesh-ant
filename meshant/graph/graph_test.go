// Package graph_test contains black-box unit tests for the graph package.
//
// Tests are organised into five groups:
//  1. Articulate — full cut (empty ObserverPositions)
//  2. Articulate — observer filter
//  3. Articulate — Cut metadata
//  4. Articulate — Node and Edge content
//  5. PrintArticulation — output
package graph_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// --- Helpers ---

// validTrace returns a minimal valid schema.Trace with the given id and observer.
// The id must be a lowercase hyphenated UUID string.
func validTrace(id, observer string) schema.Trace {
	return schema.Trace{
		ID:          id,
		Timestamp:   time.Now(),
		WhatChanged: "something changed",
		Observer:    observer,
	}
}

// failWriter is an io.Writer that always returns an error.
// Used to test that PrintArticulation propagates write errors correctly.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write error") }

// --- Group 1: Articulate — Full cut (empty ObserverPositions) ---

// TestArticulate_FullCut_IncludesAllTraces verifies that empty ObserverPositions
// includes all traces in the graph (TracesIncluded == TracesTotal).
func TestArticulate_FullCut_IncludesAllTraces(t *testing.T) {
	traces := []schema.Trace{
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-c"),
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	if g.Cut.TracesIncluded != 3 {
		t.Errorf("TracesIncluded: want 3, got %d", g.Cut.TracesIncluded)
	}
	if g.Cut.TracesTotal != 3 {
		t.Errorf("TracesTotal: want 3, got %d", g.Cut.TracesTotal)
	}
}

// TestArticulate_FullCut_EmptyShadow verifies that a full cut produces no
// shadow elements, because no traces are excluded.
func TestArticulate_FullCut_EmptyShadow(t *testing.T) {
	traces := []schema.Trace{
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b"),
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	if len(g.Cut.ShadowElements) != 0 {
		t.Errorf("ShadowElements: want empty, got %d elements", len(g.Cut.ShadowElements))
	}
}

// TestArticulate_FullCut_NodeCount verifies that all distinct elements from
// source and target slices appear as nodes in the graph.
func TestArticulate_FullCut_NodeCount(t *testing.T) {
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"element-alpha"}
	t1.Target = []string{"element-beta"}

	t2 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b")
	t2.Source = []string{"element-beta"}
	t2.Target = []string{"element-gamma"}

	t3 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-c")
	t3.Source = []string{"element-alpha"}
	// no target

	traces := []schema.Trace{t1, t2, t3}
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	// Distinct elements: element-alpha, element-beta, element-gamma = 3
	if len(g.Nodes) != 3 {
		t.Errorf("Nodes count: want 3, got %d", len(g.Nodes))
	}
}

// TestArticulate_FullCut_EdgeCount verifies that there is exactly one edge
// per included trace.
func TestArticulate_FullCut_EdgeCount(t *testing.T) {
	traces := []schema.Trace{
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-c"),
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	if len(g.Edges) != 3 {
		t.Errorf("Edges count: want 3, got %d", len(g.Edges))
	}
}

// --- Group 2: Articulate — Observer filter ---

// TestArticulate_SingleObserver_TracesIncluded verifies that filtering to one
// observer includes only traces with a matching Observer field.
func TestArticulate_SingleObserver_TracesIncluded(t *testing.T) {
	traces := []schema.Trace{
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-a"),
	}
	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate(traces, opts)
	if g.Cut.TracesIncluded != 2 {
		t.Errorf("TracesIncluded: want 2, got %d", g.Cut.TracesIncluded)
	}
}

// TestArticulate_SingleObserver_ShadowPopulated verifies that applying an
// observer filter produces a non-empty shadow (excluded traces exist).
func TestArticulate_SingleObserver_ShadowPopulated(t *testing.T) {
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"element-alpha"}

	t2 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b")
	t2.Source = []string{"element-beta"} // only visible from observer-b

	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate([]schema.Trace{t1, t2}, opts)
	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty when filter excludes traces with unique elements")
	}
}

// TestArticulate_SingleObserver_ShadowNotInNodes verifies that elements that
// appear only in shadow traces are not present in the Nodes map.
func TestArticulate_SingleObserver_ShadowNotInNodes(t *testing.T) {
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"element-alpha"}

	t2 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b")
	t2.Source = []string{"element-shadow-only"}

	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate([]schema.Trace{t1, t2}, opts)

	if _, ok := g.Nodes["element-shadow-only"]; ok {
		t.Error("Nodes: element-shadow-only should not appear in Nodes (shadow-only element)")
	}
}

// TestArticulate_SingleObserver_NodesFromIncludedOnly verifies that the Nodes
// map contains only elements from included traces (matching observer filter).
func TestArticulate_SingleObserver_NodesFromIncludedOnly(t *testing.T) {
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"included-element"}

	t2 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b")
	t2.Source = []string{"excluded-element"}

	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate([]schema.Trace{t1, t2}, opts)

	if _, ok := g.Nodes["included-element"]; !ok {
		t.Error("Nodes: included-element should be in Nodes")
	}
	if _, ok := g.Nodes["excluded-element"]; ok {
		t.Error("Nodes: excluded-element should not be in Nodes")
	}
}

// TestArticulate_MultiObserver_Union verifies that multiple observer positions
// produce the union of their respective trace sets.
func TestArticulate_MultiObserver_Union(t *testing.T) {
	traces := []schema.Trace{
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-c"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee04", "observer-a"),
	}
	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a", "observer-b"}}
	g := graph.Articulate(traces, opts)
	// observer-a: traces 01, 04 (2); observer-b: trace 02 (1) = 3 total
	if g.Cut.TracesIncluded != 3 {
		t.Errorf("TracesIncluded: want 3, got %d", g.Cut.TracesIncluded)
	}
}

// TestArticulate_UnknownObserver_ZeroTraces verifies that filtering to an
// observer that matches no traces yields TracesIncluded == 0 and all elements
// in the shadow.
func TestArticulate_UnknownObserver_ZeroTraces(t *testing.T) {
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"element-alpha"}
	t1.Target = []string{"element-beta"}

	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-unknown"}}
	g := graph.Articulate([]schema.Trace{t1}, opts)

	if g.Cut.TracesIncluded != 0 {
		t.Errorf("TracesIncluded: want 0, got %d", g.Cut.TracesIncluded)
	}
	// All elements from the dataset should be in shadow
	shadowNames := make(map[string]bool)
	for _, se := range g.Cut.ShadowElements {
		shadowNames[se.Name] = true
	}
	for _, name := range []string{"element-alpha", "element-beta"} {
		if !shadowNames[name] {
			t.Errorf("ShadowElements: want %q in shadow, but not found", name)
		}
	}
}

// --- Group 3: Articulate — Cut metadata ---

// TestArticulate_Cut_TracesTotal verifies that Cut.TracesTotal always equals
// the length of the input slice, regardless of any filter.
func TestArticulate_Cut_TracesTotal(t *testing.T) {
	traces := []schema.Trace{
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-c"),
	}
	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate(traces, opts)
	if g.Cut.TracesTotal != 3 {
		t.Errorf("TracesTotal: want 3, got %d", g.Cut.TracesTotal)
	}
}

// TestArticulate_Cut_DistinctObserversTotal verifies that Cut.DistinctObserversTotal
// counts distinct observer strings across the entire input, not just included traces.
func TestArticulate_Cut_DistinctObserversTotal(t *testing.T) {
	traces := []schema.Trace{
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b"),
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-a"), // duplicate
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee04", "observer-c"),
	}
	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate(traces, opts)
	// Distinct: observer-a, observer-b, observer-c = 3
	if g.Cut.DistinctObserversTotal != 3 {
		t.Errorf("DistinctObserversTotal: want 3, got %d", g.Cut.DistinctObserversTotal)
	}
}

// TestArticulate_Cut_ObserverPositionsStored verifies that the ObserverPositions
// slice from ArticulationOptions is stored verbatim in Cut.ObserverPositions.
func TestArticulate_Cut_ObserverPositionsStored(t *testing.T) {
	traces := []schema.Trace{
		validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a"),
	}
	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a", "observer-b"}}
	g := graph.Articulate(traces, opts)
	if len(g.Cut.ObserverPositions) != 2 {
		t.Fatalf("ObserverPositions length: want 2, got %d", len(g.Cut.ObserverPositions))
	}
	if g.Cut.ObserverPositions[0] != "observer-a" || g.Cut.ObserverPositions[1] != "observer-b" {
		t.Errorf("ObserverPositions: want [observer-a observer-b], got %v", g.Cut.ObserverPositions)
	}
}

// TestArticulate_Cut_ShadowSeenFrom verifies that ShadowElement.SeenFrom lists
// the observer strings from the shadow traces that contain this element.
func TestArticulate_Cut_ShadowSeenFrom(t *testing.T) {
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"visible-element"}

	t2 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b")
	t2.Source = []string{"shadow-element"}

	t3 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-c")
	t3.Target = []string{"shadow-element"} // same element seen from a different observer

	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate([]schema.Trace{t1, t2, t3}, opts)

	var found *graph.ShadowElement
	for i := range g.Cut.ShadowElements {
		if g.Cut.ShadowElements[i].Name == "shadow-element" {
			found = &g.Cut.ShadowElements[i]
			break
		}
	}
	if found == nil {
		t.Fatal("ShadowElements: shadow-element not found")
	}
	// SeenFrom should contain observer-b and observer-c (sorted alphabetically)
	if len(found.SeenFrom) != 2 {
		t.Fatalf("SeenFrom length: want 2, got %d: %v", len(found.SeenFrom), found.SeenFrom)
	}
	if found.SeenFrom[0] != "observer-b" || found.SeenFrom[1] != "observer-c" {
		t.Errorf("SeenFrom: want [observer-b observer-c], got %v", found.SeenFrom)
	}
}

// TestArticulate_Cut_ShadowSortedAlphabetically verifies that Cut.ShadowElements
// is sorted by Name alphabetically, not by order of appearance.
func TestArticulate_Cut_ShadowSortedAlphabetically(t *testing.T) {
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"included-element"}

	// Shadow traces with elements out of alphabetical order
	t2 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b")
	t2.Source = []string{"zebra-element"}

	t3 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-b")
	t3.Source = []string{"apple-element"}

	t4 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee04", "observer-b")
	t4.Source = []string{"mango-element"}

	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate([]schema.Trace{t1, t2, t3, t4}, opts)

	if len(g.Cut.ShadowElements) < 3 {
		t.Fatalf("ShadowElements: want at least 3, got %d", len(g.Cut.ShadowElements))
	}
	names := make([]string, len(g.Cut.ShadowElements))
	for i, se := range g.Cut.ShadowElements {
		names[i] = se.Name
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("ShadowElements not sorted: %q < %q at index %d", names[i], names[i-1], i)
		}
	}
}

// --- Group 4: Articulate — Node and Edge content ---

// TestArticulate_NodeAppearanceCount verifies that AppearanceCount reflects
// total appearances across source and target slices of included traces.
func TestArticulate_NodeAppearanceCount(t *testing.T) {
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"shared-element"}

	t2 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-a")
	t2.Target = []string{"shared-element"}

	opts := graph.ArticulationOptions{}
	g := graph.Articulate([]schema.Trace{t1, t2}, opts)

	node, ok := g.Nodes["shared-element"]
	if !ok {
		t.Fatal("Nodes: shared-element not found")
	}
	if node.AppearanceCount != 2 {
		t.Errorf("AppearanceCount: want 2, got %d", node.AppearanceCount)
	}
}

// TestArticulate_NodeShadowCount verifies that ShadowCount on a node reflects
// how many shadow traces reference it, and is zero for non-shadow nodes.
func TestArticulate_NodeShadowCount(t *testing.T) {
	// included-only is in an included trace and also in a shadow trace
	t1 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a")
	t1.Source = []string{"shared-element"}

	t2 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "observer-b")
	t2.Target = []string{"shared-element"} // shadow trace referencing shared-element

	// pure-included is only in included traces
	t3 := validTrace("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "observer-a")
	t3.Source = []string{"pure-included"}

	opts := graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}}
	g := graph.Articulate([]schema.Trace{t1, t2, t3}, opts)

	sharedNode, ok := g.Nodes["shared-element"]
	if !ok {
		t.Fatal("Nodes: shared-element not found")
	}
	// shared-element appears in 1 shadow trace (t2)
	if sharedNode.ShadowCount != 1 {
		t.Errorf("shared-element ShadowCount: want 1, got %d", sharedNode.ShadowCount)
	}

	pureNode, ok := g.Nodes["pure-included"]
	if !ok {
		t.Fatal("Nodes: pure-included not found")
	}
	// pure-included does not appear in any shadow trace
	if pureNode.ShadowCount != 0 {
		t.Errorf("pure-included ShadowCount: want 0, got %d", pureNode.ShadowCount)
	}
}

// TestArticulate_EdgeFields verifies that an Edge carries all fields from
// the source trace correctly.
func TestArticulate_EdgeFields(t *testing.T) {
	tr := schema.Trace{
		ID:          "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01",
		Timestamp:   time.Now(),
		WhatChanged: "the policy changed",
		Source:      []string{"source-entity"},
		Target:      []string{"target-entity"},
		Mediation:   "some-protocol",
		Tags:        []string{"translation", "delay"},
		Observer:    "observer-a",
	}

	opts := graph.ArticulationOptions{}
	g := graph.Articulate([]schema.Trace{tr}, opts)

	if len(g.Edges) != 1 {
		t.Fatalf("Edges count: want 1, got %d", len(g.Edges))
	}
	e := g.Edges[0]
	if e.TraceID != "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01" {
		t.Errorf("Edge.TraceID: want %q, got %q", "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", e.TraceID)
	}
	if e.WhatChanged != "the policy changed" {
		t.Errorf("Edge.WhatChanged: want %q, got %q", "the policy changed", e.WhatChanged)
	}
	if e.Mediation != "some-protocol" {
		t.Errorf("Edge.Mediation: want %q, got %q", "some-protocol", e.Mediation)
	}
	if e.Observer != "observer-a" {
		t.Errorf("Edge.Observer: want %q, got %q", "observer-a", e.Observer)
	}
	if len(e.Sources) != 1 || e.Sources[0] != "source-entity" {
		t.Errorf("Edge.Sources: want [source-entity], got %v", e.Sources)
	}
	if len(e.Targets) != 1 || e.Targets[0] != "target-entity" {
		t.Errorf("Edge.Targets: want [target-entity], got %v", e.Targets)
	}
	if len(e.Tags) != 2 || e.Tags[0] != "translation" || e.Tags[1] != "delay" {
		t.Errorf("Edge.Tags: want [translation delay], got %v", e.Tags)
	}
}

// TestArticulate_Edge_TagsCopied verifies that mutating a returned Edge.Tags
// slice does not affect subsequent Articulate calls on the same input.
func TestArticulate_Edge_TagsCopied(t *testing.T) {
	tr := schema.Trace{
		ID:          "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01",
		Timestamp:   time.Now(),
		WhatChanged: "something changed",
		Tags:        []string{"translation"},
		Observer:    "observer-a",
	}
	traces := []schema.Trace{tr}

	g1 := graph.Articulate(traces, graph.ArticulationOptions{})
	if len(g1.Edges) != 1 {
		t.Fatalf("first call: want 1 edge, got %d", len(g1.Edges))
	}
	// Mutate the returned Tags slice
	g1.Edges[0].Tags[0] = "MUTATED"

	// Second call should still produce the original tag
	g2 := graph.Articulate(traces, graph.ArticulationOptions{})
	if len(g2.Edges) != 1 {
		t.Fatalf("second call: want 1 edge, got %d", len(g2.Edges))
	}
	if len(g2.Edges[0].Tags) == 0 || g2.Edges[0].Tags[0] != "translation" {
		t.Errorf("Edge.Tags after mutation: want [translation], got %v", g2.Edges[0].Tags)
	}
}

// TestArticulate_Edge_SourcesCopied verifies that mutating a returned
// Edge.Sources slice does not affect subsequent Articulate calls on the same input.
func TestArticulate_Edge_SourcesCopied(t *testing.T) {
	tr := schema.Trace{
		ID:          "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee07",
		Timestamp:   time.Now(),
		WhatChanged: "source mutation test",
		Source:      []string{"original-source"},
		Observer:    "observer-a",
	}
	traces := []schema.Trace{tr}

	g1 := graph.Articulate(traces, graph.ArticulationOptions{})
	if len(g1.Edges) == 0 || len(g1.Edges[0].Sources) == 0 {
		t.Fatalf("first call: want edge with sources, got %v", g1.Edges)
	}
	g1.Edges[0].Sources[0] = "MUTATED"

	g2 := graph.Articulate(traces, graph.ArticulationOptions{})
	if len(g2.Edges) == 0 || len(g2.Edges[0].Sources) == 0 {
		t.Fatalf("second call: want edge with sources, got %v", g2.Edges)
	}
	if g2.Edges[0].Sources[0] != "original-source" {
		t.Errorf("Edge.Sources after mutation: want %q, got %q", "original-source", g2.Edges[0].Sources[0])
	}
}

// TestArticulate_Edge_TargetsCopied verifies that mutating a returned
// Edge.Targets slice does not affect subsequent Articulate calls on the same input.
func TestArticulate_Edge_TargetsCopied(t *testing.T) {
	tr := schema.Trace{
		ID:          "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee08",
		Timestamp:   time.Now(),
		WhatChanged: "target mutation test",
		Target:      []string{"original-target"},
		Observer:    "observer-a",
	}
	traces := []schema.Trace{tr}

	g1 := graph.Articulate(traces, graph.ArticulationOptions{})
	if len(g1.Edges) == 0 || len(g1.Edges[0].Targets) == 0 {
		t.Fatalf("first call: want edge with targets, got %v", g1.Edges)
	}
	g1.Edges[0].Targets[0] = "MUTATED"

	g2 := graph.Articulate(traces, graph.ArticulationOptions{})
	if len(g2.Edges) == 0 || len(g2.Edges[0].Targets) == 0 {
		t.Fatalf("second call: want edge with targets, got %v", g2.Edges)
	}
	if g2.Edges[0].Targets[0] != "original-target" {
		t.Errorf("Edge.Targets after mutation: want %q, got %q", "original-target", g2.Edges[0].Targets[0])
	}
}

// TestArticulate_NodeShadowCount_PerTraceDedup verifies that an element
// appearing in both Source and Target of the same excluded trace contributes
// ShadowCount of 1, not 2. ShadowCount is per-trace, not per-appearance.
func TestArticulate_NodeShadowCount_PerTraceDedup(t *testing.T) {
	// included trace: element-x in source (so it ends up in Nodes)
	included := schema.Trace{
		ID:          "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee09",
		Timestamp:   time.Now(),
		WhatChanged: "included trace",
		Source:      []string{"element-x"},
		Observer:    "observer-a",
	}
	// excluded trace: element-x in BOTH source and target of the same trace
	excluded := schema.Trace{
		ID:          "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee10",
		Timestamp:   time.Now(),
		WhatChanged: "excluded trace — element-x in both source and target",
		Source:      []string{"element-x"},
		Target:      []string{"element-x"},
		Observer:    "observer-b",
	}

	g := graph.Articulate([]schema.Trace{included, excluded},
		graph.ArticulationOptions{ObserverPositions: []string{"observer-a"}})

	node, ok := g.Nodes["element-x"]
	if !ok {
		t.Fatal("Nodes: element-x not found")
	}
	if node.ShadowCount != 1 {
		t.Errorf("Node.ShadowCount for element-x: want 1 (per-trace dedup), got %d", node.ShadowCount)
	}
}

// TestArticulate_EmptyInput verifies that Articulate on nil input returns a
// zero MeshGraph without panicking.
func TestArticulate_EmptyInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Articulate(nil, opts) panicked: %v", r)
		}
	}()
	g := graph.Articulate(nil, graph.ArticulationOptions{})
	// All fields should be zero/nil — just confirm no panic and basic invariants
	if g.Cut.TracesTotal != 0 {
		t.Errorf("TracesTotal: want 0, got %d", g.Cut.TracesTotal)
	}
	if g.Cut.TracesIncluded != 0 {
		t.Errorf("TracesIncluded: want 0, got %d", g.Cut.TracesIncluded)
	}
}

// --- Group 5: PrintArticulation — Output ---

// newGraphForPrint builds a minimal MeshGraph suitable for PrintArticulation tests.
func newGraphForPrint() graph.MeshGraph {
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"element-alpha": {Name: "element-alpha", AppearanceCount: 2},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01",
				WhatChanged: "something changed",
				Observer:    "observer-a",
				Tags:        []string{"translation"},
			},
		},
		Cut: graph.Cut{
			ObserverPositions:         []string{"observer-a"},
			TracesIncluded:            1,
			TracesTotal:               2,
			DistinctObserversTotal:    2,
			ShadowElements:            []graph.ShadowElement{{Name: "shadow-element", SeenFrom: []string{"observer-b"}}},
			ExcludedObserverPositions: []string{"observer-b"},
		},
	}
}

// TestPrintArticulation_ContainsHeader verifies that PrintArticulation output
// contains the expected articulation header line.
func TestPrintArticulation_ContainsHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrint()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "=== Mesh Articulation") {
		t.Errorf("output missing header %q\nGot:\n%s", "=== Mesh Articulation", buf.String())
	}
}

// TestPrintArticulation_ContainsObserverLine verifies that PrintArticulation
// output contains the observer position string used for the cut.
func TestPrintArticulation_ContainsObserverLine(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrint()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "observer-a") {
		t.Errorf("output missing observer position %q\nGot:\n%s", "observer-a", buf.String())
	}
}

// TestPrintArticulation_ContainsNodesSection verifies that PrintArticulation
// output contains a section labelled "Nodes".
func TestPrintArticulation_ContainsNodesSection(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrint()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Nodes") {
		t.Errorf("output missing %q section\nGot:\n%s", "Nodes", buf.String())
	}
}

// TestPrintArticulation_ContainsEdgesSection verifies that PrintArticulation
// output contains a section labelled "Edges".
func TestPrintArticulation_ContainsEdgesSection(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrint()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Edges") {
		t.Errorf("output missing %q section\nGot:\n%s", "Edges", buf.String())
	}
}

// TestPrintArticulation_ContainsShadowSection verifies that PrintArticulation
// always outputs a Shadow section, even when the shadow is non-empty.
func TestPrintArticulation_ContainsShadowSection(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrint()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Shadow") {
		t.Errorf("output missing %q section\nGot:\n%s", "Shadow", buf.String())
	}
}

// TestPrintArticulation_EmptyShadow_ShowsNoneMarker verifies that when the
// shadow is empty (full cut taken), the Shadow section still appears and
// contains the "(none — full cut taken)" marker. This encodes the ANT
// commitment that the absence of shadow is itself a named state, not silence.
func TestPrintArticulation_EmptyShadow_ShowsNoneMarker(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{"element-a": {Name: "element-a", AppearanceCount: 1}},
		Edges: []graph.Edge{{TraceID: "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", WhatChanged: "x", Observer: "obs"}},
		Cut: graph.Cut{
			ObserverPositions:      nil, // full cut — no filter
			TracesIncluded:         1,
			TracesTotal:            1,
			DistinctObserversTotal: 1,
			ShadowElements:         nil, // empty shadow
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, g); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Shadow") {
		t.Errorf("output missing Shadow section\nGot:\n%s", out)
	}
	if !strings.Contains(out, "none") {
		t.Errorf("output missing %q marker for empty shadow\nGot:\n%s", "none", out)
	}
}

// TestPrintArticulation_ContainsFooter verifies that PrintArticulation output
// contains the mandatory footer note encoding the graph-as-cut commitment.
func TestPrintArticulation_ContainsFooter(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrint()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "this graph is a cut") {
		t.Errorf("output missing footer phrase %q\nGot:\n%s", "this graph is a cut", buf.String())
	}
}

// TestPrintArticulation_WriterErrorPropagated verifies that PrintArticulation
// returns a non-nil error when the underlying writer fails.
func TestPrintArticulation_WriterErrorPropagated(t *testing.T) {
	err := graph.PrintArticulation(failWriter{}, newGraphForPrint())
	if err == nil {
		t.Error("PrintArticulation: want non-nil error from failing writer, got nil")
	}
}

// TestPrintArticulation_EmptyGraph_DoesNotPanic verifies that PrintArticulation
// on a zero-value MeshGraph does not panic.
func TestPrintArticulation_EmptyGraph_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintArticulation on zero MeshGraph panicked: %v", r)
		}
	}()
	var buf bytes.Buffer
	_ = graph.PrintArticulation(&buf, graph.MeshGraph{})
}

// --- Group 6: Articulate — TimeWindow filter ---

// traceAtTime returns a minimal valid trace with the given id, observer, and timestamp.
// The id must be a lowercase hyphenated UUID string.
// Used by Group 6 and 7 tests to exercise time-window filtering.
func traceAtTime(id, observer string, ts time.Time) schema.Trace {
	return schema.Trace{
		ID:          id,
		Timestamp:   ts,
		WhatChanged: "test",
		Observer:    observer,
		Source:      []string{"src"},
		Target:      []string{"tgt"},
	}
}

// mustParseTime parses an RFC3339 string and panics on failure.
// Panic is appropriate in test helpers: a parse failure is a test authoring
// error, not a runtime failure under test.
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("mustParseTime: " + err.Error())
	}
	return t
}

// TestArticulate_TimeWindow_Start_ExcludesEarlierTraces verifies that a trace
// with a timestamp before TimeWindow.Start is excluded from the graph.
func TestArticulate_TimeWindow_Start_ExcludesEarlierTraces(t *testing.T) {
	start := mustParseTime("2026-03-11T10:00:00Z")
	before := mustParseTime("2026-03-11T09:00:00Z")
	after := mustParseTime("2026-03-11T11:00:00Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", before),
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-a", after),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: start},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1, got %d", g.Cut.TracesIncluded)
	}
	if g.Edges[0].TraceID != "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02" {
		t.Errorf("included trace: want ee02, got %s", g.Edges[0].TraceID)
	}
}

// TestArticulate_TimeWindow_End_ExcludesLaterTraces verifies that a trace
// with a timestamp after TimeWindow.End is excluded from the graph.
func TestArticulate_TimeWindow_End_ExcludesLaterTraces(t *testing.T) {
	end := mustParseTime("2026-03-11T10:00:00Z")
	before := mustParseTime("2026-03-11T09:00:00Z")
	after := mustParseTime("2026-03-11T11:00:00Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", before),
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-a", after),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{End: end},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1, got %d", g.Cut.TracesIncluded)
	}
	if g.Edges[0].TraceID != "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01" {
		t.Errorf("included trace: want ee01, got %s", g.Edges[0].TraceID)
	}
}

// TestArticulate_TimeWindow_BothBounds_IncludesOnly_WithinWindow verifies that
// only traces whose timestamps fall within [Start, End] are included.
func TestArticulate_TimeWindow_BothBounds_IncludesOnly_WithinWindow(t *testing.T) {
	start := mustParseTime("2026-03-11T09:00:00Z")
	end := mustParseTime("2026-03-11T11:00:00Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", mustParseTime("2026-03-11T08:00:00Z")), // before
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-a", mustParseTime("2026-03-11T10:00:00Z")), // within
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "obs-a", mustParseTime("2026-03-11T12:00:00Z")), // after
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: start, End: end},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1, got %d", g.Cut.TracesIncluded)
	}
	if g.Edges[0].TraceID != "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02" {
		t.Errorf("included trace: want ee02, got %s", g.Edges[0].TraceID)
	}
}

// TestArticulate_TimeWindow_StartAtTimestamp_InclusiveLowerBound verifies
// that a trace with timestamp exactly equal to Start is included (inclusive bound).
func TestArticulate_TimeWindow_StartAtTimestamp_InclusiveLowerBound(t *testing.T) {
	ts := mustParseTime("2026-03-11T10:00:00Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", ts),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: ts},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1 (inclusive lower bound), got %d", g.Cut.TracesIncluded)
	}
}

// TestArticulate_TimeWindow_EndAtTimestamp_InclusiveUpperBound verifies
// that a trace with timestamp exactly equal to End is included (inclusive bound).
func TestArticulate_TimeWindow_EndAtTimestamp_InclusiveUpperBound(t *testing.T) {
	ts := mustParseTime("2026-03-11T10:00:00Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", ts),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{End: ts},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1 (inclusive upper bound), got %d", g.Cut.TracesIncluded)
	}
}

// TestArticulate_TimeWindow_ZeroStart_NoLowerBound verifies that a zero Start
// means no lower bound: even very old traces are included.
func TestArticulate_TimeWindow_ZeroStart_NoLowerBound(t *testing.T) {
	end := mustParseTime("2026-03-11T10:00:00Z")
	ancient := mustParseTime("1970-01-01T00:00:00Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", ancient),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{End: end}, // Start is zero — no lower bound
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1 (zero Start = unbounded below), got %d", g.Cut.TracesIncluded)
	}
}

// TestArticulate_TimeWindow_ZeroEnd_NoUpperBound verifies that a zero End
// means no upper bound: even far-future traces are included.
func TestArticulate_TimeWindow_ZeroEnd_NoUpperBound(t *testing.T) {
	start := mustParseTime("2026-03-11T10:00:00Z")
	future := mustParseTime("2099-12-31T23:59:59Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", future),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: start}, // End is zero — no upper bound
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1 (zero End = unbounded above), got %d", g.Cut.TracesIncluded)
	}
}

// TestArticulate_TimeWindow_IsZero_FullCut verifies that a zero-value TimeWindow
// (both Start and End zero) produces the same result as having no filter at all.
func TestArticulate_TimeWindow_IsZero_FullCut(t *testing.T) {
	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", mustParseTime("2020-01-01T00:00:00Z")),
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-a", mustParseTime("2025-06-15T12:00:00Z")),
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "obs-a", mustParseTime("2030-12-31T23:59:59Z")),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{}, // zero — no filter
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 3 {
		t.Errorf("TracesIncluded: want 3 (zero TimeWindow = no filter), got %d", g.Cut.TracesIncluded)
	}
}

// TestArticulate_TimeWindow_ZeroTracesInWindow verifies that when the window
// matches no traces, TracesIncluded is 0 and all elements appear in shadow.
func TestArticulate_TimeWindow_ZeroTracesInWindow(t *testing.T) {
	start := mustParseTime("2026-06-01T00:00:00Z")
	end := mustParseTime("2026-06-30T23:59:59Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", mustParseTime("2026-03-11T10:00:00Z")),
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-a", mustParseTime("2026-07-01T00:00:00Z")),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: start, End: end},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 0 {
		t.Errorf("TracesIncluded: want 0, got %d", g.Cut.TracesIncluded)
	}
	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty when all traces excluded by time window")
	}
}

// TestArticulate_TimeWindow_CombinedWithObserver_AND_Semantics verifies that
// the observer and time-window filters use AND semantics: a trace must pass
// BOTH filters to be included.
func TestArticulate_TimeWindow_CombinedWithObserver_AND_Semantics(t *testing.T) {
	start := mustParseTime("2026-03-11T09:00:00Z")
	end := mustParseTime("2026-03-11T11:00:00Z")

	traces := []schema.Trace{
		// passes observer but fails time window (too early)
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", mustParseTime("2026-03-11T07:00:00Z")),
		// passes time window but fails observer
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-b", mustParseTime("2026-03-11T10:00:00Z")),
		// passes both observer and time window
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "obs-a", mustParseTime("2026-03-11T10:00:00Z")),
	}
	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"obs-a"},
		TimeWindow:        graph.TimeWindow{Start: start, End: end},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1 (AND semantics), got %d", g.Cut.TracesIncluded)
	}
	if len(g.Edges) == 0 || g.Edges[0].TraceID != "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03" {
		t.Errorf("included trace: want ee03 (passes both filters), got %v", g.Edges)
	}
}

// TestArticulate_TimeWindow_StoredInCut verifies that Cut.TimeWindow reflects
// the Start and End values passed in ArticulationOptions.
func TestArticulate_TimeWindow_StoredInCut(t *testing.T) {
	start := mustParseTime("2026-03-11T00:00:00Z")
	end := mustParseTime("2026-03-11T23:59:59Z")

	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", start),
	}
	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: start, End: end},
	}
	g := graph.Articulate(traces, opts)

	if !g.Cut.TimeWindow.Start.Equal(start) {
		t.Errorf("Cut.TimeWindow.Start: want %v, got %v", start, g.Cut.TimeWindow.Start)
	}
	if !g.Cut.TimeWindow.End.Equal(end) {
		t.Errorf("Cut.TimeWindow.End: want %v, got %v", end, g.Cut.TimeWindow.End)
	}
}

// TestArticulate_TimeWindow_FullCut_StoredAsZero verifies that when no time
// window filter is set, Cut.TimeWindow.IsZero() returns true.
func TestArticulate_TimeWindow_FullCut_StoredAsZero(t *testing.T) {
	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", mustParseTime("2026-03-11T10:00:00Z")),
	}
	opts := graph.ArticulationOptions{} // no TimeWindow
	g := graph.Articulate(traces, opts)

	if !g.Cut.TimeWindow.IsZero() {
		t.Errorf("Cut.TimeWindow.IsZero(): want true for full cut (no window set), got false")
	}
}

// --- Group 7: ShadowReason ---

// TestArticulate_ShadowReason_ObserverOnly verifies that an element excluded
// solely because of the observer filter has Reasons == [ShadowReasonObserver].
func TestArticulate_ShadowReason_ObserverOnly(t *testing.T) {
	ts := mustParseTime("2026-03-11T10:00:00Z")

	t1 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", ts)
	t1.Source = []string{"visible-elem"}

	t2 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-b", ts)
	t2.Source = []string{"shadow-elem"} // excluded only because of observer filter

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"obs-a"},
	}
	g := graph.Articulate([]schema.Trace{t1, t2}, opts)

	var found *graph.ShadowElement
	for i := range g.Cut.ShadowElements {
		if g.Cut.ShadowElements[i].Name == "shadow-elem" {
			found = &g.Cut.ShadowElements[i]
			break
		}
	}
	if found == nil {
		t.Fatal("ShadowElements: shadow-elem not found")
	}
	if len(found.Reasons) != 1 || found.Reasons[0] != graph.ShadowReasonObserver {
		t.Errorf("Reasons: want [observer], got %v", found.Reasons)
	}
}

// TestArticulate_ShadowReason_TimeWindowOnly verifies that an element excluded
// solely because of the time-window filter has Reasons == [ShadowReasonTimeWindow].
func TestArticulate_ShadowReason_TimeWindowOnly(t *testing.T) {
	start := mustParseTime("2026-03-11T12:00:00Z")
	inWindow := mustParseTime("2026-03-11T13:00:00Z")
	outOfWindow := mustParseTime("2026-03-11T08:00:00Z")

	t1 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", inWindow)
	t1.Source = []string{"visible-elem"}

	t2 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-a", outOfWindow)
	t2.Source = []string{"shadow-elem"} // excluded only because of time window

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: start},
	}
	g := graph.Articulate([]schema.Trace{t1, t2}, opts)

	var found *graph.ShadowElement
	for i := range g.Cut.ShadowElements {
		if g.Cut.ShadowElements[i].Name == "shadow-elem" {
			found = &g.Cut.ShadowElements[i]
			break
		}
	}
	if found == nil {
		t.Fatal("ShadowElements: shadow-elem not found")
	}
	if len(found.Reasons) != 1 || found.Reasons[0] != graph.ShadowReasonTimeWindow {
		t.Errorf("Reasons: want [time-window], got %v", found.Reasons)
	}
}

// TestArticulate_ShadowReason_Both verifies that an element excluded by both
// observer filter and time-window filter has both reasons in sorted order.
func TestArticulate_ShadowReason_Both(t *testing.T) {
	start := mustParseTime("2026-03-11T12:00:00Z")
	inWindow := mustParseTime("2026-03-11T13:00:00Z")
	outOfWindow := mustParseTime("2026-03-11T08:00:00Z")

	t1 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", inWindow)
	t1.Source = []string{"visible-elem"}

	// excluded by BOTH: wrong observer AND before Start
	t2 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-b", outOfWindow)
	t2.Source = []string{"shadow-elem-both"}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"obs-a"},
		TimeWindow:        graph.TimeWindow{Start: start},
	}
	g := graph.Articulate([]schema.Trace{t1, t2}, opts)

	var found *graph.ShadowElement
	for i := range g.Cut.ShadowElements {
		if g.Cut.ShadowElements[i].Name == "shadow-elem-both" {
			found = &g.Cut.ShadowElements[i]
			break
		}
	}
	if found == nil {
		t.Fatal("ShadowElements: shadow-elem-both not found")
	}
	// Sorted: observer before time-window
	if len(found.Reasons) != 2 ||
		found.Reasons[0] != graph.ShadowReasonObserver ||
		found.Reasons[1] != graph.ShadowReasonTimeWindow {
		t.Errorf("Reasons: want [observer time-window], got %v", found.Reasons)
	}
}

// TestArticulate_ShadowReason_FullCut_NoReasons verifies that a full cut
// (no filters) produces no shadow elements, so the Reasons question is moot.
func TestArticulate_ShadowReason_FullCut_NoReasons(t *testing.T) {
	traces := []schema.Trace{
		traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", mustParseTime("2026-03-11T10:00:00Z")),
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{})

	if len(g.Cut.ShadowElements) != 0 {
		t.Errorf("ShadowElements: want 0 for full cut, got %d", len(g.Cut.ShadowElements))
	}
}

// TestArticulate_ShadowReason_ObserverCutOnly_AllReasonsAreObserver verifies
// that when only an observer filter is set, every shadow element has exactly
// Reasons == [ShadowReasonObserver].
func TestArticulate_ShadowReason_ObserverCutOnly_AllReasonsAreObserver(t *testing.T) {
	ts := mustParseTime("2026-03-11T10:00:00Z")

	t1 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", ts)
	t1.Source = []string{"included-elem"}

	t2 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-b", ts)
	t2.Source = []string{"shadow-b"}

	t3 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "obs-c", ts)
	t3.Source = []string{"shadow-c"}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"obs-a"},
	}
	g := graph.Articulate([]schema.Trace{t1, t2, t3}, opts)

	for _, se := range g.Cut.ShadowElements {
		if len(se.Reasons) != 1 || se.Reasons[0] != graph.ShadowReasonObserver {
			t.Errorf("shadow element %q: Reasons want [observer], got %v", se.Name, se.Reasons)
		}
	}
}

// TestArticulate_ShadowReason_TimeWindowCutOnly_AllReasonsAreTimeWindow verifies
// that when only a time-window filter is set, every shadow element has exactly
// Reasons == [ShadowReasonTimeWindow].
func TestArticulate_ShadowReason_TimeWindowCutOnly_AllReasonsAreTimeWindow(t *testing.T) {
	start := mustParseTime("2026-03-11T12:00:00Z")
	inWindow := mustParseTime("2026-03-11T13:00:00Z")
	outOfWindow := mustParseTime("2026-03-11T08:00:00Z")

	t1 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "obs-a", inWindow)
	t1.Source = []string{"included-elem"}

	t2 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", "obs-a", outOfWindow)
	t2.Source = []string{"shadow-early"}

	t3 := traceAtTime("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee03", "obs-b", outOfWindow)
	t3.Source = []string{"shadow-also-early"}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: start},
	}
	g := graph.Articulate([]schema.Trace{t1, t2, t3}, opts)

	for _, se := range g.Cut.ShadowElements {
		if len(se.Reasons) != 1 || se.Reasons[0] != graph.ShadowReasonTimeWindow {
			t.Errorf("shadow element %q: Reasons want [time-window], got %v", se.Name, se.Reasons)
		}
	}
}

// --- Group 8: TimeWindow.IsZero ---

// TestTimeWindow_IsZero_BothZero verifies that a zero-value TimeWindow
// (both Start and End unset) returns true from IsZero.
func TestTimeWindow_IsZero_BothZero(t *testing.T) {
	tw := graph.TimeWindow{}
	if !tw.IsZero() {
		t.Error("IsZero: want true when both Start and End are zero")
	}
}

// TestTimeWindow_IsZero_StartSet verifies that a TimeWindow with only Start
// set returns false from IsZero.
func TestTimeWindow_IsZero_StartSet(t *testing.T) {
	tw := graph.TimeWindow{Start: mustParseTime("2026-03-11T00:00:00Z")}
	if tw.IsZero() {
		t.Error("IsZero: want false when Start is set")
	}
}

// TestTimeWindow_IsZero_EndSet verifies that a TimeWindow with only End
// set returns false from IsZero.
func TestTimeWindow_IsZero_EndSet(t *testing.T) {
	tw := graph.TimeWindow{End: mustParseTime("2026-03-11T23:59:59Z")}
	if tw.IsZero() {
		t.Error("IsZero: want false when End is set")
	}
}

// TestTimeWindow_IsZero_BothSet verifies that a TimeWindow with both Start and
// End set returns false from IsZero.
func TestTimeWindow_IsZero_BothSet(t *testing.T) {
	tw := graph.TimeWindow{
		Start: mustParseTime("2026-03-11T00:00:00Z"),
		End:   mustParseTime("2026-03-11T23:59:59Z"),
	}
	if tw.IsZero() {
		t.Error("IsZero: want false when both Start and End are set")
	}
}

// --- Group 9: PrintArticulation — TimeWindow output ---

// newGraphForPrintWithTimeWindow builds a MeshGraph with a time-window-filtered
// cut for PrintArticulation tests that need a window set.
func newGraphForPrintWithTimeWindow() graph.MeshGraph {
	start := mustParseTime("2026-03-11T00:00:00Z")
	end := mustParseTime("2026-03-14T23:59:59Z")
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"element-alpha": {Name: "element-alpha", AppearanceCount: 1},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01",
				WhatChanged: "something changed",
				Observer:    "observer-a",
				Tags:        []string{"translation"},
			},
		},
		Cut: graph.Cut{
			ObserverPositions:      []string{"observer-a"},
			TimeWindow:             graph.TimeWindow{Start: start, End: end},
			TracesIncluded:         1,
			TracesTotal:            3,
			DistinctObserversTotal: 2,
			ShadowElements: []graph.ShadowElement{
				{
					Name:     "shadow-element",
					SeenFrom: []string{"observer-b"},
					Reasons:  []graph.ShadowReason{graph.ShadowReasonTimeWindow},
				},
			},
			ExcludedObserverPositions: []string{"observer-b"},
		},
	}
}

// newGraphForPrintWithObserverAndTimeWindowShadow builds a graph where one
// shadow element has both observer and time-window reasons, for annotation tests.
func newGraphForPrintWithObserverAndTimeWindowShadow() graph.MeshGraph {
	start := mustParseTime("2026-03-11T00:00:00Z")
	end := mustParseTime("2026-03-14T23:59:59Z")
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"element-alpha": {Name: "element-alpha", AppearanceCount: 1},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01",
				WhatChanged: "something changed",
				Observer:    "observer-a",
			},
		},
		Cut: graph.Cut{
			ObserverPositions:      []string{"observer-a"},
			TimeWindow:             graph.TimeWindow{Start: start, End: end},
			TracesIncluded:         1,
			TracesTotal:            3,
			DistinctObserversTotal: 2,
			ShadowElements: []graph.ShadowElement{
				{
					Name:     "observer-shadow",
					SeenFrom: []string{"observer-b"},
					Reasons:  []graph.ShadowReason{graph.ShadowReasonObserver},
				},
				{
					Name:     "time-shadow",
					SeenFrom: []string{"observer-a"},
					Reasons:  []graph.ShadowReason{graph.ShadowReasonTimeWindow},
				},
				{
					Name:     "both-shadow",
					SeenFrom: []string{"observer-b"},
					Reasons:  []graph.ShadowReason{graph.ShadowReasonObserver, graph.ShadowReasonTimeWindow},
				},
			},
			ExcludedObserverPositions: []string{"observer-b"},
		},
	}
}

// TestPrintArticulation_TimeWindow_LinePresent_WhenSet verifies that
// PrintArticulation output contains a "Time window:" line with RFC3339 dates
// when a TimeWindow is set in the Cut.
func TestPrintArticulation_TimeWindow_LinePresent_WhenSet(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrintWithTimeWindow()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Time window:") {
		t.Errorf("output missing %q line\nGot:\n%s", "Time window:", out)
	}
	// Should contain both RFC3339 timestamps
	if !strings.Contains(out, "2026-03-11T00:00:00Z") {
		t.Errorf("output missing Start timestamp\nGot:\n%s", out)
	}
	if !strings.Contains(out, "2026-03-14T23:59:59Z") {
		t.Errorf("output missing End timestamp\nGot:\n%s", out)
	}
}

// TestPrintArticulation_TimeWindow_LinePresent_WhenZero verifies that
// PrintArticulation output contains a "Time window:" line with a "(none)"
// marker when no time window is set.
func TestPrintArticulation_TimeWindow_LinePresent_WhenZero(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrint()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Time window:") {
		t.Errorf("output missing %q line\nGot:\n%s", "Time window:", out)
	}
	if !strings.Contains(out, "(none") {
		t.Errorf("output missing none marker for zero time window\nGot:\n%s", out)
	}
}

// TestPrintArticulation_TimeWindow_ShadowReasonAnnotated verifies that shadow
// element lines include bracket-enclosed reason annotations.
func TestPrintArticulation_TimeWindow_ShadowReasonAnnotated(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrintWithObserverAndTimeWindowShadow()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[") {
		t.Errorf("shadow output missing bracket annotation\nGot:\n%s", out)
	}
}

// TestPrintArticulation_TimeWindow_ShadowReason_Observer_Annotation verifies
// that a shadow element with Reasons=[observer] has [observer] in the output.
func TestPrintArticulation_TimeWindow_ShadowReason_Observer_Annotation(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrintWithObserverAndTimeWindowShadow()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[observer]") {
		t.Errorf("output missing %q annotation\nGot:\n%s", "[observer]", out)
	}
}

// TestPrintArticulation_TimeWindow_ShadowReason_TimeWindow_Annotation verifies
// that a shadow element with Reasons=[time-window] has [time-window] in the output.
func TestPrintArticulation_TimeWindow_ShadowReason_TimeWindow_Annotation(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrintWithObserverAndTimeWindowShadow()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[time-window]") {
		t.Errorf("output missing %q annotation\nGot:\n%s", "[time-window]", out)
	}
}

// TestPrintArticulation_TimeWindow_ShadowReason_Both_Annotation verifies that
// a shadow element with both reasons renders [observer, time-window] in the output.
func TestPrintArticulation_TimeWindow_ShadowReason_Both_Annotation(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, newGraphForPrintWithObserverAndTimeWindowShadow()); err != nil {
		t.Fatalf("PrintArticulation returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[observer, time-window]") {
		t.Errorf("output missing %q annotation\nGot:\n%s", "[observer, time-window]", out)
	}
}

// --- Group 10: TimeWindow.Validate ---

// TestTimeWindow_Validate_ZeroWindow verifies that a zero TimeWindow is valid.
func TestTimeWindow_Validate_ZeroWindow(t *testing.T) {
	tw := graph.TimeWindow{}
	if err := tw.Validate(); err != nil {
		t.Errorf("Validate on zero TimeWindow: want nil, got %v", err)
	}
}

// TestTimeWindow_Validate_StartOnly verifies that a non-zero Start with zero End is valid.
func TestTimeWindow_Validate_StartOnly(t *testing.T) {
	tw := graph.TimeWindow{Start: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)}
	if err := tw.Validate(); err != nil {
		t.Errorf("Validate on Start-only TimeWindow: want nil, got %v", err)
	}
}

// TestTimeWindow_Validate_EndOnly verifies that a zero Start with non-zero End is valid.
func TestTimeWindow_Validate_EndOnly(t *testing.T) {
	tw := graph.TimeWindow{End: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)}
	if err := tw.Validate(); err != nil {
		t.Errorf("Validate on End-only TimeWindow: want nil, got %v", err)
	}
}

// TestTimeWindow_Validate_StartBeforeEnd verifies that Start < End is valid.
func TestTimeWindow_Validate_StartBeforeEnd(t *testing.T) {
	tw := graph.TimeWindow{
		Start: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
	}
	if err := tw.Validate(); err != nil {
		t.Errorf("Validate on valid TimeWindow: want nil, got %v", err)
	}
}

// TestTimeWindow_Validate_StartEqualsEnd verifies that Start == End is valid
// (a window containing exactly one point in time is legal).
func TestTimeWindow_Validate_StartEqualsEnd(t *testing.T) {
	ts := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	tw := graph.TimeWindow{Start: ts, End: ts}
	if err := tw.Validate(); err != nil {
		t.Errorf("Validate on Start==End TimeWindow: want nil, got %v", err)
	}
}

// TestTimeWindow_Validate_InvertedWindow verifies that Start after End returns an error.
// An inverted window would silently produce a zero-trace articulation, which is a
// programming error rather than a valid empty-result state.
func TestTimeWindow_Validate_InvertedWindow(t *testing.T) {
	tw := graph.TimeWindow{
		Start: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
	}
	if err := tw.Validate(); err == nil {
		t.Error("Validate on inverted TimeWindow (Start > End): want error, got nil")
	}
}

// --- Group 11: Articulate — ShadowReason cross-trace accumulation ---

// TestArticulate_ShadowReason_TwoSeparateTraces_BothReasons verifies that an element
// can accumulate both ShadowReasonObserver and ShadowReasonTimeWindow from two
// *different* excluded traces — one that fails only the observer filter, another
// that fails only the time-window filter. This exercises the union-across-traces
// semantics of shadowInfo.failsObserver / failsTimeWindow.
func TestArticulate_ShadowReason_TwoSeparateTraces_BothReasons(t *testing.T) {
	base := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)

	// Observer filter: "observer-a" only. Time window: base only (not day2).
	// trace1 passes both → included (shared-element enters Nodes, not shadow)
	// trace2 fails observer → excluded (failsObserver)
	// trace3 fails time → excluded (failsTimeWindow)
	//
	// Since shared-element is in an included trace, it will NOT be in
	// ShadowElements. We need an element that is ONLY in excluded traces.
	// Use unique elements per excluded trace to isolate the reason accumulation:
	// elem-obs-only: only in trace2 (observer-excluded)
	// elem-time-only: only in trace3 (time-excluded)
	// elem-both: appears in trace2 AND trace3, each contributing a different reason

	traces2 := []schema.Trace{
		{
			ID: "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee04", Timestamp: base,
			WhatChanged: "excluded-by-observer", Observer: "other-observer",
			Source: []string{"elem-obs-only", "elem-both"}, Target: []string{"tgt4"},
		},
		{
			ID: "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee05", Timestamp: day2,
			WhatChanged: "excluded-by-time", Observer: "observer-a",
			Source: []string{"elem-time-only", "elem-both"}, Target: []string{"tgt5"},
		},
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"observer-a"},
		TimeWindow: graph.TimeWindow{
			Start: base.Add(-time.Hour),
			End:   base.Add(time.Hour),
		},
	}
	g := graph.Articulate(traces2, opts)

	// Find shadow elements
	shadowByName := make(map[string]graph.ShadowElement)
	for _, se := range g.Cut.ShadowElements {
		shadowByName[se.Name] = se
	}

	// elem-obs-only: excluded only by observer filter → [observer]
	if se, ok := shadowByName["elem-obs-only"]; !ok {
		t.Error("elem-obs-only: want in shadow, not found")
	} else if len(se.Reasons) != 1 || se.Reasons[0] != graph.ShadowReasonObserver {
		t.Errorf("elem-obs-only Reasons: want [observer], got %v", se.Reasons)
	}

	// elem-time-only: excluded only by time-window filter → [time-window]
	if se, ok := shadowByName["elem-time-only"]; !ok {
		t.Error("elem-time-only: want in shadow, not found")
	} else if len(se.Reasons) != 1 || se.Reasons[0] != graph.ShadowReasonTimeWindow {
		t.Errorf("elem-time-only Reasons: want [time-window], got %v", se.Reasons)
	}

	// elem-both: in trace2 (fails observer) and trace3 (fails time) →
	// accumulated [observer, time-window] via two separate traces
	if se, ok := shadowByName["elem-both"]; !ok {
		t.Error("elem-both: want in shadow, not found")
	} else if len(se.Reasons) != 2 ||
		se.Reasons[0] != graph.ShadowReasonObserver ||
		se.Reasons[1] != graph.ShadowReasonTimeWindow {
		t.Errorf("elem-both Reasons: want [observer, time-window], got %v", se.Reasons)
	}
}

// TestArticulate_TimeWindow_ShadowCount_FromTimeExcludedTraces verifies that
// Node.ShadowCount is incremented for elements that appear in included traces AND
// in time-window-excluded traces. This covers the case where the shadow contributor
// is a time-excluded trace rather than an observer-excluded trace.
func TestArticulate_TimeWindow_ShadowCount_FromTimeExcludedTraces(t *testing.T) {
	inWindow := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	outWindow := time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)

	// "cross-element" appears in both an in-window trace and an out-of-window trace.
	// It should be in Nodes (from the included trace) with ShadowCount == 1 (one
	// time-excluded trace mentions it).
	traces := []schema.Trace{
		{
			ID: "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", Timestamp: inWindow,
			WhatChanged: "in-window trace", Observer: "observer-a",
			Source: []string{"cross-element"}, Target: []string{"tgt-in"},
		},
		{
			ID: "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee02", Timestamp: outWindow,
			WhatChanged: "out-of-window trace", Observer: "observer-a",
			Source: []string{"cross-element"}, Target: []string{"tgt-out"},
		},
	}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{
			Start: inWindow.Add(-time.Hour),
			End:   inWindow.Add(time.Hour),
		},
	}
	g := graph.Articulate(traces, opts)

	node, ok := g.Nodes["cross-element"]
	if !ok {
		t.Fatal("cross-element: want in Nodes, not found")
	}
	if node.AppearanceCount != 1 {
		t.Errorf("cross-element AppearanceCount: want 1, got %d", node.AppearanceCount)
	}
	if node.ShadowCount != 1 {
		t.Errorf("cross-element ShadowCount: want 1 (one time-excluded trace), got %d", node.ShadowCount)
	}
	// cross-element is in Nodes, so it must NOT also be in ShadowElements
	for _, se := range g.Cut.ShadowElements {
		if se.Name == "cross-element" {
			t.Error("cross-element: must not be in ShadowElements (it is visible from included trace)")
		}
	}
}
