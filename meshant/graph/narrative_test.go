// narrative_test.go tests DraftNarrative and PrintNarrativeDraft.
//
// All tests are in package graph_test (black-box) and use the shared helpers
// from testhelpers_test.go: buildGraph and validTraceWithElements.
//
// Test strategy:
//   - Most tests use narrativeGraph() which builds a MeshGraph directly via
//     buildGraph (testhelpers) with manually set Nodes and Cut fields, giving
//     precise control over AppearanceCount and shadow data without needing a
//     trace file.
//   - The immutability test clones edge/node/shadow state and checks none is
//     modified after the call.
//   - PrintNarrativeDraft tests capture output in a bytes.Buffer.
package graph_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// narrativeGraph returns a MeshGraph with three edges, four nodes with known
// AppearanceCount values, and an empty shadow. It is used as the baseline
// input for narrative tests. Callers mutate Cut fields as needed.
//
// Nodes and their counts:
//   wind-sensor          2  (appears in edge-001 source, edge-003 source)
//   evacuation-decision  2  (appears in edge-001 target, edge-002 source)
//   data-logger          1  (appears in edge-003 target)
//   public-alert         1  (appears in edge-002 target)
//
// Alpha tiebreak: data-logger < public-alert — top-3 are wind-sensor,
// evacuation-decision, data-logger.
//
// Mediations: "threshold-protocol" (edges 1,3) and "political-authority" (edge 2).
func narrativeGraph() graph.MeshGraph {
	edges := []graph.Edge{
		{
			TraceID:     "trace-001",
			WhatChanged: "wind exceeded threshold",
			Mediation:   "threshold-protocol",
			Observer:    "meteorological-analyst",
			Sources:     []string{"wind-sensor"},
			Targets:     []string{"evacuation-decision"},
		},
		{
			TraceID:     "trace-002",
			WhatChanged: "alert issued",
			Mediation:   "political-authority",
			Observer:    "local-mayor",
			Sources:     []string{"evacuation-decision"},
			Targets:     []string{"public-alert"},
		},
		{
			TraceID:     "trace-003",
			WhatChanged: "sensor read",
			Mediation:   "threshold-protocol",
			Observer:    "meteorological-analyst",
			Sources:     []string{"wind-sensor"},
			Targets:     []string{"data-logger"},
		},
	}
	g := buildGraph(edges)

	// Override Nodes with explicit AppearanceCount values so sorting is predictable.
	g.Nodes = map[string]graph.Node{
		"wind-sensor":         {Name: "wind-sensor", AppearanceCount: 2},
		"evacuation-decision": {Name: "evacuation-decision", AppearanceCount: 2},
		"public-alert":        {Name: "public-alert", AppearanceCount: 1},
		"data-logger":         {Name: "data-logger", AppearanceCount: 1},
	}
	g.Cut.TracesTotal = 3
	g.Cut.TracesIncluded = 3
	return g
}

// --- Group 1: empty graph ---

// TestDraftNarrative_EmptyGraph verifies that DraftNarrative returns a
// zero-value NarrativeDraft without panicking when given an empty graph
// (no edges). The spec requires zero-value return when len(g.Edges) == 0.
func TestDraftNarrative_EmptyGraph(t *testing.T) {
	g := graph.MeshGraph{}
	got := graph.DraftNarrative(g)

	if got.PositionStatement != "" {
		t.Errorf("PositionStatement: want empty string for empty graph, got %q", got.PositionStatement)
	}
	if got.Body != "" {
		t.Errorf("Body: want empty string for empty graph, got %q", got.Body)
	}
	if got.ShadowStatement != "" {
		t.Errorf("ShadowStatement: want empty string for empty graph, got %q", got.ShadowStatement)
	}
	if len(got.Caveats) != 0 {
		t.Errorf("Caveats: want nil/empty for empty graph, got %v", got.Caveats)
	}
}

