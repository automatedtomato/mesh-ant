// e2e_test.go exercises the full Load → Articulate → PrintArticulation pipeline
// against the real deforestation example dataset. It is kept separate from unit
// tests so that failures here point clearly to pipeline integration rather than
// individual function behaviour.
//
// The deforestation dataset contains 20 traces across 8 distinct observer
// positions: satellite-operator, deforestation-detection-algorithm,
// national-forest-agency, policy-enforcement-officer, ngo-field-coordinator,
// international-treaty-body, carbon-registry-auditor, carbon-credit-broker.
package graph_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// deforestationPath is the relative path from the graph package directory to
// the deforestation example dataset.
const deforestationPath = "../../data/examples/deforestation.json"

// TestE2E_FullCut loads all 20 deforestation traces and articulates with no
// observer filter, verifying that all traces are included and the shadow is empty.
func TestE2E_FullCut(t *testing.T) {
	traces, err := loader.Load(deforestationPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	g := graph.Articulate(traces, graph.ArticulationOptions{})

	if g.Cut.TracesIncluded != 20 {
		t.Errorf("TracesIncluded: want 20, got %d", g.Cut.TracesIncluded)
	}
	if g.Cut.TracesTotal != 20 {
		t.Errorf("TracesTotal: want 20, got %d", g.Cut.TracesTotal)
	}
	if len(g.Cut.ShadowElements) != 0 {
		t.Errorf("ShadowElements: want empty for full cut, got %d elements", len(g.Cut.ShadowElements))
	}
	if len(g.Edges) != 20 {
		t.Errorf("Edges: want 20 (one per trace), got %d", len(g.Edges))
	}
	if len(g.Nodes) == 0 {
		t.Error("Nodes: want non-empty for a dataset with source/target elements")
	}
	// 8 distinct observer positions across the deforestation dataset.
	if g.Cut.DistinctObserversTotal != 8 {
		t.Errorf("DistinctObserversTotal: want 8, got %d", g.Cut.DistinctObserversTotal)
	}

	// Print the full cut and verify output structure
	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, g); err != nil {
		t.Fatalf("PrintArticulation: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "=== Mesh Articulation") {
		t.Errorf("output missing header %q", "=== Mesh Articulation")
	}
	if !strings.Contains(out, "Shadow") {
		t.Errorf("output missing Shadow section")
	}
}

// TestE2E_CarbonRegistryAuditorCut articulates the deforestation dataset from
// the carbon-registry-auditor observer position and verifies the cut includes
// the correct traces and produces a non-empty shadow.
func TestE2E_CarbonRegistryAuditorCut(t *testing.T) {
	traces, err := loader.Load(deforestationPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"carbon-registry-auditor"},
	}
	g := graph.Articulate(traces, opts)

	// carbon-registry-auditor observes traces: d901, d902, d904 = 3 traces
	// (Note: d905 has observer carbon-credit-broker, not carbon-registry-auditor,
	//  even though carbon-registry-auditor appears as a source element in that trace)
	if g.Cut.TracesIncluded != 3 {
		t.Errorf("TracesIncluded: want 3, got %d", g.Cut.TracesIncluded)
	}
	if g.Cut.TracesTotal != 20 {
		t.Errorf("TracesTotal: want 20, got %d", g.Cut.TracesTotal)
	}
	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty — carbon-registry-auditor cannot see all elements")
	}

	// Verify only elements from included traces appear in Nodes
	// The included traces reference: raw-spectral-anomaly-report-20260311,
	// carbon-credit-invalidation-notice-vcs-2847, broker-notification-verde-carbon-ltd,
	// market-correction-report-vcs-2847-20260311.
	// Elements from other observers' traces should not be in Nodes.
	shadowNames := make(map[string]bool)
	for _, se := range g.Cut.ShadowElements {
		shadowNames[se.Name] = true
	}
	// cerrado-timber-operations-4412 only appears in a policy-enforcement-officer
	// trace (e606), so it should be a shadow element
	if _, ok := g.Nodes["cerrado-timber-operations-4412"]; ok {
		t.Error("Nodes: cerrado-timber-operations-4412 should not be in Nodes for carbon-registry-auditor cut")
	}
}

