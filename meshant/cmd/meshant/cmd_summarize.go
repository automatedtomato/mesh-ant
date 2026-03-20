package main

import (
	"fmt"
	"io"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

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
