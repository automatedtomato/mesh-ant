// Package main contains the test suite for the meshant CLI entry point.
//
// Tests call run(), cmdSummarize(), and cmdValidate() directly using the
// package main approach (internal testing), which is the standard Go method
// for testing main packages that cannot be imported externally.
//
// Coverage note: main() itself is untestable from Go tests (Go cannot cover
// the main function). The testable surface — run(), cmdSummarize(),
// cmdValidate(), usage() — should exceed 80% coverage.
//
// Dataset paths are relative to the test execution directory
// (meshant/cmd/meshant/). The module root is meshant/, so three levels up
// (../../../) from cmd/meshant/ reaches the repository root before descending
// into data/examples/. This matches the demo's defaultDatasetPath convention.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Group 1: cmdSummarize ---

// TestCmdSummarize_HappyPath verifies that cmdSummarize produces non-empty
// output containing trace count information when given a valid dataset.
// It uses the evacuation_order.json dataset (28 traces, all valid).
func TestCmdSummarize_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdSummarize(&buf, []string{"../../../data/examples/evacuation_order.json"})
	if err != nil {
		t.Fatalf("cmdSummarize() returned unexpected error: %v", err)
	}
	out := buf.String()
	if len(out) == 0 {
		t.Error("cmdSummarize() produced empty output; want non-empty")
	}
	// PrintSummary output must reference traces in some form.
	// The loader's PrintSummary uses "traces" in its flagged-traces line.
	if !strings.Contains(strings.ToLower(out), "traces") {
		t.Errorf("cmdSummarize() output does not contain \"traces\" (case-insensitive); got:\n%s", out)
	}
}

// TestCmdSummarize_MissingArg verifies that cmdSummarize returns an error
// when called with no path argument.
func TestCmdSummarize_MissingArg(t *testing.T) {
	var buf bytes.Buffer
	err := cmdSummarize(&buf, []string{})
	if err == nil {
		t.Error("cmdSummarize() with no args: want non-nil error, got nil")
	}
}

// TestCmdSummarize_BadPath verifies that cmdSummarize returns an error when
// the path does not point to an existing file.
func TestCmdSummarize_BadPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdSummarize(&buf, []string{"notafile.json"})
	if err == nil {
		t.Error("cmdSummarize() with nonexistent path: want non-nil error, got nil")
	}
}

// --- Group 2: cmdValidate ---

// TestCmdValidate_HappyPath verifies that cmdValidate writes a success message
// containing "all valid" when all traces pass validation.
// Uses evacuation_order.json (28 traces, all valid).
func TestCmdValidate_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdValidate(&buf, []string{"../../../data/examples/evacuation_order.json"})
	if err != nil {
		t.Fatalf("cmdValidate() returned unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "all valid") {
		t.Errorf("cmdValidate() output does not contain \"all valid\"; got: %q", out)
	}
}

// TestCmdValidate_MissingArg verifies that cmdValidate returns an error when
// called with no path argument.
func TestCmdValidate_MissingArg(t *testing.T) {
	var buf bytes.Buffer
	err := cmdValidate(&buf, []string{})
	if err == nil {
		t.Error("cmdValidate() with no args: want non-nil error, got nil")
	}
}

// --- Group 3: run() dispatch ---

// TestRun_NoArgs verifies that run() returns an error when called with an
// empty argument slice (no subcommand).
func TestRun_NoArgs(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{})
	if err == nil {
		t.Error("run() with no args: want non-nil error, got nil")
	}
}

// TestRun_UnknownCommand verifies that run() returns an error containing
// "unknown command" when the first argument is not a known subcommand.
func TestRun_UnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{"badcmd"})
	if err == nil {
		t.Fatal("run() with unknown command: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("run() error = %q; want it to contain \"unknown command\"", err.Error())
	}
}

// TestRun_Summarize_Integration verifies the full dispatch path end-to-end:
// run() receives ["summarize", <path>] and produces output without error.
// This exercises the routing from run() through cmdSummarize() through the
// loader pipeline.
func TestRun_Summarize_Integration(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{"summarize", "../../../data/examples/evacuation_order.json"})
	if err != nil {
		t.Fatalf("run([\"summarize\", path]) returned unexpected error: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("run([\"summarize\", path]) produced empty output; want non-empty")
	}
}

// TestCmdValidate_BadPath verifies that cmdValidate returns an error when
// the path does not point to an existing file.
func TestCmdValidate_BadPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdValidate(&buf, []string{"notafile.json"})
	if err == nil {
		t.Error("cmdValidate() with nonexistent path: want non-nil error, got nil")
	}
}

