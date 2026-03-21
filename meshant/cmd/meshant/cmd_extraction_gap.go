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
// Groups drafts by analyst, compares the two requested extraction sets, and
// writes an ExtractionGap report. Requires --analyst-a, --analyst-b, and a
// positional path to a drafts JSON file.
func cmdExtractionGap(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("extraction-gap", flag.ContinueOnError)

	var analystA, analystB, outputPath string
	fs.StringVar(&analystA, "analyst-a", "", "label for analyst position A (required)")
	fs.StringVar(&analystB, "analyst-b", "", "label for analyst position B (required)")
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. gap.txt)")

	if err := fs.Parse(args); err != nil {
		return err
	}

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

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("extraction-gap: %w", err)
	}

	byAnalyst := loader.GroupByAnalyst(drafts)

	// Reports all available labels on failure so the user can correct without re-running.
	lookupSet := func(label string) ([]schema.TraceDraft, error) {
		set, ok := byAnalyst[label]
		if !ok {
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

	gap := loader.CompareExtractions(analystA, setA, analystB, setB)

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
