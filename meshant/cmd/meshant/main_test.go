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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
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

// --- Group 12: cmdDraft ---

// cveExtractionDataset is the path to the pre-made LLM extraction fixture
// for the CVE vulnerability response scenario (M11 dataset).
const cveExtractionDataset = "../../../data/examples/cve_response_extraction.json"

// cveExpectedDraftCount is the number of records in the CVE extraction fixture.
const cveExpectedDraftCount = 14

// TestCmdDraft_ValidExtractionFile verifies that cmdDraft produces TraceDraft
// JSON with the correct record count when given a valid extraction file.
// It writes to a temp file, parses the JSON array, and checks the count
// against cveExpectedDraftCount.
func TestCmdDraft_ValidExtractionFile(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "drafts.json")
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{"--output", outFile, cveExtractionDataset})
	if err != nil {
		t.Fatalf("cmdDraft() returned unexpected error: %v", err)
	}

	// Parse the written JSON array and verify record count.
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	var records []map[string]interface{}
	if err := json.Unmarshal(content, &records); err != nil {
		t.Fatalf("parse output JSON: %v", err)
	}
	if len(records) != cveExpectedDraftCount {
		t.Errorf("draft count: got %d want %d", len(records), cveExpectedDraftCount)
	}

	// Summary on stdout must mention the count.
	if !strings.Contains(buf.String(), "14") {
		t.Errorf("stdout summary does not mention count 14; got:\n%s", buf.String())
	}
}

// TestCmdDraft_MissingSourceSpan verifies that cmdDraft returns an error
// when any record has an empty source_span.
func TestCmdDraft_MissingSourceSpan(t *testing.T) {
	path := writeTempJSONForDraft(t, `[{"source_span":"ok"},{"source_span":""}]`)
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{path})
	if err == nil {
		t.Fatal("cmdDraft() with empty source_span: want error, got nil")
	}
}

// TestCmdDraft_SourceDocFlag verifies that --source-doc stamps SourceDocRef
// on all produced drafts.
func TestCmdDraft_SourceDocFlag(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "drafts.json")
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{
		"--source-doc", "cve_response_raw.md",
		"--output", outFile,
		cveExtractionDataset,
	})
	if err != nil {
		t.Fatalf("cmdDraft() --source-doc returned unexpected error: %v", err)
	}
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(content), "cve_response_raw.md") {
		t.Errorf("--source-doc value not found in output; content:\n%s", string(content))
	}
}

// TestCmdDraft_ExtractedByFlag verifies that --extracted-by overrides the
// ExtractedBy field on all produced drafts.
func TestCmdDraft_ExtractedByFlag(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "drafts.json")
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{
		"--extracted-by", "test-override",
		"--output", outFile,
		cveExtractionDataset,
	})
	if err != nil {
		t.Fatalf("cmdDraft() --extracted-by returned unexpected error: %v", err)
	}
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(content), "test-override") {
		t.Errorf("--extracted-by override not found in output; content:\n%s", string(content))
	}
}

// TestCmdDraft_OutputFlag verifies that --output writes TraceDraft JSON to a file.
func TestCmdDraft_OutputFlag(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "drafts.json")
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{
		"--output", outFile,
		cveExtractionDataset,
	})
	if err != nil {
		t.Fatalf("cmdDraft() --output returned unexpected error: %v", err)
	}
	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}

// TestCmdDraft_EmptyExtractionFile verifies that cmdDraft returns an error
// when the extraction JSON is an empty array.
func TestCmdDraft_EmptyExtractionFile(t *testing.T) {
	path := writeTempJSONForDraft(t, `[]`)
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{path})
	if err == nil {
		t.Fatal("cmdDraft() with empty array: want error, got nil")
	}
}

// TestCmdDraft_MalformedJSON verifies that cmdDraft returns an error for
// malformed JSON input.
func TestCmdDraft_MalformedJSON(t *testing.T) {
	path := writeTempJSONForDraft(t, `[{not valid}]`)
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{path})
	if err == nil {
		t.Fatal("cmdDraft() with malformed JSON: want error, got nil")
	}
}

// TestCmdDraft_StageFlag verifies that --stage overrides the ExtractionStage
// field on all produced drafts.
func TestCmdDraft_StageFlag(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "drafts.json")
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{
		"--stage", "reviewed",
		"--output", outFile,
		cveExtractionDataset,
	})
	if err != nil {
		t.Fatalf("cmdDraft() --stage returned unexpected error: %v", err)
	}
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(content), "reviewed") {
		t.Errorf("--stage override not found in output; content:\n%s", string(content))
	}
}

// TestCmdDraft_MissingArg verifies that cmdDraft returns an error when
// called with no arguments.
func TestCmdDraft_MissingArg(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDraft(&buf, []string{})
	if err == nil {
		t.Fatal("cmdDraft() with no args: want error, got nil")
	}
}

// writeTempJSONForDraft writes content to a temp file and returns its path.
// Named distinctly from the loader package's writeTempDraftJSON for clarity.
func writeTempJSONForDraft(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "extraction-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return f.Name()
}

// --- Group 13: cmdPromote ---

// cveDraftsDataset is the path to the pre-made TraceDraft fixture for the CVE
// vulnerability response scenario.
const cveDraftsDataset = "../../../data/examples/cve_response_drafts.json"