// TestRun_Validate_Integration verifies the full dispatch path for the validate
// command end-to-end: run() receives ["validate", <path>] and writes a success
// message without returning an error.
func TestRun_Validate_Integration(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{"validate", "../../../data/examples/evacuation_order.json"})
	if err != nil {
		t.Fatalf("run([\"validate\", path]) returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "all valid") {
		t.Errorf("run([\"validate\", path]) output does not contain \"all valid\"; got: %q", buf.String())
	}
}

// --- Group 4: cmdArticulate ---

// evacuationDataset is the path to the evacuation_order.json dataset relative
// to the meshant/cmd/meshant/ test execution directory.
const evacuationDataset = "../../../data/examples/evacuation_order.json"

// TestCmdArticulate_HappyPath verifies that cmdArticulate produces non-empty
// output when given a valid --observer and a valid dataset path.
// Uses the meteorological-analyst observer from evacuation_order.json.
func TestCmdArticulate_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() returned unexpected error: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("cmdArticulate() produced empty output; want non-empty")
	}
}

// TestCmdArticulate_WithTimeWindow verifies that cmdArticulate accepts --from
// and --to RFC3339 flags alongside --observer without error.
func TestCmdArticulate_WithTimeWindow(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--from", "2026-04-14T00:00:00Z",
		"--to", "2026-04-14T23:59:59Z",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() with time window returned unexpected error: %v", err)
	}
}

// TestCmdArticulate_FormatJSON verifies that --format json produces output
// that begins with '{' (JSON object).
func TestCmdArticulate_FormatJSON(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--format", "json",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --format json returned unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") {
		t.Errorf("cmdArticulate() --format json: output does not start with '{'; got: %q", out[:min(len(out), 40)])
	}
}

// TestCmdArticulate_FormatDOT verifies that --format dot produces output
// containing "digraph" (DOT language graph declaration).
func TestCmdArticulate_FormatDOT(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--format", "dot",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --format dot returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "digraph") {
		t.Errorf("cmdArticulate() --format dot: output does not contain \"digraph\"; got:\n%s", buf.String())
	}
}

// TestCmdArticulate_FormatMermaid verifies that --format mermaid produces
// output containing "flowchart" (Mermaid graph declaration).
func TestCmdArticulate_FormatMermaid(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--format", "mermaid",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --format mermaid returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "flowchart") {
		t.Errorf("cmdArticulate() --format mermaid: output does not contain \"flowchart\"; got:\n%s", buf.String())
	}
}

// TestCmdArticulate_MissingObserver verifies that cmdArticulate returns an
// error containing "required" when no --observer flag is provided.
func TestCmdArticulate_MissingObserver(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{evacuationDataset})
	if err == nil {
		t.Fatal("cmdArticulate() with no --observer: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("cmdArticulate() error = %q; want it to contain \"required\"", err.Error())
	}
}

// TestCmdArticulate_BadFromTime verifies that cmdArticulate returns an error
// containing "RFC3339" when --from is not a valid RFC3339 timestamp.
func TestCmdArticulate_BadFromTime(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--from", "notadate",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdArticulate() with bad --from: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("cmdArticulate() error = %q; want it to contain \"RFC3339\"", err.Error())
	}
}

// TestCmdArticulate_InvertedWindow verifies that cmdArticulate returns an
// error when --from is after --to (invalid time window).
func TestCmdArticulate_InvertedWindow(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--from", "2026-04-16T00:00:00Z",
		"--to", "2026-04-14T00:00:00Z",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdArticulate() with inverted time window: want non-nil error, got nil")
	}
}

// TestCmdArticulate_UnknownFormat verifies that cmdArticulate returns an error
// containing "unknown" when --format is not one of text|json|dot|mermaid.
func TestCmdArticulate_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--format", "xml",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdArticulate() with unknown --format: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("cmdArticulate() error = %q; want it to contain \"unknown\"", err.Error())
	}
}

// TestCmdArticulate_MissingPath verifies that cmdArticulate returns an error
// when no positional path argument is provided.
func TestCmdArticulate_MissingPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{"--observer", "meteorological-analyst"})
	if err == nil {
		t.Fatal("cmdArticulate() with no path: want non-nil error, got nil")
	}
}

