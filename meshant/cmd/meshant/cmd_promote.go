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

// cmdPromote implements the "promote" subcommand.
//
// Promotes qualifying TraceDraft records to canonical Traces and writes a
// summary of promoted/skipped counts with skip reasons.
func cmdPromote(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("promote", flag.ContinueOnError)

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write promoted traces JSON to file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("promote: path to drafts.json required\n\nUsage: meshant promote [--output <file>] <drafts.json>")
	}
	path := remaining[0]

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}

	var promoted []schema.Trace
	type failedDraft struct {
		idx    int
		id     string
		reason string
	}
	var failures []failedDraft

	for i, d := range drafts {
		tr, err := d.Promote()
		if err != nil {
			failures = append(failures, failedDraft{idx: i, id: d.ID, reason: err.Error()})
			continue
		}
		promoted = append(promoted, tr)
	}

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	out := promoted
	if out == nil {
		out = []schema.Trace{}
	}
	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("promote: encode output: %w", err)
	}

	fmt.Fprintf(w, "\nPromotion summary: %d promoted, %d not promotable (out of %d)\n",
		len(promoted), len(failures), len(drafts))
	for _, f := range failures {
		fmt.Fprintf(w, "  draft %d (id=%s): %s\n", f.idx, f.id, f.reason)
	}

	return confirmOutput(w, outputPath)
}
