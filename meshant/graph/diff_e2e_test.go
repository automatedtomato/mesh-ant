// diff_e2e_test.go exercises the full Load → Articulate → Diff → PrintDiff
// pipeline against the real deforestation_longitudinal dataset. Tests here
// verify integration behaviour, not individual function logic — failures point
// to pipeline wiring rather than unit-level correctness.
//
// Group 16: E2E diff tests
//
// Dataset: deforestation_longitudinal.json — 40 traces, 3 days:
//   - 2026-03-11: 20 traces (satellite-operator has 1)
//   - 2026-03-14: 10 traces (satellite-operator has 1)
//   - 2026-03-18: 10 traces (satellite-operator has 1)
//
// Satellite-operator day-1 trace elements:
//   src: landsat-9-overpass-3147
//   tgt: raw-spectral-anomaly-report-20260311
//
// Satellite-operator day-3 trace elements:
//   src: landsat-9-satellite
//   tgt: confirmation-scan-20260318
package graph_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// articulateSatDay1 returns a graph articulated for satellite-operator on day 1
// (2026-03-11) from the longitudinal dataset.
func articulateSatDay1(t *testing.T) graph.MeshGraph {
	t.Helper()
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
	return graph.Articulate(traces, opts)
}

// articulateSatDay3 returns a graph articulated for satellite-operator on day 3
// (2026-03-18) from the longitudinal dataset.
func articulateSatDay3(t *testing.T) graph.MeshGraph {
	t.Helper()
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"satellite-operator"},
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-03-18T00:00:00Z"),
			End:   mustParseTime(t, "2026-03-18T23:59:59Z"),
		},
	}
	return graph.Articulate(traces, opts)
}

// --- Group 16: E2E diff tests ---

// TestE2E_Diff_SatelliteOperator_Day1VsDay3_NodesAdded verifies that diffing
// satellite-operator day-1 against day-3 produces at least one node in
// NodesAdded. The day-3 trace introduces new elements not present on day 1.
func TestE2E_Diff_SatelliteOperator_Day1VsDay3_NodesAdded(t *testing.T) {
	g1 := articulateSatDay1(t)
	g3 := articulateSatDay3(t)
	d := graph.Diff(g1, g3)

	if len(d.NodesAdded) == 0 {
		t.Error("NodesAdded: want at least 1 (new elements in day-3 cut), got 0")
	}
	// landsat-9-satellite appears in day-3 sat-op trace source.
	found := false
	for _, name := range d.NodesAdded {
		if name == "landsat-9-satellite" {
			found = true
		}
	}
	if !found {
		t.Errorf("NodesAdded: want landsat-9-satellite (day-3 source element), got %v", d.NodesAdded)
	}
}

// TestE2E_Diff_SatelliteOperator_Day1VsDay3_NodesRemoved verifies that the
// diff reports at least one node removed: elements visible on day 1 but no
// longer in the day-3 cut.
func TestE2E_Diff_SatelliteOperator_Day1VsDay3_NodesRemoved(t *testing.T) {
	g1 := articulateSatDay1(t)
	g3 := articulateSatDay3(t)
	d := graph.Diff(g1, g3)

	if len(d.NodesRemoved) == 0 {
		t.Error("NodesRemoved: want at least 1 (day-1 elements absent from day-3 cut), got 0")
	}
	// landsat-9-overpass-3147 is the day-1 sat-op source element.
	found := false
	for _, name := range d.NodesRemoved {
		if name == "landsat-9-overpass-3147" {
			found = true
		}
	}
	if !found {
		t.Errorf("NodesRemoved: want landsat-9-overpass-3147 (day-1 source element), got %v", d.NodesRemoved)
	}
}