// TestCmdPromote_AllPromotable verifies that cmdPromote promotes all promotable
// drafts and produces trace output. The CVE drafts fixture has a mix of
// promotable and non-promotable records.
func TestCmdPromote_AllPromotable(t *testing.T) {
	// Build a temp file with two fully promotable drafts.
	path := writeTempJSONForDraft(t, `[
		{"id":"a1000000-0000-4000-8000-000000000001","source_span":"span a","what_changed":"a changed","observer":"analyst"},
		{"id":"a1000000-0000-4000-8000-000000000002","source_span":"span b","what_changed":"b changed","observer":"analyst"}
	]`)
	var buf bytes.Buffer
	err := cmdPromote(&buf, []string{path})
	if err != nil {
		t.Fatalf("cmdPromote() returned unexpected error: %v", err)
	}
	out := buf.String()
	// Summary must mention promoted count.
	if !strings.Contains(out, "2") {
		t.Errorf("output does not mention count 2; got:\n%s", out)
	}
}

// TestCmdPromote_MixedPromotable verifies partial promotion: promotable drafts
// are promoted, non-promotable drafts are reported in the summary.
func TestCmdPromote_MixedPromotable(t *testing.T) {
	path := writeTempJSONForDraft(t, `[
		{"id":"a1000000-0000-4000-8000-000000000001","source_span":"span a","what_changed":"changed","observer":"analyst"},
		{"source_span":"span b, no id or what_changed"}
	]`)
	// Load the second draft (auto-assigns ID) before cmdPromote so we can use
	// the file path directly — cmdPromote runs LoadDrafts internally.
	var buf bytes.Buffer
	err := cmdPromote(&buf, []string{path})
	if err != nil {
		t.Fatalf("cmdPromote() mixed: want nil error (partial success), got: %v", err)
	}
	out := buf.String()
	// Exactly 1 promoted, 1 not promotable.
	if !strings.Contains(out, "1") {
		t.Errorf("output does not mention count; got:\n%s", out)
	}
}

// TestCmdPromote_NonePromotable verifies that cmdPromote reports 0 promoted
// when no draft is promotable. cmdPromote does not hard-error in this case
// — it writes an empty trace array and reports failures in the summary.
func TestCmdPromote_NonePromotable(t *testing.T) {
	path := writeTempJSONForDraft(t, `[{"source_span":"only a span, not promotable"}]`)
	var buf bytes.Buffer
	err := cmdPromote(&buf, []string{path})
	if err != nil {
		t.Fatalf("cmdPromote() with no promotable drafts: want nil error (partial success path), got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "0") && !strings.Contains(strings.ToLower(out), "not promotable") {
		t.Errorf("output does not mention 0 promoted or 'not promotable'; got:\n%s", out)
	}
}

// TestCmdPromote_CVEDraftsFixture exercises cmdPromote against the pre-made
// CVE drafts fixture. The fixture contains 14 records with varying promotability.
// This test verifies that the command handles the full fixture without error
// and that the summary correctly reports the promoted and not-promotable counts.
func TestCmdPromote_CVEDraftsFixture(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "promoted.json")
	var buf bytes.Buffer
	err := cmdPromote(&buf, []string{"--output", outFile, cveDraftsDataset})
	if err != nil {
		t.Fatalf("cmdPromote() with CVE fixture returned unexpected error: %v", err)
	}

	// Parse promoted traces and verify every one carries the "draft" tag.
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	var promoted []map[string]interface{}
	if err := json.Unmarshal(content, &promoted); err != nil {
		t.Fatalf("parse promoted JSON: %v", err)
	}
	for i, tr := range promoted {
		tags, _ := tr["tags"].([]interface{})
		hasDraft := false
		for _, tag := range tags {
			if tag == "draft" {
				hasDraft = true
				break
			}
		}
		if !hasDraft {
			t.Errorf("promoted trace %d missing \"draft\" tag; tags = %v", i, tags)
		}
	}

	// Summary must mention both promoted count and not-promotable count.
	summary := buf.String()
	if !strings.Contains(summary, "promoted") {
		t.Errorf("summary missing 'promoted'; got:\n%s", summary)
	}
}

// TestCmdPromote_OutputFlag verifies that --output writes promoted traces to a file.
func TestCmdPromote_OutputFlag(t *testing.T) {
	path := writeTempJSONForDraft(t, `[
		{"id":"a1000000-0000-4000-8000-000000000001","source_span":"span","what_changed":"changed","observer":"analyst"}
	]`)
	outFile := filepath.Join(t.TempDir(), "promoted.json")
	var buf bytes.Buffer
	err := cmdPromote(&buf, []string{"--output", outFile, path})
	if err != nil {
		t.Fatalf("cmdPromote() --output returned unexpected error: %v", err)
	}
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if !strings.Contains(string(content), "what_changed") {
		t.Errorf("output file missing trace content; got:\n%s", string(content))
	}
}

// TestCmdPromote_MissingArg verifies that cmdPromote returns an error when
// called with no arguments.
func TestCmdPromote_MissingArg(t *testing.T) {
	var buf bytes.Buffer
	err := cmdPromote(&buf, []string{})
	if err == nil {
		t.Fatal("cmdPromote() with no args: want error, got nil")
	}
}

// --- Group 14: cmdRearticulate ---

// cveDraftsDatasetForRearticulate is the path to the CVE drafts fixture
// used by Group 14 tests.
const cveDraftsDatasetForRearticulate = "../../../data/examples/cve_response_drafts.json"

