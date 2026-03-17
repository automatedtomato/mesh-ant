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
//
// The testable logic lives in run() and each cmd* function. main() itself is
// a thin wrapper that wires os.Stdout and os.Args, then exits non-zero on
// error — a pattern that makes every meaningful path independently testable
// without I/O redirection.
//
// Usage:
//
//	meshant <command> [flags] <file.json>
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
	case "rearticulate":
		return cmdRearticulate(w, args[1:])
	case "lineage":
		return cmdLineage(w, args[1:])
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

// cmdRearticulate implements the "rearticulate" subcommand.
//
// Re-articulation is a cut, not a correction. This command reads a TraceDraft
// JSON file and produces a skeleton JSON array: one skeleton record per draft,
// with SourceSpan copied verbatim, DerivedFrom set to the original's ID, and
// all content fields left blank. The critiquing agent fills in the
// interpretation. Blank content fields are correct scaffold output — they are
// honest abstentions, not missing data (P3 in plan_m12.md).
//
// Design constraints:
//   - cmdRearticulate must NOT pre-fill content fields from the original (P3)
//   - cmdRearticulate must NOT call Validate() on the skeleton output — the
//     skeleton is intentionally incomplete (blank ID); Validate() would pass
//     since source_span is present, but ID assignment is left to cmdDraft
//   - "reviewed" is the extraction_stage for all skeletons (pipeline position,
//     not a quality claim — Decision 7 in tracedraft-v1.md)
//
// Flags:
//   - --id <id>             produce skeleton for a single draft by ID (default: all)
//   - --output <path>       write skeleton JSON to file (default: stdout)
//   - --criterion-file <path> load an EquivalenceCriterion and set its Name as
//     CriterionRef on each skeleton, making the critique pass self-situated
func cmdRearticulate(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("rearticulate", flag.ContinueOnError)

	var idFilter string
	fs.StringVar(&idFilter, "id", "", "produce skeleton for a single draft by ID")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write skeleton JSON to file (default: stdout)")

	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to EquivalenceCriterion JSON; sets criterion_ref on each skeleton")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("rearticulate: path to drafts.json required\n\nUsage: meshant rearticulate [--id <id>] [--output <file>] <drafts.json>")
	}
	path := remaining[0]

	// Load originals. LoadDrafts assigns UUIDs and validates SourceSpan.
	originals, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("rearticulate: %w", err)
	}

	// Apply --id filter if provided.
	if idFilter != "" {
		var found *schema.TraceDraft
		for i := range originals {
			if originals[i].ID == idFilter {
				found = &originals[i]
				break
			}
		}
		if found == nil {
			return fmt.Errorf("rearticulate: draft with id %q not found in %s", idFilter, path)
		}
		originals = []schema.TraceDraft{*found}
	}

	// Load criterion if provided. criterionName is empty when no flag was given.
	var criterionName string
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("rearticulate: %w", err)
		}
		criterionName = c.Name
	}

	// Build skeleton records. Each skeleton:
	//   - copies SourceSpan verbatim (the invariant, Decision 2)
	//   - copies SourceDocRef if present (ground truth provenance, not interpretation)
	//   - sets DerivedFrom to the original's ID
	//   - sets ExtractionStage to "reviewed" (pipeline position, not quality)
	//   - leaves all content fields blank (P3: no pre-filling)
	//   - leaves ID and ExtractedBy blank (to be assigned by meshant draft)
	//   - sets CriterionRef when --criterion-file was provided (self-situated skeleton)
	skeletons := make([]schema.TraceDraft, len(originals))
	for i, orig := range originals {
		skeletons[i] = schema.TraceDraft{
			SourceSpan:      orig.SourceSpan,
			SourceDocRef:    orig.SourceDocRef,
			DerivedFrom:     orig.ID,
			ExtractionStage: "reviewed",
			CriterionRef:    criterionName,
			// IntentionallyBlank declares which content fields were
			// deliberately left empty by this cut — the critique agent
			// provides its own interpretation. Blank is correct, not incomplete.
			IntentionallyBlank: []string{
				"what_changed", "source", "target",
				"mediation", "observer", "tags",
			},
		}
	}

	// Determine output destination: file or stdout.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("rearticulate: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(skeletons); err != nil {
		return fmt.Errorf("rearticulate: encode output: %w", err)
	}

	return confirmOutput(w, outputPath)
}


// lineageNode holds a draft and its subsequent readings in the DerivedFrom chain.
// Used internally by cmdLineage to build and render chains.
type lineageNode struct {
	draft    schema.TraceDraft
	subsequent []*lineageNode
}

// lineageResult holds the parsed chain structure returned by buildLineage.
// anchors are drafts that start a reading sequence (no DerivedFrom, or prior
// not in dataset). Chain order is positional — earlier readings are not more
// authentic than later ones; they simply came first in the production sequence.
type lineageResult struct {
	anchors    []*lineageNode // drafts starting a reading sequence
	standalone int            // count of anchors with no subsequent readings
}

