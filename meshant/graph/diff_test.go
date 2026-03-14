// diff_test.go specifies the behaviour of Diff and PrintDiff in the graph package.
//
// Tests are organised into six groups:
//
//	10. Diff — empty and trivial cases
//	11. Diff — node differences (added, removed, persisted)
//	12. Diff — edge differences (added, removed)
//	13. Diff — shadow shifts (emerged, submerged, reason-changed)
//	14. Diff — cut metadata (From/To stored verbatim, independent copies)
//	15. PrintDiff — output format and section rendering
//
// All tests in this file are written in the RED phase. They will not compile
// until the GraphDiff, PersistedNode, ShadowShift, ShadowShiftKind types and
// Diff / PrintDiff functions are added to graph.go.
package graph_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// --- helpers local to diff tests ---

// emptyGraph returns a zero-value MeshGraph with initialised Nodes map and
// empty Edges/Cut, suitable as a baseline for diff tests.
func emptyGraph() graph.MeshGraph {
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{},
		Edges: []graph.Edge{},
	}
}

// singleNodeGraph returns a MeshGraph containing one node (name) with the
// given appearance count and no edges or shadow elements.
func singleNodeGraph(name string, count int) graph.MeshGraph {
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{
			name: {Name: name, AppearanceCount: count},
		},
		Edges: []graph.Edge{},
	}
}

// singleEdgeGraph returns a MeshGraph containing one edge built from the
// given trace. Source elements are also populated as nodes.
func singleEdgeGraph(tr schema.Trace) graph.MeshGraph {
	nodes := map[string]graph.Node{}
	for _, s := range tr.Source {
		nodes[s] = graph.Node{Name: s, AppearanceCount: 1}
	}
	for _, tgt := range tr.Target {
		nodes[tgt] = graph.Node{Name: tgt, AppearanceCount: 1}
	}
	tags := make([]string, len(tr.Tags))
	copy(tags, tr.Tags)
	srcs := make([]string, len(tr.Source))
	copy(srcs, tr.Source)
	tgts := make([]string, len(tr.Target))
	copy(tgts, tr.Target)
	return graph.MeshGraph{
		Nodes: nodes,
		Edges: []graph.Edge{
			{
				TraceID:     tr.ID,
				WhatChanged: tr.WhatChanged,
				Observer:    tr.Observer,
				Tags:        tags,
				Sources:     srcs,
				Targets:     tgts,
			},
		},
	}
}

// shadowGraph returns a MeshGraph with no included nodes/edges but one shadow
// element carrying the given name and reasons.
func shadowGraph(name string, reasons []graph.ShadowReason) graph.MeshGraph {
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{},
		Edges: []graph.Edge{},
		Cut: graph.Cut{
			ShadowElements: []graph.ShadowElement{
				{Name: name, SeenFrom: []string{"other-observer"}, Reasons: reasons},
			},
		},
	}
}

