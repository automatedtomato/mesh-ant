// bottleneck.go provides provisional centrality analysis over a MeshGraph.
//
// IdentifyBottlenecks reads the already-articulated graph and applies a
// heuristic to surface elements that appear central from this cut. The
// three measures (AppearanceCount, MediationCount, ShadowCount) are
// independent — they are reported separately, never combined into a
// composite score. Any aggregation would imply a god's-eye reading that
// this framework explicitly refuses.
//
// A BottleneckNote is provisional: it is a reading from one cut. A different
// observer position, time window, or tag filter would produce different notes.
// This is not a deficiency of the analysis — it is the correct methodological
// stance.
//
// PrintBottleneckNotes follows the PrintShadowSummary output convention
// (header, cut context, per-element lines, footer caveat). It does NOT use
// writeLines to avoid inheriting the hardcoded "PrintShadowSummary" in that
// helper's error wrap.
package graph

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// BottleneckOptions configures IdentifyBottlenecks.
// v1: intentionally empty — reserved as an extension point for future
// thresholds, weight overrides, or heuristic toggles without a breaking
// API change.
type BottleneckOptions struct{}

// BottleneckNote is a provisional reading of one element's centrality from
// a cut. The three counts are independent measures derived from different
// aspects of the graph — combining them into a single score would obscure
// what each dimension is saying and imply a false precision.
type BottleneckNote struct {
	// Element is the element name as it appears in the graph Nodes.
	Element string

	// AppearanceCount mirrors Node.AppearanceCount: total source+target
	// appearances across all included traces.
	AppearanceCount int

	// MediationCount is the number of edges (included traces) in which
	// this element appears as Edge.Mediation. A mediator is an element
	// that transforms what passes through it — not a neutral conduit.
	MediationCount int

	// ShadowCount mirrors Node.ShadowCount: the number of excluded traces
	// in which this element also appears. A non-zero ShadowCount means
	// this element crosses the cut boundary — visible from here AND present
	// in traces the cut cannot see.
	ShadowCount int

	// Reason is a provisional, human-readable explanation of why this
	// element was flagged. Always non-empty. Must contain "appears" or
	// "from this cut" to signal provisionality.
	Reason string
}

// IdentifyBottlenecks applies the v1 centrality heuristic to g and returns
// a sorted slice of BottleneckNote values for elements that meet the threshold.
//
// Inclusion heuristic (v1):
//
//	include if mediationCount > 0 || node.AppearanceCount >= 2 || node.ShadowCount > 0
//
// Elements with AC=1, MC=0, SC=0 are excluded — a single appearance with no
// mediation role and no cross-cut presence is not a signal of centrality from
// this position.
//
// Sort order: MediationCount descending → AppearanceCount descending → name ascending.
//
// Always returns a non-nil slice (empty slice when no nodes qualify, never nil).
// The returned slice is independent of g — callers may mutate it safely.
func IdentifyBottlenecks(g MeshGraph, _ BottleneckOptions) []BottleneckNote {
	// Guard: empty graph — return non-nil empty slice, never nil.
	if len(g.Nodes) == 0 {
		return []BottleneckNote{}
	}

	// Build a mediation count map: count edges where each element is the mediator.
	// Iterates g.Edges once, independent of g.Nodes, so elements that appear only
	// as mediators (not in source/target) are not in Nodes and won't be surfaced.
	// That is intentional — we only report Nodes (visible elements in this cut).
	mediationCounts := make(map[string]int)
	for _, e := range g.Edges {
		if e.Mediation != "" {
			mediationCounts[e.Mediation]++
		}
	}

	// Apply inclusion heuristic to each node.
	var notes []BottleneckNote
	for name, node := range g.Nodes {
		mc := mediationCounts[name]

		// v1 heuristic: include if any of the three measures signals centrality.
		if mc == 0 && node.AppearanceCount < 2 && node.ShadowCount == 0 {
			continue // exclude — no signal from any dimension
		}

		reason := buildBottleneckReason(node.AppearanceCount, mc, node.ShadowCount)

		notes = append(notes, BottleneckNote{
			Element:         name,
			AppearanceCount: node.AppearanceCount,
			MediationCount:  mc,
			ShadowCount:     node.ShadowCount,
			Reason:          reason,
		})
	}

	// Sort: MediationCount desc → AppearanceCount desc → name asc.
	// Stable deterministic ordering avoids implying significance in position
	// beyond what the three measures express.
	sort.SliceStable(notes, func(i, j int) bool {
		if notes[i].MediationCount != notes[j].MediationCount {
			return notes[i].MediationCount > notes[j].MediationCount
		}
		if notes[i].AppearanceCount != notes[j].AppearanceCount {
			return notes[i].AppearanceCount > notes[j].AppearanceCount
		}
		return notes[i].Element < notes[j].Element
	})

	// Return non-nil empty slice if no elements qualified — callers can use
	// len(result) == 0 safely without a nil check.
	if len(notes) == 0 {
		return []BottleneckNote{}
	}
	return notes
}

