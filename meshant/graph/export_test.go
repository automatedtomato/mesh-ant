// Package graph_test — export_test.go tests PrintGraphJSON, PrintDiffJSON,
// PrintGraphDOT, and PrintGraphMermaid.
//
// All tests follow the black-box convention: they use only the exported API of
// the graph package and json.Unmarshal from the standard library to verify
// round-trip fidelity. No internal state is inspected.
package graph_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// errWriter is an io.Writer that always returns the given error.
// Used to verify that PrintGraphJSON and PrintDiffJSON propagate write errors
// back to the caller without swallowing them.
type errWriter struct{ err error }

func (e errWriter) Write(_ []byte) (int, error) { return 0, e.err }

// buildTestGraph constructs a non-trivial MeshGraph suitable for round-trip
// testing. It includes a non-empty ID (identified graph), a non-zero
// TimeWindow, at least one Node, one Edge, and one ShadowElement.
func buildTestGraph(t *testing.T) graph.MeshGraph {
	t.Helper()
	start := mustParseTime(t, "2026-04-14T00:00:00Z")
	end := mustParseTime(t, "2026-04-14T23:59:59Z")

	// Identify the graph so its ID is a non-empty actor handle.
	g := graph.MeshGraph{
		ID: "test-graph-id-0001",
		Nodes: map[string]graph.Node{
			"storm-model-alpha": {Name: "storm-model-alpha", AppearanceCount: 3, ShadowCount: 1},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "aabbccdd-0000-4000-8000-000000000001",
				WhatChanged: "storm model updated forecast",
				Mediation:   "NWS advisory pipeline",
				Observer:    "meteorological-analyst",
				Sources:     []string{"storm-model-alpha"},
				Targets:     []string{"evacuation-order"},
				Tags:        []string{"mediated", "critical"},
			},
		},
		Cut: graph.Cut{
			ObserverPositions:      []string{"meteorological-analyst"},
			TimeWindow:             graph.TimeWindow{Start: start, End: end},
			TracesIncluded:         5,
			TracesTotal:            28,
			DistinctObserversTotal: 6,
			ShadowElements: []graph.ShadowElement{
				{
					Name:     "local-shelter-overflow",
					SeenFrom: []string{"local-mayor"},
					Reasons:  []graph.ShadowReason{graph.ShadowReasonObserver},
				},
			},
			ExcludedObserverPositions: []string{"local-mayor", "resident"},
		},
	}
	return g
}

// buildTestDiff constructs a non-trivial GraphDiff suitable for round-trip
// testing. It includes a non-empty ID, at least one PersistedNode, one
// ShadowShift, non-empty EdgesAdded and EdgesRemoved slices.
func buildTestDiff(t *testing.T) graph.GraphDiff {
	t.Helper()
	start1 := mustParseTime(t, "2026-04-14T00:00:00Z")
	end1 := mustParseTime(t, "2026-04-14T23:59:59Z")
	start2 := mustParseTime(t, "2026-04-15T00:00:00Z")
	end2 := mustParseTime(t, "2026-04-15T23:59:59Z")

	// Build two cuts — From and To.
	fromCut := graph.Cut{
		ObserverPositions:      []string{"meteorological-analyst"},
		TimeWindow:             graph.TimeWindow{Start: start1, End: end1},
		TracesIncluded:         5,
		TracesTotal:            28,
		DistinctObserversTotal: 6,
		ShadowElements: []graph.ShadowElement{
			{
				Name:     "evacuation-shelter-b",
				SeenFrom: []string{"local-mayor"},
				Reasons:  []graph.ShadowReason{graph.ShadowReasonObserver},
			},
		},
		ExcludedObserverPositions: []string{"local-mayor"},
	}
	toCut := graph.Cut{
		ObserverPositions:      []string{"meteorological-analyst"},
		TimeWindow:             graph.TimeWindow{Start: start2, End: end2},
		TracesIncluded:         9,
		TracesTotal:            28,
		DistinctObserversTotal: 6,
		ShadowElements:         []graph.ShadowElement{},
		ExcludedObserverPositions: []string{"local-mayor"},
	}

	edge1 := graph.Edge{
		TraceID:     "aabbccdd-0000-4000-8000-000000000001",
		WhatChanged: "model update",
		Observer:    "meteorological-analyst",
		Sources:     []string{"storm-model-alpha"},
		Targets:     []string{"advisory-board"},
		Tags:        []string{"mediated"},
	}
	edge2 := graph.Edge{
		TraceID:     "aabbccdd-0000-4000-8000-000000000002",
		WhatChanged: "evacuation confirmed",
		Observer:    "meteorological-analyst",
		Sources:     []string{"advisory-board"},
		Targets:     []string{"evacuation-order"},
		Tags:        []string{"critical"},
	}

	d := graph.GraphDiff{
		ID:           "test-diff-id-0001",
		NodesAdded:   []string{"evacuation-order"},
		NodesRemoved: []string{"old-model"},
		NodesPersisted: []graph.PersistedNode{
			{Name: "storm-model-alpha", CountFrom: 2, CountTo: 3},
		},
		EdgesAdded:   []graph.Edge{edge2},
		EdgesRemoved: []graph.Edge{edge1},
		ShadowShifts: []graph.ShadowShift{
			{
				Name:        "evacuation-shelter-b",
				Kind:        graph.ShadowShiftEmerged,
				FromReasons: []graph.ShadowReason{graph.ShadowReasonObserver},
				ToReasons:   nil,
			},
		},
		From: fromCut,
		To:   toCut,
	}
	return d
}

