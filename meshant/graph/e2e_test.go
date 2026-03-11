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

// longitudinalPath is the relative path from the graph package directory to
// the longitudinal deforestation example dataset (40 traces across 3 days).
const longitudinalPath = "../../data/examples/deforestation_longitudinal.json"


// TestE2E_LongitudinalDataset_FullCut loads all 40 longitudinal traces and
// verifies that a full cut includes all of them and reports the correct total
// number of distinct observers.
func TestE2E_LongitudinalDataset_FullCut(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	g := graph.Articulate(traces, graph.ArticulationOptions{})

	if g.Cut.TracesIncluded != 40 {
		t.Errorf("TracesIncluded: want 40, got %d", g.Cut.TracesIncluded)
	}
	if g.Cut.TracesTotal != 40 {
		t.Errorf("TracesTotal: want 40, got %d", g.Cut.TracesTotal)
	}
	if len(g.Cut.ShadowElements) != 0 {
		t.Errorf("ShadowElements: want empty for full cut, got %d elements", len(g.Cut.ShadowElements))
	}
	if g.Cut.DistinctObserversTotal < 8 {
		t.Errorf("DistinctObserversTotal: want >= 8, got %d", g.Cut.DistinctObserversTotal)
	}
}

// TestE2E_LongitudinalDataset_Day1Window verifies that a time window covering
// only day 1 (2026-03-11) yields exactly 20 included traces.
func TestE2E_LongitudinalDataset_Day1Window(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-11T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-11T23:59:59Z"),
		},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 20 {
		t.Errorf("TracesIncluded: want 20 (day 1 only), got %d", g.Cut.TracesIncluded)
	}
}

// TestE2E_LongitudinalDataset_Day2Window verifies that a time window covering
// only day 2 (2026-03-14) yields exactly 10 included traces.
func TestE2E_LongitudinalDataset_Day2Window(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-14T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-14T23:59:59Z"),
		},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 10 {
		t.Errorf("TracesIncluded: want 10 (day 2 only), got %d", g.Cut.TracesIncluded)
	}
}

// TestE2E_LongitudinalDataset_Day3Window verifies that a time window covering
// only day 3 (2026-03-18) yields exactly 10 included traces.
func TestE2E_LongitudinalDataset_Day3Window(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-18T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-18T23:59:59Z"),
		},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 10 {
		t.Errorf("TracesIncluded: want 10 (day 3 only), got %d", g.Cut.TracesIncluded)
	}
}

// TestE2E_LongitudinalDataset_Days1And2Window verifies that a time window
// spanning days 1 and 2 yields exactly 30 included traces.
func TestE2E_LongitudinalDataset_Days1And2Window(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-11T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-14T23:59:59Z"),
		},
	}
	g := graph.Articulate(traces, opts)

	if g.Cut.TracesIncluded != 30 {
		t.Errorf("TracesIncluded: want 30 (days 1+2), got %d", g.Cut.TracesIncluded)
	}
}

// TestE2E_LongitudinalDataset_ShadowContainsDay3Elements verifies that a
// window covering only days 1 and 2 produces a non-empty shadow, because day
// 3 elements are invisible from this temporal position.
func TestE2E_LongitudinalDataset_ShadowContainsDay3Elements(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-11T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-14T23:59:59Z"),
		},
	}
	g := graph.Articulate(traces, opts)

	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty (day 3 elements invisible from days 1+2 window)")
	}
}

// TestE2E_LongitudinalDataset_ObserverAndTimeWindow_Combined verifies that
// filtering by both observer (satellite-operator) and a day-1 time window
// yields only satellite-operator day-1 traces and a non-empty shadow.
func TestE2E_LongitudinalDataset_ObserverAndTimeWindow_Combined(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"satellite-operator"},
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-11T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-11T23:59:59Z"),
		},
	}
	g := graph.Articulate(traces, opts)

	// satellite-operator on day 1: only trace e601 (timestamp 2026-03-11T02:14:00Z)
	if g.Cut.TracesIncluded != 1 {
		t.Errorf("TracesIncluded: want 1 (satellite-operator day-1 only), got %d", g.Cut.TracesIncluded)
	}
	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty — most elements invisible from this narrow cut")
	}
}

// TestE2E_LongitudinalDataset_ShadowReason_TimeWindow_Day3Element verifies
// that when only a days 1+2 time window is set (no observer filter), at least
// one shadow element has a Reason containing ShadowReasonTimeWindow.
func TestE2E_LongitudinalDataset_ShadowReason_TimeWindow_Day3Element(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-11T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-14T23:59:59Z"),
		},
	}
	g := graph.Articulate(traces, opts)

	found := false
	for _, se := range g.Cut.ShadowElements {
		for _, r := range se.Reasons {
			if r == graph.ShadowReasonTimeWindow {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Error("ShadowElements: want at least one element with ShadowReasonTimeWindow (day 3 elements excluded by window)")
	}
}

// TestE2E_LongitudinalDataset_PrintArticulation_TimeWindowLine verifies that
// a time-filtered articulation of the longitudinal dataset produces output
// containing the "Time window:" line with actual timestamps.
func TestE2E_LongitudinalDataset_PrintArticulation_TimeWindowLine(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-11T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-14T23:59:59Z"),
		},
	}
	g := graph.Articulate(traces, opts)

	var buf bytes.Buffer
	if err := graph.PrintArticulation(&buf, g); err != nil {
		t.Fatalf("PrintArticulation: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Time window:") {
		t.Errorf("output missing %q line\nGot (first 500 chars):\n%.500s", "Time window:", out)
	}
	if !strings.Contains(out, "2026-03-11T00:00:00Z") {
		t.Errorf("output missing start timestamp 2026-03-11T00:00:00Z\nGot (first 500 chars):\n%.500s", out)
	}
}