// buildLineage walks DerivedFrom links in the dataset and constructs a tree.
// Returns an error if a cycle is detected. A cycle is detected using DFS with
// a "currently visiting" set (grey set in standard DFS cycle detection).
func buildLineage(drafts []schema.TraceDraft) (lineageResult, error) {
	// Index drafts by ID for O(1) prior lookup.
	byID := make(map[string]*lineageNode, len(drafts))
	nodes := make([]*lineageNode, len(drafts))
	for i := range drafts {
		n := &lineageNode{draft: drafts[i]}
		nodes[i] = n
		byID[drafts[i].ID] = n
	}

	// Link subsequent readings to their prior readings.
	for _, n := range nodes {
		if n.draft.DerivedFrom == "" {
			continue
		}
		prior, ok := byID[n.draft.DerivedFrom]
		if !ok {
			// Prior not in dataset — treat this draft as a chain anchor.
			continue
		}
		prior.subsequent = append(prior.subsequent, n)
	}

	// Identify anchors: drafts with no DerivedFrom, or whose DerivedFrom is not
	// present in the dataset.
	var anchors []*lineageNode
	for _, n := range nodes {
		if n.draft.DerivedFrom == "" {
			anchors = append(anchors, n)
		} else if _, ok := byID[n.draft.DerivedFrom]; !ok {
			anchors = append(anchors, n)
		}
	}

	// Cycle detection: DFS from every anchor. If we reach a node already in the
	// current path (grey set), a cycle exists. Cycles involving nodes that have
	// no path from any anchor are detected separately via the "visited" set.
	visited := make(map[string]bool, len(drafts))
	for _, root := range anchors {
		if err := detectCycleDFS(root, byID, visited, make(map[string]bool)); err != nil {
			return lineageResult{}, err
		}
	}

	// Check for cycles among unreachable nodes (orphaned cycles: A→B→A with no
	// external root). Any unvisited node is part of a cycle or orphaned cycle.
	for _, n := range nodes {
		if !visited[n.draft.ID] {
			// Attempt DFS from this node to detect and name the cycle.
			if err := detectCycleDFS(n, byID, visited, make(map[string]bool)); err != nil {
				return lineageResult{}, err
			}
		}
	}

	// Count standalone anchors (no subsequent readings).
	standalone := 0
	for _, r := range anchors {
		if len(r.subsequent) == 0 {
			standalone++
		}
	}

	return lineageResult{anchors: anchors, standalone: standalone}, nil
}

// detectCycleDFS performs a depth-first search from node, using the grey set
// (inPath) to detect back edges (cycles). Visited nodes are marked in the
// shared visited map so that each node is processed at most once across all
// DFS calls. byID is used to follow DerivedFrom links not already wired into
// the tree (handles orphaned cycles not reachable from any root).
func detectCycleDFS(n *lineageNode, byID map[string]*lineageNode, visited, inPath map[string]bool) error {
	if inPath[n.draft.ID] {
		return fmt.Errorf("lineage: cycle detected involving draft id %q", n.draft.ID)
	}
	if visited[n.draft.ID] {
		return nil
	}
	visited[n.draft.ID] = true
	inPath[n.draft.ID] = true

	for _, child := range n.subsequent {
		if err := detectCycleDFS(child, byID, visited, inPath); err != nil {
			return err
		}
	}

	// Also follow DerivedFrom links to catch orphaned cycles (A→B→A where
	// neither A nor B is a root). This handles the case where inPath contains
	// an orphaned cycle node reached via DerivedFrom from an unvisited node.
	if n.draft.DerivedFrom != "" {
		if prior, ok := byID[n.draft.DerivedFrom]; ok && !visited[prior.draft.ID] {
			if err := detectCycleDFS(prior, byID, visited, inPath); err != nil {
				return err
			}
		}
	}

	delete(inPath, n.draft.ID)
	return nil
}

