// classify_test.go — tests for ClassifyChain and step classification types.
// TDD: these tests are written before the implementation.
package graph_test

import (
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// --- Group 1: All-intermediary chain (no mediation on any edge) ---

func TestClassifyChain_AllIntermediary(t *testing.T) {
	chain := graph.TranslationChain{
		StartElement: "A",
		Steps: []graph.ChainStep{
			{Edge: graph.Edge{TraceID: "t1", Mediation: ""}, ElementExited: "A", ElementEntered: "B"},
			{Edge: graph.Edge{TraceID: "t2", Mediation: ""}, ElementExited: "B", ElementEntered: "C"},
		},
	}

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})

	if len(cc.Classifications) != 2 {
		t.Fatalf("got %d classifications, want 2", len(cc.Classifications))
	}
	for i, c := range cc.Classifications {
		if c.Kind != graph.StepIntermediary {
			t.Errorf("step %d Kind = %q, want %q", i, c.Kind, graph.StepIntermediary)
		}
		if c.StepIndex != i {
			t.Errorf("step %d StepIndex = %d, want %d", i, c.StepIndex, i)
		}
	}
}

// --- Group 2: All-mediator chain (mediation present, no translation tag) ---

func TestClassifyChain_AllMediator(t *testing.T) {
	chain := graph.TranslationChain{
		StartElement: "A",
		Steps: []graph.ChainStep{
			{Edge: graph.Edge{TraceID: "t1", Mediation: "sensor-array", Tags: []string{"threshold"}}, ElementExited: "A", ElementEntered: "B"},
			{Edge: graph.Edge{TraceID: "t2", Mediation: "review-board", Tags: []string{"delay"}}, ElementExited: "B", ElementEntered: "C"},
		},
	}

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})

	if len(cc.Classifications) != 2 {
		t.Fatalf("got %d classifications, want 2", len(cc.Classifications))
	}
	for i, c := range cc.Classifications {
		if c.Kind != graph.StepMediator {
			t.Errorf("step %d Kind = %q, want %q", i, c.Kind, graph.StepMediator)
		}
	}
}

// --- Group 3: Mixed chain ---

func TestClassifyChain_Mixed(t *testing.T) {
	chain := graph.TranslationChain{
		StartElement: "A",
		Steps: []graph.ChainStep{
			{Edge: graph.Edge{TraceID: "t1", Mediation: "sensor"}, ElementExited: "A", ElementEntered: "B"},
			{Edge: graph.Edge{TraceID: "t2", Mediation: ""}, ElementExited: "B", ElementEntered: "C"},
			{Edge: graph.Edge{TraceID: "t3", Mediation: "committee"}, ElementExited: "C", ElementEntered: "D"},
		},
	}

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})

	if len(cc.Classifications) != 3 {
		t.Fatalf("got %d classifications, want 3", len(cc.Classifications))
	}
	if cc.Classifications[0].Kind != graph.StepMediator {
		t.Errorf("step 0 Kind = %q, want %q", cc.Classifications[0].Kind, graph.StepMediator)
	}
	if cc.Classifications[1].Kind != graph.StepIntermediary {
		t.Errorf("step 1 Kind = %q, want %q", cc.Classifications[1].Kind, graph.StepIntermediary)
	}
	if cc.Classifications[2].Kind != graph.StepMediator {
		t.Errorf("step 2 Kind = %q, want %q", cc.Classifications[2].Kind, graph.StepMediator)
	}
}

// --- Group 4: Translation step (mediation + translation tag) ---

func TestClassifyChain_Translation(t *testing.T) {
	chain := graph.TranslationChain{
		StartElement: "A",
		Steps: []graph.ChainStep{
			{Edge: graph.Edge{TraceID: "t1", Mediation: "legal-framework", Tags: []string{"translation"}}, ElementExited: "A", ElementEntered: "B"},
		},
	}

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})

	if len(cc.Classifications) != 1 {
		t.Fatalf("got %d classifications, want 1", len(cc.Classifications))
	}
	if cc.Classifications[0].Kind != graph.StepTranslation {
		t.Errorf("Kind = %q, want %q", cc.Classifications[0].Kind, graph.StepTranslation)
	}
}

func TestClassifyChain_TranslationRequiresBothMediationAndTag(t *testing.T) {
	// Translation tag but no mediation → intermediary (no mediator observed)
	chain := graph.TranslationChain{
		StartElement: "A",
		Steps: []graph.ChainStep{
			{Edge: graph.Edge{TraceID: "t1", Mediation: "", Tags: []string{"translation"}}, ElementExited: "A", ElementEntered: "B"},
		},
	}

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})

	if cc.Classifications[0].Kind != graph.StepIntermediary {
		t.Errorf("Kind = %q, want %q (translation tag without mediation → intermediary)", cc.Classifications[0].Kind, graph.StepIntermediary)
	}
}

