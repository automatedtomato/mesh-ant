// chain_test.go — tests for FollowTranslation and TranslationChain types.
// TDD: these tests are written before the implementation.
package graph_test

import (
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// --- Group 1: Linear chain, forward and backward ---

func TestFollowTranslation_LinearForward(t *testing.T) {
	// A --e1--> B --e2--> C --e3--> D
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
		{TraceID: "t2", Sources: []string{"B"}, Targets: []string{"C"}, WhatChanged: "e2"},
		{TraceID: "t3", Sources: []string{"C"}, Targets: []string{"D"}, WhatChanged: "e3"},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	if chain.StartElement != "A" {
		t.Errorf("StartElement = %q, want %q", chain.StartElement, "A")
	}
	if len(chain.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(chain.Steps))
	}

	// Step 1: exited A, entered B
	if chain.Steps[0].ElementExited != "A" {
		t.Errorf("step 0 ElementExited = %q, want %q", chain.Steps[0].ElementExited, "A")
	}
	if chain.Steps[0].ElementEntered != "B" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "B")
	}
	if chain.Steps[0].Edge.TraceID != "t1" {
		t.Errorf("step 0 Edge.TraceID = %q, want %q", chain.Steps[0].Edge.TraceID, "t1")
	}

	// Step 2: exited B, entered C
	if chain.Steps[1].ElementExited != "B" {
		t.Errorf("step 1 ElementExited = %q, want %q", chain.Steps[1].ElementExited, "B")
	}
	if chain.Steps[1].ElementEntered != "C" {
		t.Errorf("step 1 ElementEntered = %q, want %q", chain.Steps[1].ElementEntered, "C")
	}

	// Step 3: exited C, entered D
	if chain.Steps[2].ElementExited != "C" {
		t.Errorf("step 2 ElementExited = %q, want %q", chain.Steps[2].ElementExited, "C")
	}
	if chain.Steps[2].ElementEntered != "D" {
		t.Errorf("step 2 ElementEntered = %q, want %q", chain.Steps[2].ElementEntered, "D")
	}

	// Chain should end with a break at D: no outgoing edges
	hasBreak := false
	for _, b := range chain.Breaks {
		if b.AtElement == "D" && b.Reason == "no-outgoing-edges" {
			hasBreak = true
		}
	}
	if !hasBreak {
		t.Error("expected break at D with reason 'no-outgoing-edges'")
	}
}

func TestFollowTranslation_LinearBackward(t *testing.T) {
	// A --e1--> B --e2--> C --e3--> D
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
		{TraceID: "t2", Sources: []string{"B"}, Targets: []string{"C"}, WhatChanged: "e2"},
		{TraceID: "t3", Sources: []string{"C"}, Targets: []string{"D"}, WhatChanged: "e3"},
	})

	chain := graph.FollowTranslation(g, "D", graph.FollowOptions{
		Direction: graph.DirectionBackward,
	})

	if chain.StartElement != "D" {
		t.Errorf("StartElement = %q, want %q", chain.StartElement, "D")
	}
	if len(chain.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(chain.Steps))
	}

	// Backward: D -> C -> B -> A
	if chain.Steps[0].ElementExited != "D" {
		t.Errorf("step 0 ElementExited = %q, want %q", chain.Steps[0].ElementExited, "D")
	}
	if chain.Steps[0].ElementEntered != "C" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "C")
	}

	if chain.Steps[2].ElementEntered != "A" {
		t.Errorf("step 2 ElementEntered = %q, want %q", chain.Steps[2].ElementEntered, "A")
	}

	// Should end with break at A: no incoming edges
	hasBreak := false
	for _, b := range chain.Breaks {
		if b.AtElement == "A" && b.Reason == "no-incoming-edges" {
			hasBreak = true
		}
	}
	if !hasBreak {
		t.Error("expected break at A with reason 'no-incoming-edges'")
	}
}

// --- Group 2: Element not in graph ---

func TestFollowTranslation_ElementNotInGraph(t *testing.T) {
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
	})

	chain := graph.FollowTranslation(g, "Z", graph.FollowOptions{})

	if chain.StartElement != "Z" {
		t.Errorf("StartElement = %q, want %q", chain.StartElement, "Z")
	}
	if len(chain.Steps) != 0 {
		t.Errorf("got %d steps, want 0", len(chain.Steps))
	}
	if len(chain.Breaks) != 1 {
		t.Fatalf("got %d breaks, want 1", len(chain.Breaks))
	}
	if chain.Breaks[0].AtElement != "Z" {
		t.Errorf("break AtElement = %q, want %q", chain.Breaks[0].AtElement, "Z")
	}
	if chain.Breaks[0].Reason != "element-not-in-graph" {
		t.Errorf("break Reason = %q, want %q", chain.Breaks[0].Reason, "element-not-in-graph")
	}
}

