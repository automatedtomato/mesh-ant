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
//
// StepIndex matches DraftStepClassification.StepIndex (1-based).
type ClassificationDiff struct {
	// StepIndex is the derivation step where the two positions diverge.
	// 1-based: StepIndex 1 = the step from chain[0] to chain[1].
	StepIndex int

	// KindA is analyst A's classification for this step.
	KindA DraftStepKind

	// KindB is analyst B's classification for this step.
	KindB DraftStepKind

	// ReasonA is analyst A's justification. Carried forward from the
	// DraftStepClassification so the caller can inspect the divergence.
	ReasonA string

	// ReasonB is analyst B's justification.
	ReasonB string
}

// CompareChainClassifications compares two classification slices step by step
// and returns diffs for steps where Kind differs. Comparison is by position
// (same slice index = same derivation depth), up to min(len(chainA), len(chainB)).
// Steps beyond the shorter chain are not compared — length difference is surfaced
// by the caller (e.g., PrintClassificationDiffs). Returns non-nil empty slice
// when both chains are empty or all steps agree.
func CompareChainClassifications(chainA, chainB []DraftStepClassification) []ClassificationDiff {
	// Return non-nil empty slice so callers can range without nil checks.
	result := []ClassificationDiff{}

	// Compare only up to the length of the shorter chain.
	limit := len(chainA)
	if len(chainB) < limit {
		limit = len(chainB)
	}

	for i := 0; i < limit; i++ {
		a := chainA[i]
		b := chainB[i]

		// Emit a diff only when the Kinds diverge. Reason differences alone
		// are not diffs — they represent different justifications for the same
		// classification judgment, which is analytically distinct from
		// disagreeing on the classification itself.
		//
		// StepIndex is derived from the loop counter (i+1, 1-based) rather than
		// a.StepIndex or b.StepIndex. Using the loop position makes the index
		// unambiguous: it is the positional comparison depth, not the raw field
		// value from either input chain. This matters if a caller passes chains
		// with non-sequential or misaligned StepIndex values — the comparison
		// remains correct and the reported index is always interpretable.
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
//
// analystA, analystB: the position labels (e.g., "alice", "bob").
// lenA, lenB: the total step counts of each chain, used to note any length
// asymmetry — steps beyond the shorter chain were not visible in the comparison.
// diffs: the divergences from CompareChainClassifications.
//
// Uses "Position A (analystA) / Position B (analystB)" framing throughout.
// Neither position is treated as authoritative. Closing note: "Neither
// classification is authoritative. Each reflects where it stands."
//
// Returns the first write error encountered, if any, wrapped with
// "loader: PrintClassificationDiffs: %w".
func PrintClassificationDiffs(w io.Writer, analystA, analystB string, lenA, lenB int, diffs []ClassificationDiff) error {
	lines := []string{
		"=== Classification Diff ===",
		"",
		fmt.Sprintf("Position A: %s", analystA),
		fmt.Sprintf("Position B: %s", analystB),
	}

	// Note length asymmetry when the two chains have different numbers of steps.
	// Steps beyond the shorter chain were not visible in this comparison.
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
		// No divergence: both positions independently produced the same reading for
		// each comparable step — not consensus, but convergence from separate positions.
		lines = append(lines, "", "No classification divergence — both positions produced the same reading for every comparable step.")
	} else {
		// List each divergence: step index, Kind from each position, and the
		// justification each position offered.
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

	// Closing note: no position holds the authoritative reading.
	// Step indices reflect positional depth in each chain, not a shared
	// identity across positions — two analysts may classify "step 2" without
	// having produced the same derivation moment.
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