// reasonsEqual returns true if two ShadowReason slices contain the same
// elements in the same order. Used for asserting reason fields in ShadowShift.
func reasonsEqual(a, b []graph.ShadowReason) bool {
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

// containsShift returns the first ShadowShift with the given name from shifts,
// and a boolean indicating whether it was found.
func containsShift(shifts []graph.ShadowShift, name string) (graph.ShadowShift, bool) {
	for _, s := range shifts {
		if s.Name == name {
			return s, true
		}
	}
	return graph.ShadowShift{}, false
}

// --- Group 10: Diff — empty and trivial cases ---

// TestDiff_IdenticalGraphs_NoChanges verifies that diffing a graph against
// itself produces an empty diff: no nodes added/removed, no edges, no shifts.
// The From and To cuts should be equal.
func TestDiff_IdenticalGraphs_NoChanges(t *testing.T) {
	tr := validTraceWithElements(
		"aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a",
		[]string{"element-alpha"}, []string{"element-beta"},
	)
	g := singleEdgeGraph(tr)
	d := graph.Diff(g, g)

	if len(d.NodesAdded) != 0 {
		t.Errorf("NodesAdded: want 0, got %d: %v", len(d.NodesAdded), d.NodesAdded)
	}
	if len(d.NodesRemoved) != 0 {
		t.Errorf("NodesRemoved: want 0, got %d: %v", len(d.NodesRemoved), d.NodesRemoved)
	}
	if len(d.EdgesAdded) != 0 {
		t.Errorf("EdgesAdded: want 0, got %d", len(d.EdgesAdded))
	}
	if len(d.EdgesRemoved) != 0 {
		t.Errorf("EdgesRemoved: want 0, got %d", len(d.EdgesRemoved))
	}
	if len(d.ShadowShifts) != 0 {
		t.Errorf("ShadowShifts: want 0, got %d", len(d.ShadowShifts))
	}
}

// TestDiff_TwoEmptyGraphs_NoChanges verifies that diffing two zero-value graphs
// produces a completely empty diff. No panic, no stray entries.
func TestDiff_TwoEmptyGraphs_NoChanges(t *testing.T) {
	d := graph.Diff(emptyGraph(), emptyGraph())

	if len(d.NodesAdded) != 0 {
		t.Errorf("NodesAdded: want 0, got %d", len(d.NodesAdded))
	}
	if len(d.NodesRemoved) != 0 {
		t.Errorf("NodesRemoved: want 0, got %d", len(d.NodesRemoved))
	}
	if len(d.NodesPersisted) != 0 {
		t.Errorf("NodesPersisted: want 0, got %d", len(d.NodesPersisted))
	}
	if len(d.EdgesAdded) != 0 {
		t.Errorf("EdgesAdded: want 0, got %d", len(d.EdgesAdded))
	}
	if len(d.EdgesRemoved) != 0 {
		t.Errorf("EdgesRemoved: want 0, got %d", len(d.EdgesRemoved))
	}
	if len(d.ShadowShifts) != 0 {
		t.Errorf("ShadowShifts: want 0, got %d", len(d.ShadowShifts))
	}
}

// TestDiff_EmptyGraphs_CutsStored verifies that From and To are stored even
// when both graphs are empty. TracesTotal and TracesIncluded should be 0.
func TestDiff_EmptyGraphs_CutsStored(t *testing.T) {
	g1 := emptyGraph()
	g1.Cut.TracesTotal = 5
	g1.Cut.TracesIncluded = 2

	g2 := emptyGraph()
	g2.Cut.TracesTotal = 10
	g2.Cut.TracesIncluded = 3

	d := graph.Diff(g1, g2)

	if d.From.TracesTotal != 5 {
		t.Errorf("From.TracesTotal: want 5, got %d", d.From.TracesTotal)
	}
	if d.From.TracesIncluded != 2 {
		t.Errorf("From.TracesIncluded: want 2, got %d", d.From.TracesIncluded)
	}
	if d.To.TracesTotal != 10 {
		t.Errorf("To.TracesTotal: want 10, got %d", d.To.TracesTotal)
	}
	if d.To.TracesIncluded != 3 {
		t.Errorf("To.TracesIncluded: want 3, got %d", d.To.TracesIncluded)
	}
}

// --- Group 11: Diff — node differences ---

// TestDiff_NodeAdded_InG2NotG1 verifies that an element present in g2.Nodes
// but absent from g1.Nodes appears in NodesAdded.
func TestDiff_NodeAdded_InG2NotG1(t *testing.T) {
	g1 := emptyGraph()
	g2 := singleNodeGraph("element-new", 1)

	d := graph.Diff(g1, g2)

	if len(d.NodesAdded) != 1 || d.NodesAdded[0] != "element-new" {
		t.Errorf("NodesAdded: want [element-new], got %v", d.NodesAdded)
	}
	if len(d.NodesRemoved) != 0 {
		t.Errorf("NodesRemoved: want empty, got %v", d.NodesRemoved)
	}
}

// TestDiff_NodeRemoved_InG1NotG2 verifies that an element present in g1.Nodes
// but absent from g2.Nodes appears in NodesRemoved.
func TestDiff_NodeRemoved_InG1NotG2(t *testing.T) {
	g1 := singleNodeGraph("element-gone", 1)
	g2 := emptyGraph()

	d := graph.Diff(g1, g2)

	if len(d.NodesRemoved) != 1 || d.NodesRemoved[0] != "element-gone" {
		t.Errorf("NodesRemoved: want [element-gone], got %v", d.NodesRemoved)
	}
	if len(d.NodesAdded) != 0 {
		t.Errorf("NodesAdded: want empty, got %v", d.NodesAdded)
	}
}

// TestDiff_NodePersisted_InBothGraphs verifies that an element present in both
// g1 and g2 appears in NodesPersisted and not in NodesAdded or NodesRemoved.
func TestDiff_NodePersisted_InBothGraphs(t *testing.T) {
	g1 := singleNodeGraph("element-shared", 2)
	g2 := singleNodeGraph("element-shared", 3)

	d := graph.Diff(g1, g2)

	if len(d.NodesPersisted) != 1 {
		t.Fatalf("NodesPersisted: want 1, got %d: %v", len(d.NodesPersisted), d.NodesPersisted)
	}
	if d.NodesPersisted[0].Name != "element-shared" {
		t.Errorf("NodesPersisted[0].Name: want element-shared, got %q", d.NodesPersisted[0].Name)
	}
	if len(d.NodesAdded) != 0 {
		t.Errorf("NodesAdded: want empty, got %v", d.NodesAdded)
	}
	if len(d.NodesRemoved) != 0 {
		t.Errorf("NodesRemoved: want empty, got %v", d.NodesRemoved)
	}
}

// TestDiff_NodePersisted_CountUnchanged verifies that a persisted node whose
// appearance count did not change has CountFrom == CountTo.
func TestDiff_NodePersisted_CountUnchanged(t *testing.T) {
	g1 := singleNodeGraph("element-stable", 4)
	g2 := singleNodeGraph("element-stable", 4)

	d := graph.Diff(g1, g2)

	if len(d.NodesPersisted) != 1 {
		t.Fatalf("NodesPersisted: want 1, got %d", len(d.NodesPersisted))
	}
	p := d.NodesPersisted[0]
	if p.CountFrom != 4 {
		t.Errorf("CountFrom: want 4, got %d", p.CountFrom)
	}
	if p.CountTo != 4 {
		t.Errorf("CountTo: want 4, got %d", p.CountTo)
	}
}

// TestDiff_NodePersisted_CountChanged verifies that a persisted node whose
// appearance count changed has distinct CountFrom and CountTo values.
func TestDiff_NodePersisted_CountChanged(t *testing.T) {
	g1 := singleNodeGraph("element-growing", 1)
	g2 := singleNodeGraph("element-growing", 5)

	d := graph.Diff(g1, g2)

	if len(d.NodesPersisted) != 1 {
		t.Fatalf("NodesPersisted: want 1, got %d", len(d.NodesPersisted))
	}
	p := d.NodesPersisted[0]
	if p.CountFrom != 1 {
		t.Errorf("CountFrom: want 1, got %d", p.CountFrom)
	}
	if p.CountTo != 5 {
		t.Errorf("CountTo: want 5, got %d", p.CountTo)
	}
}

// TestDiff_NodesAdded_SortedAlphabetically verifies that NodesAdded is
// returned in alphabetical order regardless of map iteration order in g2.
func TestDiff_NodesAdded_SortedAlphabetically(t *testing.T) {
	g1 := emptyGraph()
	g2 := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"zebra":   {Name: "zebra", AppearanceCount: 1},
			"antelope": {Name: "antelope", AppearanceCount: 1},
			"meerkat": {Name: "meerkat", AppearanceCount: 1},
		},
		Edges: []graph.Edge{},
	}

	d := graph.Diff(g1, g2)

	want := []string{"antelope", "meerkat", "zebra"}
	if len(d.NodesAdded) != len(want) {
		t.Fatalf("NodesAdded: want %v, got %v", want, d.NodesAdded)
	}
	for i, name := range want {
		if d.NodesAdded[i] != name {
			t.Errorf("NodesAdded[%d]: want %q, got %q", i, name, d.NodesAdded[i])
		}
	}
}