// TestDraftNarrative_EmptyGraphCaveatsNil verifies that Caveats is empty
// specifically when Edges is nil/empty, even when Nodes is non-empty.
// This ensures the zero-value return is triggered by the edge count, not node
// count — Edges are the proxy for "does this graph contain any data?".
func TestDraftNarrative_EmptyGraphCaveatsNil(t *testing.T) {
	// Nodes non-empty, Edges empty → still zero-value return.
	g := graph.MeshGraph{
		Nodes: map[string]graph.Node{"x": {Name: "x", AppearanceCount: 1}},
	}
	got := graph.DraftNarrative(g)
	if len(got.Caveats) != 0 {
		t.Errorf("Caveats: want empty for zero-edge graph, got %v", got.Caveats)
	}
}

// --- Group 2: PositionStatement ---

// TestDraftNarrative_PositionStatement verifies that PositionStatement
// starts with the required prefix phrase and contains the cutLabel output
// (observer name when ObserverPositions is set).
func TestDraftNarrative_PositionStatement(t *testing.T) {
	g := narrativeGraph()
	g.Cut.ObserverPositions = []string{"meteorological-analyst"}

	got := graph.DraftNarrative(g)

	const prefix = "This reading is taken from the position:"
	if !strings.Contains(got.PositionStatement, prefix) {
		t.Errorf("PositionStatement: want prefix %q, got %q", prefix, got.PositionStatement)
	}
	if !strings.Contains(got.PositionStatement, "meteorological-analyst") {
		t.Errorf("PositionStatement: want observer name 'meteorological-analyst', got %q", got.PositionStatement)
	}
}

// --- Group 3: Body ---

// TestDraftNarrative_BodyMentionsTopElements verifies that the Body string
// contains the names of the top-3 elements by AppearanceCount. Given
// narrativeGraph's node counts (wind-sensor:2, evacuation-decision:2,
// data-logger:1, public-alert:1) and alpha tiebreak, the top-3 are:
// wind-sensor, evacuation-decision, data-logger.
func TestDraftNarrative_BodyMentionsTopElements(t *testing.T) {
	g := narrativeGraph()
	got := graph.DraftNarrative(g)

	// The top-3 elements must appear in the Body.
	for _, name := range []string{"wind-sensor", "evacuation-decision", "data-logger"} {
		if !strings.Contains(got.Body, name) {
			t.Errorf("Body: want element %q, got:\n%s", name, got.Body)
		}
	}
}

// TestDraftNarrative_BodyMentionsMediations verifies that distinct non-empty
// Edge.Mediation strings appear in the Body.
func TestDraftNarrative_BodyMentionsMediations(t *testing.T) {
	g := narrativeGraph()
	got := graph.DraftNarrative(g)

	for _, med := range []string{"threshold-protocol", "political-authority"} {
		if !strings.Contains(got.Body, med) {
			t.Errorf("Body: want mediation %q, got:\n%s", med, got.Body)
		}
	}
}

// --- Group 4: ShadowStatement ---

// TestDraftNarrative_ShadowStatement verifies that the ShadowStatement
// contains the shadow element count and the distinct exclusion reasons when
// shadow > 0.
func TestDraftNarrative_ShadowStatement(t *testing.T) {
	g := narrativeGraph()
	g.Cut.ShadowElements = []graph.ShadowElement{
		{
			Name:     "hidden-actor",
			SeenFrom: []string{"other-obs"},
			Reasons:  []graph.ShadowReason{graph.ShadowReasonObserver},
		},
		{
			Name:    "time-gated",
			SeenFrom: nil,
			Reasons: []graph.ShadowReason{graph.ShadowReasonTimeWindow},
		},
	}

	got := graph.DraftNarrative(g)

	// Must mention count (2 elements in shadow).
	if !strings.Contains(got.ShadowStatement, "2") {
		t.Errorf("ShadowStatement: want count '2', got %q", got.ShadowStatement)
	}
	// Must mention distinct exclusion reasons.
	if !strings.Contains(got.ShadowStatement, "observer") {
		t.Errorf("ShadowStatement: want reason 'observer', got %q", got.ShadowStatement)
	}
	if !strings.Contains(got.ShadowStatement, "time-window") {
		t.Errorf("ShadowStatement: want reason 'time-window', got %q", got.ShadowStatement)
	}
}

