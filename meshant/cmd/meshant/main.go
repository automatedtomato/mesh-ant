// Package main is the meshant CLI entry point.
//
// Subcommand implementations live in cmd_*.go files within this package.
// Shared helpers (loadCriterionFile, stringSliceFlag, parseTimeWindow, etc.)
// are defined here and available to all subcommand files in package main.
//
// main() is a thin wrapper that wires os.Stdout and os.Args, then exits
// non-zero on error. Testable logic lives in run() and each cmd* function.
//
// Usage:
//
//	meshant <command> [flags] <file.json>
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// maxCriterionBytes caps the size of a criterion JSON file read by
// loadCriterionFile. 1 MiB is generous for a human-authored declaration.
const maxCriterionBytes = 1 * 1024 * 1024

// noop is a no-op cleanup function returned by loadTraces when no store
// resources need releasing (e.g. the JSON file path).
var noop = func() {}

// loadTraces resolves []schema.Trace from either a Neo4j database URL or a
// JSON file path. dbURL and fileArgs are mutually exclusive at the call site —
// callers validate this before calling loadTraces.
//
// When dbURL is non-empty, openDB is called and all traces are fetched via
// Query(ctx, QueryOpts{}). No pre-filtering is applied at the store layer;
// the analytical engine performs all cut logic on the full substrate. The
// returned cleanup function closes the store; callers must defer it.
//
// When dbURL is empty, fileArgs[0] is loaded via loader.Load. The cleanup
// function is a no-op.
func loadTraces(ctx context.Context, dbURL string, fileArgs []string) ([]schema.Trace, func(), error) {
	if dbURL != "" {
		ts, err := openDB(ctx, dbURL)
		if err != nil {
			return nil, noop, err
		}
		traces, err := ts.Query(ctx, store.QueryOpts{})
		if err != nil {
			ts.Close()
			return nil, noop, fmt.Errorf("query db: %w", err)
		}
		return traces, func() { ts.Close() }, nil
	}
	traces, err := loader.Load(fileArgs[0])
	return traces, noop, err
}

// loadCriterionFile reads and decodes a JSON EquivalenceCriterion from path.
// Fails if the file is unreadable, contains unknown fields (DisallowUnknownFields
// — precision matters for an interpretive declaration), decodes to zero-value,
// or has a layer ordering violation. Reads are capped at maxCriterionBytes.
func loadCriterionFile(path string) (graph.EquivalenceCriterion, error) {
	f, err := os.Open(path)
	if err != nil {
		return graph.EquivalenceCriterion{}, fmt.Errorf("criterion-file: cannot open %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, maxCriterionBytes)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()

	var c graph.EquivalenceCriterion
	if err := dec.Decode(&c); err != nil {
		return graph.EquivalenceCriterion{}, fmt.Errorf("criterion-file: malformed JSON in %q: %w", path, err)
	}

	if c.IsZero() {
		return graph.EquivalenceCriterion{}, fmt.Errorf(
			"criterion-file: %q decoded to a zero-value criterion — file must contain at least a declaration (or a name as a handle)",
			path,
		)
	}

	if err := c.Validate(); err != nil {
		return graph.EquivalenceCriterion{}, fmt.Errorf("criterion-file: %w", err)
	}

	return c, nil
}

// stringSliceFlag is a custom flag.Value that accumulates values on each Set()
// call, enabling repeatable flags like --observer a --observer b.
type stringSliceFlag []string

// String returns accumulated values joined by commas. Required by flag.Value.
func (f *stringSliceFlag) String() string { return strings.Join(*f, ",") }

// Set appends v to the slice. Rejects blank values so --observer "" is caught
// early rather than producing an empty-observer articulation. Required by flag.Value.
func (f *stringSliceFlag) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New("value must not be empty")
	}
	*f = append(*f, v)
	return nil
}

// parseTimeFlag parses an RFC3339 timestamp for a named flag, returning a
// user-readable error with a formatting hint on failure.
func parseTimeFlag(name, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --%s value %q: expected RFC3339 (e.g. 2026-04-14T00:00:00Z): %w", name, value, err)
	}
	return t, nil
}

// parseTimeWindow parses a pair of RFC3339 strings into a graph.TimeWindow.
// Either end may be empty (half-open window). Both ends are validated only
// when both are non-zero.
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
	// Validate only when both bounds are set; a half-open window is valid.
	if !tw.Start.IsZero() && !tw.End.IsZero() {
		if err := tw.Validate(); err != nil {
			return graph.TimeWindow{}, err
		}
	}
	return tw, nil
}

// outputWriter returns w when outputPath is empty, or creates and returns
// a file at outputPath. The caller is responsible for closing the file.
// Uses 0o600 permissions — consistent with writeSessionRecord — since output
// files may contain LLM-extracted observations from sensitive source material.
func outputWriter(w io.Writer, outputPath string) (io.Writer, error) {
	if outputPath == "" {
		return w, nil
	}
	f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot create output file: %w", err)
	}
	return f, nil
}