// TestDiff_NodesRemoved_SortedAlphabetically verifies that NodesRemoved is
// returned in alphabetical order regardless of map iteration order in g1.
func TestDiff_NodesRemoved_SortedAlphabetically(t *testing.T) {
	g1 := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"zebra":   {Name: "zebra", AppearanceCount: 1},
			"antelope": {Name: "antelope", AppearanceCount: 1},
			"meerkat": {Name: "meerkat", AppearanceCount: 1},
		},
		Edges: []graph.Edge{},
	}
	g2 := emptyGraph()

	d := graph.Diff(g1, g2)

	want := []string{"antelope", "meerkat", "zebra"}
	if len(d.NodesRemoved) != len(want) {
		t.Fatalf("NodesRemoved: want %v, got %v", want, d.NodesRemoved)
	}
	for i, name := range want {
		if d.NodesRemoved[i] != name {
			t.Errorf("NodesRemoved[%d]: want %q, got %q", i, name, d.NodesRemoved[i])
		}
	}
}

// TestDiff_NodesPersisted_SortedAlphabetically verifies that NodesPersisted is
// sorted by Name alphabetically.
func TestDiff_NodesPersisted_SortedAlphabetically(t *testing.T) {
	nodes := map[string]graph.Node{
		"zebra":   {Name: "zebra", AppearanceCount: 1},
		"antelope": {Name: "antelope", AppearanceCount: 1},
		"meerkat": {Name: "meerkat", AppearanceCount: 1},
	}
	g1 := graph.MeshGraph{Nodes: nodes, Edges: []graph.Edge{}}
	g2 := graph.MeshGraph{Nodes: nodes, Edges: []graph.Edge{}}

	d := graph.Diff(g1, g2)

	want := []string{"antelope", "meerkat", "zebra"}
	if len(d.NodesPersisted) != len(want) {
		t.Fatalf("NodesPersisted: want %d entries, got %d", len(want), len(d.NodesPersisted))
	}
	for i, name := range want {
		if d.NodesPersisted[i].Name != name {
			t.Errorf("NodesPersisted[%d].Name: want %q, got %q", i, name, d.NodesPersisted[i].Name)
		}
	}
}

// --- Group 12: Diff — edge differences ---

// TestDiff_EdgeAdded_TraceIDInG2NotG1 verifies that an edge whose TraceID
// appears in g2 but not g1 is reported in EdgesAdded.
func TestDiff_EdgeAdded_TraceIDInG2NotG1(t *testing.T) {
	tr := validTraceWithElements(
		"aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a",
		[]string{"alpha"}, []string{"beta"},
	)
	g1 := emptyGraph()
	g2 := singleEdgeGraph(tr)

	d := graph.Diff(g1, g2)

	if len(d.EdgesAdded) != 1 {
		t.Fatalf("EdgesAdded: want 1, got %d", len(d.EdgesAdded))
	}
	if d.EdgesAdded[0].TraceID != tr.ID {
		t.Errorf("EdgesAdded[0].TraceID: want %q, got %q", tr.ID, d.EdgesAdded[0].TraceID)
	}
	if len(d.EdgesRemoved) != 0 {
		t.Errorf("EdgesRemoved: want empty, got %d", len(d.EdgesRemoved))
	}
}

// TestDiff_EdgeRemoved_TraceIDInG1NotG2 verifies that an edge whose TraceID
// appears in g1 but not g2 is reported in EdgesRemoved.
func TestDiff_EdgeRemoved_TraceIDInG1NotG2(t *testing.T) {
	tr := validTraceWithElements(
		"aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a",
		[]string{"alpha"}, []string{"beta"},
	)
	g1 := singleEdgeGraph(tr)
	g2 := emptyGraph()

	d := graph.Diff(g1, g2)

	if len(d.EdgesRemoved) != 1 {
		t.Fatalf("EdgesRemoved: want 1, got %d", len(d.EdgesRemoved))
	}
	if d.EdgesRemoved[0].TraceID != tr.ID {
		t.Errorf("EdgesRemoved[0].TraceID: want %q, got %q", tr.ID, d.EdgesRemoved[0].TraceID)
	}
	if len(d.EdgesAdded) != 0 {
		t.Errorf("EdgesAdded: want empty, got %d", len(d.EdgesAdded))
	}
}

