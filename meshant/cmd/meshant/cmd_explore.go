// cmd_explore.go wires the meshant interactive analysis session to the CLI.
//
// Entry point: invoked by run() in main.go when no subcommand is given or when
// the first argument is not a known subcommand (treated as a trace file path).
//
//   meshant                        — REPL with no trace substrate loaded
//   meshant <traces.json>          — REPL backed by the JSON file
//   meshant --db bolt://localhost   — REPL backed by a Neo4j store
//
// All REPL logic lives in meshant/explore/session.go; this file is thin glue:
// parse flags, open the store (if any), start the session.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/automatedtomato/mesh-ant/meshant/explore"
	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// cmdExplore is the CLI entry point for the interactive analysis session.
//
// in is the terminal reader (os.Stdin in production; strings.NewReader in tests).
// w is the output writer (os.Stdout / a buffer in tests).
// args are the arguments after the executable name — no subcommand prefix.
//
// When args is empty the session starts with no substrate loaded; commands that
// require a substrate (articulate, shadow, etc.) will return an inline error.
func cmdExplore(in io.Reader, w io.Writer, args []string) error {
	fs := flag.NewFlagSet("meshant", flag.ContinueOnError)
	fs.SetOutput(w)

	var dbURL string
	var analyst string
	fs.StringVar(&dbURL, "db", "", "Neo4j bolt URL (e.g. bolt://localhost:7687)")
	fs.StringVar(&analyst, "analyst", "", "analyst name (required for `suggest`; sets provenance on all turns)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	fileArgs := fs.Args()

	if dbURL != "" && len(fileArgs) > 0 {
		return fmt.Errorf("meshant: --db and a file path are mutually exclusive")
	}

	ctx := context.Background()

	// Open the store. ts remains nil when neither --db nor a file path is given;
	// NewSession handles nil (commands that need the store will error inline).
	var ts store.TraceStore
	switch {
	case dbURL != "":
		s, err := openDB(ctx, dbURL)
		if err != nil {
			return err
		}
		defer func() { _ = s.Close() }()
		ts = s

	case len(fileArgs) > 0:
		// JSONFileStore opens lazily; no error on a non-existent path at open time
		// (it returns an empty substrate and writes on Store calls). For a read-only
		// explore session over an existing file this is sufficient.
		jfs := store.NewJSONFileStore(fileArgs[0])
		defer func() { _ = jfs.Close() }()
		ts = jfs
	}

	// Build the LLM suggest client. Optional: when the API key is absent or
	// analyst is empty, suggest will refuse gracefully with an inline error.
	// All other REPL commands are unaffected by whether sc is nil.
	var sc explore.SuggestClient
	if analyst != "" {
		lc, lcErr := llm.NewAnthropicClient("claude-haiku-4-5-20251001")
		if lcErr == nil {
			sc = lc
		} else {
			// Analyst is set but key is absent — warn at startup so the analyst
			// knows suggest is unavailable before running any commands.
			fmt.Fprintf(w, "warning: --analyst set but MESHANT_LLM_API_KEY is not configured; 'suggest' command will be unavailable\n")
		}
	}

	s := explore.NewSession(ts, analyst, sc)

	if ts == nil {
		fmt.Fprintln(w, "meshant — no trace substrate loaded")
		fmt.Fprintln(w, "  run 'meshant <traces.json>' to load traces")
	} else {
		fmt.Fprintln(w, "meshant — interactive analysis session")
	}
	fmt.Fprintln(w, "  type 'help' for available commands, 'quit' to exit")

	return s.Run(ctx, in, w)
}
