// Package main is the meshant CLI entry point.
//
// It provides subcommands for trace-first network analysis:
//   - summarize:  load traces from a JSON file and print a mesh summary
//   - validate:   load and validate all traces, reporting success or error
//   - articulate: articulate an observer-situated graph from traces
//   - diff:       diff two observer-situated articulations of the same trace set
//   - follow:     follow a translation chain through an articulated graph
//   - draft:      ingest LLM extraction JSON and produce TraceDraft records
//   - promote:    promote TraceDraft records to canonical Traces
//   - rearticulate:  produce a blank critique skeleton for each draft (M12)
//   - lineage:       walk DerivedFrom links and print chains (M12)
//   - bottleneck:      identify provisionally central elements from an articulation (B.1)
//   - review:          interactively accept/edit/skip weak-draft records (A.5)
//   - extraction-gap:  compare extraction positions across a shared draft set (C.2)
//   - chain-diff:      compare derivation-chain classifications across two analyst positions (C.3)
//   - extract:         call an LLM to produce TraceDraft records from a source document (F.2)
//   - assist:          interactively refine span text into TraceDraft records with LLM assistance (F.3)
//   - critique:        call an LLM to produce "critiqued" derived drafts from existing TraceDrafts (F.4)
//
// The testable logic lives in run() and each cmd* function. main() itself is
// a thin wrapper that wires os.Stdout and os.Args, then exits non-zero on
// error — a pattern that makes every meaningful path independently testable
// without I/O redirection.
//
// Subcommand implementations live in cmd_*.go files within this package.
// Shared helpers (loadCriterionFile, stringSliceFlag, parseTimeWindow, etc.)
// are defined here and available to all subcommand files in package main.
//
// Usage:
//
//	meshant <command> [flags] <file.json>
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// maxCriterionBytes caps the size of a criterion JSON file read by
// loadCriterionFile. 1 MiB is generous for a human-authored declaration.
const maxCriterionBytes = 1 * 1024 * 1024

// loadCriterionFile reads a JSON file at path and decodes it into an
// EquivalenceCriterion. Four failure modes, each a distinct error:
//  1. File cannot be opened (non-existent, permissions)
//  2. File contains invalid JSON or a field not in EquivalenceCriterion
//     (DisallowUnknownFields — precision over forward-compatibility
//     tolerance for an interpretive declaration)
//  3. Decoded criterion is zero-value — no interpretive content
//     (hard error: silent fallback not acceptable when --criterion-file
//     was explicitly provided)
//  4. Layer ordering violation: Preserve or Ignore without Declaration
//
// Reads are capped at maxCriterionBytes. A file exceeding that cap is
// truncated at the I/O boundary and produces a "malformed JSON" error
// (unexpected EOF), not a distinct "file too large" error.
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
		return errors.New("value must not be empty")
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

// outputWriter returns the destination writer for command output.
// If outputPath is empty, it returns w (stdout). Otherwise it creates the
// file at outputPath and returns the *os.File. The caller must call
// closeAndConfirm after writing to handle file closing and confirmation.
func outputWriter(w io.Writer, outputPath string) (io.Writer, error) {
	if outputPath == "" {
		return w, nil
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("cannot create output file: %w", err)
	}
	return f, nil
}

// confirmOutput writes a confirmation message to stdout (w) when output was
// written to a file. If outputPath is empty (stdout mode), this is a no-op.
// File closing is handled by the caller's deferred Close.
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
		// cmdReview receives os.Stdin so the interactive prompts can read from
		// the terminal. The extra in parameter keeps the session testable
		// without a real terminal — tests pass a strings.Reader instead.
		return cmdReview(w, os.Stdin, args[1:])
	case "extract":
		// cmdExtract receives a nil client so the real AnthropicClient is
		// constructed from env vars at runtime. Tests inject a mock client.
		return cmdExtract(w, nil, args[1:])
	case "assist":
		// cmdAssist receives a nil client (real AnthropicClient from env) and
		// os.Stdin so the interactive prompts can read from the terminal. The
		// extra in parameter keeps the session testable without a real terminal.
		return cmdAssist(w, nil, os.Stdin, args[1:])
	case "critique":
		// cmdCritique receives a nil client so the real AnthropicClient is
		// constructed from env vars at runtime. Tests inject a mock client.
		return cmdCritique(w, nil, args[1:])
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

Run 'meshant <command> --help' for command-specific flags.`
}
