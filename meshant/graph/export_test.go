// Package graph_test — export_test.go tests PrintGraphJSON and PrintDiffJSON.
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