// --- Group 3: Cycle detection ---

func TestFollowTranslation_CycleDetection(t *testing.T) {
	// A -> B -> C -> A (cycle)
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
		{TraceID: "t2", Sources: []string{"B"}, Targets: []string{"C"}, WhatChanged: "e2"},
		{TraceID: "t3", Sources: []string{"C"}, Targets: []string{"A"}, WhatChanged: "e3"},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	// Should follow A -> B -> C, then detect A is already visited
	if len(chain.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(chain.Steps))
	}

	// The third step enters A again — cycle detected
	if chain.Steps[2].ElementEntered != "A" {
		t.Errorf("step 2 ElementEntered = %q, want %q", chain.Steps[2].ElementEntered, "A")
	}

	hasBreak := false
	for _, b := range chain.Breaks {
		if b.AtElement == "A" && b.Reason == "cycle-detected" {
			hasBreak = true
		}
	}
	if !hasBreak {
		t.Error("expected break with reason 'cycle-detected'")
	}
}

// --- Group 4: Depth limit ---

func TestFollowTranslation_DepthLimit(t *testing.T) {
	// A -> B -> C -> D -> E (chain of 4 edges)
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
		{TraceID: "t2", Sources: []string{"B"}, Targets: []string{"C"}, WhatChanged: "e2"},
		{TraceID: "t3", Sources: []string{"C"}, Targets: []string{"D"}, WhatChanged: "e3"},
		{TraceID: "t4", Sources: []string{"D"}, Targets: []string{"E"}, WhatChanged: "e4"},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{MaxDepth: 2})

	if len(chain.Steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(chain.Steps))
	}

	// Should stop at C (2 steps taken: A->B, B->C)
	if chain.Steps[1].ElementEntered != "C" {
		t.Errorf("step 1 ElementEntered = %q, want %q", chain.Steps[1].ElementEntered, "C")
	}

	hasBreak := false
	for _, b := range chain.Breaks {
		if b.AtElement == "C" && b.Reason == "depth-limit" {
			hasBreak = true
		}
	}
	if !hasBreak {
		t.Error("expected break at C with reason 'depth-limit'")
	}
}

// --- Group 5: Branch-not-taken ---

func TestFollowTranslation_BranchNotTaken(t *testing.T) {
	// A -> B (edge 1) and A -> C (edge 2)
	// First-match should follow edge 1 (A->B) and record A->C as branch-not-taken
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
		{TraceID: "t2", Sources: []string{"A"}, Targets: []string{"C"}, WhatChanged: "e2"},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	// Should follow A -> B
	if len(chain.Steps) < 1 {
		t.Fatalf("got %d steps, want >= 1", len(chain.Steps))
	}
	if chain.Steps[0].ElementEntered != "B" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "B")
	}

	// Should record branch-not-taken for the alternative
	hasBranchBreak := false
	for _, b := range chain.Breaks {
		if b.Reason == "branch-not-taken" {
			hasBranchBreak = true
		}
	}
	if !hasBranchBreak {
		t.Error("expected break with reason 'branch-not-taken'")
	}
}

// --- Group 6: Multi-source/multi-target edges ---

func TestFollowTranslation_MultiTarget(t *testing.T) {
	// A -> [B, C] (single edge with two targets)
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B", "C"}, WhatChanged: "e1"},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	// Should follow to first target (B), record C as branch-not-taken
	if len(chain.Steps) < 1 {
		t.Fatalf("got %d steps, want >= 1", len(chain.Steps))
	}
	if chain.Steps[0].ElementEntered != "B" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "B")
	}
}

func TestFollowTranslation_MultiSource(t *testing.T) {
	// [A, B] -> C (single edge with two sources)
	// Following from A should traverse this edge (A is one of the sources)
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A", "B"}, Targets: []string{"C"}, WhatChanged: "e1"},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	if len(chain.Steps) < 1 {
		t.Fatalf("got %d steps, want >= 1", len(chain.Steps))
	}
	if chain.Steps[0].ElementExited != "A" {
		t.Errorf("step 0 ElementExited = %q, want %q", chain.Steps[0].ElementExited, "A")
	}
	if chain.Steps[0].ElementEntered != "C" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "C")
	}
}

func TestFollowTranslation_MultiSourceBackward(t *testing.T) {
	// [A, B] -> C — following backward from C should enter one of the sources
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A", "B"}, Targets: []string{"C"}, WhatChanged: "e1"},
	})

	chain := graph.FollowTranslation(g, "C", graph.FollowOptions{
		Direction: graph.DirectionBackward,
	})

	if len(chain.Steps) < 1 {
		t.Fatalf("got %d steps, want >= 1", len(chain.Steps))
	}
	if chain.Steps[0].ElementExited != "C" {
		t.Errorf("step 0 ElementExited = %q, want %q", chain.Steps[0].ElementExited, "C")
	}
	// Should enter the first source (A)
	if chain.Steps[0].ElementEntered != "A" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "A")
	}
}

