// Package main is the meshant CLI entry point.
//
// It provides subcommands for trace-first network analysis:
//   - summarize:  load traces from a JSON file and print a mesh summary
//   - validate:   load and validate all traces, reporting success or error
//   - articulate: articulate an observer-situated graph from traces
//
// The testable logic lives in run(), cmdSummarize(), cmdValidate(), and
// cmdArticulate(). main() itself is a thin wrapper that wires os.Stdout and
// os.Args, then exits non-zero on error — a pattern that makes every
// meaningful path independently testable without I/O redirection.
//
// Usage:
//
//	meshant <command> [flags] <traces.json>
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// stringSliceFlag is a custom flag.Value that accumulates string values on
// each Set() call. This enables repeatable flags like --observer a --observer b,
// which is more ergonomic than a single comma-separated value for names that
// may themselves contain commas.
type stringSliceFlag []string

// String returns the accumulated values joined by commas. Required by flag.Value.
func (f *stringSliceFlag) String() string { return strings.Join(*f, ",") }

// Set appends a new value to the slice. Required by flag.Value.
func (f *stringSliceFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// parseTimeFlag parses an RFC3339 timestamp string for a named flag.
// Returns a clear error message with the flag name and a formatting hint
// so users understand exactly what format is expected.
func parseTimeFlag(name, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --%s value %q: expected RFC3339 (e.g. 2026-04-14T00:00:00Z)", name, value)
	}
	return t, nil
}

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run is the testable entry point for the CLI. w receives all output.
// It dispatches to the appropriate subcommand handler based on args[0].
// Returns an error if no arguments are provided, if the subcommand is
// unknown, or if the subcommand handler itself returns an error.
func run(w io.Writer, args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "summarize":
		return cmdSummarize(w, args[1:])
	case "validate":
		return cmdValidate(w, args[1:])
	case "articulate":
		return cmdArticulate(w, args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage())
	}
}

// usage returns the CLI usage text as a string. Keeping it as a pure
// string-returning function allows it to be embedded in error messages
// without going through an io.Writer, which simplifies error construction.
func usage() string {
	return `meshant — trace-first network analysis

Usage:
  meshant <command> [flags] <traces.json>

Commands:
  summarize   load traces and print mesh summary
  validate    validate all traces and report errors
  articulate  articulate an observer-situated graph (flags: --observer, --from, --to, --format)
  diff        diff two articulations (flags: --observer-a, --observer-b, --from-a, --to-a, --from-b, --to-b, --format)

Run 'meshant <command> --help' for command-specific flags.`
}

// usageError wraps the usage text in an error. Returned when run() is called
// with no arguments so the caller always gets a non-nil error and the usage
// text is the error message.
func usageError() error {
	return fmt.Errorf("%s", usage())
}

// cmdSummarize implements the "summarize" subcommand.
//
// It expects args[0] to be a path to a JSON traces file. It loads the traces
// using loader.Load (which also validates them), builds a mesh summary via
// loader.Summarise, and writes the formatted summary to w via
// loader.PrintSummary.
//
// Returns an error if no path is provided, if the file cannot be loaded or
// decoded, if any trace fails validation, or if writing to w fails.
func cmdSummarize(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("summarize: path to traces.json required\n\nUsage: meshant summarize <traces.json>")
	}
	path := args[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("summarize: %w", err)
	}

	summary := loader.Summarise(traces)
	if err := loader.PrintSummary(w, summary); err != nil {
		return fmt.Errorf("summarize: %w", err)
	}
	return nil
}

// cmdValidate implements the "validate" subcommand.
//
// It expects args[0] to be a path to a JSON traces file. loader.Load already
// validates every trace during decoding — if it returns without error, all
// traces are valid. On success, cmdValidate writes a one-line confirmation
// message to w naming the trace count.
//
// Returns an error if no path is provided, if the file cannot be loaded, or
// if any trace fails validation (surfaced by loader.Load).
func cmdValidate(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("validate: path to traces.json required\n\nUsage: meshant validate <traces.json>")
	}
	path := args[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	fmt.Fprintf(w, "%d traces: all valid\n", len(traces))
	return nil
}

// cmdArticulate implements the "articulate" subcommand.
//
// It accepts the following flags:
//   - --observer (repeatable, required) — one or more observer positions to include
//   - --from     (optional, RFC3339)    — start of time window
//   - --to       (optional, RFC3339)    — end of time window
//   - --format   (optional, default "text") — output format: text|json|dot|mermaid
//
// The positional argument (after all flags) must be the path to a traces JSON file.
//
// The time window is applied as an AND filter with the observer filter. Providing
// only --from or only --to sets a half-open window (the other end is zero, which
// graph.TimeWindow treats as unbounded for that side).
//
// Returns an error if: no --observer is given, a time flag is not RFC3339,
// the time window is invalid (Start > End), the path is missing or unloadable,
// or the format is unrecognised.
func cmdArticulate(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("articulate", flag.ContinueOnError)

	// --observer is repeatable; collected by stringSliceFlag.
	var observers stringSliceFlag
	fs.Var(&observers, "observer", "observer position to include (repeatable)")

	// --from / --to are optional RFC3339 timestamps for time-window filtering.
	var fromStr, toStr string
	fs.StringVar(&fromStr, "from", "", "start of time window (RFC3339)")
	fs.StringVar(&toStr, "to", "", "end of time window (RFC3339)")

	// --format selects the output renderer; defaults to "text".
	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json|dot|mermaid")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// --observer is mandatory: a graph requires at least one observer position.
	if len(observers) == 0 {
		return fmt.Errorf("articulate: --observer is required")
	}

	// Parse optional time window flags.
	var tw graph.TimeWindow
	if fromStr != "" {
		t, err := parseTimeFlag("from", fromStr)
		if err != nil {
			return err
		}
		tw.Start = t
	}
	if toStr != "" {
		t, err := parseTimeFlag("to", toStr)
		if err != nil {
			return err
		}
		tw.End = t
	}

	// Validate the window only when both ends are set; a half-open window is
	// valid (e.g. --from only means "from this point onward").
	if !tw.Start.IsZero() && !tw.End.IsZero() {
		if err := tw.Validate(); err != nil {
			return fmt.Errorf("articulate: %w", err)
		}
	}

	// The positional argument after all flags is the path to the dataset.
	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("articulate: path to traces.json required\n\nUsage: meshant articulate --observer <pos> [--from RFC3339] [--to RFC3339] [--format text|json|dot|mermaid] <traces.json>")
	}
	path := remaining[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string(observers),
		TimeWindow:        tw,
	}
	g := graph.Articulate(traces, opts)

	// Dispatch to the requested output renderer.
	switch format {
	case "text":
		return graph.PrintArticulation(w, g)
	case "json":
		return graph.PrintGraphJSON(w, g)
	case "dot":
		return graph.PrintGraphDOT(w, g)
	case "mermaid":
		return graph.PrintGraphMermaid(w, g)
	default:
		return fmt.Errorf("articulate: unknown --format %q (text|json|dot|mermaid)", format)
	}
}
