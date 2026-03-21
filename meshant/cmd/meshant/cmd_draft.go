package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// cmdDraft implements the "draft" subcommand.
//
// Reads an extraction JSON file (source_span required, all other fields
// optional), assigns UUIDs and timestamps, applies optional field overrides,
// and writes a TraceDraft JSON array. Does not make LLM calls — the LLM's
// transformation is a named, inspectable file on disk (see
// docs/decisions/tracedraft-v1.md).
func cmdDraft(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("draft", flag.ContinueOnError)

	var sourceDoc string
	fs.StringVar(&sourceDoc, "source-doc", "", "document identifier stamped on all drafts (SourceDocRef)")

	var extractedBy string
	fs.StringVar(&extractedBy, "extracted-by", "", "override ExtractedBy field for all loaded drafts")

	var stage string
	fs.StringVar(&stage, "stage", "", "override ExtractionStage field for all loaded drafts")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write TraceDraft JSON to file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("draft: path to extraction.json required\n\nUsage: meshant draft [--source-doc <ref>] [--extracted-by <label>] [--stage <stage>] [--output <file>] <extraction.json>")
	}
	path := remaining[0]

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("draft: %w", err)
	}

	if len(drafts) == 0 {
		return fmt.Errorf("draft: %q contains no records", path)
	}

	for i := range drafts {
		if sourceDoc != "" {
			drafts[i].SourceDocRef = sourceDoc
		}
		if extractedBy != "" {
			drafts[i].ExtractedBy = extractedBy
		}
		if stage != "" {
			drafts[i].ExtractionStage = stage
		}
	}

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(drafts); err != nil {
		return fmt.Errorf("draft: encode output: %w", err)
	}

	// Print provenance summary to w (stdout), never to the output file.
	summary := loader.SummariseDrafts(drafts)
	if err := loader.PrintDraftSummary(w, summary); err != nil {
		return fmt.Errorf("draft: %w", err)
	}

	return confirmOutput(w, outputPath)
}
