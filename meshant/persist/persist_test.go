// Package persist_test tests the persist package using black-box style.
//
// All file operations use t.TempDir() so cleanup is automatic and tests are
// fully isolated from one another and from the rest of the filesystem.
package persist_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/persist"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// sampleGraph returns a non-trivial MeshGraph for round-trip testing.
// It includes: a non-empty ID, two nodes, one edge, a non-zero TimeWindow,
// and one ShadowElement — enough structure to exercise the full JSON codec.
func sampleGraph() graph.MeshGraph {
	start := time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 14, 23, 59, 59, 0, time.UTC)

	g := graph.MeshGraph{
		ID: "meshgraph:550e8400-e29b-41d4-a716-446655440000",
		Nodes: map[string]graph.Node{
			"alpha": {Name: "alpha", AppearanceCount: 3, ShadowCount: 1},
			"beta":  {Name: "beta", AppearanceCount: 1, ShadowCount: 0},
		},
		Edges: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				WhatChanged: "test edge",
				Mediation:   "relay",
				Observer:    "obs-1",
				Sources:     []string{"alpha"},
				Targets:     []string{"beta"},
				Tags:        []string{"tag1"},
			},
		},
		Cut: graph.Cut{
			ObserverPositions:      []string{"obs-1"},
			TimeWindow:             graph.TimeWindow{Start: start, End: end},
			TracesIncluded:         1,
			TracesTotal:            3,
			DistinctObserversTotal: 2,
			ShadowElements: []graph.ShadowElement{
				{
					Name:     "gamma",
					SeenFrom: []string{"obs-2"},
					Reasons:  []graph.ShadowReason{graph.ShadowReasonObserver},
				},
			},
			ExcludedObserverPositions: []string{"obs-2"},
		},
	}
	return g
}

// sampleDiff returns a non-trivial GraphDiff for round-trip testing.
// It includes ShadowShifts and PersistedNodes to exercise all diff fields.
func sampleDiff() graph.GraphDiff {
	start := time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 18, 23, 59, 59, 0, time.UTC)

	d := graph.GraphDiff{
		ID:           "meshdiff:660e8400-e29b-41d4-a716-446655440001",
		NodesAdded:   []string{"delta"},
		NodesRemoved: []string{"epsilon"},
		NodesPersisted: []graph.PersistedNode{
			{Name: "alpha", CountFrom: 2, CountTo: 4},
		},
		EdgesAdded: []graph.Edge{
			{
				TraceID:     "11111111-2222-3333-4444-555555555555",
				WhatChanged: "added edge",
				Observer:    "obs-1",
				Sources:     []string{"alpha"},
				Targets:     []string{"delta"},
				Tags:        []string{},
			},
		},
		EdgesRemoved: []graph.Edge{},
		ShadowShifts: []graph.ShadowShift{
			{
				Name:        "gamma",
				Kind:        graph.ShadowShiftEmerged,
				FromReasons: []graph.ShadowReason{graph.ShadowReasonObserver},
				ToReasons:   nil,
			},
		},
		From: graph.Cut{
			ObserverPositions:      []string{"obs-1"},
			TimeWindow:             graph.TimeWindow{Start: start, End: end},
			TracesIncluded:         5,
			TracesTotal:            10,
			DistinctObserversTotal: 3,
		},
		To: graph.Cut{
			ObserverPositions:      []string{"obs-1"},
			TimeWindow:             graph.TimeWindow{Start: start, End: end},
			TracesIncluded:         7,
			TracesTotal:            10,
			DistinctObserversTotal: 3,
		},
	}
	return d
}

// ---------------------------------------------------------------------------
// WriteJSON tests
// ---------------------------------------------------------------------------

// TestWriteJSON_CreatesFile verifies that WriteJSON creates a file at the given
// path when none exists yet.
func TestWriteJSON_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	g := sampleGraph()
	if err := persist.WriteJSON(path, g); err != nil {
		t.Fatalf("WriteJSON returned unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist at %q, got error: %v", path, err)
	}
}

// TestWriteJSON_InvalidPath verifies that WriteJSON returns an error when the
// parent directory does not exist.
func TestWriteJSON_InvalidPath(t *testing.T) {
	// Use a path inside a directory that doesn't exist.
	path := filepath.Join(t.TempDir(), "nonexistent-dir", "out.json")

	err := persist.WriteJSON(path, sampleGraph())
	if err == nil {
		t.Fatal("expected WriteJSON to return an error for invalid path, got nil")
	}
}

// ---------------------------------------------------------------------------
// MeshGraph round-trip tests
// ---------------------------------------------------------------------------

