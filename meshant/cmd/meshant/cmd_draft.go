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
// It reads an extraction JSON file produced by an external LLM tool (the
// ingestion contract: source_span required, all other fields optional),
// assigns UUIDs and timestamps to records that lack them, applies optional
// field overrides, validates each record, writes the resulting TraceDraft JSON
// array to --output (or stdout), and prints a provenance summary.
//
// The command does not make LLM calls — the LLM's transformation is a named,
// inspectable file on disk, consistent with treating the LLM as a mediator
// rather than a hidden intermediary. See docs/decisions/tracedraft-v1.md.
//
// Flags:
//   - --source-doc <ref>     stamp SourceDocRef on all drafts
//   - --extracted-by <label> override ExtractedBy on all drafts
//   - --stage <stage>        override ExtractionStage on all drafts
//   - --output <file>        write TraceDraft JSON to file (default: stdout)
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

	// Apply optional field overrides to all drafts.
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

	// Determine output destination: file or stdout.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	// Write TraceDraft JSON array to output destination.
	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(drafts); err != nil {
		return fmt.Errorf("draft: encode output: %w", err)
	}

	// Print provenance summary to w (stdout) regardless of --output.
	// When --output is stdout, the summary follows the JSON on the same stream.
	summary := loader.SummariseDrafts(drafts)
	if err := loader.PrintDraftSummary(w, summary); err != nil {
		return fmt.Errorf("draft: %w", err)
	}

	return confirmOutput(w, outputPath)
}