// TestE2E_NGOCut articulates the deforestation dataset from the
// ngo-field-coordinator observer position, verifying 5 traces are included
// and the shadow is non-empty.
func TestE2E_NGOCut(t *testing.T) {
	traces, err := loader.Load(deforestationPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"ngo-field-coordinator"},
	}
	g := graph.Articulate(traces, opts)

	// ngo-field-coordinator observes traces: b801, b802, b803, b804, b805 = 5 traces
	if g.Cut.TracesIncluded != 5 {
		t.Errorf("TracesIncluded: want 5, got %d", g.Cut.TracesIncluded)
	}
	if g.Cut.TracesTotal != 20 {
		t.Errorf("TracesTotal: want 20, got %d", g.Cut.TracesTotal)
	}
	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty — ngo-field-coordinator cannot see satellite or carbon threads")
	}

	// Elements from the satellite thread should be in the shadow
	// raw-spectral-anomaly-report-20260311 appears in satellite-operator and
	// deforestation-detection-algorithm traces — both excluded from this cut
	shadowNames := make(map[string]bool)
	for _, se := range g.Cut.ShadowElements {
		shadowNames[se.Name] = true
	}
	if _, ok := g.Nodes["raw-spectral-anomaly-report-20260311"]; ok {
		// raw-spectral-anomaly-report-20260311 should be in shadow, not nodes,
		// unless it also appears in an NGO trace
		// Checking whether it's correctly absent from nodes (it only appears in A-thread traces)
		t.Error("Nodes: raw-spectral-anomaly-report-20260311 should not be in Nodes for ngo-field-coordinator cut")
	}
}

// TestE2E_PolicyOfficerCut articulates from the policy-enforcement-officer
// observer position, verifying 3 traces are included (e605 = A05, e606 = A06,
// e101 = X01) and the shadow covers community and carbon market threads.
func TestE2E_PolicyOfficerCut(t *testing.T) {
	traces, err := loader.Load(deforestationPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"policy-enforcement-officer"},
	}
	g := graph.Articulate(traces, opts)

	// policy-enforcement-officer observes: e605, e606, e101 = 3 traces
	if g.Cut.TracesIncluded != 3 {
		t.Errorf("TracesIncluded: want 3, got %d", g.Cut.TracesIncluded)
	}
	if g.Cut.TracesTotal != 20 {
		t.Errorf("TracesTotal: want 20, got %d", g.Cut.TracesTotal)
	}

	// The shadow should cover both the community thread (NGO traces with
	// community-deforestation-report-20260311) and the carbon market thread
	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty — policy officer cannot see community or carbon threads")
	}

	// community-deforestation-report-20260311 only in NGO traces — must be shadow
	if _, ok := g.Nodes["community-deforestation-report-20260311"]; ok {
		t.Error("Nodes: community-deforestation-report-20260311 should not be in policy-enforcement-officer Nodes")
	}
	// carbon-credit-invalidation-notice-vcs-2847 only in carbon registry traces — must be shadow
	if _, ok := g.Nodes["carbon-credit-invalidation-notice-vcs-2847"]; ok {
		t.Error("Nodes: carbon-credit-invalidation-notice-vcs-2847 should not be in policy-enforcement-officer Nodes")
	}
}

// TestE2E_PrintArticulation_FullCut verifies that a full-cut articulation of
// the deforestation dataset produces correctly structured PrintArticulation output.
func TestE2E_PrintArticulation_FullCut(t *testing.T) {
	traces, err := loader.Load(deforestationPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	g := graph.Articulate(traces, graph.ArticulationOptions{})

	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, g); err != nil {
		t.Fatalf("PrintArticulation: %v", err)
	}
	out := buf.String()

	requiredPhrases := []string{
		"=== Mesh Articulation",
		"Shadow",
	}
	for _, phrase := range requiredPhrases {
		if !strings.Contains(out, phrase) {
			t.Errorf("output missing %q\nGot (first 500 chars):\n%.500s", phrase, out)
		}
	}
}