// TestPrintGraphJSON_RoundTrip verifies that marshalling a non-trivial MeshGraph
// via PrintGraphJSON and then unmarshalling via json.Unmarshal produces a value
// that is deep-equal to the original. This exercises all struct fields including
// the non-zero TimeWindow (which uses a custom codec).
func TestPrintGraphJSON_RoundTrip(t *testing.T) {
	original := buildTestGraph(t)

	var buf bytes.Buffer
	if err := graph.PrintGraphJSON(&buf, original); err != nil {
		t.Fatalf("PrintGraphJSON: unexpected error: %v", err)
	}

	var got graph.MeshGraph
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: unexpected error: %v\nJSON:\n%s", err, buf.String())
	}

	if !reflect.DeepEqual(original, got) {
		t.Errorf("round-trip mismatch\noriginal: %+v\ngot:      %+v", original, got)
	}
}

// TestPrintGraphJSON_ZeroTimeWindow verifies that a MeshGraph with a zero
// TimeWindow (both Start and End unset) serialises with "start":null and
// "end":null rather than the RFC3339 zero-time string. This is the M7 null
// convention for unbounded time windows.
func TestPrintGraphJSON_ZeroTimeWindow(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{},
		Edges: []graph.Edge{},
		Cut: graph.Cut{
			// TimeWindow is zero — both bounds should appear as null in JSON.
			TimeWindow: graph.TimeWindow{},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphJSON(&buf, g); err != nil {
		t.Fatalf("PrintGraphJSON: unexpected error: %v", err)
	}

	data := buf.Bytes()
	// Indented JSON uses "key": value format (with a space after the colon).
	if !bytes.Contains(data, []byte(`"start": null`)) {
		t.Errorf("expected %q in JSON output, got:\n%s", `"start": null`, data)
	}
	if !bytes.Contains(data, []byte(`"end": null`)) {
		t.Errorf("expected %q in JSON output, got:\n%s", `"end": null`, data)
	}
}

// TestPrintGraphJSON_EmptyGraph verifies that PrintGraphJSON does not return an
// error for a fully empty MeshGraph (no ID, no nodes, no edges, no shadow).
func TestPrintGraphJSON_EmptyGraph(t *testing.T) {
	g := graph.MeshGraph{}

	var buf bytes.Buffer
	if err := graph.PrintGraphJSON(&buf, g); err != nil {
		t.Fatalf("PrintGraphJSON on empty graph: unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output for empty graph; got empty buffer")
	}
}

// TestPrintDiffJSON_RoundTrip verifies that marshalling a non-trivial GraphDiff
// via PrintDiffJSON and then unmarshalling via json.Unmarshal produces a value
// that is deep-equal to the original. This includes ShadowShifts and
// PersistedNodes, plus non-zero TimeWindows in From and To cuts.
func TestPrintDiffJSON_RoundTrip(t *testing.T) {
	original := buildTestDiff(t)

	var buf bytes.Buffer
	if err := graph.PrintDiffJSON(&buf, original); err != nil {
		t.Fatalf("PrintDiffJSON: unexpected error: %v", err)
	}

	var got graph.GraphDiff
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: unexpected error: %v\nJSON:\n%s", err, buf.String())
	}

	if !reflect.DeepEqual(original, got) {
		t.Errorf("round-trip mismatch\noriginal: %+v\ngot:      %+v", original, got)
	}
}

