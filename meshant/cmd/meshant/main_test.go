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
