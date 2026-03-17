// shadow.go provides shadow analysis operations — interrogating what the cut
// excludes as a first-class analytical object.
//
// A shadow is not missing data. It is the structured record of what this
// articulation cannot see: elements present in the full dataset that fall
// outside the chosen cut. SummariseShadow makes the shadow legible without
// treating it as a deficiency.
//
// The operations here read data already present on MeshGraph.Cut — no new
// analysis is needed. SummariseShadow reorganises that data into a form
// oriented toward the shadow rather than toward what is visible.
package graph

import (
	"fmt"
	"io"
	"sort"
)

// ShadowSummary is a provenance-aware reading of the shadow of a MeshGraph.
// It reorganises Cut.ShadowElements for shadow-first analysis: how many
// elements are excluded, for what reasons, and from which observer positions
// the shadow would lift.
type ShadowSummary struct {
	// TotalShadowed is the number of elements in the shadow.
	TotalShadowed int

	// ByReason maps each ShadowReason string to the count of shadow elements
	// that carry that reason. An element that was excluded for multiple reasons
	// is counted once per reason.
	ByReason map[string]int

	// Elements is the full list of shadow elements, in the same order as
	// Cut.ShadowElements (alphabetical by name).
	Elements []ShadowElement

	// SeenFromCounts maps observer strings to the number of shadow elements
	// for which that observer appears in SeenFrom. This answers: "from which
	// position would the most shadow lift?" Observers with high counts are the
	// positions that see the most of what this cut excludes.
	SeenFromCounts map[string]int

	// Cut is the articulation parameters that produced this shadow. Retained
	// so the summary is self-situated — a shadow report without its cut is
	// uninterpretable.
	Cut Cut
}

// SummariseShadow reads the shadow of g and returns a ShadowSummary.
// It does not re-articulate — it operates on data already present in g.Cut.
// The returned summary is immutable; slices are copied from the MeshGraph.
func SummariseShadow(g MeshGraph) ShadowSummary {
	s := ShadowSummary{
		TotalShadowed:  len(g.Cut.ShadowElements),
		ByReason:       make(map[string]int),
		SeenFromCounts: make(map[string]int),
		Cut:            g.Cut,
	}

	// Copy elements to decouple the summary from the source graph.
	s.Elements = make([]ShadowElement, len(g.Cut.ShadowElements))
	copy(s.Elements, g.Cut.ShadowElements)

	for _, elem := range g.Cut.ShadowElements {
		// Count by each reason (an element with multiple reasons is counted
		// once per reason — consistent with ANT: multiple causes can coexist).
		for _, r := range elem.Reasons {
			s.ByReason[string(r)]++
		}
		// Count how often each observer appears in SeenFrom.
		for _, obs := range elem.SeenFrom {
			s.SeenFromCounts[obs]++
		}
	}

	return s
}

// PrintShadowSummary writes a shadow analysis report to w.
// The report is oriented toward what the cut excludes: how many elements,
// why they were excluded, and which positions see what this cut cannot.
//
// Returns the first write error encountered, if any.
func PrintShadowSummary(w io.Writer, s ShadowSummary) error {
	lines := []string{
		"=== Shadow Summary ===",
		"",
	}

	// Cut context.
	if len(s.Cut.ObserverPositions) > 0 {
		lines = append(lines, fmt.Sprintf("Observer:    %s", joinStrings(s.Cut.ObserverPositions, ", ")))
	} else {
		lines = append(lines, "Observer:    (full cut — all positions)")
	}
	lines = append(lines,
		fmt.Sprintf("Traces:      %d included of %d total", s.Cut.TracesIncluded, s.Cut.TracesTotal),
		"",
		fmt.Sprintf("Shadow elements: %d", s.TotalShadowed),
	)

	if s.TotalShadowed == 0 {
		lines = append(lines,
			"",
			"No shadow — this cut includes all elements in the dataset.",
			"",
			"---",
			"Note: shadow is a cut decision, not missing data.",
		)
		return writeLines(w, lines)
	}

	// Breakdown by reason — iterate over all keys in ByReason so any future
	// ShadowReason values appear automatically without a code change here.
	lines = append(lines, "", "By exclusion reason:")
	reasonKeys := make([]string, 0, len(s.ByReason))
	for r := range s.ByReason {
		reasonKeys = append(reasonKeys, r)
	}
	sort.Strings(reasonKeys)
	for _, r := range reasonKeys {
		lines = append(lines, fmt.Sprintf("  %-20s %d", r, s.ByReason[r]))
	}

	// Observer coverage of the shadow: who sees what this cut cannot.
	if len(s.SeenFromCounts) > 0 {
		lines = append(lines, "", "Shadow visible from (observer positions that un-shadow elements):")
		// Sort observers by descending count, then alphabetically for ties.
		type obsCount struct {
			obs   string
			count int
		}
		ordered := make([]obsCount, 0, len(s.SeenFromCounts))
		for obs, n := range s.SeenFromCounts {
			ordered = append(ordered, obsCount{obs, n})
		}
		sort.Slice(ordered, func(i, j int) bool {
			if ordered[i].count != ordered[j].count {
				return ordered[i].count > ordered[j].count
			}
			return ordered[i].obs < ordered[j].obs
		})
		for _, oc := range ordered {
			lines = append(lines, fmt.Sprintf("  %-30s %d element(s)", oc.obs, oc.count))
		}
	}

	// List all shadow elements with their reasons.
	lines = append(lines, "", "Shadow elements:")
	for _, elem := range s.Elements {
		reasonStrs := make([]string, len(elem.Reasons))
		for i, r := range elem.Reasons {
			reasonStrs[i] = string(r)
		}
		lines = append(lines, fmt.Sprintf("  %-30s [%s]", elem.Name, joinStrings(reasonStrs, ", ")))
	}

	lines = append(lines,
		"",
		"---",
		"Note: shadow is a cut decision, not missing data.",
		"Each excluded element names what this position cannot see from where it stands.",
	)

	return writeLines(w, lines)
}

// writeLines writes each line followed by a newline to w.
func writeLines(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("graph: PrintShadowSummary: %w", err)
		}
	}
	return nil
}

// joinStrings concatenates ss with sep. Returns "(none)" for empty slices.
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
