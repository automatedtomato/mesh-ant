package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// cmdRearticulate implements the "rearticulate" subcommand.
//
// Re-articulation is a cut, not a correction. Produces a skeleton JSON array
// from a TraceDraft file: one record per draft, with SourceSpan copied verbatim,
// DerivedFrom set to the original's ID, and all content fields blank.
//
// Content fields are intentionally blank (P3 in plan_m12.md) — the critique
// agent supplies its own interpretation. ID assignment is left to cmdDraft.
// ExtractionStage is "reviewed" as a pipeline position, not a quality claim
// (Decision 7 in tracedraft-v1.md).
func cmdRearticulate(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("rearticulate", flag.ContinueOnError)

	var idFilter string
	fs.StringVar(&idFilter, "id", "", "produce skeleton for a single draft by ID")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write skeleton JSON to file (default: stdout)")

	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to EquivalenceCriterion JSON; sets criterion_ref on each skeleton")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("rearticulate: path to drafts.json required\n\nUsage: meshant rearticulate [--id <id>] [--output <file>] <drafts.json>")
	}
	path := remaining[0]

	originals, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("rearticulate: %w", err)
	}

	if idFilter != "" {
		var found *schema.TraceDraft
		for i := range originals {
			if originals[i].ID == idFilter {
				found = &originals[i]
				break
			}
		}
		if found == nil {
			return fmt.Errorf("rearticulate: draft with id %q not found in %s", idFilter, path)
		}
		originals = []schema.TraceDraft{*found}
	}

	var criterionName string
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("rearticulate: %w", err)
		}
		criterionName = c.Name
	}

	skeletons := make([]schema.TraceDraft, len(originals))
	for i, orig := range originals {
		skeletons[i] = schema.TraceDraft{
			SourceSpan:      orig.SourceSpan,
			SourceDocRef:    orig.SourceDocRef,
			DerivedFrom:     orig.ID,
			ExtractionStage: "reviewed",
			CriterionRef:    criterionName,
			// IntentionallyBlank: content fields are honest abstentions —
			// the critique agent supplies its own interpretation.
			IntentionallyBlank: []string{
				"what_changed", "source", "target",
				"mediation", "observer", "tags",
			},
		}
	}

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("rearticulate: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(skeletons); err != nil {
		return fmt.Errorf("rearticulate: encode output: %w", err)
	}

	return confirmOutput(w, outputPath)
}
