// Package graph_test — export_diff_test.go tests PrintDiffDOT and PrintDiffMermaid.
//
// All tests follow the black-box convention: only the exported API of the graph
// package is exercised. errWriter is defined in export_test.go (same package).
package graph_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// mustParseTime is defined in export_test.go — available here via the shared
// graph_test package.

// buildDiffWithAllKinds constructs a non-trivial GraphDiff that has nodes
// added, removed, persisted, edges added and removed, and shadow shifts of
// all three kinds (emerged, submerged, reason-changed).
func buildDiffWithAllKinds(t *testing.T) graph.GraphDiff {
	t.Helper()
	d := buildTestDiff(t) // defined in export_test.go

	// Add a submerged and reason-changed shadow shift alongside the emerged one.
	d.ShadowShifts = append(d.ShadowShifts,
		graph.ShadowShift{
			Name:        "legacy-sensor",
			Kind:        graph.ShadowShiftSubmerged,
			ToReasons:   []graph.ShadowReason{graph.ShadowReasonObserver},
		},
		graph.ShadowShift{
			Name:        "relay-station",
			Kind:        graph.ShadowShiftReasonChanged,
			FromReasons: []graph.ShadowReason{graph.ShadowReasonObserver},
			ToReasons:   []graph.ShadowReason{graph.ShadowReasonTimeWindow},
		},
	)
	return d
}

// --- PrintDiffDOT tests ---