// --- Group 5: cmdDiff ---

// TestCmdDiff_HappyPath verifies that cmdDiff produces non-empty output when
// given two distinct observer positions from the evacuation dataset and no
// time window constraints. The two observers (meteorological-analyst and
// local-mayor) are near-disjoint, so the diff should reflect structural
// divergence between their respective articulations.
func TestCmdDiff_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() returned unexpected error: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("cmdDiff() produced empty output; want non-empty")
	}
}

// TestCmdDiff_WithTimeWindows verifies that cmdDiff accepts per-side time
// window flags (--from-a/--to-a, --from-b/--to-b) alongside observer flags
// without returning an error.
func TestCmdDiff_WithTimeWindows(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--from-a", "2026-04-14T00:00:00Z",
		"--to-a", "2026-04-14T23:59:59Z",
		"--from-b", "2026-04-16T00:00:00Z",
		"--to-b", "2026-04-16T23:59:59Z",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() with time windows returned unexpected error: %v", err)
	}
}

// TestCmdDiff_FormatJSON verifies that --format json causes cmdDiff to produce
// output that begins with '{' (a JSON object).
func TestCmdDiff_FormatJSON(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--format", "json",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() --format json returned unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") {
		t.Errorf("cmdDiff() --format json: output does not start with '{'; got: %q", out[:min(len(out), 40)])
	}
}

// TestCmdDiff_MissingObserverA verifies that cmdDiff returns an error
// containing "observer-a" when no --observer-a flag is supplied.
func TestCmdDiff_MissingObserverA(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-b", "local-mayor",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() with no --observer-a: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "observer-a") {
		t.Errorf("cmdDiff() error = %q; want it to contain \"observer-a\"", err.Error())
	}
}

// TestCmdDiff_MissingObserverB verifies that cmdDiff returns an error
// containing "observer-b" when no --observer-b flag is supplied.
func TestCmdDiff_MissingObserverB(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() with no --observer-b: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "observer-b") {
		t.Errorf("cmdDiff() error = %q; want it to contain \"observer-b\"", err.Error())
	}
}

// TestCmdDiff_FormatDOT_Rejected and TestCmdDiff_FormatMermaid_Rejected
// removed: DOT and Mermaid are now supported for diffs (M10.2/M10.3).

// TestCmdDiff_UnknownFormat verifies that an unrecognised --format value
// returns an error containing "unknown".
func TestCmdDiff_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--format", "xml",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() --format xml: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("cmdDiff() error = %q; want it to contain \"unknown\"", err.Error())
	}
}

// TestCmdDiff_BadFromTime verifies that a malformed --from-a value produces
// an error containing "RFC3339".
func TestCmdDiff_BadFromTime(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--from-a", "notadate",
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() with bad --from-a: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("cmdDiff() error = %q; want it to contain \"RFC3339\"", err.Error())
	}
}

// TestCmdDiff_MissingPath verifies that cmdDiff returns an error when no
// positional path argument is provided after the flags.
func TestCmdDiff_MissingPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
	})
	if err == nil {
		t.Fatal("cmdDiff() with no path: want non-nil error, got nil")
	}
}

// TestCmdDiff_InvertedWindow verifies that cmdDiff returns an error when
// --from-a is after --to-a (invalid time window for side A).
func TestCmdDiff_InvertedWindow(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--from-a", "2026-04-16T00:00:00Z",
		"--to-a", "2026-04-14T00:00:00Z",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() with inverted window A: want non-nil error, got nil")
	}
}

// TestCmdArticulate_BadToTime verifies that cmdArticulate returns an error
// containing "RFC3339" when --to is not a valid RFC3339 timestamp.
func TestCmdArticulate_BadToTime(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--to", "notadate",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdArticulate() with bad --to: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("cmdArticulate() error = %q; want it to contain \"RFC3339\"", err.Error())
	}
}

// TestCmdDiff_BadToATime verifies that a malformed --to-a produces an error
// containing "RFC3339".
func TestCmdDiff_BadToATime(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--to-a", "notadate",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() with bad --to-a: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("cmdDiff() error = %q; want it to contain \"RFC3339\"", err.Error())
	}
}

// TestCmdDiff_BadFromBTime verifies that a malformed --from-b produces an
// error containing "RFC3339".
func TestCmdDiff_BadFromBTime(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--from-b", "notadate",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() with bad --from-b: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("cmdDiff() error = %q; want it to contain \"RFC3339\"", err.Error())
	}
}