// buildBottleneckReason constructs the provisional Reason string for a note.
// The language is intentionally hedged ("appears central from this cut") to
// prevent the reading from being mistaken for a definitive claim.
// The returned string always contains both "appears" and "from this cut".
func buildBottleneckReason(ac, mc, sc int) string {
	var parts []string
	if mc > 0 {
		parts = append(parts, fmt.Sprintf("mediation count %d", mc))
	}
	if ac >= 2 {
		parts = append(parts, fmt.Sprintf("appearance count %d", ac))
	}
	if sc > 0 {
		parts = append(parts, fmt.Sprintf("shadow count %d", sc))
	}
	// Fallback: if somehow no part was added (shouldn't happen given the
	// caller's inclusion check) produce a safe default that still satisfies
	// the provisional-language contract.
	if len(parts) == 0 {
		return "appears central from this cut"
	}
	return "appears central from this cut: " + strings.Join(parts, ", ")
}

// PrintBottleneckNotes writes a bottleneck analysis report to w.
//
// Output format:
//   - Header: "=== Bottleneck Notes (provisional reading from this cut) ==="
//   - Cut context: observer position(s) and trace counts
//   - Per-note lines: element name, AppearanceCount, MediationCount,
//     ShadowCount, Reason
//   - Footer caveat naming the provisionality of the cut
//
// Uses fmt.Fprintln directly (not writeLines) to avoid the hardcoded
// "PrintShadowSummary" in writeLines's error wrap string.
//
// Returns the first write error encountered, if any.
func PrintBottleneckNotes(w io.Writer, g MeshGraph, notes []BottleneckNote) error {
	// writeLine is a local helper that wraps one fmt.Fprintln call and
	// returns early on error. Using a closure avoids repeating the error
	// wrap pattern on every line.
	var writeErr error
	writeLine := func(line string) {
		if writeErr != nil {
			return // already failed; skip remaining writes
		}
		_, writeErr = fmt.Fprintln(w, line)
	}

	writeLine("=== Bottleneck Notes (provisional reading from this cut) ===")
	writeLine("")

	// Cut context — observer positions using the same cutLabel helper as
	// reflexive.go uses in diffWhatChanged.
	writeLine(fmt.Sprintf("Cut position: %s", cutLabel(g.Cut)))
	writeLine(fmt.Sprintf("Traces: %d included of %d total",
		g.Cut.TracesIncluded, g.Cut.TracesTotal))
	writeLine("")

	if len(notes) == 0 {
		writeLine("No elements meet the centrality threshold from this cut.")
	} else {
		for _, note := range notes {
			writeLine(fmt.Sprintf("Element:          %s", note.Element))
			writeLine(fmt.Sprintf("  AppearanceCount: %d", note.AppearanceCount))
			writeLine(fmt.Sprintf("  MediationCount:  %d", note.MediationCount))
			writeLine(fmt.Sprintf("  ShadowCount:     %d", note.ShadowCount))
			writeLine(fmt.Sprintf("  Reason:          %s", note.Reason))
			writeLine("")
		}
	}

	writeLine("---")
	writeLine("Note: these readings are from one cut. A different position would produce different notes.")

	return writeErr
}
