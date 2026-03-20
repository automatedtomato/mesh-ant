package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// cmdFollow implements the "follow" subcommand.
//
// It follows a translation chain through an articulated graph, starting from
// a named element. Each step is classified as intermediary-like, mediator-like,
// or translation. This is Layer 4 — the first analytical operation that reads
// *through* a graph rather than across graphs.
//
// It accepts the following flags:
//   - --observer  (repeatable, required) — observer positions for articulation
//   - --tag       (repeatable, optional) — tag filter (any-match / OR semantics)
//   - --from      (optional, RFC3339)    — start of time window
//   - --to        (optional, RFC3339)    — end of time window
//   - --element   (required)             — starting element name
//   - --direction (optional, default "forward") — "forward" or "backward"
//   - --depth     (optional, default 0)  — max chain depth (0 = unlimited)
//   - --format    (optional, default "text") — output format: text|json
//   - --output    (optional)             — write output to file instead of stdout
func cmdFollow(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("follow", flag.ContinueOnError)

	var observers stringSliceFlag
	fs.Var(&observers, "observer", "observer position to include (repeatable)")

	var tags stringSliceFlag
	fs.Var(&tags, "tag", "tag filter (repeatable, any-match / OR semantics)")

	var fromStr, toStr string
	fs.StringVar(&fromStr, "from", "", "start of time window (RFC3339)")
	fs.StringVar(&toStr, "to", "", "end of time window (RFC3339)")

	var element string
	fs.StringVar(&element, "element", "", "starting element name (required)")

	var direction string
	fs.StringVar(&direction, "direction", "forward", "traversal direction: forward|backward")

	var depth int
	fs.IntVar(&depth, "depth", 0, "max chain depth (0 = unlimited)")

	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file")

	// --criterion-file is an optional path to a JSON file containing an
	// EquivalenceCriterion declaration. When provided, the criterion is
	// loaded, validated, and attached to the ClassifyOptions so it appears
	// in the chain output (text and JSON). Without this flag, the command
	// behaves identically to v1 (no criterion block rendered).
	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to JSON file containing an EquivalenceCriterion declaration")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(observers) == 0 {
		return fmt.Errorf("follow: --observer is required")
	}
	if element == "" {
		return fmt.Errorf("follow: --element is required")
	}

	// Validate direction.
	var dir graph.Direction
	switch direction {
	case "forward":
		dir = graph.DirectionForward
	case "backward":
		dir = graph.DirectionBackward
	default:
		return fmt.Errorf("follow: unknown --direction %q (forward|backward)", direction)
	}

	// Validate format.
	switch format {
	case "text", "json":
		// valid
	default:
		return fmt.Errorf("follow: unknown --format %q (text|json)", format)
	}

	// Parse optional time window.
	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}

	// Load and validate criterion file when provided. Zero criterion (no flag)
	// is the default and preserves v1 rendering behavior unchanged.
	var criterion graph.EquivalenceCriterion
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("follow: %w", err)
		}
		criterion = c
	}

	// Positional argument: path to traces file.
	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("follow: path to traces.json required\n\nUsage: meshant follow --observer <pos> --element <name> [--tag <tag>] [--from RFC3339] [--to RFC3339] [--direction forward|backward] [--depth N] [--format text|json] [--criterion-file <file>] [--output <file>] <traces.json>")
	}
	path := remaining[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}

	// Articulate the graph, then follow the chain.
	opts := graph.ArticulationOptions{
		ObserverPositions: []string(observers),
		TimeWindow:        tw,
		Tags:              []string(tags),
	}
	g := graph.Articulate(traces, opts)

	chain := graph.FollowTranslation(g, element, graph.FollowOptions{
		Direction: dir,
		MaxDepth:  depth,
	})

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{Criterion: criterion})

	// Determine output destination.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	switch format {
	case "text":
		err = graph.PrintChain(dest, cc)
	default: // "json"
		err = graph.PrintChainJSON(dest, cc)
	}
	if err != nil {
		return err
	}

	return confirmOutput(w, outputPath)
}
