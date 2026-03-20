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
// It loads a drafts JSON file, groups drafts by analyst (ExtractedBy), builds
// a derivation chain for each analyst position for the requested SourceSpan,
// classifies each chain with ClassifyDraftChain, and compares the resulting
// classifications with CompareChainClassifications.
//
// The comparison surfaces steps where the two positions assigned different
// DraftStepKind values. Length asymmetry is reported when the chains have
// different numbers of steps — steps beyond the shorter chain are not visible.
//
// Required flags:
//   - --analyst-a: label for the first analyst position
//   - --analyst-b: label for the second analyst position
//   - --span: SourceSpan to compare classification chains for
//
// Optional flags:
//   - --output: write report to file instead of stdout
//
// Required positional argument: path to a drafts JSON file.
//
// Note: the span string is treated as a shared key to align the two analyst
// positions. Identical span strings do not guarantee that both positions
// worked from identical source material — the comparison assumes they did.
//
// Returns an error if required flags are missing, the file cannot be loaded,
// either analyst label is not found, or the span is absent from one position.
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

	// Validate required flags.
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

	// Load and validate all draft records.
	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("chain-diff: %w", err)
	}

	// Partition drafts by analyst position (ExtractedBy field).
	byAnalyst := loader.GroupByAnalyst(drafts)

	// Look up each analyst label; report all available labels on failure.
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

	// buildChain finds the root draft for the given span in analystDrafts,
	// follows the derivation chain, and returns the classification of each step.
	// The root is the span draft whose DerivedFrom is absent from this span's
	// draft IDs — that is, the analyst's earliest draft for this span.
	//
	// Scope note: root-finding uses the span-scoped ID set (spanIDs), not the
	// full analyst set. This prevents a cross-span DerivedFrom link from being
	// mistaken for an in-span parent, which would cause the root to be placed
	// at a mid-chain node. FollowDraftChain similarly receives only spanDrafts
	// so the traversal cannot follow links outside this span.
	buildChain := func(analystLabel string, analystDrafts []schema.TraceDraft) ([]loader.DraftStepClassification, error) {
		// Filter to drafts for the requested span.
		var spanDrafts []schema.TraceDraft
		for _, d := range analystDrafts {
			if d.SourceSpan == span {
				spanDrafts = append(spanDrafts, d)
			}
		}
		if len(spanDrafts) == 0 {
			return nil, fmt.Errorf("chain-diff: span %q not found for analyst %q", span, analystLabel)
		}

		// Build a span-scoped ID set. Root candidates are span drafts whose
		// DerivedFrom is absent from this set — their parent (if any) is outside
		// the span, making them the starting point of this analyst's chain.
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
			// Multiple independent roots mean the chain is ambiguous — refuse
			// to pick one silently, as the comparison would vary by JSON order.
			return nil, fmt.Errorf("chain-diff: ambiguous chain root for span %q under analyst %q: multiple root candidates %v", span, analystLabel, roots)
		}
		rootID := roots[0]

		// Follow the derivation chain from the root within the span-scoped set.
		// Passing spanDrafts (not analystDrafts) ensures the traversal cannot
		// cross into a draft that belongs to a different SourceSpan.
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

	// Compare the two classification chains and render the report.
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
