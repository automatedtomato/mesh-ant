// Package graph_test — tests for reflexive tracing functions (M7.2).
//
// Reflexive tracing is the act of recording the act of observation: when a
// graph is articulated or two graphs are diffed, the framework can produce a
// Trace that places that act inside the mesh it observes. This embodies
// Principle 8 (the designer is inside the mesh) and generalised symmetry
// (observation apparatus is an actant like any other).
//
// Tests cover:
//   - ArticulationTrace: happy path (10 assertions), error cases (2)
//   - DiffTrace: happy path (6 assertions), error cases (4)
//
// All produced traces are validated against schema.Validate() to guarantee
// they are mesh-compatible records from the moment of creation.
package graph_test

import (
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// --- helpers ---

// identifiedGraph returns a MeshGraph with a stable ID, constructed via
// IdentifyGraph so that ArticulationTrace and DiffTrace accept it.
// opts controls the Cut recorded in the graph (observer positions, time window).
func identifiedGraph(opts graph.ArticulationOptions) graph.MeshGraph {
	var traces []schema.Trace
	g := graph.Articulate(traces, opts)
	return graph.IdentifyGraph(g)
}

// identifiedDiff returns an identified GraphDiff produced from two identified
// graphs. All three results (g1, g2, d) have non-empty IDs.
func identifiedDiff(opts1, opts2 graph.ArticulationOptions) (graph.MeshGraph, graph.MeshGraph, graph.GraphDiff) {
	g1 := identifiedGraph(opts1)
	g2 := identifiedGraph(opts2)
	d := graph.IdentifyDiff(graph.Diff(g1, g2))
	return g1, g2, d
}

// --- Group A1: ArticulationTrace happy path ---

// TestArticulationTrace_ProducedTracePassesValidate verifies that the trace
// returned by ArticulationTrace is a well-formed schema.Trace. It must carry a
// valid UUID, non-empty WhatChanged, non-empty Observer, and a non-zero Timestamp.
func TestArticulationTrace_ProducedTracePassesValidate(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("produced trace failed schema.Validate(): %v", err)
	}
}

// TestArticulationTrace_TargetContainsGraphRef verifies that the produced trace
// names the observed graph as its target using the canonical graph-ref string.
// This is what makes the articulation apparatus traceable inside the mesh.
func TestArticulationTrace_TargetContainsGraphRef(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	wantRef := "meshgraph:" + g.ID
	found := false
	for _, tgt := range tr.Target {
		if tgt == wantRef {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Target does not contain %q; got %v", wantRef, tr.Target)
	}
}

// TestArticulationTrace_TagsContainArticulation verifies the reflexive trace is
// tagged with "articulation" so it can be identified and queried as such.
func TestArticulationTrace_TagsContainArticulation(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	found := false
	for _, tag := range tr.Tags {
		if tag == string(schema.TagValueArticulation) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tags do not contain %q; got %v", schema.TagValueArticulation, tr.Tags)
	}
}

// TestArticulationTrace_MediationIsGraphArticulate verifies the mediation field
// names the function responsible for the articulation so the mechanism is
// preserved in the trace record.
func TestArticulationTrace_MediationIsGraphArticulate(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if tr.Mediation != "graph.Articulate" {
		t.Errorf("Mediation = %q; want %q", tr.Mediation, "graph.Articulate")
	}
}

// TestArticulationTrace_ObserverMatchesInput verifies the observer field in the
// produced trace matches the observer argument passed to ArticulationTrace.
func TestArticulationTrace_ObserverMatchesInput(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if tr.Observer != "analyst-1" {
		t.Errorf("Observer = %q; want %q", tr.Observer, "analyst-1")
	}
}

// TestArticulationTrace_NilSourceProducesValidTrace verifies that absent source
// is a valid state. When the input traces have no collective identity, the caller
// passes nil and the produced trace should still satisfy schema.Validate() because
// Target is non-empty (the graph-ref).
func TestArticulationTrace_NilSourceProducesValidTrace(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("trace with nil source failed schema.Validate(): %v", err)
	}
	// Source may be nil or empty — both are absent. Just confirm no panic and valid.
}

// TestArticulationTrace_NonNilSourceAppearsInTrace verifies that when the caller
// provides a non-nil source slice, those strings appear in the produced trace's
// Source field.
func TestArticulationTrace_NonNilSourceAppearsInTrace(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	src := []string{"meshgraph:some-earlier-graph-id"}
	tr, err := graph.ArticulationTrace(g, "analyst-1", src)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if len(tr.Source) == 0 {
		t.Fatal("expected non-empty Source when non-nil source provided, got empty")
	}
	if tr.Source[0] != src[0] {
		t.Errorf("Source[0] = %q; want %q", tr.Source[0], src[0])
	}
}

