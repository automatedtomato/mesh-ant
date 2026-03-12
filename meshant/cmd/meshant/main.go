// Package main is the meshant CLI entry point.
//
// It provides subcommands for trace-first network analysis:
//   - summarize:  load traces from a JSON file and print a mesh summary
//   - validate:   load and validate all traces, reporting success or error
//   - articulate: articulate an observer-situated graph from traces
//   - diff:       diff two observer-situated articulations of the same trace set
//
// The testable logic lives in run(), cmdSummarize(), cmdValidate(),
// cmdArticulate(), and cmdDiff(). main() itself is a thin wrapper that wires
// os.Stdout and os.Args, then exits non-zero on error — a pattern that makes
// every meaningful path independently testable without I/O redirection.
//
// Usage:
//
//	meshant <command> [flags] <traces.json>
package main

import (
	"errors"
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

// Set appends a new non-empty value to the slice. Required by flag.Value.
// Returns an error if v is empty or blank so that --observer "" is rejected
// early rather than silently producing an empty-observer articulation.
func (f *stringSliceFlag) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New("observer value must not be empty")
	}
	*f = append(*f, v)
	return nil
}

// parseTimeFlag parses an RFC3339 timestamp string for a named flag.
// Returns a clear error message with the flag name and a formatting hint
// so users understand exactly what format is expected. The underlying parse
// error is wrapped so callers can inspect it with errors.Is/errors.As.
func parseTimeFlag(name, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --%s value %q: expected RFC3339 (e.g. 2026-04-14T00:00:00Z): %w", name, value, err)
	}
	return t, nil
}

// parseTimeWindow parses a pair of RFC3339 strings into a graph.TimeWindow.
// fromName/toName are the flag names used in error messages.
// Either string may be empty (half-open window). Both ends are validated
// together only when both are non-zero.
func parseTimeWindow(fromName, fromStr, toName, toStr string) (graph.TimeWindow, error) {
	var tw graph.TimeWindow
	if fromStr != "" {
		t, err := parseTimeFlag(fromName, fromStr)
		if err != nil {
			return graph.TimeWindow{}, err
		}
		tw.Start = t
	}
	if toStr != "" {
		t, err := parseTimeFlag(toName, toStr)
		if err != nil {
			return graph.TimeWindow{}, err
		}
		tw.End = t
	}
	// Validate only when both bounds are set; a half-open window is valid
	// (e.g. --from only means "from this point onward").
	if !tw.Start.IsZero() && !tw.End.IsZero() {
		if err := tw.Validate(); err != nil {
			return graph.TimeWindow{}, err
		}
	}
	return tw, nil
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
		return errors.New(usage())
	}
	switch args[0] {
	case "summarize":
		return cmdSummarize(w, args[1:])
	case "validate":
		return cmdValidate(w, args[1:])
	case "articulate":
		return cmdArticulate(w, args[1:])
	case "diff":
		return cmdDiff(w, args[1:])
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
// Returns an error if: no --observer is given, a time flag is not RFC3339,
// the time window is invalid (Start > End), --format is unrecognised,
// the path is missing or unloadable, or writing to w fails.
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

	// Reject unknown formats before file I/O so the error is immediate.
	switch format {
	case "text", "json", "dot", "mermaid":
		// valid
	default:
		return fmt.Errorf("articulate: unknown --format %q (text|json|dot|mermaid)", format)
	}

	// Parse optional time window flags.
	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
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
	default: // "mermaid"
		return graph.PrintGraphMermaid(w, g)
	}
}

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
//   - --from-a, --to-a (optional, RFC3339) — time window for graph A
//   - --from-b, --to-b (optional, RFC3339) — time window for graph B
//   - --format (optional, default "text")  — output format: text|json
//
// Note: DOT and Mermaid output are NOT available for diffs (PrintDiffDOT and
// PrintDiffMermaid do not exist). Requesting them returns an explicit error.
//
// Returns an error if: --observer-a or --observer-b is missing, a time flag
// is not RFC3339, a time window is invalid (Start > End), --format is "dot"
// or "mermaid" (not supported), --format is unrecognised, the path is missing
// or unloadable, or writing to w fails.
func cmdDiff(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)

	// --observer-a and --observer-b are both repeatable and required.
	var observersA, observersB stringSliceFlag
	fs.Var(&observersA, "observer-a", "observer position for graph A (repeatable)")
	fs.Var(&observersB, "observer-b", "observer position for graph B (repeatable)")

	// Per-side time window flags.
	var fromAStr, toAStr, fromBStr, toBStr string
	fs.StringVar(&fromAStr, "from-a", "", "start of time window for graph A (RFC3339)")
	fs.StringVar(&toAStr, "to-a", "", "end of time window for graph A (RFC3339)")
	fs.StringVar(&fromBStr, "from-b", "", "start of time window for graph B (RFC3339)")
	fs.StringVar(&toBStr, "to-b", "", "end of time window for graph B (RFC3339)")

	// --format selects the output renderer; only text and json are supported.
	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json")

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

	// Reject DOT and Mermaid explicitly before touching the dataset; the
	// message distinguishes "not supported for diffs" from a plain unknown format.
	switch format {
	case "dot", "mermaid":
		return fmt.Errorf("diff: --format %q not supported (text|json only)", format)
	case "text", "json":
		// valid
	default:
		return fmt.Errorf("diff: unknown --format %q (text|json)", format)
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
		return fmt.Errorf("diff: path to traces.json required\n\nUsage: meshant diff --observer-a <pos> --observer-b <pos> [--from-a RFC3339] [--to-a RFC3339] [--from-b RFC3339] [--to-b RFC3339] [--format text|json] <traces.json>")
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
	}
	optsB := graph.ArticulationOptions{
		ObserverPositions: []string(observersB),
		TimeWindow:        twB,
	}
	gA := graph.Articulate(traces, optsA)
	gB := graph.Articulate(traces, optsB)
	d := graph.Diff(gA, gB)

	switch format {
	case "text":
		return graph.PrintDiff(w, d)
	default: // "json"
		return graph.PrintDiffJSON(w, d)
	}
}
