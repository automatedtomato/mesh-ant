package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/automatedtomato/mesh-ant/meshant/mcp"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// cmdMcp implements the "mcp" subcommand.
//
// Starts a Model Context Protocol server on stdio. LLM clients (Claude Code,
// Cursor, Cline) invoke this as a subprocess and call MeshAnt's analytical
// engine as MCP tools without shell commands.
//
// --analyst is required (D2 in mcp-v1.md). An LLM client that does not
// declare an analyst position is hiding the cut — the server refuses to start.
//
// The --db flag and the <traces.json> positional argument are mutually
// exclusive — the same pattern as all other analytical commands.
//
// Signal context: SIGINT/SIGTERM cancel the server Run loop (stdio closes,
// Run returns).
func cmdMcp(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)

	var analyst string
	fs.StringVar(&analyst, "analyst", "",
		"Name of the human or agent requesting cuts. Required — hides the cut if absent.")

	var dbURL string
	fs.StringVar(&dbURL, "db", os.Getenv("MESHANT_DB_URL"),
		"Neo4j Bolt URL; mutually exclusive with <traces.json> (or set MESHANT_DB_URL)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// D2: --analyst is required. Hard refusal, not soft suggestion.
	// The error message must state the epistemic obligation explicitly.
	if analyst == "" {
		return fmt.Errorf(`meshant mcp: --analyst is required
An LLM client calling MeshAnt without declaring an analyst position is not
performing an articulation — it is hiding the cut. Provide --analyst <name>.`)
	}

	remaining := fs.Args()

	// --db and <file> are mutually exclusive.
	if dbURL != "" && len(remaining) > 0 {
		return fmt.Errorf("mcp: --db and <file> are mutually exclusive")
	}
	// One of --db or <file> is required.
	if dbURL == "" && len(remaining) == 0 {
		return fmt.Errorf("mcp: path to traces.json or --db required\n\nUsage: meshant mcp --analyst <name> [--db bolt://...] <traces.json>")
	}

	// Open the trace store. The mcp package receives a uniform TraceStore
	// interface regardless of backend — mirrors serve and store commands.
	var ts store.TraceStore
	if dbURL != "" {
		s, err := openDB(context.Background(), dbURL)
		if err != nil {
			return fmt.Errorf("mcp: %w", err)
		}
		ts = s
	} else {
		ts = store.NewJSONFileStore(remaining[0])
	}
	defer ts.Close()

	srv := mcp.NewServer(ts, analyst)

	// Set up cancellation on SIGINT/SIGTERM. When the signal arrives, the
	// context is cancelled and srv.Run returns after the current message.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Startup banner goes to stderr — stdout is the MCP framing channel.
	// Writing plain text to stdout before the JSON-RPC loop begins would
	// corrupt the framing and cause any compliant MCP client to disconnect.
	fmt.Fprintf(os.Stderr, "meshant mcp: analyst=%q listening on stdio\n", analyst)

	// Run reads from os.Stdin and writes to os.Stdout.
	// The process exits when the client closes stdin (EOF) or a signal arrives.
	if err := srv.Run(ctx, os.Stdin, os.Stdout); err != nil {
		// Context cancellation is a normal shutdown path — not an error.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("mcp: %w", err)
	}
	return nil
}