// TestArticulationTrace_WhatChangedIsNonEmpty verifies that the what_changed field
// is always populated — schema.Validate requires it, and the description should
// convey the cut parameters.
func TestArticulationTrace_WhatChangedIsNonEmpty(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if tr.WhatChanged == "" {
		t.Error("WhatChanged is empty; must be non-empty for schema.Validate to pass")
	}
}

// TestArticulationTrace_IDIsNonEmptyUUID verifies that the produced trace has a
// non-empty ID that matches UUID format (lowercase hyphenated). The trace must
// be a first-class mesh record, not an anonymous annotation.
func TestArticulationTrace_IDIsNonEmptyUUID(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if tr.ID == "" {
		t.Fatal("ID is empty; want a non-empty UUID")
	}
	// Validate via schema — it checks the UUID pattern.
	if err := tr.Validate(); err != nil {
		t.Errorf("trace failed validation (likely bad ID format): %v", err)
	}
}

// TestArticulationTrace_TimestampIsNonZero verifies that the produced trace carries
// a real timestamp (time.Now().UTC()), not the zero time.Time value.
func TestArticulationTrace_TimestampIsNonZero(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if tr.Timestamp.IsZero() {
		t.Fatal("Timestamp is zero; want current time")
	}
	if tr.Timestamp.Before(before) {
		t.Errorf("Timestamp %v is before test start %v; want a recent timestamp", tr.Timestamp, before)
	}
}

// --- Group A2: ArticulationTrace error cases ---

// TestArticulationTrace_EmptyGraphID_ReturnsError verifies that passing an
// unidentified graph (g.ID == "") produces an error rather than a trace.
// Callers must call IdentifyGraph first.
func TestArticulationTrace_EmptyGraphID_ReturnsError(t *testing.T) {
	g := graph.MeshGraph{} // not identified
	_, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err == nil {
		t.Error("expected error for unidentified graph (empty ID), got nil")
	}
}

// TestArticulationTrace_EmptyObserver_ReturnsError verifies that an empty observer
// string is rejected. Observer is required by schema.Validate, and the function
// should fail fast rather than producing an invalid trace.
func TestArticulationTrace_EmptyObserver_ReturnsError(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	_, err := graph.ArticulationTrace(g, "", nil)
	if err == nil {
		t.Error("expected error for empty observer, got nil")
	}
}

// --- Group A3: DiffTrace happy path ---

// TestDiffTrace_ProducedTracePassesValidate verifies that DiffTrace produces a
// well-formed schema.Trace that passes schema.Validate().
func TestDiffTrace_ProducedTracePassesValidate(t *testing.T) {
	g1, g2, d := identifiedDiff(
		graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}},
		graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}},
	)
	tr, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err != nil {
		t.Fatalf("DiffTrace returned unexpected error: %v", err)
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("produced trace failed schema.Validate(): %v", err)
	}
}

// TestDiffTrace_SourceContainsBothGraphRefs verifies that the produced trace's
// Source field contains the graph-ref strings for both input graphs. The diff
// is derived from g1 and g2, so both are named as sources of the diff trace.
func TestDiffTrace_SourceContainsBothGraphRefs(t *testing.T) {
	g1, g2, d := identifiedDiff(
		graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}},
		graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}},
	)
	tr, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err != nil {
		t.Fatalf("DiffTrace returned unexpected error: %v", err)
	}
	wantRef1 := "meshgraph:" + g1.ID
	wantRef2 := "meshgraph:" + g2.ID

	srcSet := make(map[string]bool, len(tr.Source))
	for _, s := range tr.Source {
		srcSet[s] = true
	}
	if !srcSet[wantRef1] {
		t.Errorf("Source does not contain %q; got %v", wantRef1, tr.Source)
	}
	if !srcSet[wantRef2] {
		t.Errorf("Source does not contain %q; got %v", wantRef2, tr.Source)
	}
}

// TestDiffTrace_TargetContainsDiffRef verifies that the produced trace's Target
// field contains the diff-ref string for the GraphDiff. The diff is named as the
// target — it is what the observation produced.
func TestDiffTrace_TargetContainsDiffRef(t *testing.T) {
	g1, g2, d := identifiedDiff(
		graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}},
		graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}},
	)
	tr, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err != nil {
		t.Fatalf("DiffTrace returned unexpected error: %v", err)
	}
	wantRef := "meshdiff:" + d.ID
	found := false
	for _, tgt := range tr.Target {
		if tgt == wantRef {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Target does not contain %q; got %v", wantRef, tr.Target)
	}
}

