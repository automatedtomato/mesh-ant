package main

import (
	"fmt"
	"io"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// cmdSummarize implements the "summarize" subcommand.
// Loads traces from args[0], builds a mesh summary, and writes it to w.
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
