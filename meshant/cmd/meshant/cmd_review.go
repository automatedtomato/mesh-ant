package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/review"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// cmdReview implements the "review" subcommand.
//
// Runs an interactive accept/edit/skip/quit session over TraceDraft records.
// Prompts go to os.Stderr so the JSON stream on w stays clean for piping.
// EOF and "q" are treated identically. Nil result is normalised to "[]".
func cmdReview(w io.Writer, in io.Reader, args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write reviewed drafts JSON to file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("review: path to draft file required\n\nUsage: meshant review [--output <file>] <drafts.json>")
	}
	path := remaining[0]

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	results, err := review.RunReviewSession(drafts, in, os.Stderr)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	output := results
	if output == nil {
		output = []schema.TraceDraft{}
	}

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("review: encode output: %w", err)
	}

	// Close explicitly so write errors are not swallowed (NFS/low-disk defence).
	if f, ok := dest.(*os.File); ok {
		if err := f.Close(); err != nil {
			return fmt.Errorf("review: close output: %w", err)
		}
	}

	// Summary always goes to w (stdout). len(output) not len(results) so the
	// count matches what was actually encoded.
	fmt.Fprintf(w, "\nReview complete: %d accepted/edited out of %d loaded\n",
		len(output), len(drafts))

	return confirmOutput(w, outputPath)
}
