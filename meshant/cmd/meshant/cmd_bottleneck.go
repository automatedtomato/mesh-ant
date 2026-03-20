package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// cmdBottleneck implements the "bottleneck" subcommand.
//
// It articulates an observer-situated graph from the traces file, calls
// IdentifyBottlenecks to surface provisionally central elements, and prints
// the notes via PrintBottleneckNotes.
//
// Unlike cmdShadow, --observer is optional here. Omitting it means a full
// cut (all traces included, no observer filter), which is a deliberate
// analytical choice — not an error.
//
// It accepts the following flags:
//   - --observer (repeatable, optional) — observer position(s) for articulation
//   - --tag      (repeatable, optional) — tag filter (any-match / OR semantics)
//   - --from, --to (optional, RFC3339) — time window
//   - --output (optional)             — write output to file instead of stdout
//
// Returns an error if a time flag is not RFC3339, the path is missing or
// unloadable, or writing fails.
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

	if err := fs.Parse(args); err != nil {
		return err
	}

	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("bottleneck: %w", err)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("bottleneck: path to traces.json required\n\nUsage: meshant bottleneck [--observer <pos>] [--tag <tag>] [--from RFC3339] [--to RFC3339] [--output <file>] <traces.json>")
	}
	path := remaining[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("bottleneck: %w", err)
	}

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