// TestDiff_EdgePersisted_SameTraceID_NotInEitherSlice verifies that an edge
// present in both g1 and g2 (matched by TraceID) is absent from both
// EdgesAdded and EdgesRemoved.
func TestDiff_EdgePersisted_SameTraceID_NotInEitherSlice(t *testing.T) {
	tr := validTraceWithElements(
		"aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a",
		[]string{"alpha"}, []string{"beta"},
	)
	g1 := singleEdgeGraph(tr)
	g2 := singleEdgeGraph(tr)

	d := graph.Diff(g1, g2)

	if len(d.EdgesAdded) != 0 {
		t.Errorf("EdgesAdded: want 0, got %d", len(d.EdgesAdded))
	}
	if len(d.EdgesRemoved) != 0 {
		t.Errorf("EdgesRemoved: want 0, got %d", len(d.EdgesRemoved))
	}
}

// TestDiff_EdgesAdded_FullEdgeStored verifies that the full Edge struct is
// preserved in EdgesAdded, not just the TraceID.
func TestDiff_EdgesAdded_FullEdgeStored(t *testing.T) {
	tr := validTraceWithElements(
		"aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a",
		[]string{"src-node"}, []string{"tgt-node"},
	)
	tr.WhatChanged = "specific change description"
	tr.Mediation = "a-mediator"
	tr.Tags = []string{"translation"}

	g1 := emptyGraph()
	g2 := singleEdgeGraph(tr)

	d := graph.Diff(g1, g2)

	if len(d.EdgesAdded) != 1 {
		t.Fatalf("EdgesAdded: want 1, got %d", len(d.EdgesAdded))
	}
	e := d.EdgesAdded[0]
	if e.WhatChanged != tr.WhatChanged {
		t.Errorf("WhatChanged: want %q, got %q", tr.WhatChanged, e.WhatChanged)
	}
	if e.Observer != tr.Observer {
		t.Errorf("Observer: want %q, got %q", tr.Observer, e.Observer)
	}
	if len(e.Sources) != 1 || e.Sources[0] != "src-node" {
		t.Errorf("Sources: want [src-node], got %v", e.Sources)
	}
	if len(e.Targets) != 1 || e.Targets[0] != "tgt-node" {
		t.Errorf("Targets: want [tgt-node], got %v", e.Targets)
	}
}

// TestDiff_EdgesRemoved_FullEdgeStored verifies that the full Edge struct is
// preserved in EdgesRemoved.
func TestDiff_EdgesRemoved_FullEdgeStored(t *testing.T) {
	tr := validTraceWithElements(
		"aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01", "observer-a",
		[]string{"src-node"}, []string{"tgt-node"},
	)
	tr.WhatChanged = "specific removal description"

	g1 := singleEdgeGraph(tr)
	g2 := emptyGraph()

	d := graph.Diff(g1, g2)

	if len(d.EdgesRemoved) != 1 {
		t.Fatalf("EdgesRemoved: want 1, got %d", len(d.EdgesRemoved))
	}
	e := d.EdgesRemoved[0]
	if e.WhatChanged != tr.WhatChanged {
		t.Errorf("WhatChanged: want %q, got %q", tr.WhatChanged, e.WhatChanged)
	}
}

// TestDiff_EdgesAdded_SortedByTraceID verifies that EdgesAdded is sorted by
// TraceID alphabetically, regardless of edge insertion order in g2.
func TestDiff_EdgesAdded_SortedByTraceID(t *testing.T) {
	t1 := validTraceWithElements("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeee001", "obs", []string{"a"}, nil)
	t2 := validTraceWithElements("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeee002", "obs", []string{"b"}, nil)
	t3 := validTraceWithElements("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeee003", "obs", []string{"c"}, nil)

	g1 := emptyGraph()
	g2 := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"a": {Name: "a", AppearanceCount: 1},
			"b": {Name: "b", AppearanceCount: 1},
			"c": {Name: "c", AppearanceCount: 1},
		},
		Edges: []graph.Edge{
			{TraceID: t3.ID, WhatChanged: t3.WhatChanged, Observer: t3.Observer, Sources: []string{"c"}},
			{TraceID: t1.ID, WhatChanged: t1.WhatChanged, Observer: t1.Observer, Sources: []string{"a"}},
			{TraceID: t2.ID, WhatChanged: t2.WhatChanged, Observer: t2.Observer, Sources: []string{"b"}},
		},
	}

	d := graph.Diff(g1, g2)

	if len(d.EdgesAdded) != 3 {
		t.Fatalf("EdgesAdded: want 3, got %d", len(d.EdgesAdded))
	}
	if d.EdgesAdded[0].TraceID != t1.ID {
		t.Errorf("EdgesAdded[0].TraceID: want %q, got %q", t1.ID, d.EdgesAdded[0].TraceID)
	}
	if d.EdgesAdded[1].TraceID != t2.ID {
		t.Errorf("EdgesAdded[1].TraceID: want %q, got %q", t2.ID, d.EdgesAdded[1].TraceID)
	}
	if d.EdgesAdded[2].TraceID != t3.ID {
		t.Errorf("EdgesAdded[2].TraceID: want %q, got %q", t3.ID, d.EdgesAdded[2].TraceID)
	}
}

// TestDiff_EdgesRemoved_SortedByTraceID verifies that EdgesRemoved is sorted
// by TraceID alphabetically.
func TestDiff_EdgesRemoved_SortedByTraceID(t *testing.T) {
	t1 := validTraceWithElements("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeee001", "obs", []string{"a"}, nil)
	t2 := validTraceWithElements("aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeee002", "obs", []string{"b"}, nil)

	g1 := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"a": {Name: "a", AppearanceCount: 1},
			"b": {Name: "b", AppearanceCount: 1},
		},
		Edges: []graph.Edge{
			{TraceID: t2.ID, WhatChanged: t2.WhatChanged, Observer: t2.Observer},
			{TraceID: t1.ID, WhatChanged: t1.WhatChanged, Observer: t1.Observer},
		},
	}
	g2 := emptyGraph()

	d := graph.Diff(g1, g2)

	if len(d.EdgesRemoved) != 2 {
		t.Fatalf("EdgesRemoved: want 2, got %d", len(d.EdgesRemoved))
	}
	if d.EdgesRemoved[0].TraceID != t1.ID {
		t.Errorf("EdgesRemoved[0].TraceID: want %q, got %q", t1.ID, d.EdgesRemoved[0].TraceID)
	}
}