// TestWriteJSON_ReadGraphJSON_RoundTrip verifies that a MeshGraph survives a
// WriteJSON → ReadGraphJSON round-trip with all fields preserved.
func TestWriteJSON_ReadGraphJSON_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")

	original := sampleGraph()
	if err := persist.WriteJSON(path, original); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	got, err := persist.ReadGraphJSON(path)
	if err != nil {
		t.Fatalf("ReadGraphJSON: %v", err)
	}

	// Verify actor ID is preserved.
	if got.ID != original.ID {
		t.Errorf("ID: got %q, want %q", got.ID, original.ID)
	}

	// Verify node count.
	if len(got.Nodes) != len(original.Nodes) {
		t.Errorf("Nodes count: got %d, want %d", len(got.Nodes), len(original.Nodes))
	}
	for name, wantNode := range original.Nodes {
		gotNode, ok := got.Nodes[name]
		if !ok {
			t.Errorf("Nodes: missing key %q", name)
			continue
		}
		if gotNode.AppearanceCount != wantNode.AppearanceCount {
			t.Errorf("Nodes[%q].AppearanceCount: got %d, want %d", name, gotNode.AppearanceCount, wantNode.AppearanceCount)
		}
		if gotNode.ShadowCount != wantNode.ShadowCount {
			t.Errorf("Nodes[%q].ShadowCount: got %d, want %d", name, gotNode.ShadowCount, wantNode.ShadowCount)
		}
	}

	// Verify edge count and first edge fields.
	if len(got.Edges) != len(original.Edges) {
		t.Fatalf("Edges count: got %d, want %d", len(got.Edges), len(original.Edges))
	}
	gotEdge := got.Edges[0]
	wantEdge := original.Edges[0]
	if gotEdge.TraceID != wantEdge.TraceID {
		t.Errorf("Edge.TraceID: got %q, want %q", gotEdge.TraceID, wantEdge.TraceID)
	}
	if gotEdge.WhatChanged != wantEdge.WhatChanged {
		t.Errorf("Edge.WhatChanged: got %q, want %q", gotEdge.WhatChanged, wantEdge.WhatChanged)
	}
	if gotEdge.Mediation != wantEdge.Mediation {
		t.Errorf("Edge.Mediation: got %q, want %q", gotEdge.Mediation, wantEdge.Mediation)
	}

	// Verify TimeWindow round-trip: both bounds should survive as non-zero.
	if got.Cut.TimeWindow.IsZero() {
		t.Error("TimeWindow: got zero, want non-zero")
	}
	if !got.Cut.TimeWindow.Start.Equal(original.Cut.TimeWindow.Start) {
		t.Errorf("TimeWindow.Start: got %v, want %v", got.Cut.TimeWindow.Start, original.Cut.TimeWindow.Start)
	}
	if !got.Cut.TimeWindow.End.Equal(original.Cut.TimeWindow.End) {
		t.Errorf("TimeWindow.End: got %v, want %v", got.Cut.TimeWindow.End, original.Cut.TimeWindow.End)
	}

	// Verify shadow elements.
	if len(got.Cut.ShadowElements) != 1 {
		t.Fatalf("ShadowElements count: got %d, want 1", len(got.Cut.ShadowElements))
	}
	if got.Cut.ShadowElements[0].Name != "gamma" {
		t.Errorf("ShadowElements[0].Name: got %q, want %q", got.Cut.ShadowElements[0].Name, "gamma")
	}
}

// TestWriteJSON_ReadGraphJSON_ZeroTimeWindow verifies that a zero TimeWindow
// round-trips correctly — null bounds should decode as zero time.Time.
func TestWriteJSON_ReadGraphJSON_ZeroTimeWindow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph_zero_tw.json")

	// Build a graph with a zero TimeWindow (no time filter applied).
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{},
		Edges: []graph.Edge{},
		Cut: graph.Cut{
			TimeWindow:     graph.TimeWindow{}, // zero — both bounds null in JSON
			TracesIncluded: 0,
			TracesTotal:    0,
		},
	}

	if err := persist.WriteJSON(path, g); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	got, err := persist.ReadGraphJSON(path)
	if err != nil {
		t.Fatalf("ReadGraphJSON: %v", err)
	}

	if !got.Cut.TimeWindow.IsZero() {
		t.Errorf("expected zero TimeWindow, got Start=%v End=%v",
			got.Cut.TimeWindow.Start, got.Cut.TimeWindow.End)
	}
}

