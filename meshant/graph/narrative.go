// narrative.go provides DraftNarrative and PrintNarrativeDraft for producing a
// provisional, positioned narrative reading of a MeshGraph.
//
// A narrative draft is not a conclusion. It is a positioned reading: one
// articulation, from one cut, of a partial mesh. The language throughout is
// deliberately provisional ("appearing most frequently", "from this position",
// "in this cut") — not authoritative. The shadow is always named.
//
// Design note: DraftNarrative does not embed meaning. It organises what the
// graph already says — trace counts, mediations, shadow elements — into a
// readable form that names its own limits. The caveats are mandatory: every
// narrative draft is a cut, not truth.
//
// ANT constraint (critical): never use "missing" — elements are "in shadow
// from this position", not absent. Absence implies non-existence; shadow names
// what is invisible from one vantage point, which is a very different claim.
package graph

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// NarrativeDraft is a provisional, positioned narrative reading of a MeshGraph.
//
// It is produced by DraftNarrative and rendered by PrintNarrativeDraft.
// All fields are populated only when the source graph has at least one edge —
// DraftNarrative returns a zero-value NarrativeDraft for empty graphs.
//
// NarrativeDraft is immutable once returned — callers may inspect but should
// not mutate its Caveats slice. Mutation does not affect the source graph.
type NarrativeDraft struct {
	// PositionStatement names the cut from which this reading was taken.
	// Format: "This reading is taken from the position: <cutLabel>".
	PositionStatement string

	// Body is the main prose paragraph. It names the trace count, the top-3
	// elements by appearance, and the observed mediations. All language is
	// provisional: "appearing most frequently", not "most important".
	Body string

	// ShadowStatement names what this cut cannot see. Never uses "missing".
	// If no elements are in shadow, states that this is a full cut.
	// If shadow > 0, names the count and the distinct exclusion reasons.
	ShadowStatement string

	// Caveats is a list of methodological cautions about this reading.
	// Always non-empty for a non-empty graph. The first caveat is always the
	// standard positioned-reading caveat. Additional caveats are appended
	// based on shadow ratio, time window, and tag filters.
	Caveats []string
}

// DraftNarrative produces a provisional narrative reading of g.
//
// Returns a zero-value NarrativeDraft if len(g.Edges) == 0. For non-empty
// graphs, all four fields are populated. The input graph is not mutated.
//
// Sorting for top-elements: descending by AppearanceCount, then alphabetically.
// Top-3 elements are selected; fewer than 3 are used when the graph has fewer.
//
// Mediations: up to 5 distinct non-empty Edge.Mediation strings are listed;
// if more than 5 exist, "and N more" is appended.
//
// Caveats are always at least one (standard positioned-reading caveat).
// Additional caveats fire on: shadow > 50% of TracesTotal, non-zero
// TimeWindow, and non-empty Tags filter.
func DraftNarrative(g MeshGraph) NarrativeDraft {
	// Zero-value return for empty graphs — no data to narrate.
	if len(g.Edges) == 0 {
		return NarrativeDraft{}
	}

	return NarrativeDraft{
		PositionStatement: buildPositionStatement(g.Cut),
		Body:              buildBody(g),
		ShadowStatement:   buildShadowStatement(g.Cut),
		Caveats:           buildCaveats(g.Cut),
	}
}

// buildPositionStatement constructs the PositionStatement from the cut.
// Uses the unexported cutLabel helper from reflexive.go — same package.
func buildPositionStatement(c Cut) string {
	return "This reading is taken from the position: " + cutLabel(c)
}

// buildBody constructs the main prose paragraph for the NarrativeDraft.
//
// It reads node counts and edge mediations from the graph without mutating it:
//   - sorts a copy of the node entries (never modifies g.Nodes)
//   - collects distinct mediations from edge slice (read-only)
//
// Language is provisional throughout: "appearing most frequently", not "most important".
func buildBody(g MeshGraph) string {
	// Sort a copy of node entries by descending AppearanceCount, then alpha.
	// This copy guarantees g.Nodes is not mutated — we never re-insert.
	type nodeEntry struct {
		name  string
		count int
	}
	entries := make([]nodeEntry, 0, len(g.Nodes))
	for name, node := range g.Nodes {
		entries = append(entries, nodeEntry{name, node.AppearanceCount})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].name < entries[j].name
	})

	// Take top-3 (or fewer).
	top := entries
	if len(top) > 3 {
		top = top[:3]
	}

	// Collect distinct non-empty mediations, preserving encounter order, up to 5.
	seen := make(map[string]bool)
	var mediations []string
	for _, e := range g.Edges {
		if e.Mediation != "" && !seen[e.Mediation] {
			seen[e.Mediation] = true
			mediations = append(mediations, e.Mediation)
		}
	}
	totalMediations := len(mediations)
	if len(mediations) > 5 {
		mediations = mediations[:5]
	}

	// Build element phrase: "X (N times), Y (M times)..."
	var elemPhrases []string
	for _, ne := range top {
		unit := "times"
		if ne.count == 1 {
			unit = "time"
		}
		elemPhrases = append(elemPhrases, fmt.Sprintf("%s (%d %s)", ne.name, ne.count, unit))
	}

	// Build mediation phrase.
	var mediationClause string
	if len(mediations) > 0 {
		mediationPhrase := strings.Join(mediations, ", ")
		if totalMediations > 5 {
			mediationPhrase += fmt.Sprintf(", and %d more", totalMediations-5)
		}
		mediationClause = fmt.Sprintf(" Mediations observed include: %s.", mediationPhrase)
	}

	return fmt.Sprintf(
		"From this position, %d traces are included. The elements appearing most frequently are %s.%s",
		g.Cut.TracesIncluded,
		strings.Join(elemPhrases, ", "),
		mediationClause,
	)
}

