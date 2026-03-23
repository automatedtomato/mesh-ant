package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// cmdBottleneck implements the "bottleneck" subcommand.
//
// Articulates an observer-situated graph and surfaces provisionally central
// elements via IdentifyBottlenecks. --observer is optional: omitting it is a
// deliberate analytical choice (full cut, no observer filter), not an error.
func cmdBottleneck(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("bottleneck", flag.ContinueOnError)

	var observers stringSliceFlag
	fs.Var(&observers, "observer", "observer position to include (repeatable, optional)")

	var tags stringSliceFlag
	fs.Var(&tags, "tag", "tag filter (repeatable, any-match / OR semantics)")

	var fromStr, toStr string
	fs.StringVar(&fromStr, "from", "", "start of time window (RFC3339)")
	fs.StringVar(&toStr, "to", "", "end of time window (RFC3339)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. bottleneck.txt)")

	var dbURL string
	fs.StringVar(&dbURL, "db", os.Getenv("MESHANT_DB_URL"),
		"Neo4j Bolt URL; mutually exclusive with <traces.json> (or set MESHANT_DB_URL)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("bottleneck: %w", err)
	}

	remaining := fs.Args()
	if dbURL != "" && len(remaining) > 0 {
		return fmt.Errorf("bottleneck: --db and <file> are mutually exclusive")
	}
	if dbURL == "" && len(remaining) == 0 {
		return fmt.Errorf("bottleneck: path to traces.json or --db required\n\nUsage: meshant bottleneck [--observer <pos>] [--tag <tag>] [--from RFC3339] [--to RFC3339] [--output <file>] [--db bolt://...] <traces.json>")
	}

	traces, closeStore, err := loadTraces(context.Background(), dbURL, remaining)
	if err != nil {
		return fmt.Errorf("bottleneck: %w", err)
	}
	defer closeStore()

	opts := graph.ArticulationOptions{
		ObserverPositions: []string(observers),
		TimeWindow:        tw,
		Tags:              []string(tags),
	}
	g := graph.Articulate(traces, opts)
	notes := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("bottleneck: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	if err := graph.PrintBottleneckNotes(dest, g, notes); err != nil {
		return err
	}
	return confirmOutput(w, outputPath)
}
