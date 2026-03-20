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
// Re-articulation is a cut, not a correction. This command reads a TraceDraft
// JSON file and produces a skeleton JSON array: one skeleton record per draft,
// with SourceSpan copied verbatim, DerivedFrom set to the original's ID, and
// all content fields left blank. The critiquing agent fills in the
// interpretation. Blank content fields are correct scaffold output — they are
// honest abstentions, not missing data (P3 in plan_m12.md).
//
// Design constraints:
//   - cmdRearticulate must NOT pre-fill content fields from the original (P3)
//   - cmdRearticulate must NOT call Validate() on the skeleton output — the
//     skeleton is intentionally incomplete (blank ID); Validate() would pass
//     since source_span is present, but ID assignment is left to cmdDraft
//   - "reviewed" is the extraction_stage for all skeletons (pipeline position,
//     not a quality claim — Decision 7 in tracedraft-v1.md)
//
// Flags:
//   - --id <id>             produce skeleton for a single draft by ID (default: all)
//   - --output <path>       write skeleton JSON to file (default: stdout)
//   - --criterion-file <path> load an EquivalenceCriterion and set its Name as
//     CriterionRef on each skeleton, making the critique pass self-situated
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

	// Load originals. LoadDrafts assigns UUIDs and validates SourceSpan.
	originals, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("rearticulate: %w", err)
	}

	// Apply --id filter if provided.
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

	// Load criterion if provided. criterionName is empty when no flag was given.
	var criterionName string
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("rearticulate: %w", err)
		}
		criterionName = c.Name
	}

	// Build skeleton records. Each skeleton:
	//   - copies SourceSpan verbatim (the invariant, Decision 2)
	//   - copies SourceDocRef if present (ground truth provenance, not interpretation)
	//   - sets DerivedFrom to the original's ID
	//   - sets ExtractionStage to "reviewed" (pipeline position, not quality)
	//   - leaves all content fields blank (P3: no pre-filling)
	//   - leaves ID and ExtractedBy blank (to be assigned by meshant draft)
	//   - sets CriterionRef when --criterion-file was provided (self-situated skeleton)
	skeletons := make([]schema.TraceDraft, len(originals))
	for i, orig := range originals {
		skeletons[i] = schema.TraceDraft{
			SourceSpan:      orig.SourceSpan,
			SourceDocRef:    orig.SourceDocRef,
			DerivedFrom:     orig.ID,
			ExtractionStage: "reviewed",
			CriterionRef:    criterionName,
			// IntentionallyBlank declares which content fields were
			// deliberately left empty by this cut — the critique agent
			// provides its own interpretation. Blank is correct, not incomplete.
			IntentionallyBlank: []string{
				"what_changed", "source", "target",
				"mediation", "observer", "tags",
			},
		}
	}

	// Determine output destination: file or stdout.
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