// buildShadowStatement constructs the ShadowStatement from the cut.
//
// Zero shadow: states this is a full cut (not "no missing elements" — that
// would imply the others are missing, which is the wrong framing).
// Non-zero shadow: names the count, uses "in shadow from this position", and
// lists distinct ShadowReason strings from all shadow elements.
//
// CRITICAL: never uses the word "missing".
func buildShadowStatement(c Cut) string {
	shadowCount := len(c.ShadowElements)

	if shadowCount == 0 {
		return "No elements are in shadow from this position — this is a full cut."
	}

	// Collect distinct ShadowReason strings across all shadow elements.
	seen := make(map[string]bool)
	var reasons []string
	for _, se := range c.ShadowElements {
		for _, r := range se.Reasons {
			rs := string(r)
			if !seen[rs] {
				seen[rs] = true
				reasons = append(reasons, rs)
			}
		}
	}
	sort.Strings(reasons)

	reasonPhrase := strings.Join(reasons, ", ")
	return fmt.Sprintf(
		"%d elements are in shadow from this position. Exclusion reasons: %s.",
		shadowCount,
		reasonPhrase,
	)
}

// buildCaveats constructs the Caveats slice for a non-empty graph.
//
// The standard positioned-reading caveat is always first. Conditional caveats
// are appended based on shadow ratio, time window, and tag filters.
func buildCaveats(c Cut) []string {
	// Standard caveat — always present.
	caveats := []string{
		"This draft is a positioned reading, not a complete account. A different cut would produce a different narrative.",
	}

	// Shadow ratio caveat: shadow > 50% of TracesTotal.
	shadowCount := len(c.ShadowElements)
	if shadowCount > 0 && c.TracesTotal > 0 && 2*shadowCount > c.TracesTotal {
		caveats = append(caveats,
			"A large portion of the dataset is in shadow from this position. The reading is shaped by what this cut excludes.",
		)
	}

	// Time window caveat.
	if !c.TimeWindow.IsZero() {
		caveats = append(caveats,
			"This reading is bounded by a time window. Traces outside that window are not considered.",
		)
	}

	// Tag filter caveat.
	if len(c.Tags) > 0 {
		caveats = append(caveats,
			"This reading is filtered by tags. Traces without matching tags are excluded.",
		)
	}

	return caveats
}

// PrintNarrativeDraft writes a formatted narrative draft to w.
//
// Output sections:
//   - Header: "=== Narrative Draft (provisional) ==="
//   - Position:  PositionStatement
//   - Reading:   Body
//   - Shadow:    ShadowStatement
//   - Caveats:   one bullet per caveat
//   - Footer note encoding the provisional, positioned nature of this draft
//
// Returns the first write error as "graph: PrintNarrativeDraft: %w".
// Uses fmt.Fprintln for each line — each call returns the first error encountered.
func PrintNarrativeDraft(w io.Writer, n NarrativeDraft) error {
	// Helper to write a single line and capture first error.
	write := func(line string) error {
		_, err := fmt.Fprintln(w, line)
		return err
	}

	lines := []string{
		"=== Narrative Draft (provisional) ===",
		"",
		"Position:",
		"  " + n.PositionStatement,
		"",
		"Reading:",
		"  " + n.Body,
		"",
		"Shadow:",
		"  " + n.ShadowStatement,
		"",
		"Caveats:",
	}

	for _, line := range lines {
		if err := write(line); err != nil {
			return fmt.Errorf("graph: PrintNarrativeDraft: %w", err)
		}
	}

	for _, caveat := range n.Caveats {
		if err := write("  - " + caveat); err != nil {
			return fmt.Errorf("graph: PrintNarrativeDraft: %w", err)
		}
	}

	footer := []string{
		"",
		"---",
		"Note: this is a draft, not a conclusion. It is written from one position in the mesh.",
	}
	for _, line := range footer {
		if err := write(line); err != nil {
			return fmt.Errorf("graph: PrintNarrativeDraft: %w", err)
		}
	}

	return nil
}