// --- Group 7: Empty graph ---

func TestFollowTranslation_EmptyGraph(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{},
		Edges: []graph.Edge{},
	}

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	if len(chain.Steps) != 0 {
		t.Errorf("got %d steps, want 0", len(chain.Steps))
	}
	if len(chain.Breaks) != 1 {
		t.Fatalf("got %d breaks, want 1", len(chain.Breaks))
	}
	if chain.Breaks[0].Reason != "element-not-in-graph" {
		t.Errorf("break Reason = %q, want %q", chain.Breaks[0].Reason, "element-not-in-graph")
	}
}

// --- Group 8: Single element, no connections ---

func TestFollowTranslation_SingleElementNoConnections(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"A": {Name: "A", AppearanceCount: 1},
		},
		Edges: []graph.Edge{},
	}

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	if chain.StartElement != "A" {
		t.Errorf("StartElement = %q, want %q", chain.StartElement, "A")
	}
	if len(chain.Steps) != 0 {
		t.Errorf("got %d steps, want 0", len(chain.Steps))
	}
	if len(chain.Breaks) != 1 {
		t.Fatalf("got %d breaks, want 1", len(chain.Breaks))
	}
	if chain.Breaks[0].Reason != "no-outgoing-edges" {
		t.Errorf("break Reason = %q, want %q", chain.Breaks[0].Reason, "no-outgoing-edges")
	}
}

func TestFollowTranslation_SingleElementNoConnectionsBackward(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"A": {Name: "A", AppearanceCount: 1},
		},
		Edges: []graph.Edge{},
	}

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{
		Direction: graph.DirectionBackward,
	})

	if len(chain.Breaks) != 1 {
		t.Fatalf("got %d breaks, want 1", len(chain.Breaks))
	}
	if chain.Breaks[0].Reason != "no-incoming-edges" {
		t.Errorf("break Reason = %q, want %q", chain.Breaks[0].Reason, "no-incoming-edges")
	}
}

// --- Group 9: Zero-value FollowOptions defaults ---

func TestFollowTranslation_DefaultDirection(t *testing.T) {
	// Zero-value Direction should behave as forward
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	if len(chain.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(chain.Steps))
	}
	if chain.Steps[0].ElementEntered != "B" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "B")
	}
}

func TestFollowTranslation_ZeroMaxDepthMeansUnlimited(t *testing.T) {
	// MaxDepth 0 should not limit the chain
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
		{TraceID: "t2", Sources: []string{"B"}, Targets: []string{"C"}, WhatChanged: "e2"},
		{TraceID: "t3", Sources: []string{"C"}, Targets: []string{"D"}, WhatChanged: "e3"},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{MaxDepth: 0})

	if len(chain.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(chain.Steps))
	}
}

// --- Group 10: Cut is carried through ---

func TestFollowTranslation_CutCarriedThrough(t *testing.T) {
	// The chain should carry the MeshGraph's Cut
	g := buildGraph([]graph.Edge{
		{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, WhatChanged: "e1"},
	})
	g.Cut = graph.Cut{
		ObserverPositions: []string{"obs-1"},
		TracesIncluded:    1,
		TracesTotal:       5,
	}

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	if len(chain.Cut.ObserverPositions) != 1 || chain.Cut.ObserverPositions[0] != "obs-1" {
		t.Errorf("Cut.ObserverPositions = %v, want [obs-1]", chain.Cut.ObserverPositions)
	}
	if chain.Cut.TracesIncluded != 1 {
		t.Errorf("Cut.TracesIncluded = %d, want 1", chain.Cut.TracesIncluded)
	}
}

// --- Group 11: Edge metadata preserved in steps ---

func TestFollowTranslation_EdgeMetadataPreserved(t *testing.T) {
	g := buildGraph([]graph.Edge{
		{
			TraceID:     "t1",
			Sources:     []string{"A"},
			Targets:     []string{"B"},
			WhatChanged: "storm-detected",
			Mediation:   "sensor-array",
			Observer:    "meteorologist",
			Tags:        []string{"threshold"},
		},
	})

	chain := graph.FollowTranslation(g, "A", graph.FollowOptions{})

	if len(chain.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(chain.Steps))
	}
	e := chain.Steps[0].Edge
	if e.WhatChanged != "storm-detected" {
		t.Errorf("Edge.WhatChanged = %q, want %q", e.WhatChanged, "storm-detected")
	}
	if e.Mediation != "sensor-array" {
		t.Errorf("Edge.Mediation = %q, want %q", e.Mediation, "sensor-array")
	}
}