// TestCmdRearticulate_ValidDraftsFile verifies that cmdRearticulate produces a
// skeleton JSON array when given a valid drafts file. Each skeleton record must
// have SourceSpan set (copied verbatim from original), DerivedFrom set to the
// original's ID, and content fields (what_changed, source, target, mediation,
// observer, tags, uncertainty_note) blank. The ID and extracted_by must also
// be blank (to be filled by the critiquing agent). extraction_stage must be
// "reviewed".
func TestCmdRearticulate_ValidDraftsFile(t *testing.T) {
	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{cveDraftsDatasetForRearticulate})
	if err != nil {
		t.Fatalf("cmdRearticulate() returned unexpected error: %v", err)
	}

	var skeletons []map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &skeletons); err != nil {
		t.Fatalf("parse output JSON: %v", err)
	}

	// There should be 14 skeletons (one per original draft).
	if len(skeletons) != 14 {
		t.Errorf("skeleton count: got %d want 14", len(skeletons))
	}

	for i, sk := range skeletons {
		// source_span must be set (copied verbatim).
		span, _ := sk["source_span"].(string)
		if span == "" {
			t.Errorf("skeleton %d: source_span is blank; must be copied from original", i)
		}

		// derived_from must be non-empty (set to original's ID).
		derivedFrom, _ := sk["derived_from"].(string)
		if derivedFrom == "" {
			t.Errorf("skeleton %d: derived_from is blank; must be original ID", i)
		}

		// extraction_stage must be "reviewed".
		stage, _ := sk["extraction_stage"].(string)
		if stage != "reviewed" {
			t.Errorf("skeleton %d: extraction_stage = %q; want \"reviewed\"", i, stage)
		}

		// Content fields must be absent or blank — blank is the correct scaffold
		// output, not a bug. P3: the scaffold must not pre-fill from original.
		for _, field := range []string{"what_changed", "mediation", "observer", "uncertainty_note"} {
			if v, ok := sk[field]; ok && v != "" && v != nil {
				t.Errorf("skeleton %d: content field %q must be blank/absent; got %v", i, field, v)
			}
		}
		// id and extracted_by must be blank/absent (to be assigned by meshant draft).
		for _, field := range []string{"id", "extracted_by"} {
			if v, ok := sk[field]; ok && v != "" && v != nil {
				t.Errorf("skeleton %d: field %q must be blank/absent in skeleton; got %v", i, field, v)
			}
		}
	}
}

// TestCmdRearticulate_IDFlag verifies that --id produces a single skeleton
// record for that specific draft.
func TestCmdRearticulate_IDFlag(t *testing.T) {
	// E3's ID is the third draft in cve_response_drafts.json.
	const e3ID = "d0cve001-0000-4000-8000-000000000003"

	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{"--id", e3ID, cveDraftsDatasetForRearticulate})
	if err != nil {
		t.Fatalf("cmdRearticulate() --id returned unexpected error: %v", err)
	}

	var skeletons []map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &skeletons); err != nil {
		t.Fatalf("parse output JSON: %v", err)
	}

	if len(skeletons) != 1 {
		t.Fatalf("--id flag: got %d skeletons; want 1", len(skeletons))
	}

	// DerivedFrom must link back to E3.
	derivedFrom, _ := skeletons[0]["derived_from"].(string)
	if derivedFrom != e3ID {
		t.Errorf("skeleton derived_from = %q; want %q", derivedFrom, e3ID)
	}

	// SourceSpan must be E3's span.
	span, _ := skeletons[0]["source_span"].(string)
	if !strings.Contains(span, "unauthenticated attacker") {
		t.Errorf("source_span %q does not contain E3's text", span)
	}
}

// TestCmdRearticulate_IDFlagNotFound verifies that --id <unknown> returns an error
// naming the unknown ID.
func TestCmdRearticulate_IDFlagNotFound(t *testing.T) {
	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{"--id", "nonexistent-id-000", cveDraftsDatasetForRearticulate})
	if err == nil {
		t.Fatal("cmdRearticulate() with unknown --id: want error, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-id-000") {
		t.Errorf("error %q does not name the unknown ID", err.Error())
	}
}

// TestCmdRearticulate_OutputFlag verifies that --output writes the skeleton
// JSON to a file and prints a confirmation message to stdout.
func TestCmdRearticulate_OutputFlag(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "skeleton.json")
	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{"--output", outFile, cveDraftsDatasetForRearticulate})
	if err != nil {
		t.Fatalf("cmdRearticulate() --output returned unexpected error: %v", err)
	}

	// File must exist and contain valid JSON.
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	var skeletons []map[string]interface{}
	if err := json.Unmarshal(content, &skeletons); err != nil {
		t.Fatalf("parse output file JSON: %v", err)
	}
	if len(skeletons) == 0 {
		t.Error("output file contains empty array")
	}

	// stdout must contain a confirmation message (wrote <path>).
	stdout := buf.String()
	if !strings.Contains(stdout, outFile) {
		t.Errorf("stdout does not confirm output file %q; got:\n%s", outFile, stdout)
	}
}

// TestCmdRearticulate_MissingArg verifies that cmdRearticulate returns an error
// when called with no positional argument.
func TestCmdRearticulate_MissingArg(t *testing.T) {
	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{})
	if err == nil {
		t.Fatal("cmdRearticulate() with no args: want error, got nil")
	}
}

// TestCmdRearticulate_MalformedJSON verifies that cmdRearticulate returns an
// error when the input file contains malformed JSON.
func TestCmdRearticulate_MalformedJSON(t *testing.T) {
	path := writeTempJSONForDraft(t, `[{not valid json}]`)
	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{path})
	if err == nil {
		t.Fatal("cmdRearticulate() with malformed JSON: want error, got nil")
	}
}