// TestPrintDiffDOT_Basic verifies that a non-trivial diff produces valid DOT
// output containing the digraph block, green added nodes, red removed nodes,
// and persisted nodes with count labels.
func TestPrintDiffDOT_Basic(t *testing.T) {
	d := buildTestDiff(t)

	var buf bytes.Buffer
	if err := graph.PrintDiffDOT(&buf, d); err != nil {
		t.Fatalf("PrintDiffDOT: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "digraph {") {
		t.Errorf("expected 'digraph {' in DOT output, got:\n%s", out)
	}
	// Added node: evacuation-order — should have color=green
	if !strings.Contains(out, "color=green") {
		t.Errorf("expected 'color=green' for added node in DOT output, got:\n%s", out)
	}
	// Removed node: old-model — should have color=red
	if !strings.Contains(out, "color=red") {
		t.Errorf("expected 'color=red' for removed node in DOT output, got:\n%s", out)
	}
	// Persisted node: storm-model-alpha with count label "2→3"
	if !strings.Contains(out, "storm-model-alpha") {
		t.Errorf("expected persisted node 'storm-model-alpha' in DOT output, got:\n%s", out)
	}
	if !strings.Contains(out, "2→3") {
		t.Errorf("expected count label '2→3' for persisted node, got:\n%s", out)
	}
}

// TestPrintDiffDOT_EmptyDiff verifies that an empty GraphDiff produces a valid
// minimal DOT digraph without error.
func TestPrintDiffDOT_EmptyDiff(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintDiffDOT(&buf, graph.GraphDiff{}); err != nil {
		t.Fatalf("PrintDiffDOT on empty diff: unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "digraph {") {
		t.Errorf("expected 'digraph {' even for empty diff, got:\n%s", out)
	}
	if !strings.Contains(out, "}") {
		t.Errorf("expected closing '}' in DOT output, got:\n%s", out)
	}
}

// TestPrintDiffDOT_ShadowShifts verifies that all three shadow shift kinds
// (emerged, submerged, reason-changed) render inside cluster_shadow_shifts
// with correct colors: emerged=green (now visible, consistent with added),
// submerged=red (now hidden, consistent with removed), reason-changed=orange.
func TestPrintDiffDOT_ShadowShifts(t *testing.T) {
	d := buildDiffWithAllKinds(t)

	var buf bytes.Buffer
	if err := graph.PrintDiffDOT(&buf, d); err != nil {
		t.Fatalf("PrintDiffDOT: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "cluster_shadow_shifts") {
		t.Errorf("expected 'cluster_shadow_shifts' subgraph in DOT output, got:\n%s", out)
	}
	// Per-element color assertions: check each shift node's line has the right color.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "evacuation-shelter-b") && strings.Contains(line, "[label=") {
			// emerged → green (consistent with added-node convention)
			if !strings.Contains(line, "color=green") {
				t.Errorf("emerged shift 'evacuation-shelter-b' should have color=green, got: %q", line)
			}
		}
		if strings.Contains(line, "legacy-sensor") && strings.Contains(line, "[label=") {
			// submerged → red (consistent with removed-node convention)
			if !strings.Contains(line, "color=red") {
				t.Errorf("submerged shift 'legacy-sensor' should have color=red, got: %q", line)
			}
		}
		if strings.Contains(line, "relay-station") && strings.Contains(line, "[label=") {
			// reason-changed → orange
			if !strings.Contains(line, "color=orange") {
				t.Errorf("reason-changed shift 'relay-station' should have color=orange, got: %q", line)
			}
		}
	}
}

// TestPrintDiffDOT_NoShadowShifts verifies that the cluster_shadow_shifts
// subgraph is NOT emitted when there are no shadow shifts.
func TestPrintDiffDOT_NoShadowShifts(t *testing.T) {
	d := buildTestDiff(t)
	d.ShadowShifts = nil

	var buf bytes.Buffer
	if err := graph.PrintDiffDOT(&buf, d); err != nil {
		t.Fatalf("PrintDiffDOT: unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "cluster_shadow_shifts") {
		t.Errorf("expected NO 'cluster_shadow_shifts' for diff with no shadow shifts, got:\n%s", out)
	}
}

// TestPrintDiffDOT_MultiSourceEdges verifies that an added edge with 2 sources
// and 2 targets produces 4 arcs (2×2 Cartesian product).
func TestPrintDiffDOT_MultiSourceEdges(t *testing.T) {
	d := graph.GraphDiff{
		EdgesAdded: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-0000-4000-8000-000000000001",
				WhatChanged: "multi source action",
				Sources:     []string{"src-a", "src-b"},
				Targets:     []string{"tgt-x", "tgt-y"},
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintDiffDOT(&buf, d); err != nil {
		t.Fatalf("PrintDiffDOT: unexpected error: %v", err)
	}

	out := buf.String()
	arcCount := strings.Count(out, "->")
	if arcCount != 4 {
		t.Errorf("expected 4 arcs for 2×2 Cartesian product, got %d in:\n%s", arcCount, out)
	}
}

// TestPrintDiffDOT_TwoCutComment verifies that PrintDiffDOT emits two comment
// lines — one for the From cut and one for the To cut — at the top of the output.
func TestPrintDiffDOT_TwoCutComment(t *testing.T) {
	d := buildTestDiff(t)

	var buf bytes.Buffer
	if err := graph.PrintDiffDOT(&buf, d); err != nil {
		t.Fatalf("PrintDiffDOT: unexpected error: %v", err)
	}

	out := buf.String()
	commentCount := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "// ") {
			commentCount++
		}
	}
	if commentCount < 2 {
		t.Errorf("expected at least 2 comment lines (From and To), got %d in:\n%s", commentCount, out)
	}
	if !strings.Contains(out, "// From:") {
		t.Errorf("expected '// From:' comment line in DOT output, got:\n%s", out)
	}
	if !strings.Contains(out, "// To:") {
		t.Errorf("expected '// To:' comment line in DOT output, got:\n%s", out)
	}
}

// TestPrintDiffDOT_LongEdgeLabel verifies that WhatChanged strings longer than
// 40 runes are truncated with "..." in DOT edge labels.
func TestPrintDiffDOT_LongEdgeLabel(t *testing.T) {
	longLabel := strings.Repeat("x", 50)
	d := graph.GraphDiff{
		EdgesAdded: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-0000-4000-8000-000000000001",
				WhatChanged: longLabel,
				Sources:     []string{"a"},
				Targets:     []string{"b"},
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintDiffDOT(&buf, d); err != nil {
		t.Fatalf("PrintDiffDOT: unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "...") {
		t.Errorf("expected truncated label with '...' for 50-char label, got:\n%s", buf.String())
	}
}

// TestPrintDiffDOT_WriteError verifies that PrintDiffDOT propagates a write
// error from the underlying io.Writer back to the caller without swallowing it.
func TestPrintDiffDOT_WriteError(t *testing.T) {
	sentinel := errors.New("disk full")
	w := errWriter{err: sentinel}

	err := graph.PrintDiffDOT(w, graph.GraphDiff{})
	if err == nil {
		t.Fatal("PrintDiffDOT: expected error from failing writer, got nil")
	}
}

// TestPrintDiffDOT_NewlineInjection verifies that crafted node names containing
// newlines have the newlines stripped before reaching DOT output. A newline
// in a node name could split a DOT label line in two, with the second line
// becoming raw DOT syntax (e.g. an injected arc).
//
// The test confirms that the entire crafted name appears as a single DOT node
// declaration (no extra raw lines), i.e. there is exactly one declaration line
// for the node and no extra lines that start with a bare quote followed by
// a standalone DOT arc token.
func TestPrintDiffDOT_NewlineInjection(t *testing.T) {
	// The newline in the name, if not stripped, would produce a second DOT
	// line: "x" -> "y" [label="injected"] — a real injected arc.
	d := graph.GraphDiff{
		NodesAdded: []string{"before-newline\nafter-newline"},
	}

	var buf bytes.Buffer
	if err := graph.PrintDiffDOT(&buf, d); err != nil {
		t.Fatalf("PrintDiffDOT: %v", err)
	}

	out := buf.String()
	// The implementation strips newlines, so "before-newline" and
	// "after-newline" must appear on the SAME declaration line — not on
	// separate lines where "after-newline" would look like a standalone
	// (and potentially injected) DOT statement.
	foundAfter := false
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue // comment lines are safe
		}
		// If "after-newline" appears as the first token on a non-comment line
		// (i.e., not inside a label=... attribute), injection has occurred.
		if strings.HasPrefix(trimmed, `"after-newline"`) {
			t.Errorf("DOT output has 'after-newline' as a standalone line (newline not stripped): %q", line)
		}
		if strings.Contains(trimmed, "after-newline") {
			foundAfter = true
		}
	}
	// The name (with stripped newline) must still appear somewhere in the output.
	if !foundAfter {
		t.Errorf("DOT output missing 'after-newline' entirely — expected it within a label")
	}
}

// --- PrintDiffMermaid tests ---

// TestPrintDiffMermaid_Basic verifies that PrintDiffMermaid produces valid
// Mermaid output containing the flowchart header, added/removed/persisted
// node declarations, and style directives.
func TestPrintDiffMermaid_Basic(t *testing.T) {
	d := buildTestDiff(t)

	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "flowchart LR") {
		t.Errorf("expected 'flowchart LR' in Mermaid output, got:\n%s", out)
	}
	// Added node: evacuation-order with "(added)" label
	if !strings.Contains(out, "added") {
		t.Errorf("expected '(added)' label for added node in Mermaid output, got:\n%s", out)
	}
	// Removed node: old-model with "(removed)" label
	if !strings.Contains(out, "removed") {
		t.Errorf("expected '(removed)' label for removed node in Mermaid output, got:\n%s", out)
	}
	// Persisted node: storm-model-alpha with count label
	if !strings.Contains(out, "storm-model-alpha") {
		t.Errorf("expected persisted node name in Mermaid output, got:\n%s", out)
	}
}

// TestPrintDiffMermaid_EmptyDiff verifies that an empty GraphDiff produces
// a valid minimal Mermaid flowchart without error.
func TestPrintDiffMermaid_EmptyDiff(t *testing.T) {
	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, graph.GraphDiff{}); err != nil {
		t.Fatalf("PrintDiffMermaid on empty diff: unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "flowchart LR") {
		t.Errorf("expected 'flowchart LR' even for empty diff, got:\n%s", out)
	}
}

// TestPrintDiffMermaid_ShadowShifts verifies that shadow shifts produce a
// ShadowShifts subgraph block in Mermaid output with per-node style directives
// matching the DOT color convention: emerged=green, submerged=red, reason-changed=orange.
func TestPrintDiffMermaid_ShadowShifts(t *testing.T) {
	d := buildDiffWithAllKinds(t)

	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "subgraph ShadowShifts") {
		t.Errorf("expected 'subgraph ShadowShifts' in Mermaid output, got:\n%s", out)
	}
	if !strings.Contains(out, "end") {
		t.Errorf("expected 'end' closing the ShadowShifts subgraph, got:\n%s", out)
	}
	if !strings.Contains(out, "evacuation-shelter-b") {
		t.Errorf("expected 'evacuation-shelter-b' in ShadowShifts subgraph, got:\n%s", out)
	}
	// Style directives: emerged=green, submerged=red, reason-changed=orange.
	if !strings.Contains(out, "stroke:green") {
		t.Errorf("expected 'stroke:green' style for emerged shift, got:\n%s", out)
	}
	if !strings.Contains(out, "stroke:red") {
		t.Errorf("expected 'stroke:red' style for submerged shift, got:\n%s", out)
	}
	if !strings.Contains(out, "stroke:orange") {
		t.Errorf("expected 'stroke:orange' style for reason-changed shift, got:\n%s", out)
	}
}

// TestPrintDiffMermaid_NoShadowShifts verifies that the ShadowShifts subgraph
// is NOT emitted when the diff has no shadow shifts.
func TestPrintDiffMermaid_NoShadowShifts(t *testing.T) {
	d := buildTestDiff(t)
	d.ShadowShifts = nil

	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "subgraph ShadowShifts") {
		t.Errorf("expected NO 'subgraph ShadowShifts' for diff with no shadow shifts, got:\n%s", out)
	}
}