// --- Group 5: Same trace data, two cuts, different classifications ---
// This is the most important test — validates Question ④: can something
// considered intermediary appear as mediator in a different cut?

func TestClassifyChain_CutDependentClassification(t *testing.T) {
	// Scenario: two observers see different mediations for overlapping elements.
	//
	// Observer "field-team" sees: A --[mediation: sensor]--> B --[no mediation]--> C
	// Observer "lab-team" sees:   B --[mediation: calibration]--> C
	//
	// In the field-team cut: step B→C is intermediary (no mediation observed).
	// In the lab-team cut: step B→C is mediator (calibration transforms).
	//
	// Same elements, same edge connection (B→C), different analytical judgment
	// depending on the cut.

	// Build field-team graph
	fieldGraph := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"A": {Name: "A", AppearanceCount: 1},
			"B": {Name: "B", AppearanceCount: 2},
			"C": {Name: "C", AppearanceCount: 1},
		},
		Edges: []graph.Edge{
			{TraceID: "t1", Sources: []string{"A"}, Targets: []string{"B"}, Mediation: "sensor", Observer: "field-team", WhatChanged: "detected"},
			{TraceID: "t2", Sources: []string{"B"}, Targets: []string{"C"}, Mediation: "", Observer: "field-team", WhatChanged: "forwarded"},
		},
		Cut: graph.Cut{ObserverPositions: []string{"field-team"}},
	}

	// Build lab-team graph — same B→C connection but with mediation
	labGraph := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"B": {Name: "B", AppearanceCount: 1},
			"C": {Name: "C", AppearanceCount: 1},
		},
		Edges: []graph.Edge{
			{TraceID: "t3", Sources: []string{"B"}, Targets: []string{"C"}, Mediation: "calibration", Observer: "lab-team", WhatChanged: "calibrated"},
		},
		Cut: graph.Cut{ObserverPositions: []string{"lab-team"}},
	}

	// Follow B→C in both cuts
	fieldChain := graph.FollowTranslation(fieldGraph, "B", graph.FollowOptions{})
	labChain := graph.FollowTranslation(labGraph, "B", graph.FollowOptions{})

	fieldCC := graph.ClassifyChain(fieldChain, graph.ClassifyOptions{})
	labCC := graph.ClassifyChain(labChain, graph.ClassifyOptions{})

	// Field-team: B→C is intermediary (no mediation observed)
	if len(fieldCC.Classifications) != 1 {
		t.Fatalf("field chain: got %d classifications, want 1", len(fieldCC.Classifications))
	}
	if fieldCC.Classifications[0].Kind != graph.StepIntermediary {
		t.Errorf("field chain B→C: Kind = %q, want %q", fieldCC.Classifications[0].Kind, graph.StepIntermediary)
	}

	// Lab-team: B→C is mediator (calibration transforms)
	if len(labCC.Classifications) != 1 {
		t.Fatalf("lab chain: got %d classifications, want 1", len(labCC.Classifications))
	}
	if labCC.Classifications[0].Kind != graph.StepMediator {
		t.Errorf("lab chain B→C: Kind = %q, want %q", labCC.Classifications[0].Kind, graph.StepMediator)
	}

	// The classifications must differ — this is the whole point
	if fieldCC.Classifications[0].Kind == labCC.Classifications[0].Kind {
		t.Error("field and lab cuts produced identical classification for B→C — cut-dependence not demonstrated")
	}
}

// --- Group 6: Empty chain ---

func TestClassifyChain_EmptyChain(t *testing.T) {
	chain := graph.TranslationChain{
		StartElement: "A",
		Steps:        []graph.ChainStep{},
	}

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})

	if len(cc.Classifications) != 0 {
		t.Errorf("got %d classifications, want 0", len(cc.Classifications))
	}
	if cc.Chain.StartElement != "A" {
		t.Errorf("Chain.StartElement = %q, want %q", cc.Chain.StartElement, "A")
	}
}

// --- Group 7: Reason strings are non-empty ---

func TestClassifyChain_ReasonsNonEmpty(t *testing.T) {
	chain := graph.TranslationChain{
		StartElement: "A",
		Steps: []graph.ChainStep{
			{Edge: graph.Edge{TraceID: "t1", Mediation: ""}, ElementExited: "A", ElementEntered: "B"},
			{Edge: graph.Edge{TraceID: "t2", Mediation: "x"}, ElementExited: "B", ElementEntered: "C"},
			{Edge: graph.Edge{TraceID: "t3", Mediation: "y", Tags: []string{"translation"}}, ElementExited: "C", ElementEntered: "D"},
		},
	}

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})

	for i, c := range cc.Classifications {
		if c.Reason == "" {
			t.Errorf("step %d Reason is empty", i)
		}
	}
}