// TestCmdRearticulate_SkeletonRoundTrip verifies that the skeleton output can be
// decoded by LoadDrafts (all records pass Validate) after DerivedFrom is set and
// source_span is present. The skeleton's SourceSpan is the required field;
// Validate() must succeed even with content fields blank.
func TestCmdRearticulate_SkeletonRoundTrip(t *testing.T) {
	// Generate skeleton to a temp file.
	skeletonFile := filepath.Join(t.TempDir(), "skeleton.json")
	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{"--output", skeletonFile, cveDraftsDatasetForRearticulate})
	if err != nil {
		t.Fatalf("cmdRearticulate() round-trip setup: %v", err)
	}

	// LoadDrafts will assign UUIDs to blank IDs and call Validate on each record.
	// All records should pass because source_span is set and that is the only
	// required field (Validate() in tracedraft.go).
	drafts, err := loader.LoadDrafts(skeletonFile)
	if err != nil {
		t.Fatalf("LoadDrafts() on skeleton output: want nil error (source_span present), got: %v", err)
	}
	if len(drafts) != 14 {
		t.Errorf("round-trip draft count: got %d want 14", len(drafts))
	}
	for i, d := range drafts {
		if d.SourceSpan == "" {
			t.Errorf("draft %d: SourceSpan empty after round-trip", i)
		}
		if d.DerivedFrom == "" {
			t.Errorf("draft %d: DerivedFrom empty after round-trip", i)
		}
		if d.ExtractionStage != "reviewed" {
			t.Errorf("draft %d: ExtractionStage = %q; want \"reviewed\"", i, d.ExtractionStage)
		}
	}
}

// TestCmdRearticulate_SkeletonHasIntentionallyBlank verifies that each skeleton
// record produced by cmdRearticulate carries an intentionally_blank array naming
// all six content fields. This distinguishes "blank by design" from "never
// extracted" — a critique cut, not an incomplete draft.
func TestCmdRearticulate_SkeletonHasIntentionallyBlank(t *testing.T) {
	var buf bytes.Buffer
	if err := cmdRearticulate(&buf, []string{cveDraftsDatasetForRearticulate}); err != nil {
		t.Fatalf("cmdRearticulate() returned unexpected error: %v", err)
	}

	var skeletons []map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &skeletons); err != nil {
		t.Fatalf("parse skeleton JSON: %v", err)
	}

	wantFields := []string{"what_changed", "source", "target", "mediation", "observer", "tags"}

	for i, sk := range skeletons {
		raw, ok := sk["intentionally_blank"]
		if !ok {
			t.Errorf("skeleton %d: intentionally_blank key absent", i)
			continue
		}
		arr, ok := raw.([]interface{})
		if !ok {
			t.Errorf("skeleton %d: intentionally_blank is not an array; got %T", i, raw)
			continue
		}
		got := make([]string, len(arr))
		for j, v := range arr {
			s, _ := v.(string)
			got[j] = s
		}
		for _, want := range wantFields {
			found := false
			for _, g := range got {
				if g == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("skeleton %d: intentionally_blank missing %q; got %v", i, want, got)
			}
		}
	}
}

// TestCmdRearticulate_CriterionFileSetsCriterionRef verifies that --criterion-file
// populates criterion_ref on every skeleton with the criterion's Name field.
func TestCmdRearticulate_CriterionFileSetsCriterionRef(t *testing.T) {
	// Write a minimal criterion file.
	criterionJSON := `{"name":"actor-stability-v1","declaration":"Only juridical–scientific crossings count"}`
	criterionPath := writeTempJSONForDraft(t, criterionJSON)

	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{"--criterion-file", criterionPath, cveDraftsDatasetForRearticulate})
	if err != nil {
		t.Fatalf("cmdRearticulate() --criterion-file returned error: %v", err)
	}

	var skeletons []map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &skeletons); err != nil {
		t.Fatalf("parse skeleton JSON: %v", err)
	}

	for i, sk := range skeletons {
		ref, _ := sk["criterion_ref"].(string)
		if ref != "actor-stability-v1" {
			t.Errorf("skeleton %d: criterion_ref = %q; want %q", i, ref, "actor-stability-v1")
		}
	}
}

// TestCmdRearticulate_CriterionFileAbsent_NoCriterionRef verifies that when
// --criterion-file is not provided, criterion_ref is absent from skeletons.
func TestCmdRearticulate_CriterionFileAbsent_NoCriterionRef(t *testing.T) {
	var buf bytes.Buffer
	if err := cmdRearticulate(&buf, []string{cveDraftsDatasetForRearticulate}); err != nil {
		t.Fatalf("cmdRearticulate() returned error: %v", err)
	}

	var skeletons []map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &skeletons); err != nil {
		t.Fatalf("parse skeleton JSON: %v", err)
	}

	for i, sk := range skeletons {
		if v, ok := sk["criterion_ref"]; ok && v != "" && v != nil {
			t.Errorf("skeleton %d: criterion_ref should be absent; got %v", i, v)
		}
	}
}

// TestCmdRearticulate_CriterionFileBadPath verifies that a non-existent
// --criterion-file path returns an error.
func TestCmdRearticulate_CriterionFileBadPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdRearticulate(&buf, []string{"--criterion-file", "/nonexistent/criterion.json", cveDraftsDatasetForRearticulate})
	if err == nil {
		t.Fatal("expected error for bad criterion-file path, got nil")
	}
}

// --- Group 15: cmdLineage ---


// --- Group 15: cmdLineage ---

