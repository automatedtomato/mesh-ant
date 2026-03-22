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
// All fields are populated only when the source graph has at least one edge.
type NarrativeDraft struct {
	// PositionStatement names the cut from which this reading was taken.
	PositionStatement string

	// Body is the main prose paragraph: trace count, top-3 elements, mediations.
	// Language is provisional throughout.
	Body string

	// ShadowStatement names what this cut cannot see. Never uses "missing".
	ShadowStatement string

	// Caveats is a list of methodological cautions. Always non-empty for non-empty graphs.
	Caveats []string
}

// DraftNarrative produces a provisional narrative reading of g. Returns a
// zero-value NarrativeDraft for empty graphs. Does not mutate g.
func DraftNarrative(g MeshGraph) NarrativeDraft {
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
func buildPositionStatement(c Cut) string {
	return "This reading is taken from the position: " + cutLabel(c)
}

// buildBody constructs the main prose paragraph for the NarrativeDraft.
// Language is provisional: "appearing most frequently", not "most important".
func buildBody(g MeshGraph) string {
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

	top := entries
	if len(top) > 3 {
		top = top[:3]
	}

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

	var elemPhrases []string
	for _, ne := range top {
		unit := "times"
		if ne.count == 1 {
			unit = "time"
		}
		elemPhrases = append(elemPhrases, fmt.Sprintf("%s (%d %s)", ne.name, ne.count, unit))
	}

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
// CRITICAL: never uses the word "missing" — elements are "in shadow from this position".
func buildShadowStatement(c Cut) string {
	shadowCount := len(c.ShadowElements)

	if shadowCount == 0 {
		return "No elements are in shadow from this position — this is a full cut."
	}

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
// Standard caveat is always first; shadow-ratio, time-window, and tag caveats are conditional.
func buildCaveats(c Cut) []string {
	caveats := []string{
		"This draft is a positioned reading, not a complete account. A different cut would produce a different narrative.",
	}

	shadowCount := len(c.ShadowElements)
	if shadowCount > 0 && c.TracesTotal > 0 && 2*shadowCount > c.TracesTotal {
		caveats = append(caveats,
			"A large portion of the dataset is in shadow from this position. The reading is shaped by what this cut excludes.",
		)
	}

	if !c.TimeWindow.IsZero() {
		caveats = append(caveats,
			"This reading is bounded by a time window. Traces outside that window are not considered.",
		)
	}

	if len(c.Tags) > 0 {
		caveats = append(caveats,
			"This reading is filtered by tags. Traces without matching tags are excluded.",
		)
	}

	return caveats
}

// PrintNarrativeDraft writes a formatted narrative draft to w.
// Returns the first write error as "graph: PrintNarrativeDraft: %w".
func PrintNarrativeDraft(w io.Writer, n NarrativeDraft) error {
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
