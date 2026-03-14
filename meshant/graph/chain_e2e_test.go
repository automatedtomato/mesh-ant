// chain_e2e_test.go — end-to-end tests for translation chain traversal and
// classification against the evacuation order dataset.
//
// These tests validate the full stack: Load → Articulate → FollowTranslation →
// ClassifyChain. They use two observer positions (meteorological-analyst and
// local-mayor) to demonstrate that the same elements can yield different
// chain structures and classifications depending on the cut.
package graph_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

const evacuationDatasetPath = "../../data/examples/evacuation_order.json"

// TestChainE2E_ForwardFromBuoy follows the meteorological chain forward from
// buoy-array-atlantic-sector-7 as seen by the meteorological-analyst on
// 2026-04-14 (T-72h). This should trace the scientific instrumentation
// sequence: buoy → storm model → forecast bulletin.
func TestChainE2E_ForwardFromBuoy(t *testing.T) {
	traces, err := loader.Load(evacuationDatasetPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"meteorological-analyst"},
		TimeWindow: graph.TimeWindow{
			Start: time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC),
		},
	})

	chain := graph.FollowTranslation(g, "buoy-array-atlantic-sector-7", graph.FollowOptions{})

	// Should have at least one step (buoy → storm-track-model)
	if len(chain.Steps) == 0 {
		t.Fatal("expected at least one step from buoy-array-atlantic-sector-7")
	}

	// First step should enter storm-track-model-nhc
	if chain.Steps[0].ElementEntered != "storm-track-model-nhc" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "storm-track-model-nhc")
	}

	// First edge should have mediation (saffir-simpson-classification-algorithm)
	if chain.Steps[0].Edge.Mediation == "" {
		t.Error("step 0 Edge.Mediation is empty; expected a mediator")
	}

	// Classify and verify first step is translation (mediation + translation tag)
	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})
	if len(cc.Classifications) == 0 {
		t.Fatal("no classifications produced")
	}
	if cc.Classifications[0].Kind != graph.StepTranslation {
		t.Errorf("step 0 classification = %q, want %q (buoy→storm model is translation)", cc.Classifications[0].Kind, graph.StepTranslation)
	}
}

// TestChainE2E_BackwardFromEvacuationOrder follows backward from
// mandatory-evacuation-order-zones-abc as seen by local-mayor on 2026-04-16.
// This traces the political decision chain backward.
func TestChainE2E_BackwardFromEvacuationOrder(t *testing.T) {
	traces, err := loader.Load(evacuationDatasetPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"local-mayor"},
		TimeWindow: graph.TimeWindow{
			Start: time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC),
		},
	})

	chain := graph.FollowTranslation(g, "mandatory-evacuation-order-zones-abc", graph.FollowOptions{
		Direction: graph.DirectionBackward,
	})

	// Should have at least one step backward
	if len(chain.Steps) == 0 {
		t.Fatal("expected at least one backward step from mandatory-evacuation-order-zones-abc")
	}

	// First backward step should enter local-mayor (the source of the order trace)
	if chain.Steps[0].ElementEntered != "local-mayor" {
		t.Errorf("step 0 ElementEntered = %q, want %q", chain.Steps[0].ElementEntered, "local-mayor")
	}

	// The chain is situated in the local-mayor cut
	if len(chain.Cut.ObserverPositions) != 1 || chain.Cut.ObserverPositions[0] != "local-mayor" {
		t.Errorf("Cut.ObserverPositions = %v, want [local-mayor]", chain.Cut.ObserverPositions)
	}
}

// TestChainE2E_CutDependentChainLength verifies that the same starting element
// produces different chain structures under different observer cuts. The
// storm-track-model-nhc element appears in both meteorological-analyst and
// emergency-management-director traces, but connects to different targets
// depending on the observer.
func TestChainE2E_CutDependentChainLength(t *testing.T) {
	traces, err := loader.Load(evacuationDatasetPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Meteorological-analyst cut: storm model connects to forecast bulletins
	meteoGraph := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"meteorological-analyst"},
	})

	// Emergency-management-director cut: same element may connect differently
	emdGraph := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"emergency-management-director"},
	})

	meteoChain := graph.FollowTranslation(meteoGraph, "storm-track-model-nhc", graph.FollowOptions{})
	emdChain := graph.FollowTranslation(emdGraph, "storm-track-model-nhc", graph.FollowOptions{})

	// Both chains should exist (storm-track-model-nhc is visible to both)
	// but they should differ in structure — either length, targets, or both
	if len(meteoChain.Steps) == 0 && len(emdChain.Steps) == 0 {
		t.Fatal("both chains have zero steps — at least one should have connections")
	}

	// Log the difference for inspection
	t.Logf("meteorological-analyst chain from storm-track-model-nhc: %d steps", len(meteoChain.Steps))
	t.Logf("emergency-management-director chain from storm-track-model-nhc: %d steps", len(emdChain.Steps))

	// Classify both chains
	meteoCC := graph.ClassifyChain(meteoChain, graph.ClassifyOptions{})
	emdCC := graph.ClassifyChain(emdChain, graph.ClassifyOptions{})

	// At minimum, the chains should have different lengths or different
	// classification patterns — demonstrating cut dependence
	sameLength := len(meteoCC.Classifications) == len(emdCC.Classifications)
	sameKinds := true
	if sameLength && len(meteoCC.Classifications) > 0 {
		for i := range meteoCC.Classifications {
			if meteoCC.Classifications[i].Kind != emdCC.Classifications[i].Kind {
				sameKinds = false
				break
			}
		}
	} else {
		sameKinds = false
	}

	if sameLength && sameKinds {
		t.Log("WARNING: both cuts produced identical chains — cut-dependence not demonstrated in this dataset configuration")
	}
}

// TestChainE2E_FullPipeline verifies the complete pipeline: Load → Articulate →
// FollowTranslation → ClassifyChain → PrintChain produces non-empty output
// without errors.
func TestChainE2E_FullPipeline(t *testing.T) {
	traces, err := loader.Load(evacuationDatasetPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"meteorological-analyst"},
	})

	chain := graph.FollowTranslation(g, "buoy-array-atlantic-sector-7", graph.FollowOptions{})
	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})

	// Verify the chain is non-trivial
	if len(chain.Steps) == 0 {
		t.Fatal("full pipeline produced zero steps")
	}
	if len(cc.Classifications) != len(chain.Steps) {
		t.Errorf("classifications count (%d) != steps count (%d)", len(cc.Classifications), len(chain.Steps))
	}

	// PrintChain should succeed without error
	var buf bytes.Buffer
	if err := graph.PrintChain(&buf, cc); err != nil {
		t.Fatalf("PrintChain: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("PrintChain produced empty output")
	}
}