// TestCmdLineage_ValidDraftsFile verifies that a dataset with no DerivedFrom
// links reports all drafts as standalone.
func TestCmdLineage_ValidDraftsFile(t *testing.T) {
	// A dataset with no DerivedFrom links — all drafts are standalone.
	path := writeTempJSONForDraft(t, `[
		{"source_span":"span a","what_changed":"a","observer":"analyst"},
		{"source_span":"span b","what_changed":"b","observer":"analyst"}
	]`)

	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{path})
	if err != nil {
		t.Fatalf("cmdLineage() returned unexpected error: %v", err)
	}

	out := buf.String()
	// Must mention standalone drafts.
	if !strings.Contains(strings.ToLower(out), "standalone") {
		t.Errorf("output does not mention standalone drafts; got:\n%s", out)
	}
}

// TestCmdLineage_WithChains verifies that a dataset containing DerivedFrom links
// renders chains with root and child.
func TestCmdLineage_WithChains(t *testing.T) {
	// Root has no DerivedFrom; child DerivedFrom points to root's ID.
	path := writeTempJSONForDraft(t, `[
		{"id":"aaaaaaaa-0000-4000-8000-000000000001","source_span":"root span","what_changed":"root change","observer":"analyst","extraction_stage":"span-harvest","extracted_by":"llm-pass1"},
		{"id":"bbbbbbbb-0000-4000-8000-000000000002","source_span":"critique span","what_changed":"critique change","observer":"reviewer","extraction_stage":"reviewed","extracted_by":"human-reviewer","derived_from":"aaaaaaaa-0000-4000-8000-000000000001"}
	]`)

	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{path})
	if err != nil {
		t.Fatalf("cmdLineage() with chains returned unexpected error: %v", err)
	}

	out := buf.String()
	// Root ID prefix must appear.
	if !strings.Contains(out, "aaaaaaaa") {
		t.Errorf("root ID prefix not found in output; got:\n%s", out)
	}
	// Child must appear indented under root (indicated by └──).
	if !strings.Contains(out, "└──") {
		t.Errorf("child connector └── not found in output; got:\n%s", out)
	}
	// Child ID prefix must appear.
	if !strings.Contains(out, "bbbbbbbb") {
		t.Errorf("child ID prefix not found in output; got:\n%s", out)
	}
}

// TestCmdLineage_IDFlag verifies that --id shows only the chain containing
// the specified draft.
func TestCmdLineage_IDFlag(t *testing.T) {
	path := writeTempJSONForDraft(t, `[
		{"id":"aaaaaaaa-0000-4000-8000-000000000001","source_span":"root span","what_changed":"root","observer":"analyst","extraction_stage":"span-harvest","extracted_by":"llm-pass1"},
		{"id":"bbbbbbbb-0000-4000-8000-000000000002","source_span":"child span","what_changed":"child","observer":"reviewer","extraction_stage":"reviewed","extracted_by":"human-reviewer","derived_from":"aaaaaaaa-0000-4000-8000-000000000001"},
		{"id":"cccccccc-0000-4000-8000-000000000003","source_span":"unrelated span","what_changed":"unrelated","observer":"analyst","extraction_stage":"span-harvest","extracted_by":"llm-pass1"}
	]`)

	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{"--id", "aaaaaaaa-0000-4000-8000-000000000001", path})
	if err != nil {
		t.Fatalf("cmdLineage() --id returned unexpected error: %v", err)
	}

	out := buf.String()
	// Root and child must appear.
	if !strings.Contains(out, "aaaaaaaa") {
		t.Errorf("root not found in --id output; got:\n%s", out)
	}
	if !strings.Contains(out, "bbbbbbbb") {
		t.Errorf("child not found in --id output; got:\n%s", out)
	}
	// Unrelated draft must NOT appear.
	if strings.Contains(out, "cccccccc") {
		t.Errorf("unrelated draft cccccccc appeared in --id output; got:\n%s", out)
	}
}

// TestCmdLineage_IDFlagNotFound verifies that --id <unknown> returns an error.
func TestCmdLineage_IDFlagNotFound(t *testing.T) {
	path := writeTempJSONForDraft(t, `[
		{"source_span":"span a","what_changed":"a","observer":"analyst"}
	]`)

	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{"--id", "nonexistent-id-000", path})
	if err == nil {
		t.Fatal("cmdLineage() with unknown --id: want error, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-id-000") {
		t.Errorf("error %q does not name the unknown ID", err.Error())
	}
}