// --- Group 13: Diff — shadow shifts ---

// TestDiff_ShadowShift_Emerged_ShadowToNode verifies that an element present
// in g1's shadow but visible as a Node in g2 produces a ShadowShift with
// Kind == ShadowShiftEmerged.
func TestDiff_ShadowShift_Emerged_ShadowToNode(t *testing.T) {
	g1 := shadowGraph("emerging-element", []graph.ShadowReason{graph.ShadowReasonObserver})
	g2 := singleNodeGraph("emerging-element", 1)

	d := graph.Diff(g1, g2)

	shift, ok := containsShift(d.ShadowShifts, "emerging-element")
	if !ok {
		t.Fatalf("ShadowShifts: want entry for emerging-element, got %v", d.ShadowShifts)
	}
	if shift.Kind != graph.ShadowShiftEmerged {
		t.Errorf("Kind: want %q, got %q", graph.ShadowShiftEmerged, shift.Kind)
	}
}

// TestDiff_ShadowShift_Emerged_FromReasons_Populated verifies that an emerged
// shift carries the shadow reasons from g1 in FromReasons, and ToReasons is empty.
func TestDiff_ShadowShift_Emerged_FromReasons_Populated(t *testing.T) {
	g1 := shadowGraph("emerging-element", []graph.ShadowReason{graph.ShadowReasonObserver})
	g2 := singleNodeGraph("emerging-element", 1)

	d := graph.Diff(g1, g2)

	shift, ok := containsShift(d.ShadowShifts, "emerging-element")
	if !ok {
		t.Fatalf("ShadowShifts: missing entry for emerging-element")
	}
	if !reasonsEqual(shift.FromReasons, []graph.ShadowReason{graph.ShadowReasonObserver}) {
		t.Errorf("FromReasons: want [observer], got %v", shift.FromReasons)
	}
	if len(shift.ToReasons) != 0 {
		t.Errorf("ToReasons: want empty (element became visible), got %v", shift.ToReasons)
	}
}

// TestDiff_ShadowShift_Submerged_NodeToShadow verifies that an element visible
// as a Node in g1 but in the shadow of g2 produces a ShadowShift with Kind ==
// ShadowShiftSubmerged.
func TestDiff_ShadowShift_Submerged_NodeToShadow(t *testing.T) {
	g1 := singleNodeGraph("submerging-element", 2)
	g2 := shadowGraph("submerging-element", []graph.ShadowReason{graph.ShadowReasonTimeWindow})

	d := graph.Diff(g1, g2)

	shift, ok := containsShift(d.ShadowShifts, "submerging-element")
	if !ok {
		t.Fatalf("ShadowShifts: want entry for submerging-element, got %v", d.ShadowShifts)
	}
	if shift.Kind != graph.ShadowShiftSubmerged {
		t.Errorf("Kind: want %q, got %q", graph.ShadowShiftSubmerged, shift.Kind)
	}
}

// TestDiff_ShadowShift_Submerged_ToReasons_Populated verifies that a submerged
// shift carries g2's shadow reasons in ToReasons, and FromReasons is empty.
func TestDiff_ShadowShift_Submerged_ToReasons_Populated(t *testing.T) {
	g1 := singleNodeGraph("submerging-element", 2)
	g2 := shadowGraph("submerging-element", []graph.ShadowReason{graph.ShadowReasonTimeWindow})

	d := graph.Diff(g1, g2)

	shift, ok := containsShift(d.ShadowShifts, "submerging-element")
	if !ok {
		t.Fatalf("ShadowShifts: missing entry for submerging-element")
	}
	if !reasonsEqual(shift.ToReasons, []graph.ShadowReason{graph.ShadowReasonTimeWindow}) {
		t.Errorf("ToReasons: want [time-window], got %v", shift.ToReasons)
	}
	if len(shift.FromReasons) != 0 {
		t.Errorf("FromReasons: want empty (element was visible in g1), got %v", shift.FromReasons)
	}
}

// TestDiff_ShadowShift_ReasonChanged_ShadowInBoth_DifferentReasons verifies
// that an element in the shadow of both g1 and g2 but with different reasons
// produces a ShadowShift with Kind == ShadowShiftReasonChanged.
func TestDiff_ShadowShift_ReasonChanged_ShadowInBoth_DifferentReasons(t *testing.T) {
	g1 := shadowGraph("shifting-element", []graph.ShadowReason{graph.ShadowReasonObserver})
	g2 := shadowGraph("shifting-element", []graph.ShadowReason{graph.ShadowReasonObserver, graph.ShadowReasonTimeWindow})

	d := graph.Diff(g1, g2)

	shift, ok := containsShift(d.ShadowShifts, "shifting-element")
	if !ok {
		t.Fatalf("ShadowShifts: want entry for shifting-element, got %v", d.ShadowShifts)
	}
	if shift.Kind != graph.ShadowShiftReasonChanged {
		t.Errorf("Kind: want %q, got %q", graph.ShadowShiftReasonChanged, shift.Kind)
	}
}

