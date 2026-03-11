// Package loader_test — E2E tests for graph-reference handling.
//
// These tests exercise the full load → summarise pipeline against
// data/examples/graph_ref_traces.json, a dataset in which some Source and
// Target entries are graph-reference strings (meshgraph: / meshdiff: prefixes).
// They verify that GraphRefs is populated correctly and that non-ref elements
// are still counted in Elements.
package loader_test

import (
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// graphRefPath is the path to the graph-reference example dataset,
// relative to the loader package directory (which is the working directory
// when tests run).
const graphRefPath = "../../data/examples/graph_ref_traces.json"

// knownGraphRef1 and knownGraphRef2 are the meshgraph: references present in
// graph_ref_traces.json. knownDiffRef is the meshdiff: reference.
const (
	knownGraphRef1 = "meshgraph:a1b2c3d4-bbbb-4ccc-8ddd-eeeeeeeeee01"
	knownGraphRef2 = "meshgraph:b2c3d4e5-bbbb-4ccc-8ddd-eeeeeeeeee02"
	knownDiffRef   = "meshdiff:c3d4e5f6-bbbb-4ccc-8ddd-eeeeeeeeee03"
)

// --- Group 7: GraphRef E2E ---

func TestGraphRef_Load_Count(t *testing.T) {
	traces, err := loader.Load(graphRefPath)
	if err != nil {
		t.Fatalf("Load(%q) error: %v", graphRefPath, err)
	}
	if len(traces) != 6 {
		t.Errorf("Load: got %d traces; want 6", len(traces))
	}
}

func TestGraphRef_Summarise_GraphRefsCount(t *testing.T) {
	traces, err := loader.Load(graphRefPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	s := loader.Summarise(traces)
	// Dataset has 3 distinct graph-refs: knownGraphRef1, knownGraphRef2, knownDiffRef.
	if len(s.GraphRefs) != 3 {
		t.Errorf("GraphRefs count = %d; want 3; got %v", len(s.GraphRefs), s.GraphRefs)
	}
}

func TestGraphRef_Summarise_KnownRefPresent(t *testing.T) {
	traces, err := loader.Load(graphRefPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	s := loader.Summarise(traces)
	found := false
	for _, ref := range s.GraphRefs {
		if ref == knownGraphRef1 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GraphRefs does not contain %q; got %v", knownGraphRef1, s.GraphRefs)
	}
}

func TestGraphRef_Summarise_DiffRefPresent(t *testing.T) {
	traces, err := loader.Load(graphRefPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	s := loader.Summarise(traces)
	found := false
	for _, ref := range s.GraphRefs {
		if ref == knownDiffRef {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GraphRefs does not contain %q; got %v", knownDiffRef, s.GraphRefs)
	}
}

func TestGraphRef_Summarise_ElementsStillCounted(t *testing.T) {
	traces, err := loader.Load(graphRefPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	s := loader.Summarise(traces)
	// "landsat-9-satellite" appears in traces 3 and 4 (once per trace) — count = 2.
	if s.Elements["landsat-9-satellite"] == 0 {
		t.Errorf("Elements[%q] = 0; want > 0 (plain elements still counted)", "landsat-9-satellite")
	}
}