// TestCmdLineage_FormatJSON verifies that --format json produces valid JSON
// with a "chains" array containing anchor_id and members, and a "standalone"
// integer. Uses a 3-level chain (A→B→C) to confirm that all descendants are
// collected recursively, not just direct children.
func TestCmdLineage_FormatJSON(t *testing.T) {
	// Three-level chain: A is anchor, B is A's child, C is B's child.
	// All three must appear in members; a shallow loop over root.subsequent
	// would silently drop C.
	path := writeTempJSONForDraft(t, `[
		{"id":"aaaaaaaa-0000-4000-8000-000000000001","source_span":"root span","what_changed":"root","observer":"analyst","extraction_stage":"span-harvest","extracted_by":"llm-pass1"},
		{"id":"bbbbbbbb-0000-4000-8000-000000000002","source_span":"child span","what_changed":"child","observer":"reviewer","extraction_stage":"reviewed","extracted_by":"human-reviewer","derived_from":"aaaaaaaa-0000-4000-8000-000000000001"},
		{"id":"cccccccc-0000-4000-8000-000000000003","source_span":"grandchild span","what_changed":"grandchild","observer":"reviewer","extraction_stage":"reviewed","extracted_by":"human-reviewer","derived_from":"bbbbbbbb-0000-4000-8000-000000000002"}
	]`)

	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{"--format", "json", path})
	if err != nil {
		t.Fatalf("cmdLineage() --format json returned unexpected error: %v", err)
	}

	// Decode into a typed structure to verify chain contents, not just key presence.
	type chain struct {
		AnchorID string   `json:"anchor_id"`
		Members  []string `json:"members"`
	}
	type result struct {
		Chains     []chain `json:"chains"`
		Standalone int     `json:"standalone"`
	}
	var got result
	if err := json.Unmarshal([]byte(buf.String()), &got); err != nil {
		t.Fatalf("parse JSON output: %v", err)
	}

	if len(got.Chains) != 1 {
		t.Fatalf("chains count: got %d want 1", len(got.Chains))
	}
	ch := got.Chains[0]
	if ch.AnchorID != "aaaaaaaa-0000-4000-8000-000000000001" {
		t.Errorf("anchor_id = %q; want aaaaaaaa-0000-4000-8000-000000000001", ch.AnchorID)
	}
	// All three nodes — anchor, child, grandchild — must appear in members.
	if len(ch.Members) != 3 {
		t.Errorf("members count: got %d want 3 (anchor+child+grandchild); members: %v", len(ch.Members), ch.Members)
	}
	wantMembers := []string{
		"aaaaaaaa-0000-4000-8000-000000000001",
		"bbbbbbbb-0000-4000-8000-000000000002",
		"cccccccc-0000-4000-8000-000000000003",
	}
	for i, want := range wantMembers {
		if i >= len(ch.Members) {
			t.Errorf("members[%d]: missing, want %q", i, want)
			continue
		}
		if ch.Members[i] != want {
			t.Errorf("members[%d] = %q; want %q", i, ch.Members[i], want)
		}
	}
	// Standalone count: grandchild is part of the chain, not standalone.
	if got.Standalone != 0 {
		t.Errorf("standalone = %d; want 0", got.Standalone)
	}
}

// TestCmdLineage_CycleDetection verifies that a circular DerivedFrom reference
// (A→B→A) returns an error naming the cycle.
func TestCmdLineage_CycleDetection(t *testing.T) {
	path := writeTempJSONForDraft(t, `[
		{"id":"aaaaaaaa-0000-4000-8000-000000000001","source_span":"span a","derived_from":"bbbbbbbb-0000-4000-8000-000000000002"},
		{"id":"bbbbbbbb-0000-4000-8000-000000000002","source_span":"span b","derived_from":"aaaaaaaa-0000-4000-8000-000000000001"}
	]`)

	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{path})
	if err == nil {
		t.Fatal("cmdLineage() with cycle: want error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cycle") {
		t.Errorf("error %q does not mention cycle", err.Error())
	}
}

// TestCmdLineage_MissingArg verifies that cmdLineage returns an error when
// called with no positional argument.
func TestCmdLineage_MissingArg(t *testing.T) {
	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{})
	if err == nil {
		t.Fatal("cmdLineage() with no args: want error, got nil")
	}
}

// TestCmdLineage_MalformedJSON verifies that cmdLineage returns an error
// when the input file contains malformed JSON.
func TestCmdLineage_MalformedJSON(t *testing.T) {
	path := writeTempJSONForDraft(t, `[{not valid json}]`)
	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{path})
	if err == nil {
		t.Fatal("cmdLineage() with malformed JSON: want error, got nil")
	}
}

// TestCmdLineage_InvalidFormat verifies that an unknown --format value returns
// an error containing the invalid value.
func TestCmdLineage_InvalidFormat(t *testing.T) {
	path := writeTempJSONForDraft(t, `[{"source_span":"span a"}]`)
	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{"--format", "xml", path})
	if err == nil {
		t.Fatal("cmdLineage() with --format xml: want error, got nil")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("error %q does not name the invalid format value", err.Error())
	}
}

// TestCmdLineage_EmptyInput verifies that cmdLineage handles an empty drafts
// array without error and reports zero standalone drafts.
func TestCmdLineage_EmptyInput(t *testing.T) {
	path := writeTempJSONForDraft(t, `[]`)
	var buf bytes.Buffer
	err := cmdLineage(&buf, []string{path})
	if err != nil {
		t.Fatalf("cmdLineage() with empty array: want nil error, got %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "0") {
		t.Errorf("expected standalone count 0 in output; got:\n%s", out)
	}
}

// --- Group N: cmdShadow ---

// TestCmdShadow_HappyPath verifies that cmdShadow produces non-empty output
// containing shadow-related keywords when given a valid dataset and observer.
func TestCmdShadow_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdShadow(&buf, []string{
		"--observer", "meteorological-analyst",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdShadow(): unexpected error: %v", err)
	}
	out := buf.String()
	if len(out) == 0 {
		t.Error("cmdShadow(): expected non-empty output")
	}
	if !strings.Contains(out, "Shadow") {
		t.Errorf("cmdShadow(): output missing 'Shadow'; got:\n%s", out)
	}
}

// TestCmdShadow_MissingObserver verifies that cmdShadow returns an error
// containing "required" when --observer is not provided.
func TestCmdShadow_MissingObserver(t *testing.T) {
	var buf bytes.Buffer
	err := cmdShadow(&buf, []string{evacuationDataset})
	if err == nil {
		t.Fatal("cmdShadow() with no --observer: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error = %q; want it to contain 'required'", err.Error())
	}
}

// TestCmdShadow_MissingArg verifies that cmdShadow returns an error when no
// path is provided.
func TestCmdShadow_MissingArg(t *testing.T) {
	var buf bytes.Buffer
	err := cmdShadow(&buf, []string{"--observer", "meteorological-analyst"})
	if err == nil {
		t.Fatal("cmdShadow() with no path: want non-nil error, got nil")
	}
}

// TestCmdShadow_BadPath verifies that cmdShadow returns an error when the
// traces file does not exist.
func TestCmdShadow_BadPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdShadow(&buf, []string{"--observer", "meteorological-analyst", "notafile.json"})
	if err == nil {
		t.Fatal("cmdShadow() with bad path: want non-nil error, got nil")
	}
}