// TestDiffTrace_TagsContainArticulation verifies that DiffTrace tags the produced
// trace with "articulation" so it is queryable as a reflexive trace.
func TestDiffTrace_TagsContainArticulation(t *testing.T) {
	g1, g2, d := identifiedDiff(
		graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}},
		graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}},
	)
	tr, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err != nil {
		t.Fatalf("DiffTrace returned unexpected error: %v", err)
	}
	found := false
	for _, tag := range tr.Tags {
		if tag == string(schema.TagValueArticulation) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tags do not contain %q; got %v", schema.TagValueArticulation, tr.Tags)
	}
}

// TestDiffTrace_MediationIsGraphDiff verifies the mediation field names the Diff
// function as the mechanism that produced the GraphDiff.
func TestDiffTrace_MediationIsGraphDiff(t *testing.T) {
	g1, g2, d := identifiedDiff(
		graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}},
		graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}},
	)
	tr, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err != nil {
		t.Fatalf("DiffTrace returned unexpected error: %v", err)
	}
	if tr.Mediation != "graph.Diff" {
		t.Errorf("Mediation = %q; want %q", tr.Mediation, "graph.Diff")
	}
}

// TestDiffTrace_ObserverMatchesInput verifies the observer field in the produced
// trace matches the observer argument passed to DiffTrace.
func TestDiffTrace_ObserverMatchesInput(t *testing.T) {
	g1, g2, d := identifiedDiff(
		graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}},
		graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}},
	)
	tr, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err != nil {
		t.Fatalf("DiffTrace returned unexpected error: %v", err)
	}
	if tr.Observer != "analyst-1" {
		t.Errorf("Observer = %q; want %q", tr.Observer, "analyst-1")
	}
}

// --- Group A4: DiffTrace error cases ---

// TestDiffTrace_EmptyDiffID_ReturnsError verifies that an unidentified diff
// (d.ID == "") produces an error.
func TestDiffTrace_EmptyDiffID_ReturnsError(t *testing.T) {
	g1 := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	g2 := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}})
	d := graph.Diff(g1, g2) // not identified — d.ID is empty
	_, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err == nil {
		t.Error("expected error for unidentified diff (empty d.ID), got nil")
	}
}

// TestDiffTrace_EmptyG1ID_ReturnsError verifies that an unidentified g1 graph
// produces an error — the source graph-refs cannot be constructed without IDs.
func TestDiffTrace_EmptyG1ID_ReturnsError(t *testing.T) {
	g1 := graph.MeshGraph{} // not identified
	g2 := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}})
	d := graph.IdentifyDiff(graph.Diff(g1, g2))
	_, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err == nil {
		t.Error("expected error for unidentified g1 (empty g1.ID), got nil")
	}
}

// TestDiffTrace_EmptyG2ID_ReturnsError verifies that an unidentified g2 graph
// produces an error.
func TestDiffTrace_EmptyG2ID_ReturnsError(t *testing.T) {
	g1 := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}})
	g2 := graph.MeshGraph{} // not identified
	d := graph.IdentifyDiff(graph.Diff(g1, g2))
	_, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err == nil {
		t.Error("expected error for unidentified g2 (empty g2.ID), got nil")
	}
}

// TestDiffTrace_EmptyObserver_ReturnsError verifies that an empty observer string
// is rejected, consistent with the ArticulationTrace behaviour and schema requirements.
func TestDiffTrace_EmptyObserver_ReturnsError(t *testing.T) {
	g1, g2, d := identifiedDiff(
		graph.ArticulationOptions{ObserverPositions: []string{"pos-A"}},
		graph.ArticulationOptions{ObserverPositions: []string{"pos-B"}},
	)
	_, err := graph.DiffTrace(d, g1, g2, "")
	if err == nil {
		t.Error("expected error for empty observer, got nil")
	}
}

// --- Group A5: ArticulationTrace what_changed content ---

// TestArticulationTrace_WhatChanged_WithObserverPositions verifies that when the
// graph's cut has observer positions set, what_changed mentions them.
func TestArticulationTrace_WhatChanged_WithObserverPositions(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{ObserverPositions: []string{"pos-A", "pos-B"}})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if !strings.Contains(tr.WhatChanged, "articulate:") {
		t.Errorf("WhatChanged %q does not contain 'articulate:'; want description of cut", tr.WhatChanged)
	}
}

// TestArticulationTrace_WhatChanged_FullCut verifies that when no observer positions
// or time window are set, what_changed describes a full cut.
func TestArticulationTrace_WhatChanged_FullCut(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{}) // no filters
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if tr.WhatChanged == "" {
		t.Error("WhatChanged is empty; want description of full cut")
	}
}