// TestE2E_Diff_SatelliteOperator_Day1VsDay3_EdgesAdded verifies that the
// day-3 satellite-operator trace appears in EdgesAdded (present in g3, absent
// from g1).
func TestE2E_Diff_SatelliteOperator_Day1VsDay3_EdgesAdded(t *testing.T) {
	g1 := articulateSatDay1(t)
	g3 := articulateSatDay3(t)
	d := graph.Diff(g1, g3)

	// g3 includes the one day-3 sat-op trace; g1 does not.
	if len(d.EdgesAdded) == 0 {
		t.Fatal("EdgesAdded: want at least 1 (day-3 sat-op trace), got 0")
	}
	// The day-3 edge should mention confirmation-scan-20260318.
	foundScan := false
	for _, e := range d.EdgesAdded {
		if strings.Contains(e.WhatChanged, "confirmation") ||
			containsStr(e.Targets, "confirmation-scan-20260318") {
			foundScan = true
		}
	}
	if !foundScan {
		t.Errorf("EdgesAdded: want edge referencing confirmation-scan-20260318, edges: %v",
			edgeIDs(d.EdgesAdded))
	}
}

// TestE2E_Diff_SatelliteOperator_Day1VsDay3_EdgesRemoved verifies that the
// day-1 satellite-operator trace appears in EdgesRemoved (present in g1, absent
// from g3).
func TestE2E_Diff_SatelliteOperator_Day1VsDay3_EdgesRemoved(t *testing.T) {
	g1 := articulateSatDay1(t)
	g3 := articulateSatDay3(t)
	d := graph.Diff(g1, g3)

	if len(d.EdgesRemoved) == 0 {
		t.Fatal("EdgesRemoved: want at least 1 (day-1 sat-op trace), got 0")
	}
	foundAnomaly := false
	for _, e := range d.EdgesRemoved {
		if containsStr(e.Targets, "raw-spectral-anomaly-report-20260311") ||
			containsStr(e.Sources, "landsat-9-overpass-3147") {
			foundAnomaly = true
		}
	}
	if !foundAnomaly {
		t.Errorf("EdgesRemoved: want day-1 sat-op trace edge, edges: %v", edgeIDs(d.EdgesRemoved))
	}
}

// TestE2E_Diff_SatelliteOperator_Day1VsDay3_ShadowShifts verifies that at
// least one shadow shift is detected. landsat-9-satellite appears in the day-1
// shadow (excluded by time window from day-2/day-3 traces) and becomes a node
// in the day-3 cut, so it should emerge.
func TestE2E_Diff_SatelliteOperator_Day1VsDay3_ShadowShifts(t *testing.T) {
	g1 := articulateSatDay1(t)
	g3 := articulateSatDay3(t)
	d := graph.Diff(g1, g3)

	if len(d.ShadowShifts) == 0 {
		t.Fatal("ShadowShifts: want at least 1 (elements moving between shadow and visibility), got 0")
	}
	// landsat-9-satellite should emerge: shadow in g1 (time-excluded day-2/3
	// traces), visible node in g3.
	_, ok := containsShift(d.ShadowShifts, "landsat-9-satellite")
	if !ok {
		t.Errorf("ShadowShifts: want entry for landsat-9-satellite (emerged), got %v",
			shiftNames(d.ShadowShifts))
	}
}

// TestE2E_Diff_SatelliteOperator_Day1VsDay3_CutsStored verifies that the From
// cut carries the day-1 time window and To carries the day-3 time window.
func TestE2E_Diff_SatelliteOperator_Day1VsDay3_CutsStored(t *testing.T) {
	g1 := articulateSatDay1(t)
	g3 := articulateSatDay3(t)
	d := graph.Diff(g1, g3)

	if d.From.TimeWindow.IsZero() {
		t.Error("From.TimeWindow: want non-zero (day-1 window), got zero")
	}
	if d.To.TimeWindow.IsZero() {
		t.Error("To.TimeWindow: want non-zero (day-3 window), got zero")
	}
	fromDay := d.From.TimeWindow.Start.Format("2006-01-02")
	if fromDay != "2026-03-11" {
		t.Errorf("From.TimeWindow.Start: want 2026-03-11, got %s", fromDay)
	}
	toDay := d.To.TimeWindow.Start.Format("2006-01-02")
	if toDay != "2026-03-18" {
		t.Errorf("To.TimeWindow.Start: want 2026-03-18, got %s", toDay)
	}
}