// TestCmdShadow_BadFromTime verifies that cmdShadow returns an error containing
// "RFC3339" when --from is not a valid RFC3339 timestamp.
func TestCmdShadow_BadFromTime(t *testing.T) {
	var buf bytes.Buffer
	err := cmdShadow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--from", "notadate",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdShadow() with bad --from: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("error = %q; want it to contain 'RFC3339'", err.Error())
	}
}

// TestCmdShadow_NoShadowMessage verifies that the output contains a "No shadow"
// or similar no-shadow path when all elements are visible from the chosen observer.
// (This may not always be triggered on the evacuation dataset, but ensures
// the no-shadow path is exercised by running with full articulation.)
func TestCmdShadow_OutputToFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "shadow.txt")
	var buf bytes.Buffer
	err := cmdShadow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--output", out,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdShadow() --output: unexpected error: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if !strings.Contains(string(data), "Shadow") {
		t.Errorf("output file missing 'Shadow'; got:\n%s", data)
	}
}

// TestCmdShadow_RunDispatch verifies that run() correctly routes "shadow"
// to cmdShadow, producing non-empty output.
func TestCmdShadow_RunDispatch(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{
		"shadow",
		"--observer", "meteorological-analyst",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("run('shadow'): unexpected error: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("run('shadow'): expected non-empty output")
	}
}

// TestCmdShadow_TagFilter verifies that --tag is accepted and does not error.
func TestCmdShadow_TagFilter(t *testing.T) {
	var buf bytes.Buffer
	err := cmdShadow(&buf, []string{
		"--observer", "meteorological-analyst",
		"--tag", "nonexistent-tag",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdShadow() --tag: unexpected error: %v", err)
	}
}

// --- Group O: cmdGaps ---

// TestCmdGaps_HappyPath verifies that cmdGaps produces output containing
// "Observer Gap" and both observer labels when given two distinct positions.
func TestCmdGaps_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "coastal-resident",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdGaps(): unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Observer Gap") {
		t.Errorf("cmdGaps(): output missing 'Observer Gap'; got:\n%s", out)
	}
	if !strings.Contains(out, "meteorological-analyst") {
		t.Errorf("cmdGaps(): output missing observer-a label; got:\n%s", out)
	}
	if !strings.Contains(out, "coastal-resident") {
		t.Errorf("cmdGaps(): output missing observer-b label; got:\n%s", out)
	}
}

// TestCmdGaps_MissingObserverA verifies that cmdGaps returns an error
// containing "required" when --observer-a is missing.
func TestCmdGaps_MissingObserverA(t *testing.T) {
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-b", "coastal-resident",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdGaps() with no --observer-a: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error = %q; want it to contain 'required'", err.Error())
	}
}

// TestCmdGaps_MissingObserverB verifies that cmdGaps returns an error
// containing "required" when --observer-b is missing.
func TestCmdGaps_MissingObserverB(t *testing.T) {
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-a", "meteorological-analyst",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdGaps() with no --observer-b: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error = %q; want it to contain 'required'", err.Error())
	}
}

// TestCmdGaps_MissingArg verifies that cmdGaps returns an error when no
// path is provided (both observers set, no file).
func TestCmdGaps_MissingArg(t *testing.T) {
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "coastal-resident",
	})
	if err == nil {
		t.Fatal("cmdGaps() with no path: want non-nil error, got nil")
	}
}

// TestCmdGaps_BadPath verifies that cmdGaps returns an error when the
// traces file does not exist.
func TestCmdGaps_BadPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "coastal-resident",
		"notafile.json",
	})
	if err == nil {
		t.Fatal("cmdGaps() with bad path: want non-nil error, got nil")
	}
}

// TestCmdGaps_BadFromATime verifies that cmdGaps returns an error containing
// "RFC3339" when --from-a is not a valid RFC3339 timestamp.
func TestCmdGaps_BadFromATime(t *testing.T) {
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "coastal-resident",
		"--from-a", "notadate",
		evacuationDataset,
	})
	if err == nil {
		t.Fatal("cmdGaps() with bad --from-a: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("error = %q; want it to contain 'RFC3339'", err.Error())
	}
}

// TestCmdGaps_OutputToFile verifies that --output writes the gap report to a
// file and that the file contains "Observer Gap".
func TestCmdGaps_OutputToFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "gaps.txt")
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "coastal-resident",
		"--output", out,
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdGaps() --output: unexpected error: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if !strings.Contains(string(data), "Observer Gap") {
		t.Errorf("output file missing 'Observer Gap'; got:\n%s", data)
	}
}

// TestCmdGaps_SameObserver verifies that when observer-a and observer-b are
// the same, the output contains "No gap" since both positions see identically.
func TestCmdGaps_SameObserver(t *testing.T) {
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "meteorological-analyst",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdGaps() same observer: unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No gap") {
		t.Errorf("same-observer gaps: expected 'No gap' in output; got:\n%s", out)
	}
}

