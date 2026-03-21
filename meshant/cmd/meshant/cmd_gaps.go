package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// cmdGaps implements the "gaps" subcommand.
//
// Articulates two observer-situated graphs from the same traces file and
// prints a gap report — what each position sees that the other cannot.
// Neither position is treated as authoritative. When --suggest is set,
// heuristic re-articulation suggestions follow the gap report.
func cmdGaps(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("gaps", flag.ContinueOnError)

	var observersA, observersB stringSliceFlag
	fs.Var(&observersA, "observer-a", "observer positions for graph A (repeatable)")
	fs.Var(&observersB, "observer-b", "observer positions for graph B (repeatable)")

	var tagsA, tagsB stringSliceFlag
	fs.Var(&tagsA, "tag-a", "tag filter for graph A (repeatable, any-match / OR semantics)")
	fs.Var(&tagsB, "tag-b", "tag filter for graph B (repeatable, any-match / OR semantics)")

	var fromAStr, toAStr, fromBStr, toBStr string
	fs.StringVar(&fromAStr, "from-a", "", "start of time window for graph A (RFC3339)")
	fs.StringVar(&toAStr, "to-a", "", "end of time window for graph A (RFC3339)")
	fs.StringVar(&fromBStr, "from-b", "", "start of time window for graph B (RFC3339)")
	fs.StringVar(&toBStr, "to-b", "", "end of time window for graph B (RFC3339)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. gaps.txt)")

	var suggest bool
	fs.BoolVar(&suggest, "suggest", false, "print re-articulation suggestions after the gap report")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(observersA) == 0 {
		return fmt.Errorf("gaps: --observer-a is required")
	}
	if len(observersB) == 0 {
		return fmt.Errorf("gaps: --observer-b is required")
	}

	twA, err := parseTimeWindow("from-a", fromAStr, "to-a", toAStr)
	if err != nil {
		return fmt.Errorf("gaps: %w", err)
	}
	twB, err := parseTimeWindow("from-b", fromBStr, "to-b", toBStr)
	if err != nil {
		return fmt.Errorf("gaps: %w", err)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("gaps: path to traces.json required\n\nUsage: meshant gaps --observer-a <pos> --observer-b <pos> [flags] <traces.json>")
	}
	path := remaining[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("gaps: %w", err)
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
	gap := graph.AnalyseGaps(gA, gB)

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("gaps: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	if err := graph.PrintObserverGap(dest, gap); err != nil {
		return err
	}

	if suggest {
		suggestions := graph.SuggestRearticulations(gap)
		if err := graph.PrintRearticSuggestions(dest, gap, suggestions); err != nil {
			return err
		}
	}

	return confirmOutput(w, outputPath)
}