// TestCmdDiff_BadToBTime verifies that a malformed --to-b produces an error
// containing "RFC3339".
func TestCmdDiff_BadToBTime(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--to-b", "notadate",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() with bad --to-b: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("cmdDiff() error = %q; want it to contain \"RFC3339\"", err.Error())
	}
}

// TestStringSliceFlag_EmptyValueRejected verifies that passing an empty string
// to --observer is rejected with an error rather than silently producing an
// empty-observer articulation that yields a zero-node graph.
func TestStringSliceFlag_EmptyValueRejected(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdArticulate() with --observer \"\": want non-nil error, got nil")
	}
}

// --- Group 6: --tag flag on articulate (M10.3) ---

// TestCmdArticulate_WithTag verifies that --tag is accepted and produces
// non-empty output when the tag exists in the dataset. The evacuation dataset
// has traces tagged "delay".
func TestCmdArticulate_WithTag(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--tag", "delay",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --tag delay returned unexpected error: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("cmdArticulate() --tag delay produced empty output; want non-empty")
	}
}

// TestCmdArticulate_WithMultipleTags verifies that --tag is repeatable and
// passes multiple tags to ArticulationOptions.Tags (any-match / OR semantics).
func TestCmdArticulate_WithMultipleTags(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--tag", "delay",
		"--tag", "threshold",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --tag x --tag y returned unexpected error: %v", err)
	}
}

// TestCmdArticulate_TagEmptyRejected verifies that --tag "" is rejected.
func TestCmdArticulate_TagEmptyRejected(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--tag", "",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdArticulate() --tag \"\": want non-nil error, got nil")
	}
}

// --- Group 7: diff --format dot|mermaid unlocked (M10.3) ---

// TestCmdDiff_FormatDOT verifies that --format dot now produces output
// containing "digraph" (DOT language). Previously rejected; unlocked by M10.2.
func TestCmdDiff_FormatDOT(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--format", "dot",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() --format dot returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "digraph") {
		t.Errorf("cmdDiff() --format dot: output does not contain \"digraph\"; got:\n%s", buf.String())
	}
}

// TestCmdDiff_FormatMermaid verifies that --format mermaid now produces output
// containing "flowchart" (Mermaid graph declaration).
func TestCmdDiff_FormatMermaid(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--format", "mermaid",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() --format mermaid returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "flowchart") {
		t.Errorf("cmdDiff() --format mermaid: output does not contain \"flowchart\"; got:\n%s", buf.String())
	}
}

// --- Group 8: --tag-a / --tag-b on diff (M10.3) ---

// TestCmdDiff_WithTagA verifies that --tag-a is accepted and passes tag
// filters to side A's ArticulationOptions.
func TestCmdDiff_WithTagA(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--tag-a", "delay",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() --tag-a delay returned unexpected error: %v", err)
	}
}

// TestCmdDiff_WithTagB verifies that --tag-b is accepted and passes tag
// filters to side B's ArticulationOptions.
func TestCmdDiff_WithTagB(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--tag-b", "delay",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() --tag-b delay returned unexpected error: %v", err)
	}
}

// TestCmdDiff_WithBothTags verifies that --tag-a and --tag-b can be used
// together to filter each side independently.
func TestCmdDiff_WithBothTags(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--tag-a", "delay",
		"--tag-b", "threshold",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() --tag-a + --tag-b returned unexpected error: %v", err)
	}
}

// TestCmdDiff_TagAEmptyRejected verifies that --tag-a "" is rejected.
func TestCmdDiff_TagAEmptyRejected(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--tag-a", "",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() --tag-a \"\": want non-nil error, got nil")
	}
}

// TestCmdDiff_TagBEmptyRejected verifies that --tag-b "" is rejected.
func TestCmdDiff_TagBEmptyRejected(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--tag-b", "",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdDiff() --tag-b \"\": want non-nil error, got nil")
	}
}

// --- Group 9: --output flag (M10.3) ---

