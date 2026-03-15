// Package main is the meshant CLI entry point.
//
// It provides subcommands for trace-first network analysis:
//   - summarize:  load traces from a JSON file and print a mesh summary
//   - validate:   load and validate all traces, reporting success or error
//   - articulate: articulate an observer-situated graph from traces
//   - diff:       diff two observer-situated articulations of the same trace set
//   - follow:     follow a translation chain through an articulated graph
//
// The testable logic lives in run(), cmdSummarize(), cmdValidate(),
// cmdArticulate(), and cmdDiff(). main() itself is a thin wrapper that wires
// os.Stdout and os.Args, then exits non-zero on error — a pattern that makes
// every meaningful path independently testable without I/O redirection.
//
// Usage:
//
//	meshant <command> [flags] <traces.json>
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
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
  articulate  articulate an observer-situated graph (flags: --observer, --tag, --from, --to, --format, --output)
  diff        diff two articulations (flags: --observer-a, --observer-b, --tag-a, --tag-b, --from-a, --to-a, --from-b, --to-b, --format, --output)
  follow      follow a translation chain through an articulation (flags: --observer, --tag, --from, --to, --element, --direction, --depth, --format, --criterion-file, --output)
  draft       ingest extraction JSON and produce TraceDraft records (flags: --source-doc, --extracted-by, --stage, --output)
  promote     promote TraceDraft records to canonical Traces (flags: --output)