// TestDiff_ShadowShift_ReasonChanged_FromAndToReasons_Populated verifies that
// a reason-changed shift has both FromReasons (from g1) and ToReasons (from g2)
// populated.
func TestDiff_ShadowShift_ReasonChanged_FromAndToReasons_Populated(t *testing.T) {
	g1 := shadowGraph("shifting-element", []graph.ShadowReason{graph.ShadowReasonObserver})
	g2 := shadowGraph("shifting-element", []graph.ShadowReason{graph.ShadowReasonObserver, graph.ShadowReasonTimeWindow})

	d := graph.Diff(g1, g2)

	shift, ok := containsShift(d.ShadowShifts, "shifting-element")
	if !ok {
		t.Fatalf("ShadowShifts: missing entry for shifting-element")
	}
	if !reasonsEqual(shift.FromReasons, []graph.ShadowReason{graph.ShadowReasonObserver}) {
		t.Errorf("FromReasons: want [observer], got %v", shift.FromReasons)
	}
	if !reasonsEqual(shift.ToReasons, []graph.ShadowReason{graph.ShadowReasonObserver, graph.ShadowReasonTimeWindow}) {
		t.Errorf("ToReasons: want [observer, time-window], got %v", shift.ToReasons)
	}
}

// TestDiff_NoShadowShift_ShadowInBoth_SameReasons verifies that an element in
// the shadow of both g1 and g2 with identical Reasons produces no ShadowShift.
// Elements with unchanged shadow status are not interesting — only movement is.
func TestDiff_NoShadowShift_ShadowInBoth_SameReasons(t *testing.T) {
	reasons := []graph.ShadowReason{graph.ShadowReasonObserver}
	g1 := shadowGraph("static-shadow", reasons)
	g2 := shadowGraph("static-shadow", reasons)

	d := graph.Diff(g1, g2)

	if _, ok := containsShift(d.ShadowShifts, "static-shadow"); ok {
		t.Errorf("ShadowShifts: want no entry for static-shadow (same reasons in both), got one")
	}
}

// TestDiff_NoShadowShift_NodeInBoth verifies that an element that is a Node
// in both g1 and g2 does not appear in ShadowShifts, even if ShadowCount
// differs. ShadowCount is captured in NodesPersisted, not as a shift.
func TestDiff_NoShadowShift_NodeInBoth(t *testing.T) {
	g1 := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"stable-node": {Name: "stable-node", AppearanceCount: 2, ShadowCount: 0},
		},
		Edges: []graph.Edge{},
	}
	g2 := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"stable-node": {Name: "stable-node", AppearanceCount: 2, ShadowCount: 3},
		},
		Edges: []graph.Edge{},
	}

	d := graph.Diff(g1, g2)

	if _, ok := containsShift(d.ShadowShifts, "stable-node"); ok {
		t.Errorf("ShadowShifts: want no entry for stable-node (visible in both graphs)")
	}
}

// TestDiff_ShadowShifts_SortedAlphabetically verifies that ShadowShifts is
// sorted alphabetically by Name.
func TestDiff_ShadowShifts_SortedAlphabetically(t *testing.T) {
	// Two elements that both emerge from shadow to node.
	g1 := graph.MeshGraph{
		Nodes: map[string]graph.Node{},
		Edges: []graph.Edge{},
		Cut: graph.Cut{
			ShadowElements: []graph.ShadowElement{
				{Name: "zebra-elem", SeenFrom: []string{"obs"}, Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
				{Name: "aardvark-elem", SeenFrom: []string{"obs"}, Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
			},
		},
	}
	g2 := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"zebra-elem":   {Name: "zebra-elem", AppearanceCount: 1},
			"aardvark-elem": {Name: "aardvark-elem", AppearanceCount: 1},
		},
		Edges: []graph.Edge{},
	}

	d := graph.Diff(g1, g2)

	if len(d.ShadowShifts) != 2 {
		t.Fatalf("ShadowShifts: want 2, got %d: %v", len(d.ShadowShifts), d.ShadowShifts)
	}
	if d.ShadowShifts[0].Name != "aardvark-elem" {
		t.Errorf("ShadowShifts[0].Name: want aardvark-elem, got %q", d.ShadowShifts[0].Name)
	}
	if d.ShadowShifts[1].Name != "zebra-elem" {
		t.Errorf("ShadowShifts[1].Name: want zebra-elem, got %q", d.ShadowShifts[1].Name)
	}
}

// --- Group 14: Diff — cut metadata ---

// TestDiff_From_StoresG1Cut_Verbatim verifies that GraphDiff.From stores the
// full Cut of g1 verbatim, including ObserverPositions, TimeWindow, and counts.
func TestDiff_From_StoresG1Cut_Verbatim(t *testing.T) {
	g1 := emptyGraph()
	g1.Cut = graph.Cut{
		ObserverPositions:      []string{"satellite-operator"},
		TimeWindow:             graph.TimeWindow{Start: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)},
		TracesIncluded:         5,
		TracesTotal:            40,
		DistinctObserversTotal: 8,
	}
	g2 := emptyGraph()

	d := graph.Diff(g1, g2)

	if len(d.From.ObserverPositions) != 1 || d.From.ObserverPositions[0] != "satellite-operator" {
		t.Errorf("From.ObserverPositions: want [satellite-operator], got %v", d.From.ObserverPositions)
	}
	if d.From.TracesIncluded != 5 {
		t.Errorf("From.TracesIncluded: want 5, got %d", d.From.TracesIncluded)
	}
	if d.From.TracesTotal != 40 {
		t.Errorf("From.TracesTotal: want 40, got %d", d.From.TracesTotal)
	}
	if d.From.DistinctObserversTotal != 8 {
		t.Errorf("From.DistinctObserversTotal: want 8, got %d", d.From.DistinctObserversTotal)
	}
	if d.From.TimeWindow.Start.IsZero() {
		t.Errorf("From.TimeWindow.Start: want non-zero, got zero")
	}
}

