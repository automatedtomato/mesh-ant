// gaps.go provides observer-gap analysis — comparing what two articulations
// can and cannot see, without running a full diff.
//
// AnalyseGaps takes two already-articulated MeshGraphs and compares their
// node sets. This is lighter than Diff: it focuses on element visibility
// asymmetry rather than full structural comparison. The analyst's question
// is: "what does A see that B does not, and vice versa?" — not "how did
// the graph change?"
//
// Each graph carries its Cut, so ObserverGap is self-situated: it names
// both positions being compared and names the asymmetry without treating
// either as authoritative.
package graph

import (
	"fmt"
	"io"
	"sort"
)

// ObserverGap records the visibility asymmetry between two articulations.
// It does not say which is more correct — it says what each can see that
// the other cannot, and what both can see.
type ObserverGap struct {
	// OnlyInA lists elements visible in graph A but not in graph B.
	// Sorted alphabetically.
	OnlyInA []string

	// OnlyInB lists elements visible in graph B but not in graph A.
	// Sorted alphabetically.
	OnlyInB []string

	// InBoth lists elements visible in both graphs.
	// Sorted alphabetically.
	InBoth []string

	// CutA is the articulation parameters of graph A. Retained so the gap
	// report is self-situated — a comparison without its positions is
	// uninterpretable.
	CutA Cut

	// CutB is the articulation parameters of graph B.
	CutB Cut
}

// AnalyseGaps compares the node sets of two already-articulated MeshGraphs.
// Does not re-articulate. Returns an immutable ObserverGap.
func AnalyseGaps(g1, g2 MeshGraph) ObserverGap {
	gap := ObserverGap{
		CutA: g1.Cut,
		CutB: g2.Cut,
	}

	inA := make(map[string]bool, len(g1.Nodes))
	for name := range g1.Nodes {
		inA[name] = true
	}
	inB := make(map[string]bool, len(g2.Nodes))
	for name := range g2.Nodes {
		inB[name] = true
	}

	for name := range inA {
		if inB[name] {
			gap.InBoth = append(gap.InBoth, name)
		} else {
			gap.OnlyInA = append(gap.OnlyInA, name)
		}
	}
	for name := range inB {
		if !inA[name] {
			gap.OnlyInB = append(gap.OnlyInB, name)
		}
	}

	sort.Strings(gap.OnlyInA)
	sort.Strings(gap.OnlyInB)
	sort.Strings(gap.InBoth)

	return gap
}

// PrintObserverGap writes an observer-gap report to w.
// The report shows both cut positions and the three-way element partition:
// only in A, only in B, and in both. Neither position is treated as primary.
//
// Returns the first write error encountered, if any.
func PrintObserverGap(w io.Writer, gap ObserverGap) error {
	labelA := cutLabel(gap.CutA)
	labelB := cutLabel(gap.CutB)

	lines := []string{
		"=== Observer Gap ===",
		"",
		fmt.Sprintf("Position A: %s", labelA),
		fmt.Sprintf("Position B: %s", labelB),
		"",
		fmt.Sprintf("Only in A: %d  |  Only in B: %d  |  In both: %d",
			len(gap.OnlyInA), len(gap.OnlyInB), len(gap.InBoth)),
	}

	if len(gap.OnlyInA) > 0 {
		lines = append(lines, "", fmt.Sprintf("Elements only visible from A (%s):", labelA))
		for _, name := range gap.OnlyInA {
			lines = append(lines, "  "+name)
		}
	}

	if len(gap.OnlyInB) > 0 {
		lines = append(lines, "", fmt.Sprintf("Elements only visible from B (%s):", labelB))
		for _, name := range gap.OnlyInB {
			lines = append(lines, "  "+name)
		}
	}

	if len(gap.InBoth) > 0 {
		lines = append(lines, "", "Elements visible from both:")
		for _, name := range gap.InBoth {
			lines = append(lines, "  "+name)
		}
	}

	if len(gap.OnlyInA) == 0 && len(gap.OnlyInB) == 0 {
		lines = append(lines, "", "No gap — both positions see the same elements.")
	}

	lines = append(lines,
		"",
		"---",
		"Note: neither position is authoritative. Each sees from where it stands.",
	)

	return writeLines(w, lines)
}