// TestPrintDiffJSON_EmptyDiff verifies that PrintDiffJSON does not return an
// error for a fully empty GraphDiff (all slice fields nil, no ID, zero cuts).
func TestPrintDiffJSON_EmptyDiff(t *testing.T) {
	d := graph.GraphDiff{}

	var buf bytes.Buffer
	if err := graph.PrintDiffJSON(&buf, d); err != nil {
		t.Fatalf("PrintDiffJSON on empty diff: unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output for empty diff; got empty buffer")
	}
}

// TestPrintGraphJSON_WriteError verifies that PrintGraphJSON propagates a write
// error returned by the underlying io.Writer back to the caller.
func TestPrintGraphJSON_WriteError(t *testing.T) {
	sentinel := errors.New("write failed: disk full")
	w := errWriter{err: sentinel}

	err := graph.PrintGraphJSON(w, graph.MeshGraph{})
	if err == nil {
		t.Fatal("PrintGraphJSON: expected error from failing writer, got nil")
	}
	// The error must either be the sentinel itself or wrap it.
	if !errors.Is(err, sentinel) && !strings.Contains(err.Error(), sentinel.Error()) {
		t.Errorf("PrintGraphJSON: expected error to wrap %q, got %q", sentinel, err)
	}
}

// TestPrintDiffJSON_WriteError verifies that PrintDiffJSON propagates a write
// error returned by the underlying io.Writer back to the caller.
func TestPrintDiffJSON_WriteError(t *testing.T) {
	sentinel := errors.New("write failed: disk full")
	w := errWriter{err: sentinel}

	err := graph.PrintDiffJSON(w, graph.GraphDiff{})
	if err == nil {
		t.Fatal("PrintDiffJSON: expected error from failing writer, got nil")
	}
	if !errors.Is(err, sentinel) && !strings.Contains(err.Error(), sentinel.Error()) {
		t.Errorf("PrintDiffJSON: expected error to wrap %q, got %q", sentinel, err)
	}
}

// TestPrintGraphJSON_OutputIsIndented verifies that PrintGraphJSON produces
// indented (multi-line) JSON rather than a compact single-line object.
// Indented output is the explicit contract from the function documentation.
func TestPrintGraphJSON_OutputIsIndented(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{"a": {Name: "a", AppearanceCount: 1}},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphJSON(&buf, g); err != nil {
		t.Fatalf("PrintGraphJSON: unexpected error: %v", err)
	}

	// Indented output must contain at least one newline character.
	if !bytes.Contains(buf.Bytes(), []byte("\n")) {
		t.Errorf("PrintGraphJSON: expected indented (multi-line) output, got: %q", buf.String())
	}
}

// TestPrintDiffJSON_OutputIsIndented verifies that PrintDiffJSON produces
// indented (multi-line) JSON.
func TestPrintDiffJSON_OutputIsIndented(t *testing.T) {
	d := graph.GraphDiff{
		NodesAdded: []string{"element-x"},
	}

	var buf bytes.Buffer
	if err := graph.PrintDiffJSON(&buf, d); err != nil {
		t.Fatalf("PrintDiffJSON: unexpected error: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("\n")) {
		t.Errorf("PrintDiffJSON: expected indented (multi-line) output, got: %q", buf.String())
	}
}

// TestPrintGraphJSON_NonZeroTimeWindowRoundTrip verifies that a MeshGraph with
// a fully specified (non-zero) TimeWindow round-trips correctly — both bounds
// are preserved as RFC3339 strings.
func TestPrintGraphJSON_NonZeroTimeWindowRoundTrip(t *testing.T) {
	start := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC)

	original := graph.MeshGraph{
		Nodes: map[string]graph.Node{},
		Edges: []graph.Edge{},
		Cut: graph.Cut{
			TimeWindow: graph.TimeWindow{Start: start, End: end},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphJSON(&buf, original); err != nil {
		t.Fatalf("PrintGraphJSON: unexpected error: %v", err)
	}

	var got graph.MeshGraph
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: unexpected error: %v", err)
	}

	if !got.Cut.TimeWindow.Start.Equal(start) {
		t.Errorf("TimeWindow.Start: got %v, want %v", got.Cut.TimeWindow.Start, start)
	}
	if !got.Cut.TimeWindow.End.Equal(end) {
		t.Errorf("TimeWindow.End: got %v, want %v", got.Cut.TimeWindow.End, end)
	}
}

// --- DOT export tests ---

// buildGraphWithShadow constructs a MeshGraph that has visible nodes, edges,
// and at least one shadow element, suitable for testing shadow rendering.
func buildGraphWithShadow(t *testing.T) graph.MeshGraph {
	t.Helper()
	return graph.MeshGraph{
		ID: "graph-with-shadow",
		Nodes: map[string]graph.Node{
			"api-gateway":  {Name: "api-gateway", AppearanceCount: 3},
			"order-service": {Name: "order-service", AppearanceCount: 2},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-0000-4000-8000-000000000001",
				WhatChanged: "request routed to order service",
				Sources:     []string{"api-gateway"},
				Targets:     []string{"order-service"},
			},
		},
		Cut: graph.Cut{
			ObserverPositions: []string{"api-gateway"},
			TracesIncluded:    5,
			TracesTotal:       10,
			ShadowElements: []graph.ShadowElement{
				{Name: "database-primary", SeenFrom: []string{"db-admin"}, Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
			},
		},
	}
}

// buildMultiSourceEdgeGraph constructs a MeshGraph with a single edge that has
// 2 sources and 2 targets, used to verify Cartesian product arc rendering.
func buildMultiSourceEdgeGraph(t *testing.T) graph.MeshGraph {
	t.Helper()
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"src-a": {Name: "src-a", AppearanceCount: 1},
			"src-b": {Name: "src-b", AppearanceCount: 1},
			"tgt-x": {Name: "tgt-x", AppearanceCount: 1},
			"tgt-y": {Name: "tgt-y", AppearanceCount: 1},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "bbbbbbbb-0000-4000-8000-000000000001",
				WhatChanged: "multi source action",
				Sources:     []string{"src-a", "src-b"},
				Targets:     []string{"tgt-x", "tgt-y"},
			},
		},
	}
}

