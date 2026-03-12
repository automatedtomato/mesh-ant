// Package main contains the test suite for the meshant CLI entry point.
//
// Tests call run(), cmdSummarize(), and cmdValidate() directly using the
// package main approach (internal testing), which is the standard Go method
// for testing main packages that cannot be imported externally.
//
// Coverage note: main() itself is untestable from Go tests (Go cannot cover
// the main function). The testable surface — run(), cmdSummarize(),
// cmdValidate(), usage(), usageError() — should exceed 80% coverage.
//
// Dataset paths are relative to the test execution directory
// (meshant/cmd/meshant/). The module root is meshant/, so three levels up
// (../../../) from cmd/meshant/ reaches the repository root before descending
// into data/examples/. This matches the demo's defaultDatasetPath convention.
package main

import (
	"bytes"
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

// min returns the smaller of two ints. Used to safely truncate output in
// error messages without panicking on short strings.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