// TestCmdArticulate_Output verifies that --output writes the articulation to
// the specified file instead of stdout. The buffer (stdout) should receive a
// confirmation message, not the articulation output.
func TestCmdArticulate_Output(t *testing.T) {
	outFile := t.TempDir() + "/test.dot"
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--format", "dot",
		"--output", outFile,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --output returned unexpected error: %v", err)
	}
	// stdout should have a confirmation message naming the file.
	if !strings.Contains(buf.String(), outFile) {
		t.Errorf("cmdArticulate() --output: stdout does not mention output file; got: %q", buf.String())
	}
	// The file should exist and contain DOT output.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "digraph") {
		t.Errorf("output file does not contain \"digraph\"; got:\n%s", string(data))
	}
}

// TestCmdArticulate_OutputMermaid verifies --output with --format mermaid.
func TestCmdArticulate_OutputMermaid(t *testing.T) {
	outFile := t.TempDir() + "/test.mmd"
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--format", "mermaid",
		"--output", outFile,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --output mermaid returned unexpected error: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "flowchart") {
		t.Errorf("output file does not contain \"flowchart\"; got:\n%s", string(data))
	}
}

// TestCmdDiff_Output verifies that --output writes the diff to a file.
func TestCmdDiff_Output(t *testing.T) {
	outFile := t.TempDir() + "/diff.dot"
	var buf bytes.Buffer
	err := cmdDiff(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "local-mayor",
		"--format", "dot",
		"--output", outFile,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdDiff() --output returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), outFile) {
		t.Errorf("cmdDiff() --output: stdout does not mention output file; got: %q", buf.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "digraph") {
		t.Errorf("output file does not contain \"digraph\"; got:\n%s", string(data))
	}
}

// TestCmdArticulate_OutputBadPath verifies that --output with an invalid path
// returns an error rather than silently failing.
func TestCmdArticulate_OutputBadPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--format", "dot",
		"--output", "/nonexistent/dir/file.dot",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdArticulate() --output bad path: want non-nil error, got nil")
	}
}

// TestCmdArticulate_OutputText verifies that --output works with --format text
// (default format), not just DOT/Mermaid.
func TestCmdArticulate_OutputText(t *testing.T) {
	outFile := t.TempDir() + "/graph.txt"
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--output", outFile,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --output text returned unexpected error: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if len(data) == 0 {
		t.Error("output file is empty; want non-empty")
	}
}

// --- Group 10: cmdFollow ---

// TestCmdFollow_HappyPath verifies that cmdFollow produces non-empty output
// containing chain-specific content when given valid flags and the evacuation dataset.
func TestCmdFollow_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-array-atlantic-sector-7",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() returned unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Translation Chain") {
		t.Error("cmdFollow() output missing 'Translation Chain' header")
	}
	if !strings.Contains(out, "buoy-array-atlantic-sector-7") {
		t.Error("cmdFollow() output missing start element name")
	}
}

// TestCmdFollow_MissingObserver verifies that cmdFollow returns an error
// when --observer is not provided.
func TestCmdFollow_MissingObserver(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--element", "A",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with no --observer: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "observer") {
		t.Errorf("error = %q; want it to contain 'observer'", err.Error())
	}
}

// TestCmdFollow_MissingElement verifies that cmdFollow returns an error
// when --element is not provided.
func TestCmdFollow_MissingElement(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with no --element: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "element") {
		t.Errorf("error = %q; want it to contain 'element'", err.Error())
	}
}

// TestCmdFollow_UnknownDirection verifies that an invalid --direction value
// returns an error.
func TestCmdFollow_UnknownDirection(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "A",
		"--direction", "sideways",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() --direction sideways: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error = %q; want it to contain 'unknown'", err.Error())
	}
}

// TestCmdFollow_UnknownFormat verifies that an invalid --format value
// returns an error.
func TestCmdFollow_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "A",
		"--format", "xml",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() --format xml: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error = %q; want it to contain 'unknown'", err.Error())
	}
}

// TestCmdFollow_Backward verifies that --direction backward produces output.
func TestCmdFollow_Backward(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "local-mayor",
		"--element", "evacuation-order-16apr",
		"--direction", "backward",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() --direction backward returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Translation Chain") {
		t.Error("cmdFollow() backward output missing header")
	}
}

// TestCmdFollow_FormatJSON verifies that --format json produces valid JSON output.
func TestCmdFollow_FormatJSON(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-array-atlantic-sector-7",
		"--format", "json",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() --format json returned unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") {
		t.Errorf("cmdFollow() --format json: output does not start with '{'; got: %q", out[:min(len(out), 40)])
	}
}