// TestPrintDiffMermaid_NodeIDSanitization verifies that node names with hyphens
// are sanitized to underscores for Mermaid IDs, while the original names are
// preserved as display labels.
func TestPrintDiffMermaid_NodeIDSanitization(t *testing.T) {
	d := graph.GraphDiff{
		NodesAdded: []string{"storm-sensor-network"},
	}

	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	// Sanitized ID should appear (hyphens → underscores).
	if !strings.Contains(out, "storm_sensor_network") {
		t.Errorf("expected sanitized ID 'storm_sensor_network' in Mermaid output, got:\n%s", out)
	}
	// Original name should appear as the label.
	if !strings.Contains(out, "storm-sensor-network") {
		t.Errorf("expected original label 'storm-sensor-network' in Mermaid output, got:\n%s", out)
	}
}

// TestPrintDiffMermaid_AddedEdgeArrow verifies that added edges use the '-->'
// solid arrow syntax in Mermaid output.
func TestPrintDiffMermaid_AddedEdgeArrow(t *testing.T) {
	d := graph.GraphDiff{
		EdgesAdded: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-0000-4000-8000-000000000001",
				WhatChanged: "new connection",
				Sources:     []string{"node-a"},
				Targets:     []string{"node-b"},
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "-->") {
		t.Errorf("expected '-->' arrow for added edge in Mermaid output, got:\n%s", out)
	}
}

