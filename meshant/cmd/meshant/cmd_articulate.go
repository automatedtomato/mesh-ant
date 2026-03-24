package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// cmdArticulate implements the "articulate" subcommand.
//
// Produces an observer-situated graph from a traces file. Accepts --observer
// (repeatable, required), --tag, --from/--to (RFC3339), --format
// (text|json|dot|mermaid), --output, and --narrative (text only).
func cmdArticulate(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("articulate", flag.ContinueOnError)

	var observers stringSliceFlag
	fs.Var(&observers, "observer", "observer position to include (repeatable)")

	var tags stringSliceFlag
	fs.Var(&tags, "tag", "tag filter (repeatable, any-match / OR semantics)")

	var fromStr, toStr string
	fs.StringVar(&fromStr, "from", "", "start of time window (RFC3339)")
	fs.StringVar(&toStr, "to", "", "end of time window (RFC3339)")

	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json|dot|mermaid")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. graph.dot)")

	// --narrative is silently skipped for non-text formats (narrative prose is
	// not meaningful inside JSON, DOT, or Mermaid output).
	var narrative bool
	fs.BoolVar(&narrative, "narrative", false, "append a provisional narrative draft (text format only)")

	// --db switches the trace source from a JSON file to a Neo4j database.
	// Mutually exclusive with the <traces.json> positional argument.
	// Credentials are read from MESHANT_DB_USER/MESHANT_DB_PASS env vars.
	var dbURL string
	fs.StringVar(&dbURL, "db", os.Getenv("MESHANT_DB_URL"),
		"Neo4j Bolt URL; mutually exclusive with <traces.json> (or set MESHANT_DB_URL)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(observers) == 0 {
		return fmt.Errorf("articulate: --observer is required")
	}

	switch format {
	case "text", "json", "dot", "mermaid":
		// valid
	default:
		return fmt.Errorf("articulate: unknown --format %q (text|json|dot|mermaid)", format)
	}

	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}

	remaining := fs.Args()
	if dbURL != "" && len(remaining) > 0 {
		return fmt.Errorf("articulate: --db and <file> are mutually exclusive")
	}
	if dbURL == "" && len(remaining) == 0 {
		return fmt.Errorf("articulate: path to traces.json or --db required\n\nUsage: meshant articulate --observer <pos> [--tag <tag>] [--from RFC3339] [--to RFC3339] [--format text|json|dot|mermaid] [--output <file>] [--db bolt://...] <traces.json>")
	}

	traces, closeStore, err := loadTraces(context.Background(), dbURL, remaining)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}
	defer closeStore()

	opts := graph.ArticulationOptions{
		ObserverPositions: []string(observers),
		TimeWindow:        tw,
		Tags:              []string(tags),
	}
	g := graph.Articulate(traces, opts)

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

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

	if narrative && format == "text" {
		n := graph.DraftNarrative(g)
		if _, err := fmt.Fprintln(dest, ""); err != nil {
			return err
		}
		if err := graph.PrintNarrativeDraft(dest, n); err != nil {
			return err
		}
	}

	return confirmOutput(w, outputPath)
}
