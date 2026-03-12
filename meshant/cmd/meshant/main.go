// Package main is the meshant CLI entry point.
//
// It provides two subcommands for the current milestone (M9.1):
//   - summarize: load traces from a JSON file and print a mesh summary
//   - validate:  load and validate all traces, reporting success or error
//
// Two additional commands are listed in the usage text (articulate, diff) but
// are not implemented in this milestone. They appear in usage so users can see
// the intended full surface area of the CLI.
//
// The testable logic lives in run(), cmdSummarize(), and cmdValidate().
// main() itself is a thin wrapper that wires os.Stdout and os.Args, then
// exits non-zero on error — a pattern that makes every meaningful path
// independently testable without I/O redirection.
//
// Usage:
//
//	meshant <command> [flags] <traces.json>
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

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