// TestDraftNarrative_ShadowLanguage verifies ANT-correct language in
// ShadowStatement: must contain "in shadow" (positional framing) and must
// NOT contain "missing" (which implies absence rather than positional
// invisibility — a category error in ANT terms).
func TestDraftNarrative_ShadowLanguage(t *testing.T) {
	g := narrativeGraph()
	g.Cut.ShadowElements = []graph.ShadowElement{
		{Name: "shadow-actor", Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
	}

	got := graph.DraftNarrative(g)

	if !strings.Contains(got.ShadowStatement, "in shadow") {
		t.Errorf("ShadowStatement: want phrase 'in shadow', got %q", got.ShadowStatement)
	}
	if strings.Contains(strings.ToLower(got.ShadowStatement), "missing") {
		t.Errorf("ShadowStatement: must NOT contain 'missing' (ANT constraint), got %q", got.ShadowStatement)
	}
}

// --- Group 5: Caveats ---

// TestDraftNarrative_CaveatsNonEmpty verifies that Caveats has at least one
// entry for any non-empty graph (len(g.Edges) > 0), and that the standard
// positioned-reading caveat is always present.
func TestDraftNarrative_CaveatsNonEmpty(t *testing.T) {
	g := narrativeGraph()
	got := graph.DraftNarrative(g)

	if len(got.Caveats) == 0 {
		t.Fatal("Caveats: want at least one caveat for non-empty graph, got empty")
	}
	// Standard caveat about positioned reading must be present.
	found := false
	for _, c := range got.Caveats {
		if strings.Contains(c, "positioned reading") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Caveats: want standard 'positioned reading' caveat; got %v", got.Caveats)
	}
}

// --- Group 6: immutability ---

// TestDraftNarrative_ImmutableInput verifies that DraftNarrative does not
// mutate the input MeshGraph. Edges, Nodes, and Cut.ShadowElements lengths
// must be unchanged after the call.
func TestDraftNarrative_ImmutableInput(t *testing.T) {
	g := narrativeGraph()
	g.Cut.ShadowElements = []graph.ShadowElement{
		{Name: "shadow-x", Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
	}

	// Snapshot state before the call.
	edgesBefore := len(g.Edges)
	nodesBefore := len(g.Nodes)
	shadowBefore := len(g.Cut.ShadowElements)

	_ = graph.DraftNarrative(g)

	if len(g.Edges) != edgesBefore {
		t.Errorf("ImmutableInput: Edges mutated — before=%d after=%d", edgesBefore, len(g.Edges))
	}
	if len(g.Nodes) != nodesBefore {
		t.Errorf("ImmutableInput: Nodes mutated — before=%d after=%d", nodesBefore, len(g.Nodes))
	}
	if len(g.Cut.ShadowElements) != shadowBefore {
		t.Errorf("ImmutableInput: ShadowElements mutated — before=%d after=%d", shadowBefore, len(g.Cut.ShadowElements))
	}
}

// --- Group 7: PrintNarrativeDraft ---

// TestPrintNarrativeDraft_ContainsSections verifies that the output of
// PrintNarrativeDraft contains all required section headers:
// "Position:", "Reading:", "Shadow:", and "Caveats:".
func TestPrintNarrativeDraft_ContainsSections(t *testing.T) {
	g := narrativeGraph()
	n := graph.DraftNarrative(g)

	var buf bytes.Buffer
	if err := graph.PrintNarrativeDraft(&buf, n); err != nil {
		t.Fatalf("PrintNarrativeDraft: unexpected error: %v", err)
	}
	out := buf.String()

	for _, section := range []string{"Position:", "Reading:", "Shadow:", "Caveats:"} {
		if !strings.Contains(out, section) {
			t.Errorf("PrintNarrativeDraft: output missing section %q\ngot:\n%s", section, out)
		}
	}
}

// TestPrintNarrativeDraft_DraftLabel verifies that the output contains "Draft"
// or "draft" — signalling that this is a provisional reading, not a conclusion.
func TestPrintNarrativeDraft_DraftLabel(t *testing.T) {
	g := narrativeGraph()
	n := graph.DraftNarrative(g)

	var buf bytes.Buffer
	if err := graph.PrintNarrativeDraft(&buf, n); err != nil {
		t.Fatalf("PrintNarrativeDraft: unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "draft") {
		t.Errorf("PrintNarrativeDraft: output does not contain 'draft'\ngot:\n%s", buf.String())
	}
}