// TestCmdFollow_DepthLimit verifies that --depth flag is accepted.
func TestCmdFollow_DepthLimit(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-array-atlantic-sector-7",
		"--depth", "1",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() --depth 1 returned unexpected error: %v", err)
	}
}

// TestCmdFollow_MissingPath verifies that cmdFollow returns an error when
// no positional path argument is provided.
func TestCmdFollow_MissingPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "A",
	})
	if err == nil {
		t.Fatal("cmdFollow() with no path: want non-nil error, got nil")
	}
}

// TestCmdFollow_Output verifies that --output writes the chain to a file.
func TestCmdFollow_Output(t *testing.T) {
	outFile := t.TempDir() + "/chain.txt"
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-array-atlantic-sector-7",
		"--output", outFile,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() --output returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), outFile) {
		t.Errorf("stdout does not mention output file; got: %q", buf.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "Translation Chain") {
		t.Error("output file missing chain header")
	}
}

// TestRun_Follow verifies that run() dispatches "follow" to cmdFollow.
func TestRun_Follow(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{
		"follow",
		"--observer", "meteorological-analyst",
		"--element", "buoy-array-atlantic-sector-7",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("run(follow) returned unexpected error: %v", err)
	}
}

// --- Group 11: --criterion-file flag on follow ---

// writeCriterionFile writes content to a new file in a temp dir and returns
// the path. It is a helper shared by all Group 11 tests.
func writeCriterionFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "criterion.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeCriterionFile: %v", err)
	}
	return path
}

// TC-01: full criterion, text output — verifies declaration appears and
// heuristics disclaimer is emitted.
func TestCmdFollow_CriterionFile_TextOutput(t *testing.T) {
	criterionPath := writeCriterionFile(t, `{"declaration":"Preserve operational meaning","preserve":["target"],"ignore":["display_format"]}`)

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() returned unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Declaration: Preserve operational meaning") {
		t.Errorf("output missing Declaration line\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "(criterion carried — classification uses v1 heuristics)") {
		t.Errorf("output missing heuristics disclaimer\nfull output:\n%s", out)
	}
}