// TestReadGraphJSON_FileNotFound verifies that ReadGraphJSON returns an error
// when the target file does not exist.
func TestReadGraphJSON_FileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	_, err := persist.ReadGraphJSON(path)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestReadGraphJSON_InvalidJSON verifies that ReadGraphJSON returns an error
// when the file contains invalid JSON.
func TestReadGraphJSON_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	// Write garbage bytes that are not valid JSON.
	if err := os.WriteFile(path, []byte("this is not JSON {{{{"), 0644); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}

	_, err := persist.ReadGraphJSON(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// GraphDiff round-trip tests
// ---------------------------------------------------------------------------

// TestWriteJSON_ReadDiffJSON_RoundTrip verifies that a GraphDiff survives a
// WriteJSON → ReadDiffJSON round-trip with all fields preserved.
func TestWriteJSON_ReadDiffJSON_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "diff.json")

	original := sampleDiff()
	if err := persist.WriteJSON(path, original); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	got, err := persist.ReadDiffJSON(path)
	if err != nil {
		t.Fatalf("ReadDiffJSON: %v", err)
	}

	// Verify actor ID is preserved.
	if got.ID != original.ID {
		t.Errorf("ID: got %q, want %q", got.ID, original.ID)
	}

	// Verify NodesAdded.
	if len(got.NodesAdded) != len(original.NodesAdded) {
		t.Fatalf("NodesAdded count: got %d, want %d", len(got.NodesAdded), len(original.NodesAdded))
	}
	if got.NodesAdded[0] != original.NodesAdded[0] {
		t.Errorf("NodesAdded[0]: got %q, want %q", got.NodesAdded[0], original.NodesAdded[0])
	}

	// Verify NodesRemoved.
	if len(got.NodesRemoved) != len(original.NodesRemoved) {
		t.Fatalf("NodesRemoved count: got %d, want %d", len(got.NodesRemoved), len(original.NodesRemoved))
	}

	// Verify NodesPersisted.
	if len(got.NodesPersisted) != len(original.NodesPersisted) {
		t.Fatalf("NodesPersisted count: got %d, want %d", len(got.NodesPersisted), len(original.NodesPersisted))
	}
	gotPN := got.NodesPersisted[0]
	wantPN := original.NodesPersisted[0]
	if gotPN.Name != wantPN.Name || gotPN.CountFrom != wantPN.CountFrom || gotPN.CountTo != wantPN.CountTo {
		t.Errorf("NodesPersisted[0]: got %+v, want %+v", gotPN, wantPN)
	}

	// Verify ShadowShifts are preserved.
	if len(got.ShadowShifts) != len(original.ShadowShifts) {
		t.Fatalf("ShadowShifts count: got %d, want %d", len(got.ShadowShifts), len(original.ShadowShifts))
	}
	gotSS := got.ShadowShifts[0]
	wantSS := original.ShadowShifts[0]
	if gotSS.Name != wantSS.Name {
		t.Errorf("ShadowShifts[0].Name: got %q, want %q", gotSS.Name, wantSS.Name)
	}
	if gotSS.Kind != wantSS.Kind {
		t.Errorf("ShadowShifts[0].Kind: got %q, want %q", gotSS.Kind, wantSS.Kind)
	}

	// Verify From/To TimeWindow round-trips.
	if got.From.TimeWindow.IsZero() {
		t.Error("From.TimeWindow: got zero, want non-zero")
	}
	if got.To.TimeWindow.IsZero() {
		t.Error("To.TimeWindow: got zero, want non-zero")
	}

	// Verify EdgesAdded preserved.
	if len(got.EdgesAdded) != 1 {
		t.Fatalf("EdgesAdded count: got %d, want 1", len(got.EdgesAdded))
	}
	if got.EdgesAdded[0].TraceID != original.EdgesAdded[0].TraceID {
		t.Errorf("EdgesAdded[0].TraceID: got %q, want %q", got.EdgesAdded[0].TraceID, original.EdgesAdded[0].TraceID)
	}
}

// TestReadDiffJSON_FileNotFound verifies that ReadDiffJSON returns an error
// when the target file does not exist.
func TestReadDiffJSON_FileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	_, err := persist.ReadDiffJSON(path)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestReadDiffJSON_InvalidJSON verifies that ReadDiffJSON returns an error
// when the file contains invalid JSON.
func TestReadDiffJSON_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	// Write garbage bytes that are not valid JSON.
	if err := os.WriteFile(path, []byte("<<<not json>>>"), 0644); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}

	_, err := persist.ReadDiffJSON(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestWriteJSON_MarshalError verifies that WriteJSON returns an error when
// passed a value that cannot be marshalled to JSON, such as a channel.
// This exercises the marshal error path in WriteJSON, which is unreachable
// for MeshGraph/GraphDiff but reachable for arbitrary `any` inputs.
func TestWriteJSON_MarshalError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	// A channel cannot be marshalled to JSON — json.Marshal returns an error.
	ch := make(chan int)
	err := persist.WriteJSON(path, ch)
	if err == nil {
		t.Fatal("expected marshal error for channel value, got nil")
	}
}