// TestDiff_To_StoresG2Cut_Verbatim verifies that GraphDiff.To stores the full
// Cut of g2 verbatim.
func TestDiff_To_StoresG2Cut_Verbatim(t *testing.T) {
	g1 := emptyGraph()
	g2 := emptyGraph()
	g2.Cut = graph.Cut{
		ObserverPositions: []string{"ngo-field-coordinator"},
		TracesIncluded:    3,
		TracesTotal:       40,
	}

	d := graph.Diff(g1, g2)

	if len(d.To.ObserverPositions) != 1 || d.To.ObserverPositions[0] != "ngo-field-coordinator" {
		t.Errorf("To.ObserverPositions: want [ngo-field-coordinator], got %v", d.To.ObserverPositions)
	}
	if d.To.TracesIncluded != 3 {
		t.Errorf("To.TracesIncluded: want 3, got %d", d.To.TracesIncluded)
	}
}

// TestDiff_From_And_To_AreIndependent verifies that mutating g1.Cut.ShadowElements
// after calling Diff does not affect GraphDiff.From.ShadowElements, confirming
// that Diff makes a defensive copy of slice fields.
func TestDiff_From_And_To_AreIndependent(t *testing.T) {
	g1 := emptyGraph()
	g1.Cut.ShadowElements = []graph.ShadowElement{
		{Name: "original-shadow", SeenFrom: []string{"obs"}, Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
	}
	g2 := emptyGraph()

	d := graph.Diff(g1, g2)

	// Mutate g1 after the call.
	g1.Cut.ShadowElements[0].Name = "mutated-shadow"

	// GraphDiff.From must be unaffected.
	if len(d.From.ShadowElements) != 1 {
		t.Fatalf("From.ShadowElements: want 1 entry, got %d", len(d.From.ShadowElements))
	}
	if d.From.ShadowElements[0].Name != "original-shadow" {
		t.Errorf("From.ShadowElements[0].Name: want original-shadow, got %q (defensive copy broken)",
			d.From.ShadowElements[0].Name)
	}
}

// --- Group 15: PrintDiff — output ---

// TestPrintDiff_EmptyDiff_AllSectionsPresent verifies that PrintDiff emits all
// section headers even when the GraphDiff is completely empty. Sections must
// always be present so readers know every question was asked, not skipped.
func TestPrintDiff_EmptyDiff_AllSectionsPresent(t *testing.T) {
	var buf bytes.Buffer
	err := graph.PrintDiff(&buf, graph.GraphDiff{})
	if err != nil {
		t.Fatalf("PrintDiff: unexpected error: %v", err)
	}
	out := buf.String()

	sections := []string{
		"From cut",
		"To cut",
		"Nodes added",
		"Nodes removed",
		"Nodes persisted",
		"Edges added",
		"Edges removed",
		"Shadow shifts",
	}
	for _, s := range sections {
		if !strings.Contains(out, s) {
			t.Errorf("output: missing section %q", s)
		}
	}
}

// TestPrintDiff_EmptyDiff_NodeSections_ShowNone verifies that empty node
// sections render as "(none)" rather than being blank.
func TestPrintDiff_EmptyDiff_NodeSections_ShowNone(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, graph.GraphDiff{}); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(none)") {
		t.Errorf("output: expected at least one '(none)' for empty sections, got:\n%s", out)
	}
}

// TestPrintDiff_EmptyDiff_EdgeSections_ShowNone verifies that empty edge
// sections render as "(none)".
func TestPrintDiff_EmptyDiff_EdgeSections_ShowNone(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, graph.GraphDiff{}); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	if !strings.Contains(buf.String(), "(none)") {
		t.Errorf("output: expected (none) for empty edge sections")
	}
}

// TestPrintDiff_EmptyDiff_ShadowShifts_ShowNone verifies that an empty
// ShadowShifts slice renders as "(none)".
func TestPrintDiff_EmptyDiff_ShadowShifts_ShowNone(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, graph.GraphDiff{}); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	if !strings.Contains(buf.String(), "(none)") {
		t.Errorf("output: expected (none) for empty ShadowShifts")
	}
}

