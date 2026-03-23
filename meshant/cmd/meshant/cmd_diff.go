package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// cmdDiff implements the "diff" subcommand.
//
// Compares two observer-situated articulations of the same trace set, exposing
// the asymmetry between two positions. Accepts --observer-a/--observer-b
// (repeatable, required), --tag-a/--tag-b, --from-a/--to-a/--from-b/--to-b
// (RFC3339), --format (text|json|dot|mermaid), and --output.
func cmdDiff(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)

	var observersA, observersB stringSliceFlag
	fs.Var(&observersA, "observer-a", "observer position for graph A (repeatable)")
	fs.Var(&observersB, "observer-b", "observer position for graph B (repeatable)")

	var tagsA, tagsB stringSliceFlag
	fs.Var(&tagsA, "tag-a", "tag filter for graph A (repeatable, any-match / OR semantics)")
	fs.Var(&tagsB, "tag-b", "tag filter for graph B (repeatable, any-match / OR semantics)")

	var fromAStr, toAStr, fromBStr, toBStr string
	fs.StringVar(&fromAStr, "from-a", "", "start of time window for graph A (RFC3339)")
	fs.StringVar(&toAStr, "to-a", "", "end of time window for graph A (RFC3339)")
	fs.StringVar(&fromBStr, "from-b", "", "start of time window for graph B (RFC3339)")
	fs.StringVar(&toBStr, "to-b", "", "end of time window for graph B (RFC3339)")

	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json|dot|mermaid")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. diff.dot)")

	var dbURL string
	fs.StringVar(&dbURL, "db", os.Getenv("MESHANT_DB_URL"),
		"Neo4j Bolt URL; mutually exclusive with <traces.json> (or set MESHANT_DB_URL)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(observersA) == 0 {
		return fmt.Errorf("diff: --observer-a is required")
	}
	if len(observersB) == 0 {
		return fmt.Errorf("diff: --observer-b is required")
	}

	switch format {
	case "text", "json", "dot", "mermaid":
		// valid
	default:
		return fmt.Errorf("diff: unknown --format %q (text|json|dot|mermaid)", format)
	}

	twA, err := parseTimeWindow("from-a", fromAStr, "to-a", toAStr)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	twB, err := parseTimeWindow("from-b", fromBStr, "to-b", toBStr)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}

	remaining := fs.Args()
	if dbURL != "" && len(remaining) > 0 {
		return fmt.Errorf("diff: --db and <file> are mutually exclusive")
	}
	if dbURL == "" && len(remaining) == 0 {
		return fmt.Errorf("diff: path to traces.json or --db required\n\nUsage: meshant diff --observer-a <pos> --observer-b <pos> [--tag-a <tag>] [--tag-b <tag>] [--from-a RFC3339] [--to-a RFC3339] [--from-b RFC3339] [--to-b RFC3339] [--format text|json|dot|mermaid] [--output <file>] [--db bolt://...] <traces.json>")
	}

	traces, closeStore, err := loadTraces(context.Background(), dbURL, remaining)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	defer closeStore()

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

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
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
