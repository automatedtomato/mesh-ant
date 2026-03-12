// Package graph_test — serial_test.go tests JSON serialisation for all graph types.
//
// Groups:
//  18. TimeWindow codec — zero bounds marshal as null; null unmarshals to zero
//  19. MeshGraph round-trip — identified, unidentified, zero/non-zero TimeWindow, half-open windows
//  20. GraphDiff round-trip — with ShadowShifts; without; From/To cuts populated
//  21. JSON snapshot — minimal MeshGraph; exact JSON string pinned
//  22. Unmarshal error paths — invalid JSON; wrong type for time field
package graph_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// --- Group 18: TimeWindow codec ---

// TestTimeWindow_MarshalJSON_ZeroBothBoundsAreNull verifies that a zero
// TimeWindow (both Start and End unset) marshals with both fields as JSON null.
func TestTimeWindow_MarshalJSON_ZeroBothBoundsAreNull(t *testing.T) {
	tw := graph.TimeWindow{}
	b, err := json.Marshal(tw)
	if err != nil {
		t.Fatalf("marshal zero TimeWindow: %v", err)
	}
	var got struct {
		Start *string `json:"start"`
		End   *string `json:"end"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got.Start != nil {
		t.Errorf("Start: want null, got %q", *got.Start)
	}
	if got.End != nil {
		t.Errorf("End: want null, got %q", *got.End)
	}
}

// TestTimeWindow_MarshalJSON_NonZeroBoundsAreRFC3339 verifies that non-zero
// Start and End are marshaled as RFC3339 strings.
func TestTimeWindow_MarshalJSON_NonZeroBoundsAreRFC3339(t *testing.T) {
	start := mustParseTime(t, "2026-04-14T00:00:00Z")
	end := mustParseTime(t, "2026-04-16T23:59:59Z")
	tw := graph.TimeWindow{Start: start, End: end}

	b, err := json.Marshal(tw)
	if err != nil {
		t.Fatalf("marshal TimeWindow: %v", err)
	}

	var got struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got.Start != "2026-04-14T00:00:00Z" {
		t.Errorf("Start: want %q, got %q", "2026-04-14T00:00:00Z", got.Start)
	}
	if got.End != "2026-04-16T23:59:59Z" {
		t.Errorf("End: want %q, got %q", "2026-04-16T23:59:59Z", got.End)
	}
}

// TestTimeWindow_MarshalJSON_HalfOpenStartOnly verifies that a TimeWindow with
// only Start set marshals Start as RFC3339 and End as null.
func TestTimeWindow_MarshalJSON_HalfOpenStartOnly(t *testing.T) {
	start := mustParseTime(t, "2026-04-14T00:00:00Z")
	tw := graph.TimeWindow{Start: start}

	b, err := json.Marshal(tw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Use raw message to check null for End.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(raw["start"]) == "null" {
		t.Error("Start: want RFC3339 string, got null")
	}
	if string(raw["end"]) != "null" {
		t.Errorf("End: want null, got %s", string(raw["end"]))
	}
}

// TestTimeWindow_MarshalJSON_HalfOpenEndOnly verifies that a TimeWindow with
// only End set marshals End as RFC3339 and Start as null.
func TestTimeWindow_MarshalJSON_HalfOpenEndOnly(t *testing.T) {
	end := mustParseTime(t, "2026-04-16T23:59:59Z")
	tw := graph.TimeWindow{End: end}

	b, err := json.Marshal(tw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(raw["start"]) != "null" {
		t.Errorf("Start: want null, got %s", string(raw["start"]))
	}
	if string(raw["end"]) == "null" {
		t.Error("End: want RFC3339 string, got null")
	}
}

// TestTimeWindow_UnmarshalJSON_NullBoundsReturnZero verifies that JSON null
// for start/end fields unmarshal back to zero time.Time (IsZero() == true).
func TestTimeWindow_UnmarshalJSON_NullBoundsReturnZero(t *testing.T) {
	input := `{"start":null,"end":null}`
	var tw graph.TimeWindow
	if err := json.Unmarshal([]byte(input), &tw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !tw.Start.IsZero() {
		t.Errorf("Start: want zero time, got %v", tw.Start)
	}
	if !tw.End.IsZero() {
		t.Errorf("End: want zero time, got %v", tw.End)
	}
}

// TestTimeWindow_UnmarshalJSON_RFC3339StringsRoundTrip verifies that RFC3339
// strings unmarshal back to the expected time.Time values.
func TestTimeWindow_UnmarshalJSON_RFC3339StringsRoundTrip(t *testing.T) {
	input := `{"start":"2026-04-14T00:00:00Z","end":"2026-04-16T23:59:59Z"}`
	var tw graph.TimeWindow
	if err := json.Unmarshal([]byte(input), &tw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantStart := mustParseTime(t, "2026-04-14T00:00:00Z")
	wantEnd := mustParseTime(t, "2026-04-16T23:59:59Z")
	if !tw.Start.Equal(wantStart) {
		t.Errorf("Start: want %v, got %v", wantStart, tw.Start)
	}
	if !tw.End.Equal(wantEnd) {
		t.Errorf("End: want %v, got %v", wantEnd, tw.End)
	}
}

// TestTimeWindow_RoundTrip_Zero verifies the full marshal→unmarshal cycle for a
// zero TimeWindow preserves the zero value.
func TestTimeWindow_RoundTrip_Zero(t *testing.T) {
	original := graph.TimeWindow{}
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.TimeWindow
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !restored.IsZero() {
		t.Errorf("want zero TimeWindow after round-trip, got Start=%v End=%v", restored.Start, restored.End)
	}
}

// TestTimeWindow_RoundTrip_NonZero verifies the full marshal→unmarshal cycle
// for a non-zero TimeWindow preserves both bounds.
func TestTimeWindow_RoundTrip_NonZero(t *testing.T) {
	original := graph.TimeWindow{
		Start: mustParseTime(t, "2026-04-14T00:00:00Z"),
		End:   mustParseTime(t, "2026-04-16T23:59:59Z"),
	}
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.TimeWindow
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !restored.Start.Equal(original.Start) {
		t.Errorf("Start: want %v, got %v", original.Start, restored.Start)
	}
	if !restored.End.Equal(original.End) {
		t.Errorf("End: want %v, got %v", original.End, restored.End)
	}
}

// --- Group 19: MeshGraph round-trip ---

// minimalGraph returns a minimal MeshGraph suitable for serialisation tests.
// The graph has one node, one edge, and a zero TimeWindow.
func minimalGraph(t *testing.T) graph.MeshGraph {
	t.Helper()
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"element-alpha": {Name: "element-alpha", AppearanceCount: 2, ShadowCount: 0},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01",
				WhatChanged: "something changed",
				Mediation:   "",
				Observer:    "analyst",
				Sources:     []string{"element-alpha"},
				Targets:     []string{"element-beta"},
				Tags:        []string{"threshold"},
			},
		},
		Cut: graph.Cut{
			ObserverPositions:         []string{"analyst"},
			TimeWindow:                graph.TimeWindow{},
			TracesIncluded:            1,
			TracesTotal:               3,
			DistinctObserversTotal:    2,
			ShadowElements:            []graph.ShadowElement{},
			ExcludedObserverPositions: []string{"other-observer"},
		},
	}
}

// TestMeshGraph_RoundTrip_Unidentified verifies a MeshGraph with empty ID
// round-trips through JSON with ID remaining empty.
func TestMeshGraph_RoundTrip_Unidentified(t *testing.T) {
	original := minimalGraph(t)
	// Verify it starts unidentified.
	if original.ID != "" {
		t.Fatalf("precondition: want empty ID, got %q", original.ID)
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.MeshGraph
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.ID != "" {
		t.Errorf("ID: want empty, got %q", restored.ID)
	}
	if len(restored.Nodes) != 1 {
		t.Errorf("Nodes: want 1, got %d", len(restored.Nodes))
	}
	if len(restored.Edges) != 1 {
		t.Errorf("Edges: want 1, got %d", len(restored.Edges))
	}
	if restored.Cut.TracesIncluded != 1 {
		t.Errorf("Cut.TracesIncluded: want 1, got %d", restored.Cut.TracesIncluded)
	}
	if restored.Cut.TracesTotal != 3 {
		t.Errorf("Cut.TracesTotal: want 3, got %d", restored.Cut.TracesTotal)
	}
}

// TestMeshGraph_RoundTrip_Identified verifies a MeshGraph with a non-empty ID
// round-trips with the ID preserved.
func TestMeshGraph_RoundTrip_Identified(t *testing.T) {
	g := minimalGraph(t)
	identified := graph.IdentifyGraph(g)

	b, err := json.Marshal(identified)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.MeshGraph
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.ID != identified.ID {
		t.Errorf("ID: want %q, got %q", identified.ID, restored.ID)
	}
}

// TestMeshGraph_RoundTrip_ZeroTimeWindow verifies that a zero TimeWindow in Cut
// round-trips back to a zero TimeWindow (IsZero() == true).
func TestMeshGraph_RoundTrip_ZeroTimeWindow(t *testing.T) {
	g := minimalGraph(t)
	// minimalGraph already uses zero TimeWindow — confirm.
	if !g.Cut.TimeWindow.IsZero() {
		t.Fatalf("precondition: want zero TimeWindow")
	}

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.MeshGraph
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.Cut.TimeWindow.IsZero() {
		t.Errorf("TimeWindow: want zero after round-trip, got Start=%v End=%v",
			restored.Cut.TimeWindow.Start, restored.Cut.TimeWindow.End)
	}
}

// TestMeshGraph_RoundTrip_NonZeroTimeWindow verifies that a non-zero TimeWindow
// round-trips with both bounds preserved.
func TestMeshGraph_RoundTrip_NonZeroTimeWindow(t *testing.T) {
	g := minimalGraph(t)
	g.Cut.TimeWindow = graph.TimeWindow{
		Start: mustParseTime(t, "2026-04-14T00:00:00Z"),
		End:   mustParseTime(t, "2026-04-16T23:59:59Z"),
	}

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.MeshGraph
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := g.Cut.TimeWindow
	got := restored.Cut.TimeWindow
	if !got.Start.Equal(want.Start) {
		t.Errorf("TimeWindow.Start: want %v, got %v", want.Start, got.Start)
	}
	if !got.End.Equal(want.End) {
		t.Errorf("TimeWindow.End: want %v, got %v", want.End, got.End)
	}
}

// TestMeshGraph_RoundTrip_HalfOpenWindowStartOnly verifies a TimeWindow with
// only Start set round-trips correctly (End remains zero).
func TestMeshGraph_RoundTrip_HalfOpenWindowStartOnly(t *testing.T) {
	g := minimalGraph(t)
	g.Cut.TimeWindow = graph.TimeWindow{
		Start: mustParseTime(t, "2026-04-14T00:00:00Z"),
	}

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.MeshGraph
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.Cut.TimeWindow.Start.Equal(g.Cut.TimeWindow.Start) {
		t.Errorf("Start: want %v, got %v", g.Cut.TimeWindow.Start, restored.Cut.TimeWindow.Start)
	}
	if !restored.Cut.TimeWindow.End.IsZero() {
		t.Errorf("End: want zero, got %v", restored.Cut.TimeWindow.End)
	}
}

// TestMeshGraph_RoundTrip_HalfOpenWindowEndOnly verifies a TimeWindow with
// only End set round-trips correctly (Start remains zero).
func TestMeshGraph_RoundTrip_HalfOpenWindowEndOnly(t *testing.T) {
	g := minimalGraph(t)
	g.Cut.TimeWindow = graph.TimeWindow{
		End: mustParseTime(t, "2026-04-16T23:59:59Z"),
	}

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.MeshGraph
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.Cut.TimeWindow.Start.IsZero() {
		t.Errorf("Start: want zero, got %v", restored.Cut.TimeWindow.Start)
	}
	if !restored.Cut.TimeWindow.End.Equal(g.Cut.TimeWindow.End) {
		t.Errorf("End: want %v, got %v", g.Cut.TimeWindow.End, restored.Cut.TimeWindow.End)
	}
}

// TestMeshGraph_RoundTrip_ShadowElements verifies that ShadowElements (including
// SeenFrom and Reasons) survive the round-trip intact.
func TestMeshGraph_RoundTrip_ShadowElements(t *testing.T) {
	g := minimalGraph(t)
	g.Cut.ShadowElements = []graph.ShadowElement{
		{
			Name:     "shadow-element",
			SeenFrom: []string{"other-observer"},
			Reasons:  []graph.ShadowReason{graph.ShadowReasonObserver},
		},
	}

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.MeshGraph
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(restored.Cut.ShadowElements) != 1 {
		t.Fatalf("ShadowElements: want 1, got %d", len(restored.Cut.ShadowElements))
	}
	se := restored.Cut.ShadowElements[0]
	if se.Name != "shadow-element" {
		t.Errorf("Name: want %q, got %q", "shadow-element", se.Name)
	}
	if len(se.Reasons) != 1 || se.Reasons[0] != graph.ShadowReasonObserver {
		t.Errorf("Reasons: want [observer], got %v", se.Reasons)
	}
}

// TestMeshGraph_RoundTrip_EdgeFields verifies that all Edge fields (including
// Sources, Targets, Tags, Mediation) survive the round-trip.
func TestMeshGraph_RoundTrip_EdgeFields(t *testing.T) {
	g := minimalGraph(t)
	g.Edges[0].Mediation = "some-mediator"
	g.Edges[0].Sources = []string{"src-a", "src-b"}
	g.Edges[0].Targets = []string{"tgt-a"}
	g.Edges[0].Tags = []string{"mediation", "translation"}

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.MeshGraph
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(restored.Edges) != 1 {
		t.Fatalf("Edges: want 1, got %d", len(restored.Edges))
	}
	e := restored.Edges[0]
	if e.Mediation != "some-mediator" {
		t.Errorf("Mediation: want %q, got %q", "some-mediator", e.Mediation)
	}
	if len(e.Sources) != 2 || e.Sources[0] != "src-a" || e.Sources[1] != "src-b" {
		t.Errorf("Sources: want [src-a src-b], got %v", e.Sources)
	}
	if len(e.Targets) != 1 || e.Targets[0] != "tgt-a" {
		t.Errorf("Targets: want [tgt-a], got %v", e.Targets)
	}
	if len(e.Tags) != 2 || e.Tags[0] != "mediation" || e.Tags[1] != "translation" {
		t.Errorf("Tags: want [mediation translation], got %v", e.Tags)
	}
}

// --- Group 20: GraphDiff round-trip ---

// minimalDiff builds a minimal GraphDiff for serialisation tests.
// Both From and To cuts are populated.
func minimalDiff(t *testing.T) graph.GraphDiff {
	t.Helper()
	g1 := minimalGraph(t)
	g2 := minimalGraph(t)
	// Alter g2 to produce a non-trivial diff.
	g2.Nodes["element-gamma"] = graph.Node{Name: "element-gamma", AppearanceCount: 1}
	return graph.Diff(g1, g2)
}

// TestGraphDiff_RoundTrip_WithoutShadowShifts verifies a GraphDiff with no
// ShadowShifts round-trips correctly.
func TestGraphDiff_RoundTrip_WithoutShadowShifts(t *testing.T) {
	d := minimalDiff(t)
	// Confirm no shadow shifts in this minimal case.
	if len(d.ShadowShifts) != 0 {
		t.Fatalf("precondition: want 0 shadow shifts, got %d", len(d.ShadowShifts))
	}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.GraphDiff
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(restored.ShadowShifts) != 0 {
		t.Errorf("ShadowShifts: want 0, got %d", len(restored.ShadowShifts))
	}
	if restored.From.TracesIncluded != d.From.TracesIncluded {
		t.Errorf("From.TracesIncluded: want %d, got %d", d.From.TracesIncluded, restored.From.TracesIncluded)
	}
	if restored.To.TracesTotal != d.To.TracesTotal {
		t.Errorf("To.TracesTotal: want %d, got %d", d.To.TracesTotal, restored.To.TracesTotal)
	}
}

// TestGraphDiff_RoundTrip_WithShadowShifts verifies a GraphDiff with
// ShadowShifts preserves them across the round-trip.
func TestGraphDiff_RoundTrip_WithShadowShifts(t *testing.T) {
	// Build a diff with shadow shifts by using graphs that have shadow elements.
	g1 := minimalGraph(t)
	g1.Cut.ShadowElements = []graph.ShadowElement{
		{Name: "shadow-elem", SeenFrom: []string{"obs-b"}, Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
	}
	// g2 promotes shadow-elem to a visible node (emerged shift).
	g2 := minimalGraph(t)
	g2.Nodes["shadow-elem"] = graph.Node{Name: "shadow-elem", AppearanceCount: 1}

	d := graph.Diff(g1, g2)
	if len(d.ShadowShifts) == 0 {
		t.Fatal("precondition: want at least one shadow shift")
	}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.GraphDiff
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(restored.ShadowShifts) != len(d.ShadowShifts) {
		t.Fatalf("ShadowShifts: want %d, got %d", len(d.ShadowShifts), len(restored.ShadowShifts))
	}
	got := restored.ShadowShifts[0]
	want := d.ShadowShifts[0]
	if got.Name != want.Name {
		t.Errorf("ShadowShift.Name: want %q, got %q", want.Name, got.Name)
	}
	if got.Kind != want.Kind {
		t.Errorf("ShadowShift.Kind: want %q, got %q", want.Kind, got.Kind)
	}
}

// TestGraphDiff_RoundTrip_Identified verifies a GraphDiff with a non-empty ID
// round-trips with the ID preserved.
func TestGraphDiff_RoundTrip_Identified(t *testing.T) {
	d := minimalDiff(t)
	identified := graph.IdentifyDiff(d)

	b, err := json.Marshal(identified)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.GraphDiff
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.ID != identified.ID {
		t.Errorf("ID: want %q, got %q", identified.ID, restored.ID)
	}
}

// TestGraphDiff_RoundTrip_CutsPopulated verifies that From/To Cut metadata
// (observer positions, time windows, trace counts) survives round-trip.
func TestGraphDiff_RoundTrip_CutsPopulated(t *testing.T) {
	g1 := minimalGraph(t)
	g1.Cut.TimeWindow = graph.TimeWindow{Start: mustParseTime(t, "2026-04-14T00:00:00Z")}
	g2 := minimalGraph(t)
	g2.Cut.TimeWindow = graph.TimeWindow{End: mustParseTime(t, "2026-04-16T23:59:59Z")}

	d := graph.Diff(g1, g2)

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.GraphDiff
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// From cut: only Start set.
	if !restored.From.TimeWindow.Start.Equal(g1.Cut.TimeWindow.Start) {
		t.Errorf("From.TimeWindow.Start: want %v, got %v",
			g1.Cut.TimeWindow.Start, restored.From.TimeWindow.Start)
	}
	if !restored.From.TimeWindow.End.IsZero() {
		t.Errorf("From.TimeWindow.End: want zero, got %v", restored.From.TimeWindow.End)
	}

	// To cut: only End set.
	if !restored.To.TimeWindow.End.Equal(g2.Cut.TimeWindow.End) {
		t.Errorf("To.TimeWindow.End: want %v, got %v",
			g2.Cut.TimeWindow.End, restored.To.TimeWindow.End)
	}
	if !restored.To.TimeWindow.Start.IsZero() {
		t.Errorf("To.TimeWindow.Start: want zero, got %v", restored.To.TimeWindow.Start)
	}
}

// TestGraphDiff_RoundTrip_EdgesAddedRemoved verifies EdgesAdded/EdgesRemoved
// fields survive the round-trip.
func TestGraphDiff_RoundTrip_EdgesAddedRemoved(t *testing.T) {
	g1 := minimalGraph(t)
	g2 := minimalGraph(t)
	// Add a new edge in g2.
	g2.Edges = append(g2.Edges, graph.Edge{
		TraceID:     "bbbbbbbb-bbbb-4ccc-dddd-eeeeeeeeee02",
		WhatChanged: "new change",
		Observer:    "analyst",
		Sources:     []string{"element-alpha"},
		Targets:     []string{"element-gamma"},
		Tags:        []string{"emergence"},
	})

	d := graph.Diff(g1, g2)
	if len(d.EdgesAdded) != 1 {
		t.Fatalf("precondition: want 1 edge added, got %d", len(d.EdgesAdded))
	}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored graph.GraphDiff
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(restored.EdgesAdded) != 1 {
		t.Fatalf("EdgesAdded: want 1, got %d", len(restored.EdgesAdded))
	}
	if restored.EdgesAdded[0].TraceID != "bbbbbbbb-bbbb-4ccc-dddd-eeeeeeeeee02" {
		t.Errorf("EdgesAdded[0].TraceID: want %q, got %q",
			"bbbbbbbb-bbbb-4ccc-dddd-eeeeeeeeee02", restored.EdgesAdded[0].TraceID)
	}
}

// --- Group 21: JSON snapshot ---

// TestMeshGraph_JSONSnapshot_MinimalGraph pins the exact JSON output for a
// minimal MeshGraph. This test will fail if field names, ordering, or null
// handling changes, making regressions immediately visible.
func TestMeshGraph_JSONSnapshot_MinimalGraph(t *testing.T) {
	// Build a fully deterministic graph with no randomness.
	g := graph.MeshGraph{
		ID: "",
		Nodes: map[string]graph.Node{
			"alpha": {Name: "alpha", AppearanceCount: 1, ShadowCount: 0},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-bbbb-4ccc-dddd-eeeeeeeeee01",
				WhatChanged: "threshold crossed",
				Mediation:   "",
				Observer:    "analyst",
				Sources:     []string{"alpha"},
				Targets:     []string{"beta"},
				Tags:        []string{"threshold"},
			},
		},
		Cut: graph.Cut{
			ObserverPositions:         []string{"analyst"},
			TimeWindow:                graph.TimeWindow{},
			TracesIncluded:            1,
			TracesTotal:               1,
			DistinctObserversTotal:    1,
			ShadowElements:            []graph.ShadowElement{},
			ExcludedObserverPositions: []string{},
		},
	}

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := string(b)

	// Check required structural characteristics of the JSON output.
	// We verify key presence and null encoding rather than full string equality
	// to avoid brittleness from map iteration order in Nodes.

	// ID must appear and be empty string.
	if !strings.Contains(got, `"id":""`) {
		t.Errorf("snapshot: expected empty id field, JSON: %s", got)
	}
	// TimeWindow zero bounds must be null.
	if !strings.Contains(got, `"start":null`) {
		t.Errorf("snapshot: expected null start in time_window, JSON: %s", got)
	}
	if !strings.Contains(got, `"end":null`) {
		t.Errorf("snapshot: expected null end in time_window, JSON: %s", got)
	}
	// Node field names must use snake_case.
	if !strings.Contains(got, `"appearance_count"`) {
		t.Errorf("snapshot: expected snake_case appearance_count, JSON: %s", got)
	}
	if !strings.Contains(got, `"shadow_count"`) {
		t.Errorf("snapshot: expected snake_case shadow_count, JSON: %s", got)
	}
	// Edge field names must use snake_case.
	if !strings.Contains(got, `"trace_id"`) {
		t.Errorf("snapshot: expected snake_case trace_id, JSON: %s", got)
	}
	if !strings.Contains(got, `"what_changed"`) {
		t.Errorf("snapshot: expected snake_case what_changed, JSON: %s", got)
	}
	// Cut field names must use snake_case.
	if !strings.Contains(got, `"observer_positions"`) {
		t.Errorf("snapshot: expected snake_case observer_positions, JSON: %s", got)
	}
	if !strings.Contains(got, `"time_window"`) {
		t.Errorf("snapshot: expected snake_case time_window, JSON: %s", got)
	}
	if !strings.Contains(got, `"traces_included"`) {
		t.Errorf("snapshot: expected snake_case traces_included, JSON: %s", got)
	}
	if !strings.Contains(got, `"traces_total"`) {
		t.Errorf("snapshot: expected snake_case traces_total, JSON: %s", got)
	}
}

// TestGraphDiff_JSONSnapshot_FieldNames verifies that GraphDiff marshals with
// the expected snake_case field names.
func TestGraphDiff_JSONSnapshot_FieldNames(t *testing.T) {
	d := minimalDiff(t)

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	requiredFields := []string{
		`"id"`,
		`"nodes_added"`,
		`"nodes_removed"`,
		`"nodes_persisted"`,
		`"edges_added"`,
		`"edges_removed"`,
		`"shadow_shifts"`,
		`"from"`,
		`"to"`,
	}
	for _, field := range requiredFields {
		if !strings.Contains(got, field) {
			t.Errorf("snapshot: expected field %s in GraphDiff JSON: %s", field, got)
		}
	}
}

// TestShadowShift_JSONSnapshot_FieldNames verifies ShadowShift marshals with
// the expected snake_case field names.
func TestShadowShift_JSONSnapshot_FieldNames(t *testing.T) {
	shift := graph.ShadowShift{
		Name:        "elem",
		Kind:        graph.ShadowShiftEmerged,
		FromReasons: []graph.ShadowReason{graph.ShadowReasonObserver},
	}
	b, err := json.Marshal(shift)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, `"from_reasons"`) {
		t.Errorf("snapshot: expected from_reasons, JSON: %s", got)
	}
	if !strings.Contains(got, `"to_reasons"`) {
		t.Errorf("snapshot: expected to_reasons, JSON: %s", got)
	}
}

// TestPersistedNode_JSONSnapshot_FieldNames verifies PersistedNode marshals
// with the expected snake_case field names.
func TestPersistedNode_JSONSnapshot_FieldNames(t *testing.T) {
	pn := graph.PersistedNode{Name: "elem", CountFrom: 2, CountTo: 3}
	b, err := json.Marshal(pn)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, `"count_from"`) {
		t.Errorf("snapshot: expected count_from, JSON: %s", got)
	}
	if !strings.Contains(got, `"count_to"`) {
		t.Errorf("snapshot: expected count_to, JSON: %s", got)
	}
}

// --- Group 22: Unmarshal error paths ---

// TestTimeWindow_UnmarshalJSON_InvalidJSON verifies that invalid JSON returns
// an error rather than silently yielding a zero value.
// Note: {invalid fails at the JSON tokenizer level before UnmarshalJSON is
// called, so this tests the outer json.Unmarshal error path.
func TestTimeWindow_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var tw graph.TimeWindow
	err := json.Unmarshal([]byte(`{invalid`), &tw)
	if err == nil {
		t.Error("want error for invalid JSON, got nil")
	}
}

// TestTimeWindow_UnmarshalJSON_InvalidInnerJSON verifies that JSON that reaches
// UnmarshalJSON but fails inner struct parsing returns an error.
// Using a non-object JSON value (e.g., an array) causes the inner
// json.Unmarshal to fail after UnmarshalJSON is entered.
func TestTimeWindow_UnmarshalJSON_InvalidInnerJSON(t *testing.T) {
	var tw graph.TimeWindow
	// An array value is valid outer JSON but fails the inner struct unmarshal.
	err := json.Unmarshal([]byte(`[1,2,3]`), &tw)
	if err == nil {
		t.Error("want error for array instead of object, got nil")
	}
}

// TestTimeWindow_UnmarshalJSON_WrongTypeForStartField verifies that a non-string,
// non-null value for the start field (e.g., a number) returns an error.
func TestTimeWindow_UnmarshalJSON_WrongTypeForStartField(t *testing.T) {
	var tw graph.TimeWindow
	err := json.Unmarshal([]byte(`{"start":42,"end":null}`), &tw)
	if err == nil {
		t.Error("want error for numeric start field, got nil")
	}
}

// TestTimeWindow_UnmarshalJSON_WrongTypeForEndField verifies that a non-string,
// non-null value for the end field returns an error, when start is valid.
func TestTimeWindow_UnmarshalJSON_WrongTypeForEndField(t *testing.T) {
	var tw graph.TimeWindow
	err := json.Unmarshal([]byte(`{"start":null,"end":true}`), &tw)
	if err == nil {
		t.Error("want error for boolean end field, got nil")
	}
}

// TestTimeWindow_UnmarshalJSON_InvalidTimeString verifies that a non-RFC3339
// string for a time field returns an error.
func TestTimeWindow_UnmarshalJSON_InvalidTimeString(t *testing.T) {
	var tw graph.TimeWindow
	err := json.Unmarshal([]byte(`{"start":"not-a-date","end":null}`), &tw)
	if err == nil {
		t.Error("want error for invalid time string, got nil")
	}
}

// TestTimeWindow_UnmarshalJSON_InvalidEndTimeString verifies that a non-RFC3339
// string for the end field returns an error when start is valid.
func TestTimeWindow_UnmarshalJSON_InvalidEndTimeString(t *testing.T) {
	var tw graph.TimeWindow
	err := json.Unmarshal([]byte(`{"start":null,"end":"not-a-date"}`), &tw)
	if err == nil {
		t.Error("want error for invalid end time string, got nil")
	}
}

// TestTimeWindow_UnmarshalJSON_MalformedStringValue verifies that a start
// field containing a Unicode escape with missing hex digits ("\u") returns
// an error. What actually happens: the outer json.Unmarshal decodes the
// Unicode escape sequence into a replacement character (U+FFFD); the
// resulting string is then passed to time.Parse as an RFC3339 value;
// time.Parse rejects it because the replacement character is not a valid
// timestamp character. The error comes from time.Parse, not from the JSON
// tokenizer or decodeTimeField’s string-unmarshal path.
func TestTimeWindow_UnmarshalJSON_MalformedStringValue(t *testing.T) {
	// Construct JSON where start appears to be a string token (begins with '"')
	// but is actually truncated — the json.Unmarshal of the string will fail.
	// We must pass valid outer JSON so the struct parser runs, then have the
	// inner raw value be malformed. The trick: embed a raw message that starts
	// with '"' but contains invalid escape sequences.
	// "\u" with no digits following is a valid way to get json.Unmarshal to fail.
	input := "{\"start\":\"\\u\",\"end\":null}"
	var tw graph.TimeWindow
	err := json.Unmarshal([]byte(input), &tw)
	if err == nil {
		t.Error("want error for malformed string value in start field, got nil")
	}
}

// TestMeshGraph_UnmarshalJSON_InvalidJSON verifies that invalid JSON for a
// MeshGraph returns an error.
func TestMeshGraph_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var g graph.MeshGraph
	err := json.Unmarshal([]byte(`{invalid`), &g)
	if err == nil {
		t.Error("want error for invalid JSON, got nil")
	}
}

// TestGraphDiff_UnmarshalJSON_InvalidJSON verifies that invalid JSON for a
// GraphDiff returns an error.
func TestGraphDiff_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var d graph.GraphDiff
	err := json.Unmarshal([]byte(`{invalid`), &d)
	if err == nil {
		t.Error("want error for invalid JSON, got nil")
	}
}
