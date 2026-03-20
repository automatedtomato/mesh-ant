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
// It loads a TraceDraft JSON file, passes the drafts to review.RunReviewSession
// for interactive accept/edit/skip/quit decisions, and writes the resulting
// reviewed drafts as a JSON array to --output (or w). A summary line is always
// written to w regardless of --output.
//
// Interactive prompts go to os.Stderr (not w) so that when output is stdout,
// the prompt lines are not mixed into the JSON stream. This keeps w clean for
// machine-readable output — piping `meshant review <file>` to `jq` works
// correctly.
//
// The session treats EOF and "q" identically: results collected so far are
// returned without error. A nil result slice is normalised to an empty slice
// so the JSON output is always "[]" rather than "null".
//
// Flags:
//   - --output <file>  write reviewed drafts JSON to file (default: w/stdout)
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

	// Load and validate the draft file. LoadDrafts assigns missing UUIDs and
	// validates SourceSpan. An empty file is valid (returns []).
	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	// Run the interactive session. Interactive prompts go to os.Stderr so
	// that stdout (w) carries only the JSON output — callers can safely pipe
	// the command output to jq or other tools without prompt contamination.
	results, err := review.RunReviewSession(drafts, in, os.Stderr)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	// Normalise nil to empty slice: JSON encodes nil slice as "null",
	// which is confusing for callers expecting an array. Always emit "[]".
	output := results
	if output == nil {
		output = []schema.TraceDraft{}
	}

	// Determine output destination: file or w (stdout).
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	// Write the reviewed drafts as indented JSON.
	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("review: encode output: %w", err)
	}

	// If dest is a file, close it explicitly so write errors are not silently
	// discarded (a deferred close would swallow errors on NFS or low-disk).
	if f, ok := dest.(*os.File); ok {
		if err := f.Close(); err != nil {
			return fmt.Errorf("review: close output: %w", err)
		}
	}

	// Summary always goes to w. When --output is a file, the JSON has already
	// been written there; the summary here is the only thing on w. When
	// --output is not set, the JSON preceded the summary on w.
	// len(output) is used (not len(results)) so the count always matches
	// what was actually encoded.
	fmt.Fprintf(w, "\nReview complete: %d accepted/edited out of %d loaded\n",
		len(output), len(drafts))

	return confirmOutput(w, outputPath)
}
