package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// cmdDiff implements the "diff" subcommand.
//
// It compares two observer-situated articulations of the same trace set,
// producing a structural diff that reveals what each observer can and cannot
// see. This is the core diagnostic operation of the mesh: a diff exposes the
// asymmetry between two positions in a network.
//
// It accepts the following flags:
//   - --observer-a (repeatable, required) — observer positions for graph A
//   - --observer-b (repeatable, required) — observer positions for graph B
//   - --tag-a      (repeatable, optional) — tag filter for graph A (any-match / OR semantics)
//   - --tag-b      (repeatable, optional) — tag filter for graph B (any-match / OR semantics)
//   - --from-a, --to-a (optional, RFC3339) — time window for graph A
//   - --from-b, --to-b (optional, RFC3339) — time window for graph B
//   - --format (optional, default "text")  — output format: text|json|dot|mermaid
//   - --output (optional)                  — write output to file instead of stdout
//
// Returns an error if: --observer-a or --observer-b is missing, a time flag
// is not RFC3339, a time window is invalid (Start > End), --format is
// unrecognised, the path is missing or unloadable, or writing fails.
func cmdDiff(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)

	// --observer-a and --observer-b are both repeatable and required.
	var observersA, observersB stringSliceFlag
	fs.Var(&observersA, "observer-a", "observer position for graph A (repeatable)")
	fs.Var(&observersB, "observer-b", "observer position for graph B (repeatable)")

	// --tag-a and --tag-b are repeatable tag filters per side (any-match / OR semantics).
	var tagsA, tagsB stringSliceFlag
	fs.Var(&tagsA, "tag-a", "tag filter for graph A (repeatable, any-match / OR semantics)")
	fs.Var(&tagsB, "tag-b", "tag filter for graph B (repeatable, any-match / OR semantics)")

	// Per-side time window flags.
	var fromAStr, toAStr, fromBStr, toBStr string
	fs.StringVar(&fromAStr, "from-a", "", "start of time window for graph A (RFC3339)")
	fs.StringVar(&toAStr, "to-a", "", "end of time window for graph A (RFC3339)")
	fs.StringVar(&fromBStr, "from-b", "", "start of time window for graph B (RFC3339)")
	fs.StringVar(&toBStr, "to-b", "", "end of time window for graph B (RFC3339)")

	// --format selects the output renderer.
	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json|dot|mermaid")

	// --output writes output to a file instead of stdout.
	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. diff.dot)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Both observer sets are mandatory: a diff requires two distinct cuts.
	if len(observersA) == 0 {
		return fmt.Errorf("diff: --observer-a is required")
	}
	if len(observersB) == 0 {
		return fmt.Errorf("diff: --observer-b is required")
	}

	// Validate format.
	switch format {
	case "text", "json", "dot", "mermaid":
		// valid
	default:
		return fmt.Errorf("diff: unknown --format %q (text|json|dot|mermaid)", format)
	}

	// Parse and validate per-side time windows.
	twA, err := parseTimeWindow("from-a", fromAStr, "to-a", toAStr)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	twB, err := parseTimeWindow("from-b", fromBStr, "to-b", toBStr)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}

	// The positional argument after all flags is the shared trace dataset.
	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("diff: path to traces.json required\n\nUsage: meshant diff --observer-a <pos> --observer-b <pos> [--tag-a <tag>] [--tag-b <tag>] [--from-a RFC3339] [--to-a RFC3339] [--from-b RFC3339] [--to-b RFC3339] [--format text|json|dot|mermaid] [--output <file>] <traces.json>")
	}
	path := remaining[0]

	// Load the shared trace set once. Both articulations derive from the
	// same underlying data; only the cut axes differ.
	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}

	optsA := graph.ArticulationOptions{
		ObserverPositions: []string(observersA),
		TimeWindow:        twA,
		Tags:              []string(tagsA),
	}
	optsB := graph.ArticulationOptions{
		ObserverPositions: []string(observersB),
		TimeWindow:        twB,
		Tags:              []string(tagsB),
	}
	gA := graph.Articulate(traces, optsA)
	gB := graph.Articulate(traces, optsB)
	d := graph.Diff(gA, gB)

	// Determine output destination: file or stdout.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	// Ensure the file is closed on all exit paths, including render errors.
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	switch format {
	case "text":
		err = graph.PrintDiff(dest, d)
	case "json":
		err = graph.PrintDiffJSON(dest, d)
	case "dot":
		err = graph.PrintDiffDOT(dest, d)
	default: // "mermaid"
		err = graph.PrintDiffMermaid(dest, d)
	}
	if err != nil {
		return err
	}

	return confirmOutput(w, outputPath)
}