// TestArticulationTrace_WhatChanged_TimeWindowOnly verifies that when a time
// window is set but no observer positions are provided, what_changed describes
// the window. This exercises the time-window-only branch of articulationWhatChanged.
func TestArticulationTrace_WhatChanged_TimeWindowOnly(t *testing.T) {
	start := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC)
	g := identifiedGraph(graph.ArticulationOptions{
		TimeWindow: graph.TimeWindow{Start: start, End: end},
	})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if !strings.Contains(tr.WhatChanged, "window=") {
		t.Errorf("WhatChanged %q does not contain 'window='; want time window description", tr.WhatChanged)
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("trace with time-window-only cut failed schema.Validate(): %v", err)
	}
}

// TestArticulationTrace_WhatChanged_BothFilters verifies that when the graph's
// cut has both ObserverPositions and a TimeWindow set, the what_changed field
// contains both "observer=" and "window=" components. This exercises the
// combined branch in articulationWhatChanged.
func TestArticulationTrace_WhatChanged_BothFilters(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{
		ObserverPositions: []string{"analyst"},
		TimeWindow: graph.TimeWindow{
			Start: time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC),
		},
	})
	tr, err := graph.ArticulationTrace(g, "meshant", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(tr.WhatChanged, "observer=") {
		t.Errorf("WhatChanged %q missing observer= component", tr.WhatChanged)
	}
	if !strings.Contains(tr.WhatChanged, "window=") {
		t.Errorf("WhatChanged %q missing window= component", tr.WhatChanged)
	}
}

// --- Group A5: WhatChanged includes tag filter ---

// TestArticulationTrace_WhatChanged_TagsOnly verifies that when the graph's
// Cut has Tags set but no other filters, the WhatChanged string includes
// a tags= component.
func TestArticulationTrace_WhatChanged_TagsOnly(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{
		Tags: []string{"critical", "delay"},
	})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if !strings.Contains(tr.WhatChanged, "tags=") {
		t.Errorf("WhatChanged %q does not contain 'tags='; want tag filter description", tr.WhatChanged)
	}
	if !strings.Contains(tr.WhatChanged, "critical") {
		t.Errorf("WhatChanged %q missing tag 'critical'", tr.WhatChanged)
	}
}

// TestArticulationTrace_WhatChanged_AllThreeAxes verifies that when all three
// cut axes are set, all appear in WhatChanged.
func TestArticulationTrace_WhatChanged_AllThreeAxes(t *testing.T) {
	g := identifiedGraph(graph.ArticulationOptions{
		ObserverPositions: []string{"pos-A"},
		TimeWindow: graph.TimeWindow{
			Start: time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC),
		},
		Tags: []string{"threshold"},
	})
	tr, err := graph.ArticulationTrace(g, "analyst-1", nil)
	if err != nil {
		t.Fatalf("ArticulationTrace returned unexpected error: %v", err)
	}
	if !strings.Contains(tr.WhatChanged, "observer=") {
		t.Errorf("WhatChanged %q missing observer= component", tr.WhatChanged)
	}
	if !strings.Contains(tr.WhatChanged, "window=") {
		t.Errorf("WhatChanged %q missing window= component", tr.WhatChanged)
	}
	if !strings.Contains(tr.WhatChanged, "tags=") {
		t.Errorf("WhatChanged %q missing tags= component", tr.WhatChanged)
	}
}

// TestDiffTrace_WhatChanged_TagsIncluded verifies that cutLabel includes tags
// when the Cut has Tags set, so diff WhatChanged distinguishes tag-filtered
// cuts from unfiltered ones.
func TestDiffTrace_WhatChanged_TagsIncluded(t *testing.T) {
	g1 := identifiedGraph(graph.ArticulationOptions{
		ObserverPositions: []string{"pos-A"},
		Tags:              []string{"critical"},
	})
	g2 := identifiedGraph(graph.ArticulationOptions{
		ObserverPositions: []string{"pos-B"},
		Tags:              []string{"delay"},
	})
	d := graph.IdentifyDiff(graph.Diff(g1, g2))
	tr, err := graph.DiffTrace(d, g1, g2, "analyst-1")
	if err != nil {
		t.Fatalf("DiffTrace returned unexpected error: %v", err)
	}
	if !strings.Contains(tr.WhatChanged, "tags=") {
		t.Errorf("WhatChanged %q missing tags= component", tr.WhatChanged)
	}
	if !strings.Contains(tr.WhatChanged, "critical") {
		t.Errorf("WhatChanged %q missing 'critical' tag", tr.WhatChanged)
	}
	if !strings.Contains(tr.WhatChanged, "delay") {
		t.Errorf("WhatChanged %q missing 'delay' tag", tr.WhatChanged)
	}
}
