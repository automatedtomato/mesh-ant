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
