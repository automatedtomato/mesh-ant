package main

import (
	"fmt"
	"io"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// cmdValidate implements the "validate" subcommand.
// Loads and validates all traces from args[0]; prints a count on success.
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