// confirmOutput prints a "wrote <path>" message to w when outputPath is set.
// No-op when output goes to stdout.
func confirmOutput(w io.Writer, outputPath string) error {
	if outputPath == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "wrote %s\n", outputPath)
	return err
}

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run is the testable CLI entry point; dispatches to the appropriate
// subcommand handler based on args[0].
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
	case "follow":
		return cmdFollow(w, args[1:])
	case "draft":
		return cmdDraft(w, args[1:])
	case "promote":
		return cmdPromote(w, args[1:])
	case "rearticulate":
		return cmdRearticulate(w, args[1:])
	case "lineage":
		return cmdLineage(w, args[1:])
	case "shadow":
		return cmdShadow(w, args[1:])
	case "gaps":
		return cmdGaps(w, args[1:])
	case "bottleneck":
		return cmdBottleneck(w, args[1:])
	case "extraction-gap":
		return cmdExtractionGap(w, args[1:])
	case "chain-diff":
		return cmdChainDiff(w, args[1:])
	case "review":
		// os.Stdin is passed so interactive prompts can read from the terminal;
		// tests inject a strings.Reader instead.
		return cmdReview(w, os.Stdin, args[1:])
	case "extract":
		// nil client: real AnthropicClient is constructed from env at runtime;
		// tests inject a mock.
		return cmdExtract(w, nil, args[1:])
	case "assist":
		// nil client + os.Stdin: real client from env, interactive input from
		// terminal; tests inject mock client and strings.Reader.
		return cmdAssist(w, nil, os.Stdin, args[1:])
	case "critique":
		// nil client: real AnthropicClient from env; tests inject a mock.
		return cmdCritique(w, nil, args[1:])
	case "split":
		// nil client: real AnthropicClient from env; tests inject a mock.
		return cmdSplit(w, nil, args[1:])
	case "promote-session":
		return cmdPromoteSession(w, args[1:])
	case "convert":
		return cmdConvert(w, args[1:])
	case "store":
		// nil store: a real TraceStore is constructed from --db at runtime;
		// tests inject a pre-built store.
		return cmdStore(w, nil, args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage())
	}
}

// usage returns the CLI usage text, suitable for embedding in error messages.
func usage() string {
	return `meshant — trace-first network analysis

Usage:
  meshant <command> [flags] <file.json>

Commands:
  summarize   load traces and print mesh summary
  validate    validate all traces and report errors
  articulate  articulate an observer-situated graph (flags: --observer, --tag, --from, --to, --format, --output, --narrative)
  diff        diff two articulations (flags: --observer-a, --observer-b, --tag-a, --tag-b, --from-a, --to-a, --from-b, --to-b, --format, --output)
  follow      follow a translation chain through an articulation (flags: --observer, --tag, --from, --to, --element, --direction, --depth, --format, --criterion-file, --output)
  draft       ingest extraction JSON and produce TraceDraft records (flags: --source-doc, --extracted-by, --stage, --output)
  promote     promote TraceDraft records to canonical Traces (flags: --output)
  shadow      summarise shadowed elements from an observer-situated articulation (flags: --observer, --tag, --from, --to, --output)
  gaps        compare element visibility between two observer positions (flags: --observer-a, --observer-b, --tag-a, --tag-b, --from-a, --to-a, --from-b, --to-b, --suggest, --output)
  bottleneck      identify provisionally central elements from an articulation (flags: --observer, --tag, --from, --to, --output)
  review          interactively accept/edit/skip weak-draft records (flags: --output)
  extraction-gap  compare extraction positions across a shared draft set (flags: --analyst-a, --analyst-b, --output)
  chain-diff      compare derivation-chain classifications across two analyst positions (flags: --analyst-a, --analyst-b, --span, --output)
  extract         call LLM to produce TraceDraft records from a source document (flags: --source-doc, --source-doc-ref, --prompt-template, --model, --criterion-file, --output, --session-output)
  assist          interactively refine spans into TraceDraft records with LLM assistance (flags: --spans-file, --prompt-template, --model, --source-doc-ref, --criterion-file, --output, --session-output)
  critique        call LLM to produce "critiqued" derived drafts from existing TraceDrafts (flags: --input, --prompt-template, --model, --source-doc-ref, --criterion-file, --output, --session-output, --id)
  split            call LLM to split a source document into observation spans (flags: --source-doc, --source-doc-ref, --prompt-template, --model, --output, --session-output)
  promote-session  promote a SessionRecord to a canonical Trace (flags: --session-file, --observer, --output)
  convert          convert a non-text source to plain text (flags: --adapter, --source-doc, --output)
  store            load traces from JSON and write to database (flags: --db)

Run 'meshant <command> --help' for command-specific flags.`
}