// TestE2E_Diff_SameDay_TwoObservers_DisjointCuts verifies that diffing two
// different observers on the same day produces non-empty NodesAdded and
// NodesRemoved. The satellite-operator and ngo-field-coordinator see entirely
// different elements on day 1, so all nodes should be added or removed.
func TestE2E_Diff_SameDay_TwoObservers_DisjointCuts(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	day1 := graph.TimeWindow{
		Start: mustParseTime(t, "2026-03-11T00:00:00Z"),
		End:   mustParseTime(t, "2026-03-11T23:59:59Z"),
	}
	gSat := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"satellite-operator"},
		TimeWindow:        day1,
	})
	gNgo := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"ngo-field-coordinator"},
		TimeWindow:        day1,
	})
	d := graph.Diff(gSat, gNgo)

	if len(d.NodesAdded) == 0 {
		t.Error("NodesAdded: want > 0 (ngo elements not visible to satellite-operator), got 0")
	}
	if len(d.NodesRemoved) == 0 {
		t.Error("NodesRemoved: want > 0 (sat-op elements not visible to ngo), got 0")
	}
}

// TestE2E_Diff_SameDay_TwoObservers_ShadowShifts verifies that diffing two
// observers on the same day produces at least one shadow shift. Elements
// visible to satellite-operator should submerge (into ngo's shadow), and
// elements visible to ngo should emerge (from sat-op's shadow).
func TestE2E_Diff_SameDay_TwoObservers_ShadowShifts(t *testing.T) {
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	day1 := graph.TimeWindow{
		Start: mustParseTime(t, "2026-03-11T00:00:00Z"),
		End:   mustParseTime(t, "2026-03-11T23:59:59Z"),
	}
	gSat := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"satellite-operator"},
		TimeWindow:        day1,
	})
	gNgo := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"ngo-field-coordinator"},
		TimeWindow:        day1,
	})
	d := graph.Diff(gSat, gNgo)

	if len(d.ShadowShifts) == 0 {
		t.Error("ShadowShifts: want > 0 (elements crossing shadow boundary between observers), got 0")
	}
	// At least one shift should be an emergence (ngo element was in sat-op shadow).
	foundEmerged := false
	for _, s := range d.ShadowShifts {
		if s.Kind == graph.ShadowShiftEmerged {
			foundEmerged = true
			break
		}
	}
	if !foundEmerged {
		t.Errorf("ShadowShifts: want at least one emerged shift (ngo elements from sat-op shadow), got %v",
			shiftNames(d.ShadowShifts))
	}
}

// TestE2E_PrintDiff_Day1VsDay3_RoundTrip verifies that PrintDiff runs without
// error on a real day-1 vs day-3 diff, and that the output contains both cuts'
// key metadata (observer, time window dates).
func TestE2E_PrintDiff_Day1VsDay3_RoundTrip(t *testing.T) {
	g1 := articulateSatDay1(t)
	g3 := articulateSatDay3(t)
	d := graph.Diff(g1, g3)

	var buf bytes.Buffer
	if err := graph.PrintDiff(&buf, d); err != nil {
		t.Fatalf("PrintDiff: unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "satellite-operator") {
		t.Errorf("output: missing observer 'satellite-operator'")
	}
	if !strings.Contains(out, "2026-03-11") {
		t.Errorf("output: missing From time window date 2026-03-11")
	}
	if !strings.Contains(out, "2026-03-18") {
		t.Errorf("output: missing To time window date 2026-03-18")
	}
	if !strings.Contains(out, "From cut") {
		t.Errorf("output: missing 'From cut' section header")
	}
	if !strings.Contains(out, "To cut") {
		t.Errorf("output: missing 'To cut' section header")
	}
	if !strings.Contains(out, "Shadow shifts") {
		t.Errorf("output: missing 'Shadow shifts' section header")
	}
}

// --- local helpers for diff e2e tests ---

// containsStr reports whether s appears in the slice.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// edgeIDs returns a slice of abbreviated TraceIDs for error messages.
func edgeIDs(edges []graph.Edge) []string {
	ids := make([]string, len(edges))
	for i, e := range edges {
		id := e.TraceID
		if len(id) > 8 {
			id = id[:8]
		}
		ids[i] = id
	}
	return ids
}

// shiftNames returns the Name field of each ShadowShift for error messages.
func shiftNames(shifts []graph.ShadowShift) []string {
	names := make([]string, len(shifts))
	for i, s := range shifts {
		names[i] = s.Name
	}
	return names
}