// TestPrintDiffMermaid_RemovedEdgeArrow verifies that removed edges use the
// '-.->'' dashed arrow syntax in Mermaid output.
func TestPrintDiffMermaid_RemovedEdgeArrow(t *testing.T) {
	d := graph.GraphDiff{
		EdgesRemoved: []graph.Edge{
			{
				TraceID:     "aaaaaaaa-0000-4000-8000-000000000001",
				WhatChanged: "old connection",
				Sources:     []string{"node-a"},
				Targets:     []string{"node-b"},
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "-.->") {
		t.Errorf("expected '-.->'' dashed arrow for removed edge in Mermaid output, got:\n%s", out)
	}
}

// TestPrintDiffMermaid_StyleDirectives verifies that added nodes receive a
// green stroke style directive and removed nodes receive a red dashed style.
func TestPrintDiffMermaid_StyleDirectives(t *testing.T) {
	d := buildTestDiff(t)

	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "stroke:green") {
		t.Errorf("expected 'stroke:green' style for added node, got:\n%s", out)
	}
	if !strings.Contains(out, "stroke:red") {
		t.Errorf("expected 'stroke:red' style for removed node, got:\n%s", out)
	}
}

// TestPrintDiffMermaid_WriteError verifies that PrintDiffMermaid propagates
// a write error from the underlying io.Writer back to the caller.
func TestPrintDiffMermaid_WriteError(t *testing.T) {
	sentinel := errors.New("disk full")
	w := errWriter{err: sentinel}

	err := graph.PrintDiffMermaid(w, graph.GraphDiff{})
	if err == nil {
		t.Fatal("PrintDiffMermaid: expected error from failing writer, got nil")
	}
}

// TestPrintDiffMermaid_NewlineInjection verifies that crafted node names
// containing newlines do not inject Mermaid click directives or other
// directives that could cause XSS in a browser renderer.
func TestPrintDiffMermaid_NewlineInjection(t *testing.T) {
	d := graph.GraphDiff{
		NodesAdded: []string{"legit\nclick legit \"javascript:alert(1)\""},
	}

	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: %v", err)
	}

	out := buf.String()
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "click ") {
			t.Errorf("Mermaid output contains a standalone 'click' directive: %q", line)
		}
	}
}

// TestPrintDiffMermaid_ShadowShiftKindInjection verifies that a crafted
// ShadowShiftKind value cannot break out of a Mermaid label and inject
// a click directive (XSS vector in browser-based renderers).
func TestPrintDiffMermaid_ShadowShiftKindInjection(t *testing.T) {
	d := graph.GraphDiff{
		ShadowShifts: []graph.ShadowShift{
			{
				Name: "sensor",
				Kind: graph.ShadowShiftKind("emerged\"\nclick sensor_id \"javascript:alert(1)\""),
			},
		},
	}
	var buf bytes.Buffer
	if err := graph.PrintDiffMermaid(&buf, d); err != nil {
		t.Fatalf("PrintDiffMermaid: %v", err)
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "click ") {
			t.Errorf("Mermaid output contains injected click directive: %q", line)
		}
	}
}