Run 'meshant <command> --help' for command-specific flags.`
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

// cmdArticulate implements the "articulate" subcommand.
//
// It accepts the following flags:
//   - --observer (repeatable, required) — one or more observer positions to include
//   - --tag      (repeatable, optional) — tag filter (any-match / OR semantics)
//   - --from     (optional, RFC3339)    — start of time window
//   - --to       (optional, RFC3339)    — end of time window
//   - --format   (optional, default "text") — output format: text|json|dot|mermaid
//   - --output   (optional)             — write output to file instead of stdout
//
// The positional argument (after all flags) must be the path to a traces JSON file.
//
// Returns an error if: no --observer is given, a time flag is not RFC3339,
// the time window is invalid (Start > End), --format is unrecognised,
// the path is missing or unloadable, or writing to w fails.
func cmdArticulate(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("articulate", flag.ContinueOnError)

	// --observer is repeatable; collected by stringSliceFlag.
	var observers stringSliceFlag
	fs.Var(&observers, "observer", "observer position to include (repeatable)")

	// --tag is repeatable; any-match / OR semantics — a trace passes if it carries any of the specified tags.
	var tags stringSliceFlag
	fs.Var(&tags, "tag", "tag filter (repeatable, any-match / OR semantics)")

	// --from / --to are optional RFC3339 timestamps for time-window filtering.
	var fromStr, toStr string
	fs.StringVar(&fromStr, "from", "", "start of time window (RFC3339)")
	fs.StringVar(&toStr, "to", "", "end of time window (RFC3339)")

	// --format selects the output renderer; defaults to "text".
	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json|dot|mermaid")

	// --output writes output to a file instead of stdout.
	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. graph.dot)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// --observer is mandatory: a graph requires at least one observer position.
	if len(observers) == 0 {
		return fmt.Errorf("articulate: --observer is required")
	}

	// Reject unknown formats before file I/O so the error is immediate.
	switch format {
	case "text", "json", "dot", "mermaid":
		// valid
	default:
		return fmt.Errorf("articulate: unknown --format %q (text|json|dot|mermaid)", format)
	}

	// Parse optional time window flags.
	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}

	// The positional argument after all flags is the path to the dataset.
	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("articulate: path to traces.json required\n\nUsage: meshant articulate --observer <pos> [--tag <tag>] [--from RFC3339] [--to RFC3339] [--format text|json|dot|mermaid] [--output <file>] <traces.json>")
	}
	path := remaining[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string(observers),
		TimeWindow:        tw,
		Tags:              []string(tags),
	}
	g := graph.Articulate(traces, opts)

	// Determine output destination: file or stdout.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("articulate: %w", err)
	}
	// Ensure the file is closed on all exit paths, including render errors.
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	// Dispatch to the requested output renderer.
	switch format {
	case "text":
		err = graph.PrintArticulation(dest, g)
	case "json":
		err = graph.PrintGraphJSON(dest, g)
	case "dot":
		err = graph.PrintGraphDOT(dest, g)
	default: // "mermaid"
		err = graph.PrintGraphMermaid(dest, g)
	}
	if err != nil {
		return err
	}

	// If writing to file, print confirmation to stdout.
	return confirmOutput(w, outputPath)
}

// cmdDiff implements the "diff" subcommand.
//
// It compares two observer-situated articulations of the same trace set,
// producing a structural diff that reveals what each observer can and cannot
// see. This is the core diagnostic operation of the mesh: a diff exposes the
// asymmetry between two positions in a network.
//
// It accepts the following flags:
//   - --observer-a (repeatable, required) — observer positions for graph A
//   - --observer-b (repeatable, required) — observer positions for graph B
//   - --tag-a      (repeatable, optional) — tag filter for graph A (any-match / OR semantics)
//   - --tag-b      (repeatable, optional) — tag filter for graph B (any-match / OR semantics)
//   - --from-a, --to-a (optional, RFC3339) — time window for graph A
//   - --from-b, --to-b (optional, RFC3339) — time window for graph B
//   - --format (optional, default "text")  — output format: text|json|dot|mermaid
//   - --output (optional)                  — write output to file instead of stdout
//
// Returns an error if: --observer-a or --observer-b is missing, a time flag
// is not RFC3339, a time window is invalid (Start > End), --format is
// unrecognised, the path is missing or unloadable, or writing fails.
func cmdDiff(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)

	// --observer-a and --observer-b are both repeatable and required.
	var observersA, observersB stringSliceFlag
	fs.Var(&observersA, "observer-a", "observer position for graph A (repeatable)")
	fs.Var(&observersB, "observer-b", "observer position for graph B (repeatable)")

	// --tag-a and --tag-b are repeatable tag filters per side (any-match / OR semantics).
	var tagsA, tagsB stringSliceFlag
	fs.Var(&tagsA, "tag-a", "tag filter for graph A (repeatable, any-match / OR semantics)")
	fs.Var(&tagsB, "tag-b", "tag filter for graph B (repeatable, any-match / OR semantics)")

	// Per-side time window flags.
	var fromAStr, toAStr, fromBStr, toBStr string
	fs.StringVar(&fromAStr, "from-a", "", "start of time window for graph A (RFC3339)")
	fs.StringVar(&toAStr, "to-a", "", "end of time window for graph A (RFC3339)")
	fs.StringVar(&fromBStr, "from-b", "", "start of time window for graph B (RFC3339)")
	fs.StringVar(&toBStr, "to-b", "", "end of time window for graph B (RFC3339)")

	// --format selects the output renderer.
	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json|dot|mermaid")

	// --output writes output to a file instead of stdout.
	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file (e.g. diff.dot)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Both observer sets are mandatory: a diff requires two distinct cuts.
	if len(observersA) == 0 {
		return fmt.Errorf("diff: --observer-a is required")
	}
	if len(observersB) == 0 {
		return fmt.Errorf("diff: --observer-b is required")
	}

	// Validate format.
	switch format {
	case "text", "json", "dot", "mermaid":
		// valid
	default:
		return fmt.Errorf("diff: unknown --format %q (text|json|dot|mermaid)", format)
	}

	// Parse and validate per-side time windows.
	twA, err := parseTimeWindow("from-a", fromAStr, "to-a", toAStr)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	twB, err := parseTimeWindow("from-b", fromBStr, "to-b", toBStr)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}

	// The positional argument after all flags is the shared trace dataset.
	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("diff: path to traces.json required\n\nUsage: meshant diff --observer-a <pos> --observer-b <pos> [--tag-a <tag>] [--tag-b <tag>] [--from-a RFC3339] [--to-a RFC3339] [--from-b RFC3339] [--to-b RFC3339] [--format text|json|dot|mermaid] [--output <file>] <traces.json>")
	}
	path := remaining[0]

	// Load the shared trace set once. Both articulations derive from the
	// same underlying data; only the cut axes differ.
	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}

	optsA := graph.ArticulationOptions{
		ObserverPositions: []string(observersA),
		TimeWindow:        twA,
		Tags:              []string(tagsA),
	}
	optsB := graph.ArticulationOptions{
		ObserverPositions: []string(observersB),
		TimeWindow:        twB,
		Tags:              []string(tagsB),
	}
	gA := graph.Articulate(traces, optsA)
	gB := graph.Articulate(traces, optsB)
	d := graph.Diff(gA, gB)

	// Determine output destination: file or stdout.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	// Ensure the file is closed on all exit paths, including render errors.
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	switch format {
	case "text":
		err = graph.PrintDiff(dest, d)
	case "json":
		err = graph.PrintDiffJSON(dest, d)
	case "dot":
		err = graph.PrintDiffDOT(dest, d)
	default: // "mermaid"
		err = graph.PrintDiffMermaid(dest, d)
	}
	if err != nil {
		return err
	}

	return confirmOutput(w, outputPath)
}

// cmdFollow implements the "follow" subcommand.
//
// It follows a translation chain through an articulated graph, starting from
// a named element. Each step is classified as intermediary-like, mediator-like,
// or translation. This is Layer 4 — the first analytical operation that reads
// *through* a graph rather than across graphs.
//
// It accepts the following flags:
//   - --observer  (repeatable, required) — observer positions for articulation
//   - --tag       (repeatable, optional) — tag filter (any-match / OR semantics)
//   - --from      (optional, RFC3339)    — start of time window
//   - --to        (optional, RFC3339)    — end of time window
//   - --element   (required)             — starting element name
//   - --direction (optional, default "forward") — "forward" or "backward"
//   - --depth     (optional, default 0)  — max chain depth (0 = unlimited)
//   - --format    (optional, default "text") — output format: text|json
//   - --output    (optional)             — write output to file instead of stdout
func cmdFollow(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("follow", flag.ContinueOnError)

	var observers stringSliceFlag
	fs.Var(&observers, "observer", "observer position to include (repeatable)")

	var tags stringSliceFlag
	fs.Var(&tags, "tag", "tag filter (repeatable, any-match / OR semantics)")

	var fromStr, toStr string
	fs.StringVar(&fromStr, "from", "", "start of time window (RFC3339)")
	fs.StringVar(&toStr, "to", "", "end of time window (RFC3339)")

	var element string
	fs.StringVar(&element, "element", "", "starting element name (required)")

	var direction string
	fs.StringVar(&direction, "direction", "forward", "traversal direction: forward|backward")

	var depth int
	fs.IntVar(&depth, "depth", 0, "max chain depth (0 = unlimited)")

	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write output to file")

	// --criterion-file is an optional path to a JSON file containing an
	// EquivalenceCriterion declaration. When provided, the criterion is
	// loaded, validated, and attached to the ClassifyOptions so it appears
	// in the chain output (text and JSON). Without this flag, the command
	// behaves identically to v1 (no criterion block rendered).
	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to JSON file containing an EquivalenceCriterion declaration")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(observers) == 0 {
		return fmt.Errorf("follow: --observer is required")
	}
	if element == "" {
		return fmt.Errorf("follow: --element is required")
	}

	// Validate direction.
	var dir graph.Direction
	switch direction {
	case "forward":
		dir = graph.DirectionForward
	case "backward":
		dir = graph.DirectionBackward
	default:
		return fmt.Errorf("follow: unknown --direction %q (forward|backward)", direction)
	}

	// Validate format.
	switch format {
	case "text", "json":
		// valid
	default:
		return fmt.Errorf("follow: unknown --format %q (text|json)", format)
	}

	// Parse optional time window.
	tw, err := parseTimeWindow("from", fromStr, "to", toStr)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}

	// Load and validate criterion file when provided. Zero criterion (no flag)
	// is the default and preserves v1 rendering behavior unchanged.
	var criterion graph.EquivalenceCriterion
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("follow: %w", err)
		}
		criterion = c
	}

	// Positional argument: path to traces file.
	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("follow: path to traces.json required\n\nUsage: meshant follow --observer <pos> --element <name> [--tag <tag>] [--from RFC3339] [--to RFC3339] [--direction forward|backward] [--depth N] [--format text|json] [--criterion-file <file>] [--output <file>] <traces.json>")
	}
	path := remaining[0]

	traces, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}

	// Articulate the graph, then follow the chain.
	opts := graph.ArticulationOptions{
		ObserverPositions: []string(observers),
		TimeWindow:        tw,
		Tags:              []string(tags),
	}
	g := graph.Articulate(traces, opts)

	chain := graph.FollowTranslation(g, element, graph.FollowOptions{
		Direction: dir,
		MaxDepth:  depth,
	})

	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{Criterion: criterion})

	// Determine output destination.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	switch format {
	case "text":
		err = graph.PrintChain(dest, cc)
	default: // "json"
		err = graph.PrintChainJSON(dest, cc)
	}
	if err != nil {
		return err
	}

	return confirmOutput(w, outputPath)
}

// cmdDraft implements the "draft" subcommand.
//
// It reads an extraction JSON file produced by an external LLM tool (the
// ingestion contract: source_span required, all other fields optional),
// assigns UUIDs and timestamps to records that lack them, applies optional
// field overrides, validates each record, writes the resulting TraceDraft JSON
// array to --output (or stdout), and prints a provenance summary.
//
// The command does not make LLM calls — the LLM's transformation is a named,
// inspectable file on disk, consistent with treating the LLM as a mediator
// rather than a hidden intermediary. See docs/decisions/tracedraft-v1.md.
//
// Flags:
//   - --source-doc <ref>     stamp SourceDocRef on all drafts
//   - --extracted-by <label> override ExtractedBy on all drafts
//   - --stage <stage>        override ExtractionStage on all drafts
//   - --output <file>        write TraceDraft JSON to file (default: stdout)
func cmdDraft(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("draft", flag.ContinueOnError)

	var sourceDoc string
	fs.StringVar(&sourceDoc, "source-doc", "", "document identifier stamped on all drafts (SourceDocRef)")

	var extractedBy string
	fs.StringVar(&extractedBy, "extracted-by", "", "override ExtractedBy field for all loaded drafts")

	var stage string
	fs.StringVar(&stage, "stage", "", "override ExtractionStage field for all loaded drafts")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write TraceDraft JSON to file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("draft: path to extraction.json required\n\nUsage: meshant draft [--source-doc <ref>] [--extracted-by <label>] [--stage <stage>] [--output <file>] <extraction.json>")
	}
	path := remaining[0]

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("draft: %w", err)
	}

	if len(drafts) == 0 {
		return fmt.Errorf("draft: %q contains no records", path)
	}

	// Apply optional field overrides to all drafts.
	for i := range drafts {
		if sourceDoc != "" {
			drafts[i].SourceDocRef = sourceDoc
		}
		if extractedBy != "" {
			drafts[i].ExtractedBy = extractedBy
		}
		if stage != "" {
			drafts[i].ExtractionStage = stage
		}
	}

	// Determine output destination: file or stdout.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	// Write TraceDraft JSON array to output destination.
	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(drafts); err != nil {
		return fmt.Errorf("draft: encode output: %w", err)
	}

	// Print provenance summary to w (stdout) regardless of --output.
	// When --output is stdout, the summary follows the JSON on the same stream.
	summary := loader.SummariseDrafts(drafts)
	if err := loader.PrintDraftSummary(w, summary); err != nil {
		return fmt.Errorf("draft: %w", err)
	}

	return confirmOutput(w, outputPath)
}

// cmdPromote implements the "promote" subcommand.
//
// It reads a TraceDraft JSON file, calls IsPromotable on each draft, promotes
// those that qualify to canonical Traces (each carries the "draft" tag as a
// provenance signal), and writes the promoted traces to --output (or stdout).
// A summary reports how many were promoted and names the reasons non-promotable
// drafts were skipped.
//
// Flags:
//   - --output <file>  write promoted traces JSON to file (default: stdout)
func cmdPromote(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("promote", flag.ContinueOnError)

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write promoted traces JSON to file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("promote: path to drafts.json required\n\nUsage: meshant promote [--output <file>] <drafts.json>")
	}
	path := remaining[0]

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}

	var promoted []schema.Trace
	type failedDraft struct {
		idx    int
		id     string
		reason string
	}
	var failures []failedDraft

	for i, d := range drafts {
		tr, err := d.Promote()
		if err != nil {
			failures = append(failures, failedDraft{idx: i, id: d.ID, reason: err.Error()})
			continue
		}
		promoted = append(promoted, tr)
	}

	// Determine output destination: file or stdout.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	// Write promoted traces JSON (empty array if none promoted).
	out := promoted
	if out == nil {
		out = []schema.Trace{}
	}
	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("promote: encode output: %w", err)
	}

	// Print promotion summary to w (stdout).
	fmt.Fprintf(w, "\nPromotion summary: %d promoted, %d not promotable (out of %d)\n",
		len(promoted), len(failures), len(drafts))
	for _, f := range failures {
		fmt.Fprintf(w, "  draft %d (id=%s): %s\n", f.idx, f.id, f.reason)
	}

	return confirmOutput(w, outputPath)
}