// TC-02: full criterion, JSON output — verifies "criterion" key and declaration
// value appear in the JSON envelope.
func TestCmdFollow_CriterionFile_JSONOutput(t *testing.T) {
	criterionPath := writeCriterionFile(t, `{"declaration":"Preserve operational meaning","preserve":["target"],"ignore":["display_format"]}`)

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		"--format", "json",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() --format json returned unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"criterion"`) {
		t.Errorf("JSON output missing \"criterion\" key\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "Preserve operational meaning") {
		t.Errorf("JSON output missing declaration value\nfull output:\n%s", out)
	}
}

// TC-03: no --criterion-file — verifies v1 output is unchanged (no Declaration
// or criterion-carried lines).
func TestCmdFollow_NoCriterionFile_V1Unchanged(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() returned unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Declaration:") {
		t.Errorf("output should not contain 'Declaration:' without --criterion-file\nfull output:\n%s", out)
	}
	if strings.Contains(out, "criterion carried") {
		t.Errorf("output should not contain 'criterion carried' without --criterion-file\nfull output:\n%s", out)
	}
}

// TC-04: non-existent criterion file path — verifies error contains
// "criterion-file" and "cannot open".
func TestCmdFollow_CriterionFile_NonExistentPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", "/nonexistent/path/no.json",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with non-existent criterion file: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "criterion-file") {
		t.Errorf("error = %q; want it to contain 'criterion-file'", err.Error())
	}
	if !strings.Contains(err.Error(), "cannot open") {
		t.Errorf("error = %q; want it to contain 'cannot open'", err.Error())
	}
}

// TC-05: malformed JSON in criterion file — verifies error contains
// "malformed JSON".
func TestCmdFollow_CriterionFile_MalformedJSON(t *testing.T) {
	criterionPath := writeCriterionFile(t, "{not valid json")

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with malformed JSON: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "malformed JSON") {
		t.Errorf("error = %q; want it to contain 'malformed JSON'", err.Error())
	}
}

// TC-06: empty JSON object {} — zero-value criterion after decode should
// produce an error containing "zero-value criterion".
func TestCmdFollow_CriterionFile_ZeroValueObject(t *testing.T) {
	criterionPath := writeCriterionFile(t, "{}")

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with zero-value criterion: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "zero-value criterion") {
		t.Errorf("error = %q; want it to contain 'zero-value criterion'", err.Error())
	}
}

// TC-07: empty file (zero bytes) — should produce an error (EOF from decoder).
func TestCmdFollow_CriterionFile_EmptyFile(t *testing.T) {
	criterionPath := writeCriterionFile(t, "")

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with empty criterion file: want non-nil error, got nil")
	}
	// An empty file produces io.EOF from the JSON decoder, which is
	// wrapped as "malformed JSON" — same error path as TC-05.
	if !strings.Contains(err.Error(), "malformed JSON") {
		t.Errorf("error = %q; want it to contain 'malformed JSON' (EOF from empty file)", err.Error())
	}
}

// TC-08: preserve without declaration — Validate() rejects Layer 2 without
// Layer 1; error must contain "Declaration".
func TestCmdFollow_CriterionFile_PreserveWithoutDeclaration(t *testing.T) {
	criterionPath := writeCriterionFile(t, `{"preserve":["target"]}`)

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with preserve-without-declaration: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "Declaration") {
		t.Errorf("error = %q; want it to contain 'Declaration'", err.Error())
	}
}

// TC-09: ignore without declaration — same layer-ordering violation as TC-08.
func TestCmdFollow_CriterionFile_IgnoreWithoutDeclaration(t *testing.T) {
	criterionPath := writeCriterionFile(t, `{"ignore":["display_format"]}`)

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with ignore-without-declaration: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "Declaration") {
		t.Errorf("error = %q; want it to contain 'Declaration'", err.Error())
	}
}

// TC-10: name-only criterion, text output — accepted as a valid handle,
// emits the handle-only warning (ANT T2) and heuristics disclaimer.
func TestCmdFollow_CriterionFile_NameOnly_Accepted(t *testing.T) {
	criterionPath := writeCriterionFile(t, `{"name":"handle-x"}`)

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() with name-only criterion returned unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Criterion: handle-x") {
		t.Errorf("output missing 'Criterion: handle-x'\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "(handle only — no declaration grounds this reading)") {
		t.Errorf("output missing handle-only warning\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "(criterion carried — classification uses v1 heuristics)") {
		t.Errorf("output missing heuristics disclaimer\nfull output:\n%s", out)
	}
}

// TC-11: name-only criterion, JSON output — verifies "criterion" key present.
func TestCmdFollow_CriterionFile_NameOnly_JSONOutput(t *testing.T) {
	criterionPath := writeCriterionFile(t, `{"name":"handle-x"}`)

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		"--format", "json",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() with name-only criterion --format json returned unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"criterion"`) {
		t.Errorf("JSON output missing \"criterion\" key\nfull output:\n%s", out)
	}
}

// TC-12: unknown field in JSON — DisallowUnknownFields rejects "declarations"
// (a common misspelling of "declaration"). Error must contain "criterion-file"
// but must NOT say "zero-value" (fails at decode step, not IsZero step).
func TestCmdFollow_CriterionFile_UnknownField_Rejected(t *testing.T) {
	criterionPath := writeCriterionFile(t, `{"declarations":"misspelled field name"}`)

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdFollow() with unknown JSON field: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "criterion-file") {
		t.Errorf("error = %q; want it to contain 'criterion-file'", err.Error())
	}
	// Must fail at decode (malformed JSON path), not IsZero path.
	// DisallowUnknownFields funnels through the same Decode error.
	if !strings.Contains(err.Error(), "malformed JSON") {
		t.Errorf("error = %q; want it to contain 'malformed JSON' (unknown field at decode step)", err.Error())
	}
	if strings.Contains(err.Error(), "zero-value") {
		t.Errorf("error = %q; must NOT say 'zero-value' (unknown field detected before IsZero check)", err.Error())
	}
}

// TC-13: full criterion with --output flag — verifies criterion content
// appears in the written file.
func TestCmdFollow_CriterionFile_WithOutput(t *testing.T) {
	criterionPath := writeCriterionFile(t, `{"declaration":"Preserve operational meaning","preserve":["target"]}`)
	outFile := filepath.Join(t.TempDir(), "out.txt")

	var buf bytes.Buffer
	err := cmdFollow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--element", "buoy-network",
		"--criterion-file", criterionPath,
		"--output", outFile,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdFollow() with --criterion-file --output returned unexpected error: %v", err)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(content), "Declaration: Preserve operational meaning") {
		t.Errorf("output file missing Declaration line\nfull content:\n%s", string(content))
	}
}
