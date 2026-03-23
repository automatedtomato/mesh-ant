package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// cmdFollow implements the "follow" subcommand.
//
// Follows a translation chain through an articulated graph from a named
// element. Each step is classified as intermediary-like, mediator-like, or
// translation. Accepts --observer (repeatable, required), --tag, --from/--to
// (RFC3339), --element (required), --direction (forward|backward), --depth
// (0=unlimited), --format (text|json), --criterion-file, and --output.
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

	// --criterion-file attaches a criterion to ClassifyOptions so it appears in
	// chain output. Without this flag, no criterion block is rendered (v1 behaviour).
	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to JSON file containing an EquivalenceCriterion declaration")

	var dbURL string
	fs.StringVar(&dbURL, "db", os.Getenv("MESHANT_DB_URL"),
		"Neo4j Bolt URL; mutually exclusive with <traces.json> (or set MESHANT_DB_URL)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(observers) == 0 {
		return fmt.Errorf("follow: --observer is required")
	}
	if element == "" {
		return fmt.Errorf("follow: --element is required")
	}

	var dir graph.Direction
	switch direction {
	case "forward":
		dir = graph.DirectionForward
	case "backward":
		dir = graph.DirectionBackward
	default:
		return fmt.Errorf("follow: unknown --direction %q (forward|backward)", direction)
	}

	switch format {
	case "text", "json":
		// valid
	default:
		return fmt.Errorf("follow: unknown --format %q (text|json)", format)
	}

	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}

	var criterion graph.EquivalenceCriterion
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("follow: %w", err)
		}
		criterion = c
	}

	remaining := fs.Args()
	if dbURL != "" && len(remaining) > 0 {
		return fmt.Errorf("follow: --db and <file> are mutually exclusive")
	}
	if dbURL == "" && len(remaining) == 0 {
		return fmt.Errorf("follow: path to traces.json or --db required\n\nUsage: meshant follow --observer <pos> --element <name> [--tag <tag>] [--from RFC3339] [--to RFC3339] [--direction forward|backward] [--depth N] [--format text|json] [--criterion-file <file>] [--output <file>] [--db bolt://...] <traces.json>")
	}

	traces, closeStore, err := loadTraces(context.Background(), dbURL, remaining)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}
	defer closeStore()

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