// idPrefix returns the first 8 characters of a draft ID for display purposes.
// Returns the full ID if it is shorter than 8 characters.
func idPrefix(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// spanPreview returns the first 60 characters of a source span for display,
// truncating with "..." if longer.
func spanPreview(span string) string {
	// Replace newlines with spaces for single-line display.
	s := strings.ReplaceAll(span, "\n", " ")
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

// printLineageText renders the lineage tree in text format to w.
// Chains are rendered as indented trees with └── connectors.
// Standalone drafts are counted at the end.
func printLineageText(w io.Writer, result lineageResult) error {
	// Chain order is positional (production sequence), not hierarchical.
	// Earlier readings are not more authentic than later ones.
	if _, err := fmt.Fprintln(w, "=== DerivedFrom Chains (positional sequence) ==="); err != nil {
		return err
	}

	// Print chains (anchors with subsequent readings).
	for _, root := range result.anchors {
		if len(root.subsequent) == 0 {
			continue // standalone — printed in summary
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		line := fmt.Sprintf("[%s] %s / %s",
			idPrefix(root.draft.ID),
			root.draft.ExtractionStage,
			root.draft.ExtractedBy,
		)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %q\n", spanPreview(root.draft.SourceSpan)); err != nil {
			return err
		}
		for _, child := range root.subsequent {
			if err := printLineageStep(w, child, "  "); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "Standalone drafts (no DerivedFrom, no subsequent readings): %d\n", result.standalone)
	return err
}

// printLineageStep recursively renders a child node with indentation.
func printLineageStep(w io.Writer, n *lineageNode, indent string) error {
	line := fmt.Sprintf("%s└── [%s] %s / %s",
		indent,
		idPrefix(n.draft.ID),
		n.draft.ExtractionStage,
		n.draft.ExtractedBy,
	)
	if _, err := fmt.Fprintln(w, line); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s      %q\n", indent, spanPreview(n.draft.SourceSpan)); err != nil {
		return err
	}
	for _, child := range n.subsequent {
		if err := printLineageStep(w, child, indent+"  "); err != nil {
			return err
		}
	}
	return nil
}

// lineageJSONChain is the JSON representation of a single chain for --format json.
type lineageJSONChain struct {
	AnchorID    string   `json:"anchor_id"`
	Members   []string `json:"members"`
}

// collectMembers recursively appends the IDs of n and all its descendants
// to members in depth-first order. Used by printLineageJSON to produce a
// complete flat list of all chain members regardless of chain depth.
func collectMembers(n *lineageNode, members *[]string) {
	*members = append(*members, n.draft.ID)
	for _, child := range n.subsequent {
		collectMembers(child, members)
	}
}

// printLineageJSON renders the lineage result as a JSON object with "chains"
// and "standalone" keys.
//
// Each chain entry lists all members (anchor + all descendants at every depth)
// in depth-first order via collectMembers. A shallow loop over root.subsequent
// would silently drop grandchildren and deeper nodes.
func printLineageJSON(w io.Writer, result lineageResult) error {
	type output struct {
		Chains     []lineageJSONChain `json:"chains"`
		Standalone int                `json:"standalone"`
	}

	var chains []lineageJSONChain
	for _, root := range result.anchors {
		if len(root.subsequent) == 0 {
			continue
		}
		var members []string
		collectMembers(root, &members)
		chains = append(chains, lineageJSONChain{
			AnchorID: root.draft.ID,
			Members:  members,
		})
	}
	if chains == nil {
		chains = []lineageJSONChain{}
	}

	out := output{
		Chains:     chains,
		Standalone: result.standalone,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// cmdLineage implements the "lineage" subcommand.
//
// It reads a TraceDraft JSON file, walks the DerivedFrom links to build chains,
// and prints the structure in text or JSON format. The lineage reader is a
// chain reader, not a diff tool — it shows structure, not differences between
// chain members (P5 in plan_m12.md, design rule 3).
//
// Cycle detection: if DerivedFrom forms a cycle, cmdLineage returns an error
// naming the cycle rather than silently looping.
//
// Flags:
//   - --id <id>          show lineage for a single draft (root or any member)
//   - --format text|json output format (default: text)
func cmdLineage(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("lineage", flag.ContinueOnError)

	var idFilter string
	fs.StringVar(&idFilter, "id", "", "show lineage for a single draft by ID")

	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate format before file I/O so the error is immediate.
	switch format {
	case "text", "json":
		// valid
	default:
		return fmt.Errorf("lineage: unknown --format %q (text|json)", format)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("lineage: path to drafts.json required\n\nUsage: meshant lineage [--id <id>] [--format text|json] <drafts.json>")
	}
	path := remaining[0]

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("lineage: %w", err)
	}

	// Build full lineage to detect cycles before applying --id filter.
	// This ensures cycles in the complete dataset are always caught.
	result, err := buildLineage(drafts)
	if err != nil {
		return fmt.Errorf("lineage: %w", err)
	}

	// Apply --id filter: restrict output to the chain containing the specified ID.
	if idFilter != "" {
		filtered, err := filterLineageByID(result, idFilter)
		if err != nil {
			return fmt.Errorf("lineage: %w", err)
		}
		result = filtered
	}

	switch format {
	case "json":
		return printLineageJSON(w, result)
	default: // "text"
		return printLineageText(w, result)
	}
}

// filterLineageByID restricts the lineage result to the chain(s) containing
// the draft with the given ID. Returns an error if no chain contains the ID.
func filterLineageByID(result lineageResult, id string) (lineageResult, error) {
	// Check if the ID is a chain anchor or a subsequent reading in any chain.
	for _, root := range result.anchors {
		if root.draft.ID == id {
			standalone := 0
			if len(root.subsequent) == 0 {
				standalone = 1
			}
			return lineageResult{anchors: []*lineageNode{root}, standalone: standalone}, nil
		}
		// Check if the ID appears in any subsequent reading of this anchor.
		if chainContainsID(root, id) {
			standalone := 0
			return lineageResult{anchors: []*lineageNode{root}, standalone: standalone}, nil
		}
	}
	return lineageResult{}, fmt.Errorf("draft with id %q not found in any chain", id)
}

// chainContainsID reports whether any subsequent reading in the chain starting
// at n has the given ID (not including n itself).
func chainContainsID(n *lineageNode, id string) bool {
	for _, child := range n.subsequent {
		if child.draft.ID == id {
			return true
		}
		if chainContainsID(child, id) {
			return true
		}
	}
	return false
}
