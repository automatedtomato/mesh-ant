package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// cmdChainDiff implements the "chain-diff" subcommand.
//
// Groups drafts by analyst, builds a derivation chain per analyst for the
// requested --span, classifies each chain, and compares classifications.
// Steps where the two positions diverge are surfaced; length asymmetry is
// reported when chain lengths differ.
//
// Note: --span is treated as a shared key. Identical span strings do not
// guarantee identical source material — the comparison assumes they do.
func cmdChainDiff(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("chain-diff", flag.ContinueOnError)

	var analystA, analystB, span, outputPath string
	fs.StringVar(&analystA, "analyst-a", "", "label for analyst position A (required)")
	fs.StringVar(&analystB, "analyst-b", "", "label for analyst position B (required)")
	fs.StringVar(&span, "span", "", "source span to compare classification chains for (required)")
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. diff.txt)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if analystA == "" {
		return fmt.Errorf("chain-diff: --analyst-a is required")
	}
	if analystB == "" {
		return fmt.Errorf("chain-diff: --analyst-b is required")
	}
	if analystA == analystB {
		return fmt.Errorf("chain-diff: --analyst-a and --analyst-b must be different labels (got %q for both)", analystA)
	}
	if span == "" {
		return fmt.Errorf("chain-diff: --span is required")
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("chain-diff: path to drafts.json required\n\nUsage: meshant chain-diff --analyst-a <label> --analyst-b <label> --span <source_span> [--output file] <drafts.json>")
	}
	path := remaining[0]

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("chain-diff: %w", err)
	}

	byAnalyst := loader.GroupByAnalyst(drafts)

	// Reports all available labels on failure so the user can correct without re-running.
	lookupAnalyst := func(label string) ([]schema.TraceDraft, error) {
		set, ok := byAnalyst[label]
		if !ok {
			available := make([]string, 0, len(byAnalyst))
			for k := range byAnalyst {
				available = append(available, k)
			}
			sort.Strings(available)
			return nil, fmt.Errorf("chain-diff: analyst %q not found; available: %s",
				label, strings.Join(available, ", "))
		}
		return set, nil
	}

	setA, err := lookupAnalyst(analystA)
	if err != nil {
		return err
	}
	setB, err := lookupAnalyst(analystB)
	if err != nil {
		return err
	}

	// buildChain finds the chain root for the requested span, then classifies
	// each step. Root-finding and traversal are scoped to spanDrafts (not the
	// full analyst set) to prevent cross-span DerivedFrom links from displacing
	// the root to a mid-chain node.
	buildChain := func(analystLabel string, analystDrafts []schema.TraceDraft) ([]loader.DraftStepClassification, error) {
		var spanDrafts []schema.TraceDraft
		for _, d := range analystDrafts {
			if d.SourceSpan == span {
				spanDrafts = append(spanDrafts, d)
			}
		}
		if len(spanDrafts) == 0 {
			return nil, fmt.Errorf("chain-diff: span %q not found for analyst %q", span, analystLabel)
		}

		// Root candidates: span drafts whose DerivedFrom is absent from this set
		// (parent is outside the span, so this draft starts the analyst's chain).
		spanIDs := make(map[string]bool, len(spanDrafts))
		for _, d := range spanDrafts {
			spanIDs[d.ID] = true
		}

		var roots []string
		for _, d := range spanDrafts {
			if d.DerivedFrom == "" || !spanIDs[d.DerivedFrom] {
				roots = append(roots, d.ID)
			}
		}
		if len(roots) == 0 {
			// Every span draft has a parent within the span — possible cycle.
			return nil, fmt.Errorf("chain-diff: no chain root found for span %q under analyst %q (possible cycle in derivation links)", span, analystLabel)
		}
		if len(roots) > 1 {
			// Refuse to pick a root silently — the comparison would vary by JSON order.
			return nil, fmt.Errorf("chain-diff: ambiguous chain root for span %q under analyst %q: multiple root candidates %v", span, analystLabel, roots)
		}
		rootID := roots[0]

		chain := loader.FollowDraftChain(spanDrafts, rootID)
		return loader.ClassifyDraftChain(chain), nil
	}

	chainA, err := buildChain(analystA, setA)
	if err != nil {
		return err
	}
	chainB, err := buildChain(analystB, setB)
	if err != nil {
		return err
	}

	diffs := loader.CompareChainClassifications(chainA, chainB)

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("chain-diff: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	if err := loader.PrintClassificationDiffs(dest, analystA, analystB, len(chainA), len(chainB), diffs); err != nil {
		return err
	}

	return confirmOutput(w, outputPath)
}