// TestPrintDiff_NodeAdded_AppearsInOutput verifies that a node name in
// NodesAdded appears in the printed output.
func TestPrintDiff_NodeAdded_AppearsInOutput(t *testing.T) {
	d := graph.GraphDiff{
		NodesAdded: []string{"newly-visible-element"},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	if !strings.Contains(buf.String(), "newly-visible-element") {
		t.Errorf("output: missing newly-visible-element in NodesAdded section")
	}
}

// TestPrintDiff_NodeRemoved_AppearsInOutput verifies that a node name in
// NodesRemoved appears in the printed output.
func TestPrintDiff_NodeRemoved_AppearsInOutput(t *testing.T) {
	d := graph.GraphDiff{
		NodesRemoved: []string{"vanished-element"},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	if !strings.Contains(buf.String(), "vanished-element") {
		t.Errorf("output: missing vanished-element in NodesRemoved section")
	}
}

// TestPrintDiff_NodePersisted_BothCounts_Shown verifies that a persisted node's
// CountFrom and CountTo values both appear in the output.
func TestPrintDiff_NodePersisted_BothCounts_Shown(t *testing.T) {
	d := graph.GraphDiff{
		NodesPersisted: []graph.PersistedNode{
			{Name: "persistent-node", CountFrom: 2, CountTo: 7},
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "persistent-node") {
		t.Errorf("output: missing persistent-node")
	}
	if !strings.Contains(out, "2") || !strings.Contains(out, "7") {
		t.Errorf("output: missing count values 2 and/or 7 for persistent-node")
	}
}

// TestPrintDiff_EdgeAdded_TraceID_Shown verifies that an added edge's TraceID
// (or its prefix) appears in the output.
func TestPrintDiff_EdgeAdded_TraceID_Shown(t *testing.T) {
	d := graph.GraphDiff{
		EdgesAdded: []graph.Edge{
			{TraceID: "abcd1234-bbbb-4ccc-dddd-eeeeeeeeee01", WhatChanged: "new connection formed"},
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	if !strings.Contains(buf.String(), "abcd1234") {
		t.Errorf("output: missing TraceID prefix abcd1234 in EdgesAdded section")
	}
}

// TestPrintDiff_EdgeRemoved_TraceID_Shown verifies that a removed edge's
// TraceID (or its prefix) appears in the output.
func TestPrintDiff_EdgeRemoved_TraceID_Shown(t *testing.T) {
	d := graph.GraphDiff{
		EdgesRemoved: []graph.Edge{
			{TraceID: "deadbeef-bbbb-4ccc-dddd-eeeeeeeeee01", WhatChanged: "connection severed"},
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	if !strings.Contains(buf.String(), "deadbeef") {
		t.Errorf("output: missing TraceID prefix deadbeef in EdgesRemoved section")
	}
}

// TestPrintDiff_ShadowShift_Emerged_KindShown verifies that an emerged
// ShadowShift shows the string "emerged" in the output.
func TestPrintDiff_ShadowShift_Emerged_KindShown(t *testing.T) {
	d := graph.GraphDiff{
		ShadowShifts: []graph.ShadowShift{
			{
				Name:        "surfacing-element",
				Kind:        graph.ShadowShiftEmerged,
				FromReasons: []graph.ShadowReason{graph.ShadowReasonObserver},
			},
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "emerged") {
		t.Errorf("output: missing kind 'emerged' for surfacing-element")
	}
	if !strings.Contains(out, "surfacing-element") {
		t.Errorf("output: missing element name surfacing-element")
	}
}

// TestPrintDiff_ShadowShift_Submerged_KindShown verifies that a submerged
// ShadowShift shows the string "submerged" in the output.
func TestPrintDiff_ShadowShift_Submerged_KindShown(t *testing.T) {
	d := graph.GraphDiff{
		ShadowShifts: []graph.ShadowShift{
			{
				Name:      "sinking-element",
				Kind:      graph.ShadowShiftSubmerged,
				ToReasons: []graph.ShadowReason{graph.ShadowReasonTimeWindow},
			},
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	if !strings.Contains(buf.String(), "submerged") {
		t.Errorf("output: missing kind 'submerged' for sinking-element")
	}
}

// TestPrintDiff_ShadowShift_ReasonChanged_KindShown verifies that a
// reason-changed ShadowShift shows "reason-changed" in the output.
func TestPrintDiff_ShadowShift_ReasonChanged_KindShown(t *testing.T) {
	d := graph.GraphDiff{
		ShadowShifts: []graph.ShadowShift{
			{
				Name:        "shifting-element",
				Kind:        graph.ShadowShiftReasonChanged,
				FromReasons: []graph.ShadowReason{graph.ShadowReasonObserver},
				ToReasons:   []graph.ShadowReason{graph.ShadowReasonObserver, graph.ShadowReasonTimeWindow},
			},
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	if !strings.Contains(buf.String(), "reason-changed") {
		t.Errorf("output: missing kind 'reason-changed' for shifting-element")
	}
}

// TestPrintDiff_CutMetadata_FromAndTo_Shown verifies that observer positions
// and time windows from both From and To cuts appear in the output.
func TestPrintDiff_CutMetadata_FromAndTo_Shown(t *testing.T) {
	d := graph.GraphDiff{
		From: graph.Cut{
			ObserverPositions: []string{"satellite-operator"},
			TimeWindow: graph.TimeWindow{
				Start: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 3, 11, 23, 59, 59, 0, time.UTC),
			},
			TracesIncluded: 5,
			TracesTotal:    40,
		},
		To: graph.Cut{
			ObserverPositions: []string{"satellite-operator"},
			TimeWindow: graph.TimeWindow{
				Start: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 3, 18, 23, 59, 59, 0, time.UTC),
			},
			TracesIncluded: 3,
			TracesTotal:    40,
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "satellite-operator") {
		t.Errorf("output: missing observer position satellite-operator")
	}
	if !strings.Contains(out, "2026-03-11") {
		t.Errorf("output: missing From time window date 2026-03-11")
	}
	if !strings.Contains(out, "2026-03-18") {
		t.Errorf("output: missing To time window date 2026-03-18")
	}
}

// TestPrintDiff_WriteError_Propagated verifies that a write failure is returned
// as a wrapped error from PrintDiff, not silently swallowed.
func TestPrintDiff_WriteError_Propagated(t *testing.T) {
	err := graph.PrintDiff(failWriter{}, graph.GraphDiff{})
	if err == nil {
		t.Fatal("PrintDiff: expected error from failWriter, got nil")
	}
	if !strings.Contains(err.Error(), "PrintDiff") {
		t.Errorf("error: want 'PrintDiff' in message, got %q", err.Error())
	}
}

