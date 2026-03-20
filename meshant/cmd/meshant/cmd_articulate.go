package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// cmdArticulate implements the "articulate" subcommand.
//
// It accepts the following flags:
//   - --observer  (repeatable, required) — one or more observer positions to include
//   - --tag       (repeatable, optional) — tag filter (any-match / OR semantics)
//   - --from      (optional, RFC3339)    — start of time window
//   - --to        (optional, RFC3339)    — end of time window
//   - --format    (optional, default "text") — output format: text|json|dot|mermaid
//   - --output    (optional)             — write output to file instead of stdout
//   - --narrative (optional, bool)       — append a NarrativeDraft section (text only)
//
// The positional argument (after all flags) must be the path to a traces JSON file.
//
// Returns an error if: no --observer is given, a time flag is not RFC3339,
// the time window is invalid (Start > End), --format is unrecognised,
// the path is missing or unloadable, or writing to w fails.
func cmdArticulate(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("articulate", flag.ContinueOnError)

	// --observer is repeatable; collected by stringSliceFlag.
	var observers stringSliceFlag
	fs.Var(&observers, "observer", "observer position to include (repeatable)")

	// --tag is repeatable; any-match / OR semantics — a trace passes if it carries any of the specified tags.
	var tags stringSliceFlag
	fs.Var(&tags, "tag", "tag filter (repeatable, any-match / OR semantics)")

	// --from / --to are optional RFC3339 timestamps for time-window filtering.
	var fromStr, toStr string
	fs.StringVar(&fromStr, "from", "", "start of time window (RFC3339)")
	fs.StringVar(&toStr, "to", "", "end of time window (RFC3339)")

	// --format selects the output renderer; defaults to "text".
	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json|dot|mermaid")

	// --output writes output to a file instead of stdout.
	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. graph.dot)")

	// --narrative appends a NarrativeDraft section after the articulation output.
	// Silently skipped for non-text formats (json, dot, mermaid) because
	// narrative prose is not meaningful inside structured machine-readable output.
	var narrative bool
	fs.BoolVar(&narrative, "narrative", false, "append a provisional narrative draft (text format only)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// --observer is mandatory: a graph requires at least one observer position.
	if len(observers) == 0 {
		return fmt.Errorf("articulate: --observer is required")
	}

	// Reject unknown formats before file I/O so the error is immediate.
	switch format {
	case "text", "json", "dot", "mermaid":
		// valid
	default:
		return fmt.Errorf("articulate: unknown --format %q (text|json|dot|mermaid)", format)
	}

	// Parse optional time window flags.
	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}

	// The positional argument after all flags is the path to the dataset.
	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("articulate: path to traces.json required\n\nUsage: meshant articulate --observer <pos> [--tag <tag>] [--from RFC3339] [--to RFC3339] [--format text|json|dot|mermaid] [--output <file>] <traces.json>")
	}
	path := remaining[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string(observers),
		TimeWindow:        tw,
		Tags:              []string(tags),
	}
	g := graph.Articulate(traces, opts)

	// Determine output destination: file or stdout.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}
	// Ensure the file is closed on all exit paths, including render errors.
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	// Dispatch to the requested output renderer.
	switch format {
	case "text":
		err = graph.PrintArticulation(dest, g)
	case "json":
		err = graph.PrintGraphJSON(dest, g)
	case "dot":
		err = graph.PrintGraphDOT(dest, g)
	default: // "mermaid"
		err = graph.PrintGraphMermaid(dest, g)
	}
	if err != nil {
		return err
	}

	// --narrative emits a NarrativeDraft section after the articulation output.
	// Silently skipped for non-text formats: narrative prose has no place inside
	// JSON, DOT, or Mermaid output — those formats have their own consumers.
	if narrative && format == "text" {
		n := graph.DraftNarrative(g)
		if _, err := fmt.Fprintln(dest, ""); err != nil {
			return err
		}
		if err := graph.PrintNarrativeDraft(dest, n); err != nil {
			return err
		}
	}

	// If writing to file, print confirmation to stdout.
	return confirmOutput(w, outputPath)
}