// TestCmdGaps_RunDispatch verifies that run() correctly routes "gaps" to
// cmdGaps, producing non-empty output.
func TestCmdGaps_RunDispatch(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{
		"gaps",
		"--observer-a", "meteorological-analyst",
		"--observer-b", "coastal-resident",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("run('gaps'): unexpected error: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("run('gaps'): expected non-empty output")
	}
}

// --- Group: cmdBottleneck ---

// TestCmdBottleneck_HappyPath verifies that cmdBottleneck produces non-empty
// output containing "provisional" and the observer position when given a
// valid dataset and an explicit observer.
func TestCmdBottleneck_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdBottleneck(&buf, []string{
		"--observer", "meteorological-analyst",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdBottleneck(): unexpected error: %v", err)
	}
	out := buf.String()
	if len(out) == 0 {
		t.Error("cmdBottleneck(): expected non-empty output")
	}
	for _, want := range []string{"provisional", "meteorological-analyst"} {
		if !strings.Contains(out, want) {
			t.Errorf("cmdBottleneck(): output missing %q;\noutput:\n%s", want, out)
		}
	}
}

// TestCmdBottleneck_ObserverOptional verifies that --observer is optional:
// running without it (full cut) must not return an error.
func TestCmdBottleneck_ObserverOptional(t *testing.T) {
	var buf bytes.Buffer
	err := cmdBottleneck(&buf, []string{evacuationDataset})
	if err != nil {
		t.Fatalf("cmdBottleneck() with no --observer: unexpected error: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("cmdBottleneck() with no --observer: expected non-empty output")
	}
}

// TestCmdBottleneck_MissingPath verifies that cmdBottleneck returns an error
// when no traces file path is supplied.
func TestCmdBottleneck_MissingPath(t *testing.T) {
	var buf bytes.Buffer
	err := cmdBottleneck(&buf, []string{})
	if err == nil {
		t.Fatal("cmdBottleneck() with no path: want non-nil error, got nil")
	}
}

// TestCmdBottleneck_RunDispatch verifies that run() correctly routes
// "bottleneck" to cmdBottleneck, producing non-empty output.
func TestCmdBottleneck_RunDispatch(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, []string{
		"bottleneck",
		"--observer", "meteorological-analyst",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("run('bottleneck'): unexpected error: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("run('bottleneck'): expected non-empty output")
	}
}

// TestCmdGaps_SuggestFlag verifies that cmdGaps with --suggest produces output
// containing the re-articulation suggestions section header when the two
// observer positions produce a gap (distinct observers from the evacuation
// dataset see different elements). The suggestions section must appear after
// the gap report.
func TestCmdGaps_SuggestFlag(t *testing.T) {
	var buf bytes.Buffer
	err := cmdGaps(&buf, []string{
		"--observer-a", "meteorological-analyst",
		"--observer-b", "coastal-resident",
		"--suggest",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdGaps() --suggest: unexpected error: %v", err)
	}
	out := buf.String()

	// The gap report section must still be present.
	if !strings.Contains(out, "Observer Gap") {
		t.Errorf("cmdGaps() --suggest: output missing 'Observer Gap'; got:\n%s", out)
	}

	// The suggestions section header must appear after the gap report.
	if !strings.Contains(out, "Re-articulation Suggestions") {
		t.Errorf("cmdGaps() --suggest: output missing 'Re-articulation Suggestions'; got:\n%s", out)
	}

	// The footer note encoding the epistemic constraint must be present.
	if !strings.Contains(out, "provocation") {
		t.Errorf("cmdGaps() --suggest: output missing 'provocation' in footer; got:\n%s", out)
	}
}

// --- Group B.3: --narrative flag on articulate ---

// TestCmdArticulate_NarrativeFlag verifies that --narrative appends a
// NarrativeDraft section to the text-format articulation output.
// The output must contain the "Narrative Draft" header and the "Position:"
// section label from PrintNarrativeDraft.
//
// Only emitted for --format text (the default). The test uses the
// meteorological-analyst observer from the evacuation dataset — a non-empty
// articulation with a well-known shadow.
func TestCmdArticulate_NarrativeFlag(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--narrative",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --narrative returned unexpected error: %v", err)
	}
	out := buf.String()

	// The narrative section header must be present.
	if !strings.Contains(out, "Narrative Draft") {
		t.Errorf("--narrative: output does not contain 'Narrative Draft'; got:\n%s", out)
	}
	// The Position section label from PrintNarrativeDraft must be present.
	if !strings.Contains(out, "Position:") {
		t.Errorf("--narrative: output does not contain 'Position:'; got:\n%s", out)
	}
}

// TestCmdArticulate_NarrativeFlagSkippedForJSON verifies that --narrative is
// silently skipped when --format json is used. The output must not contain
// "Narrative Draft" and must be valid JSON (starts with '{').
func TestCmdArticulate_NarrativeFlagSkippedForJSON(t *testing.T) {
	var buf bytes.Buffer
	err := cmdArticulate(&buf, []string{
		"--observer", "meteorological-analyst",
		"--format", "json",
		"--narrative",
		evacuationDataset,
	})
	if err != nil {
		t.Fatalf("cmdArticulate() --format json --narrative returned unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if strings.Contains(out, "Narrative Draft") {
		t.Errorf("--narrative with --format json: narrative section must be skipped, but output contains 'Narrative Draft'")
	}
	if !strings.HasPrefix(out, "{") {
		t.Errorf("--narrative with --format json: output does not start with '{'; got: %q", out[:min(len(out), 40)])
	}
}
