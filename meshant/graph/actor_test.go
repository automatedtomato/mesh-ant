// Package graph_test — tests for graph-as-actor identity functions.
//
// M5.2 adds the ability to assign a stable identifier to a MeshGraph or GraphDiff,
// enabling it to appear as a Source or Target in subsequent traces. Following the
// ANT principle of generalised symmetry, an identified graph is an actant like any
// other — it enters the mesh through the same structural positions.
//
// Tests here verify:
//   - ID assignment (non-empty, unique per call)
//   - Immutability (input not mutated)
//   - Field preservation (Nodes, Cut, From/To not changed)
//   - Reference string formatting and error on unidentified graph
//   - Default behaviour of Articulate and Diff (ID = "" by default)
package graph_test

import (
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// --- Group 17: IdentifyGraph ---

func TestIdentifyGraph_AssignsNonEmptyID(t *testing.T) {
	g := graph.MeshGraph{}
	got := graph.IdentifyGraph(g)
	if got.ID == "" {
		t.Error("IdentifyGraph: returned graph has empty ID; want non-empty")
	}
}

func TestIdentifyGraph_DoesNotMutateInput(t *testing.T) {
	g := graph.MeshGraph{}
	_ = graph.IdentifyGraph(g)
	if g.ID != "" {
		t.Errorf("IdentifyGraph mutated input: g.ID = %q; want empty", g.ID)
	}
}

func TestIdentifyGraph_IDIsUnique(t *testing.T) {
	g := graph.MeshGraph{}
	g1 := graph.IdentifyGraph(g)
	g2 := graph.IdentifyGraph(g)
	if g1.ID == g2.ID {
		t.Errorf("IdentifyGraph: two calls produced identical IDs %q; want unique", g1.ID)
	}
}

func TestIdentifyGraph_PreservesNodes(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"landsat-9-satellite": {Name: "landsat-9-satellite", AppearanceCount: 3},
		},
	}
	got := graph.IdentifyGraph(g)
	if len(got.Nodes) != 1 {
		t.Errorf("IdentifyGraph: got %d nodes; want 1", len(got.Nodes))
	}
	n, ok := got.Nodes["landsat-9-satellite"]
	if !ok || n.AppearanceCount != 3 {
		t.Errorf("IdentifyGraph: Nodes not preserved correctly, got %+v", got.Nodes)
	}
}

func TestIdentifyGraph_PreservesCut(t *testing.T) {
	g := graph.MeshGraph{
		Cut: graph.Cut{
			TracesIncluded: 5,
			TracesTotal:    20,
		},
	}
	got := graph.IdentifyGraph(g)
	if got.Cut.TracesIncluded != 5 || got.Cut.TracesTotal != 20 {
		t.Errorf("IdentifyGraph: Cut not preserved: got %+v", got.Cut)
	}
}

// --- Group 18: IdentifyDiff ---

func TestIdentifyDiff_AssignsNonEmptyID(t *testing.T) {
	d := graph.GraphDiff{}
	got := graph.IdentifyDiff(d)
	if got.ID == "" {
		t.Error("IdentifyDiff: returned diff has empty ID; want non-empty")
	}
}

func TestIdentifyDiff_DoesNotMutateInput(t *testing.T) {
	d := graph.GraphDiff{}
	_ = graph.IdentifyDiff(d)
	if d.ID != "" {
		t.Errorf("IdentifyDiff mutated input: d.ID = %q; want empty", d.ID)
	}
}

func TestIdentifyDiff_IDIsUnique(t *testing.T) {
	d := graph.GraphDiff{}
	d1 := graph.IdentifyDiff(d)
	d2 := graph.IdentifyDiff(d)
	if d1.ID == d2.ID {
		t.Errorf("IdentifyDiff: two calls produced identical IDs %q; want unique", d1.ID)
	}
}

// --- Group 19: GraphRef ---

func TestGraphRef_FormatsCorrectly(t *testing.T) {
	g := graph.IdentifyGraph(graph.MeshGraph{})
	ref, err := graph.GraphRef(g)
	if err != nil {
		t.Fatalf("GraphRef returned unexpected error: %v", err)
	}
	want := "meshgraph:" + g.ID
	if ref != want {
		t.Errorf("GraphRef = %q; want %q", ref, want)
	}
}

func TestGraphRef_EmptyID_ReturnsError(t *testing.T) {
	g := graph.MeshGraph{} // not identified
	_, err := graph.GraphRef(g)
	if err == nil {
		t.Error("GraphRef: expected error for unidentified graph, got nil")
	}
}

func TestGraphRef_ErrorMessage_Descriptive(t *testing.T) {
	g := graph.MeshGraph{}
	_, err := graph.GraphRef(g)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "IdentifyGraph") {
		t.Errorf("GraphRef error message does not mention IdentifyGraph: %q", err.Error())
	}
}

// --- Group 20: DiffRef ---

func TestDiffRef_FormatsCorrectly(t *testing.T) {
	d := graph.IdentifyDiff(graph.GraphDiff{})
	ref, err := graph.DiffRef(d)
	if err != nil {
		t.Fatalf("DiffRef returned unexpected error: %v", err)
	}
	want := "meshdiff:" + d.ID
	if ref != want {
		t.Errorf("DiffRef = %q; want %q", ref, want)
	}
}

func TestDiffRef_EmptyID_ReturnsError(t *testing.T) {
	d := graph.GraphDiff{}
	_, err := graph.DiffRef(d)
	if err == nil {
		t.Error("DiffRef: expected error for unidentified diff, got nil")
	}
}

// --- Group 21: Default ID behaviour ---

// TestArticulate_IDEmpty_ByDefault verifies that Articulate returns a graph
// with an empty ID. Actor identity is an explicit opt-in; most articulations
// are produced for analysis and do not need to be actors.
func TestArticulate_IDEmpty_ByDefault(t *testing.T) {
	// Zero traces — minimal articulation that still returns a MeshGraph.
	var traces []schema.Trace
	g := graph.Articulate(traces, graph.ArticulationOptions{})
	if g.ID != "" {
		t.Errorf("Articulate: g.ID = %q; want empty string", g.ID)
	}
}

// TestDiff_IDEmpty_ByDefault verifies that Diff returns a GraphDiff with an
// empty ID. Same opt-in principle as Articulate.
func TestDiff_IDEmpty_ByDefault(t *testing.T) {
	g1 := graph.MeshGraph{}
	g2 := graph.MeshGraph{}
	d := graph.Diff(g1, g2)
	if d.ID != "" {
		t.Errorf("Diff: d.ID = %q; want empty string", d.ID)
	}
}
