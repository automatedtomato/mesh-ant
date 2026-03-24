package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// cmdStore implements the "store" subcommand.
//
// Reads canonical Traces from a JSON file (via loader.Load) and writes them
// to the connected database via TraceStore. The operation is idempotent on
// trace ID: re-running against the same file does not create duplicates (MERGE
// semantics in the Neo4j backend; upsert semantics in JSONFileStore).
//
// ts may be nil; a real store is then constructed from --db (or MESHANT_DB_URL
// env var). Tests inject a pre-built store to avoid requiring a running Neo4j
// instance.
func cmdStore(w io.Writer, ts store.TraceStore, args []string) error {
	fs := flag.NewFlagSet("store", flag.ContinueOnError)

	// --db accepts the Bolt URL; MESHANT_DB_URL is read as default so the flag
	// can be omitted when the env var is configured. Credentials (user, pass,
	// db name) are always read from env — never as flags — to avoid leaking
	// them into process listings or shell history.
	var dbURL string
	fs.StringVar(&dbURL, "db", os.Getenv("MESHANT_DB_URL"),
		"Neo4j Bolt URL (or set MESHANT_DB_URL); credentials via MESHANT_DB_USER/MESHANT_DB_PASS")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("store: path to traces.json required\n\nUsage: meshant store [--db bolt://...] <traces.json>")
	}
	path := remaining[0]

	// Open the store when not injected by a caller (e.g. tests).
	if ts == nil {
		if dbURL == "" {
			return fmt.Errorf("store: --db is required (or set MESHANT_DB_URL)")
		}
		s, err := openDB(context.Background(), dbURL)
		if err != nil {
			return fmt.Errorf("store: %w", err)
		}
		defer s.Close()
		ts = s
	}

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}

	if err := ts.Store(context.Background(), traces); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	_, err = fmt.Fprintf(w, "stored %d trace(s)\n", len(traces))
	return err
}
