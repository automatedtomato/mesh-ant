// classdiff.go provides classification-diff analysis — comparing how two analyst
// positions classified the same derivation chain, without treating either
// position as authoritative.
//
// CompareChainClassifications takes two slices of DraftStepClassification (one
// per analyst position) and produces a []ClassificationDiff: one entry for each
// step where the two positions assigned a different Kind.
//
// PrintClassificationDiffs renders that diff to any io.Writer in a human-readable
// format that names both positions using "Position A / Position B" framing and
// notes any length asymmetry between the two chains.
//
// Neither position is treated as primary. The divergence is the data — not a verdict.
package loader

import (
	"fmt"
	"io"
)

// ClassificationDiff records that two analyst positions classified the same
// derivation step differently. Neither classification is authoritative.
type ClassificationDiff struct {
	// StepIndex is the step where the positions diverge (1-based).
	StepIndex int

	// KindA is analyst A's classification for this step.
	KindA DraftStepKind

	// KindB is analyst B's classification for this step.
	KindB DraftStepKind

	// ReasonA is analyst A's justification.
	ReasonA string

	// ReasonB is analyst B's justification.
	ReasonB string
}

// CompareChainClassifications returns diffs for steps where Kind differs.
// Comparison by position up to min(len(chainA), len(chainB)).
// Returns non-nil empty slice when all comparable steps agree.
func CompareChainClassifications(chainA, chainB []DraftStepClassification) []ClassificationDiff {
	result := []ClassificationDiff{}

	limit := len(chainA)
	if len(chainB) < limit {
		limit = len(chainB)
	}

	for i := 0; i < limit; i++ {
		a := chainA[i]
		b := chainB[i]

		// StepIndex uses loop counter (i+1) for unambiguous positional depth,
		// independent of raw StepIndex values in the input chains.
		if a.Kind != b.Kind {
			result = append(result, ClassificationDiff{
				StepIndex: i + 1,
				KindA:     a.Kind,
				KindB:     b.Kind,
				ReasonA:   a.Reason,
				ReasonB:   b.Reason,
			})
		}
	}

	return result
}

// PrintClassificationDiffs writes a classification-diff report to w.
// Neither position is treated as authoritative.
// Returns the first write error encountered, if any.
func PrintClassificationDiffs(w io.Writer, analystA, analystB string, lenA, lenB int, diffs []ClassificationDiff) error {
	lines := []string{
		"=== Classification Diff ===",
		"",
		fmt.Sprintf("Position A: %s", analystA),
		fmt.Sprintf("Position B: %s", analystB),
	}

	if lenA != lenB {
		shorter := lenA
		if lenB < shorter {
			shorter = lenB
		}
		lines = append(lines,
			"",
			fmt.Sprintf("Position A has %d steps; Position B has %d steps. Steps beyond position %d are not visible in this comparison.",
				lenA, lenB, shorter),
		)
	}

	lines = append(lines,
		"",
		fmt.Sprintf("Divergences: %d", len(diffs)),
	)

	if len(diffs) == 0 {
		lines = append(lines, "", "No classification divergence — both positions produced the same reading for every comparable step.")
	} else {
		lines = append(lines, "")
		for _, d := range diffs {
			lines = append(lines,
				fmt.Sprintf("Step %d:", d.StepIndex),
				fmt.Sprintf("  Position A (%s): %s", analystA, d.KindA),
				fmt.Sprintf("    Reason: %s", d.ReasonA),
				fmt.Sprintf("  Position B (%s): %s", analystB, d.KindB),
				fmt.Sprintf("    Reason: %s", d.ReasonB),
			)
		}
	}

	lines = append(lines,
		"",
		"---",
		"Neither classification is authoritative. Each reflects where it stands.",
		"Step indices reflect positional depth in each chain, not shared derivation identity.",
	)

	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("loader: PrintClassificationDiffs: %w", err)
		}
	}
	return nil
}

