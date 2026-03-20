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

// cmdExtractionGap implements the "extraction-gap" subcommand.
//
// It loads a drafts JSON file, groups drafts by analyst (ExtractedBy), looks
// up the two requested analyst positions, compares their extraction sets, and
// writes an ExtractionGap report to w (or an optional output file).
//
// Required flags:
//   - --analyst-a: label for the first analyst position
//   - --analyst-b: label for the second analyst position
//
// Optional flags:
//   - --output: write report to file instead of stdout
//
// Required positional argument: path to a drafts JSON file.
//
// Returns an error if required flags are missing, the file cannot be loaded,
// either analyst label is not found in the data, or writing fails.
func cmdExtractionGap(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("extraction-gap", flag.ContinueOnError)

	var analystA, analystB, outputPath string
	fs.StringVar(&analystA, "analyst-a", "", "label for analyst position A (required)")
	fs.StringVar(&analystB, "analyst-b", "", "label for analyst position B (required)")
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. gap.txt)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required flags.
	if analystA == "" {
		return fmt.Errorf("extraction-gap: --analyst-a is required")
	}
	if analystB == "" {
		return fmt.Errorf("extraction-gap: --analyst-b is required")
	}
	if analystA == analystB {
		return fmt.Errorf("extraction-gap: --analyst-a and --analyst-b must be different labels (got %q for both)", analystA)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("extraction-gap: path to drafts.json required\n\nUsage: meshant extraction-gap --analyst-a <label> --analyst-b <label> [--output file] <drafts.json>")
	}
	path := remaining[0]

	// Load and validate all draft records.
	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("extraction-gap: %w", err)
	}

	// Partition drafts by analyst position (ExtractedBy field).
	byAnalyst := loader.GroupByAnalyst(drafts)

	// Look up each analyst label; report all available labels on failure
	// so the user can correct the invocation without re-running to discover.
	lookupSet := func(label string) ([]schema.TraceDraft, error) {
		set, ok := byAnalyst[label]
		if !ok {
			// Build a sorted list of available labels for a helpful error.
			available := make([]string, 0, len(byAnalyst))
			for k := range byAnalyst {
				available = append(available, k)
			}
			sort.Strings(available)
			return nil, fmt.Errorf("extraction-gap: analyst %q not found; available: %s",
				label, strings.Join(available, ", "))
		}
		return set, nil
	}

	setA, err := lookupSet(analystA)
	if err != nil {
		return err
	}
	setB, err := lookupSet(analystB)
	if err != nil {
		return err
	}

	// Compare the two extraction positions.
	gap := loader.CompareExtractions(analystA, setA, analystB, setB)

	// Write report to output destination (file or stdout).
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("extraction-gap: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	if err := loader.PrintExtractionGap(dest, gap); err != nil {
		return err
	}

	return confirmOutput(w, outputPath)
}
