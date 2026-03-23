package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/automatedtomato/mesh-ant/meshant/serve"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// cmdServe implements the "serve" subcommand.
//
// Starts a localhost HTTP server backed by a DB connection or a JSON file.
// Every endpoint enforces the ANT constraint: no graph is returned without a
// named observer position. See docs/decisions/serve-v1.md.
//
// The --db flag and the <traces.json> positional argument are mutually
// exclusive — the same pattern as all other analytical commands.
func cmdServe(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)

	var dbURL string
	fs.StringVar(&dbURL, "db", os.Getenv("MESHANT_DB_URL"),
		"Neo4j Bolt URL; mutually exclusive with <traces.json> (or set MESHANT_DB_URL)")

	var portStr string
	fs.StringVar(&portStr, "port", "8080", "port to listen on (default 8080)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate port early — before opening any store.
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("serve: --port %q is not a valid port number", portStr)
	}

	remaining := fs.Args()
	if dbURL != "" && len(remaining) > 0 {
		return fmt.Errorf("serve: --db and <file> are mutually exclusive")
	}
	if dbURL == "" && len(remaining) == 0 {
		return fmt.Errorf("serve: path to traces.json or --db required\n\nUsage: meshant serve [--db bolt://...] [--port 8080] <traces.json>")
	}

	// Open the trace store. For a JSON file, wrap it in a JSONFileStore so the
	// serve package receives a uniform TraceStore interface.
	var ts store.TraceStore
	if dbURL != "" {
		s, err := openDB(context.Background(), dbURL)
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		ts = s
	} else {
		ts = store.NewJSONFileStore(remaining[0])
	}
	defer ts.Close()

	srv := serve.NewServer(ts)
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: srv,
	}

	// Set up graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(w, "meshant serve listening on http://localhost:%d\n", port)

	// Start server in a goroutine; main goroutine waits for the shutdown signal.
	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		stop()
		if err := httpSrv.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("serve: shutdown: %w", err)
		}
		return <-errCh
	}
}
