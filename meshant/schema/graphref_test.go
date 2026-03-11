// Package schema_test — tests for graph-reference string predicates.
//
// Graph-reference strings appear in Trace.Source and Trace.Target when an
// identified MeshGraph or GraphDiff is used as an actor in subsequent traces.
// These tests verify that the schema-layer predicates (IsGraphRef, GraphRefKind,
// GraphRefID) correctly recognise and parse those strings, and that Validate()
// continues to accept them in Source/Target without modification.
package schema_test

import (
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// wellFormedGraphRef is a known-good meshgraph: reference used across multiple tests.
const wellFormedGraphRef = "meshgraph:a1b2c3d4-bbbb-4ccc-8ddd-eeeeeeeeee01"

// wellFormedDiffRef is a known-good meshdiff: reference used across multiple tests.
const wellFormedDiffRef = "meshdiff:b2c3d4e5-bbbb-4ccc-8ddd-eeeeeeeeee02"

// --- Group 1: IsGraphRef ---

func TestIsGraphRef_MeshgraphPrefix_True(t *testing.T) {
	if !schema.IsGraphRef(wellFormedGraphRef) {
		t.Errorf("IsGraphRef(%q) = false; want true", wellFormedGraphRef)
	}
}

func TestIsGraphRef_MeshdiffPrefix_True(t *testing.T) {
	if !schema.IsGraphRef(wellFormedDiffRef) {
		t.Errorf("IsGraphRef(%q) = false; want true", wellFormedDiffRef)
	}
}

func TestIsGraphRef_PlainElement_False(t *testing.T) {
	s := "landsat-9-satellite"
	if schema.IsGraphRef(s) {
		t.Errorf("IsGraphRef(%q) = true; want false", s)
	}
}

func TestIsGraphRef_Empty_False(t *testing.T) {
	if schema.IsGraphRef("") {
		t.Errorf("IsGraphRef(%q) = true; want false", "")
	}
}

func TestIsGraphRef_PartialPrefix_False(t *testing.T) {
	// "meshgraph" without a colon — not a valid graph-reference prefix.
	s := "meshgraph"
	if schema.IsGraphRef(s) {
		t.Errorf("IsGraphRef(%q) = true; want false", s)
	}
}

func TestIsGraphRef_UnknownKindWithColon_False(t *testing.T) {
	// A colon-containing string whose prefix is not a known kind.
	s := "http:something"
	if schema.IsGraphRef(s) {
		t.Errorf("IsGraphRef(%q) = true; want false", s)
	}
}

// --- Group 2: GraphRefKind ---

func TestGraphRefKind_MeshgraphPrefix(t *testing.T) {
	got := schema.GraphRefKind(wellFormedGraphRef)
	if got != "meshgraph" {
		t.Errorf("GraphRefKind(%q) = %q; want %q", wellFormedGraphRef, got, "meshgraph")
	}
}

func TestGraphRefKind_MeshdiffPrefix(t *testing.T) {
	got := schema.GraphRefKind(wellFormedDiffRef)
	if got != "meshdiff" {
		t.Errorf("GraphRefKind(%q) = %q; want %q", wellFormedDiffRef, got, "meshdiff")
	}
}

func TestGraphRefKind_PlainElement_Empty(t *testing.T) {
	got := schema.GraphRefKind("some-plain-element")
	if got != "" {
		t.Errorf("GraphRefKind(%q) = %q; want %q", "some-plain-element", got, "")
	}
}

// --- Group 3: GraphRefID ---

func TestGraphRefID_ExtractsUUID(t *testing.T) {
	want := "a1b2c3d4-bbbb-4ccc-8ddd-eeeeeeeeee01"
	got := schema.GraphRefID(wellFormedGraphRef)
	if got != want {
		t.Errorf("GraphRefID(%q) = %q; want %q", wellFormedGraphRef, got, want)
	}
}

func TestGraphRefID_PlainElement_Empty(t *testing.T) {
	got := schema.GraphRefID("community-monitor")
	if got != "" {
		t.Errorf("GraphRefID(%q) = %q; want empty string", "community-monitor", got)
	}
}

func TestGraphRefID_ExtractsUUID_Diff(t *testing.T) {
	want := "b2c3d4e5-bbbb-4ccc-8ddd-eeeeeeeeee02"
	got := schema.GraphRefID(wellFormedDiffRef)
	if got != want {
		t.Errorf("GraphRefID(%q) = %q; want %q", wellFormedDiffRef, got, want)
	}
}

func TestGraphRefID_EmptyAfterColon_Empty(t *testing.T) {
	// "meshgraph:" with nothing after the colon — ID portion is empty string.
	s := "meshgraph:"
	got := schema.GraphRefID(s)
	if got != "" {
		t.Errorf("GraphRefID(%q) = %q; want empty string", s, got)
	}
}

// --- Group 4: Validate compatibility ---

// TestValidate_GraphRefInSource_Valid confirms that Validate() accepts a trace
// whose Source contains a meshgraph: reference string. Validate() must not
// inspect or restrict the content of Source/Target elements.
func TestValidate_GraphRefInSource_Valid(t *testing.T) {
	tr := schema.Trace{
		ID:          "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		Timestamp:   time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC),
		WhatChanged: "policy updated based on deforestation graph",
		Source:      []string{wellFormedGraphRef},
		Observer:    "policy-agent/position-A",
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("Validate() returned error for trace with graph-ref in Source: %v", err)
	}
}

// TestValidate_GraphRefInTarget_Valid confirms that Validate() accepts a trace
// whose Target contains a meshdiff: reference string.
func TestValidate_GraphRefInTarget_Valid(t *testing.T) {
	tr := schema.Trace{
		ID:          "b2c3d4e5-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		Timestamp:   time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC),
		WhatChanged: "routing matrix sent diff report to monitoring service",
		Target:      []string{wellFormedDiffRef},
		Observer:    "policy-agent/position-A",
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("Validate() returned error for trace with graph-ref in Target: %v", err)
	}
}
