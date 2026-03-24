package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// cmdShadow implements the "shadow" subcommand.
//
// Articulates an observer-situated graph and prints a shadow summary.
// Shadow is a cut decision, not missing data: a shadowed element was visible
// to some observer but not the one named by this cut.
func cmdShadow(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("shadow", flag.ContinueOnError)

	var observers stringSliceFlag
	fs.Var(&observers, "observer", "observer position to include (repeatable)")

	var tags stringSliceFlag
	fs.Var(&tags, "tag", "tag filter (repeatable, any-match / OR semantics)")

	var fromStr, toStr string
	fs.StringVar(&fromStr, "from", "", "start of time window (RFC3339)")
	fs.StringVar(&toStr, "to", "", "end of time window (RFC3339)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. shadow.txt)")

	var dbURL string
	fs.StringVar(&dbURL, "db", os.Getenv("MESHANT_DB_URL"),
		"Neo4j Bolt URL; mutually exclusive with <traces.json> (or set MESHANT_DB_URL)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(observers) == 0 {
		return fmt.Errorf("shadow: --observer is required")
	}

	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("shadow: %w", err)
	}

	remaining := fs.Args()
	if dbURL != "" && len(remaining) > 0 {
		return fmt.Errorf("shadow: --db and <file> are mutually exclusive")
	}
	if dbURL == "" && len(remaining) == 0 {
		return fmt.Errorf("shadow: path to traces.json or --db required\n\nUsage: meshant shadow --observer <pos> [--tag <tag>] [--from RFC3339] [--to RFC3339] [--output <file>] [--db bolt://...] <traces.json>")
	}

	traces, closeStore, err := loadTraces(context.Background(), dbURL, remaining)
	if err != nil {
		return fmt.Errorf("shadow: %w", err)
	}
	defer closeStore()

	opts := graph.ArticulationOptions{
		ObserverPositions: []string(observers),
		TimeWindow:        tw,
		Tags:              []string(tags),
	}
	g := graph.Articulate(traces, opts)
	s := graph.SummariseShadow(g)

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("shadow: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	if err := graph.PrintShadowSummary(dest, s); err != nil {
		return err
	}
	return confirmOutput(w, outputPath)
}