// TestPrintGraphDOT_Basic verifies that PrintGraphDOT produces valid DOT output
// containing the expected structural markers: digraph block, at least one quoted
// node, at least one arc, and a shadow subgraph for a graph that has shadow elements.
func TestPrintGraphDOT_Basic(t *testing.T) {
	g := buildGraphWithShadow(t)

	var buf bytes.Buffer
	if err := graph.PrintGraphDOT(&buf, g); err != nil {
		t.Fatalf("PrintGraphDOT: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "digraph {") {
		t.Errorf("expected 'digraph {' in DOT output, got:\n%s", out)
	}
	if !strings.Contains(out, `"api-gateway"`) {
		t.Errorf("expected quoted node 'api-gateway' in DOT output, got:\n%s", out)
	}
	if !strings.Contains(out, "->") {
		t.Errorf("expected '->' arc in DOT output, got:\n%s", out)
	}
	if !strings.Contains(out, "cluster_shadow") {
		t.Errorf("expected 'cluster_shadow' subgraph in DOT output for graph with shadow, got:\n%s", out)
	}
}

// TestPrintGraphDOT_EmptyGraph verifies that an empty MeshGraph produces a
// valid (though empty) DOT digraph without error.
func TestPrintGraphDOT_EmptyGraph(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintGraphDOT(&buf, graph.MeshGraph{}); err != nil {
		t.Fatalf("PrintGraphDOT on empty graph: unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "digraph {") {
		t.Errorf("expected 'digraph {' even for empty graph, got:\n%s", out)
	}
	if strings.Contains(out, "cluster_shadow") {
		t.Errorf("expected no shadow subgraph for graph with no shadow elements, got:\n%s", out)
	}
}

// TestPrintGraphDOT_ShadowSubgraph verifies that shadow elements appear inside
// the cluster_shadow subgraph with dashed style.
func TestPrintGraphDOT_ShadowSubgraph(t *testing.T) {
	g := buildGraphWithShadow(t)

	var buf bytes.Buffer
	if err := graph.PrintGraphDOT(&buf, g); err != nil {
		t.Fatalf("PrintGraphDOT: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "cluster_shadow") {
		t.Errorf("expected cluster_shadow in output, got:\n%s", out)
	}
	if !strings.Contains(out, "style=dashed") {
		t.Errorf("expected style=dashed for shadow subgraph, got:\n%s", out)
	}
	if !strings.Contains(out, `"database-primary"`) {
		t.Errorf("expected shadow element 'database-primary' in output, got:\n%s", out)
	}
}

// TestPrintGraphDOT_MultiSourceEdge verifies that an edge with 2 sources and 2
// targets produces exactly 4 arcs (2×2 Cartesian product).
func TestPrintGraphDOT_MultiSourceEdge(t *testing.T) {
	g := buildMultiSourceEdgeGraph(t)

	var buf bytes.Buffer
	if err := graph.PrintGraphDOT(&buf, g); err != nil {
		t.Fatalf("PrintGraphDOT: unexpected error: %v", err)
	}

	out := buf.String()
	// Count the number of arcs produced.
	arcCount := strings.Count(out, "->")
	if arcCount != 4 {
		t.Errorf("expected 4 arcs for 2×2 Cartesian product, got %d arcs in:\n%s", arcCount, out)
	}
}

// TestPrintGraphDOT_WriteError verifies that PrintGraphDOT propagates a write
// error from the underlying io.Writer back to the caller.
func TestPrintGraphDOT_WriteError(t *testing.T) {
	sentinel := errors.New("disk full")
	w := errWriter{err: sentinel}

	err := graph.PrintGraphDOT(w, graph.MeshGraph{})
	if err == nil {
		t.Fatal("PrintGraphDOT: expected error from failing writer, got nil")
	}
}

// --- Mermaid export tests ---

// TestPrintGraphMermaid_Basic verifies that PrintGraphMermaid produces valid
// Mermaid flowchart output containing structural markers.
func TestPrintGraphMermaid_Basic(t *testing.T) {
	g := buildGraphWithShadow(t)

	var buf bytes.Buffer
	if err := graph.PrintGraphMermaid(&buf, g); err != nil {
		t.Fatalf("PrintGraphMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "flowchart LR") {
		t.Errorf("expected 'flowchart LR' in Mermaid output, got:\n%s", out)
	}
	if !strings.Contains(out, "-->") {
		t.Errorf("expected '-->' arrow in Mermaid output, got:\n%s", out)
	}
}

// TestPrintGraphMermaid_EmptyGraph verifies that an empty MeshGraph produces a
// valid Mermaid flowchart header without error.
func TestPrintGraphMermaid_EmptyGraph(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintGraphMermaid(&buf, graph.MeshGraph{}); err != nil {
		t.Fatalf("PrintGraphMermaid on empty graph: unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "flowchart LR") {
		t.Errorf("expected 'flowchart LR' even for empty graph, got:\n%s", out)
	}
	if strings.Contains(out, "subgraph Shadow") {
		t.Errorf("expected no shadow subgraph for graph with no shadow elements, got:\n%s", out)
	}
}

// TestPrintGraphMermaid_ShadowSubgraph verifies that shadow elements appear
// inside a 'subgraph Shadow' block.
func TestPrintGraphMermaid_ShadowSubgraph(t *testing.T) {
	g := buildGraphWithShadow(t)

	var buf bytes.Buffer
	if err := graph.PrintGraphMermaid(&buf, g); err != nil {
		t.Fatalf("PrintGraphMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "subgraph Shadow") {
		t.Errorf("expected 'subgraph Shadow' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "end") {
		t.Errorf("expected 'end' closing the subgraph, got:\n%s", out)
	}
	// The shadow element name should appear as a label somewhere in the subgraph.
	if !strings.Contains(out, "database-primary") {
		t.Errorf("expected shadow element label 'database-primary' in output, got:\n%s", out)
	}
}

// TestPrintGraphMermaid_NodeIDSanitization verifies that node names containing
// hyphens are sanitized to underscores in the Mermaid node ID, while the
// original name is preserved as the display label.
func TestPrintGraphMermaid_NodeIDSanitization(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"storm-sensor-network": {Name: "storm-sensor-network", AppearanceCount: 2},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "cccccccc-0000-4000-8000-000000000001",
				WhatChanged: "sensor reading",
				Sources:     []string{"storm-sensor-network"},
				Targets:     []string{"storm-sensor-network"},
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphMermaid(&buf, g); err != nil {
		t.Fatalf("PrintGraphMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	// Sanitized ID should appear (hyphens → underscores).
	if !strings.Contains(out, "storm_sensor_network") {
		t.Errorf("expected sanitized node ID 'storm_sensor_network' in Mermaid output, got:\n%s", out)
	}
	// Original name should appear as the label.
	if !strings.Contains(out, "storm-sensor-network") {
		t.Errorf("expected original label 'storm-sensor-network' in Mermaid output, got:\n%s", out)
	}
}

// TestPrintGraphMermaid_WriteError verifies that PrintGraphMermaid propagates
// a write error from the underlying io.Writer back to the caller.
func TestPrintGraphMermaid_WriteError(t *testing.T) {
	sentinel := errors.New("disk full")
	w := errWriter{err: sentinel}

	err := graph.PrintGraphMermaid(w, graph.MeshGraph{})
	if err == nil {
		t.Fatal("PrintGraphMermaid: expected error from failing writer, got nil")
	}
}

// --- helper coverage tests ---

// TestPrintGraphDOT_TruncatesLongEdgeLabel verifies that edge labels longer
// than 40 runes are truncated with "..." in DOT output.
func TestPrintGraphDOT_TruncatesLongEdgeLabel(t *testing.T) {
	longLabel := strings.Repeat("x", 50)
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"a": {Name: "a", AppearanceCount: 1},
			"b": {Name: "b", AppearanceCount: 1},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "dddddddd-0000-4000-8000-000000000001",
				WhatChanged: longLabel,
				Sources:     []string{"a"},
				Targets:     []string{"b"},
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphDOT(&buf, g); err != nil {
		t.Fatalf("PrintGraphDOT: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "...") {
		t.Errorf("expected truncated label with '...' for 50-char label, got:\n%s", buf.String())
	}
}

// TestPrintGraphDOT_TimeWindowComment verifies that a non-zero TimeWindow
// appears in the DOT comment block, so the observer position and time window
// are recorded in the output file.
func TestPrintGraphDOT_TimeWindowComment(t *testing.T) {
	g := graph.MeshGraph{
		Cut: graph.Cut{
			ObserverPositions: []string{"meteorological-analyst"},
			TimeWindow: graph.TimeWindow{
				Start: mustParseTime(t, "2026-04-14T00:00:00Z"),
				End:   mustParseTime(t, "2026-04-14T23:59:59Z"),
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphDOT(&buf, g); err != nil {
		t.Fatalf("PrintGraphDOT: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "2026-04-14") {
		t.Errorf("expected date '2026-04-14' in DOT comment, got:\n%s", out)
	}
	if !strings.Contains(out, "meteorological-analyst") {
		t.Errorf("expected observer name in DOT comment, got:\n%s", out)
	}
}

// TestPrintGraphDOT_HalfOpenTimeWindow verifies that a half-open TimeWindow
// (Start only, no End) renders "(unbounded)" for the missing bound.
func TestPrintGraphDOT_HalfOpenTimeWindow(t *testing.T) {
	g := graph.MeshGraph{
		Cut: graph.Cut{
			TimeWindow: graph.TimeWindow{
				Start: mustParseTime(t, "2026-04-14T00:00:00Z"),
				// End is zero — unbounded.
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphDOT(&buf, g); err != nil {
		t.Fatalf("PrintGraphDOT: unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "(unbounded)") {
		t.Errorf("expected '(unbounded)' for half-open window, got:\n%s", buf.String())
	}
}

// TestPrintGraphMermaid_CollisionResolution verifies that two node names that
// sanitize to the same Mermaid ID both appear in the output with distinct IDs.
func TestPrintGraphMermaid_CollisionResolution(t *testing.T) {
	// "a-b" and "a_b" both sanitize to "a_b" — the second should get "a_b_2".
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"a-b": {Name: "a-b", AppearanceCount: 1},
			"a_b": {Name: "a_b", AppearanceCount: 1},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphMermaid(&buf, g); err != nil {
		t.Fatalf("PrintGraphMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	// One ID should be "a_b" and another "a_b_2" (or similar collision suffix).
	if !strings.Contains(out, "a_b_2") {
		t.Errorf("expected collision-resolved ID 'a_b_2' in output, got:\n%s", out)
	}
}

// TestPrintGraphMermaid_DigitPrefixedName verifies that a node name starting
// with a digit gets the "n_" prefix in its sanitized Mermaid ID.
func TestPrintGraphMermaid_DigitPrefixedName(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"3rd-party-api": {Name: "3rd-party-api", AppearanceCount: 1},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphMermaid(&buf, g); err != nil {
		t.Fatalf("PrintGraphMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "n_3rd_party_api") {
		t.Errorf("expected 'n_3rd_party_api' (digit-prefixed ID) in output, got:\n%s", out)
	}
}

// TestPrintGraphMermaid_EmptyNodeName verifies that a node with an empty name
// gets the fallback "n_empty" Mermaid ID rather than an empty string.
func TestPrintGraphMermaid_EmptyNodeName(t *testing.T) {
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{
			"": {Name: "", AppearanceCount: 1},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintGraphMermaid(&buf, g); err != nil {
		t.Fatalf("PrintGraphMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "n_empty") {
		t.Errorf("expected fallback ID 'n_empty' for empty node name, got:\n%s", out)
	}
}

