// Package main contains the test suite for the demo entry point.
//
// Tests call run() and printClosingNote() directly, keeping them fast and
// deterministic without executing the binary. Using package main (internal)
// is the standard Go approach for testing main packages, since main packages
// cannot be imported by external test packages.
//
// All tests resolve the dataset path relative to the package source directory,
// matching the convention used throughout the loader and graph test suites.
package main

import (
	"bytes"
	"strings"
	"testing"
)

// datasetPath is the path to the evacuation dataset relative to the package
// source directory. Go test sets the working directory to the package
// directory, so three levels up reaches the module root before descending
// into data/examples/.
const datasetPath = "../../../data/examples/evacuation_order.json"

// --- Group 1: Happy path ---

// TestDemo_Run_NoError verifies that run() returns nil for a valid dataset.
// This is the primary green-path smoke test: if this fails, all others are
// expected to fail too.
func TestDemo_Run_NoError(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf, datasetPath); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
}

// TestDemo_Run_OutputContainsMeshSummary verifies the output includes the
// mesh summary header produced by loader.PrintSummary. The summary is the
// first section of the pipeline and must always be present.
func TestDemo_Run_OutputContainsMeshSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf, datasetPath); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "=== Mesh Summary") {
		t.Error("output does not contain \"=== Mesh Summary\"")
	}
}

// TestDemo_Run_OutputContainsBothArticulations verifies the output includes
// two Mesh Articulation blocks — one for each observer cut. A single block
// would indicate one articulation silently failed or was skipped.
func TestDemo_Run_OutputContainsBothArticulations(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf, datasetPath); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	output := buf.String()
	const header = "=== Mesh Articulation"
	count := strings.Count(output, header)
	if count < 2 {
		t.Errorf("output contains %d %q block(s); want >= 2", count, header)
	}
}

// TestDemo_Run_OutputContainsDiff verifies the output includes the diff
// block produced by graph.PrintDiff. The diff is the analytical centrepiece
// of the demo; its absence would mean the comparison was not performed.
func TestDemo_Run_OutputContainsDiff(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf, datasetPath); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "=== Mesh Diff") {
		t.Error("output does not contain \"=== Mesh Diff\"")
	}
}

// TestDemo_Run_OutputNamesObservers verifies that both demo observer positions
// appear in the output. Their absence would indicate the articulation options
// were not passed correctly to graph.Articulate.
func TestDemo_Run_OutputNamesObservers(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf, datasetPath); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	output := buf.String()
	for _, obs := range []string{"meteorological-analyst", "local-mayor"} {
		if !strings.Contains(output, obs) {
			t.Errorf("output does not contain observer %q", obs)
		}
	}
}

// TestDemo_Run_OutputContainsShadow verifies the output includes at least one
// Shadow section. Both articulations should produce shadow elements given the
// near-disjoint observer positions; a missing Shadow would indicate the cut
// axes were not working or both cuts accidentally included all traces.
func TestDemo_Run_OutputContainsShadow(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf, datasetPath); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Shadow") {
		t.Error("output does not contain \"Shadow\"")
	}
}

// TestDemo_Run_OutputContainsClosingNote verifies the closing note is present.
// The note names the Principle 8 gap; its absence would mean the demo does not
// name its own shadow — contradicting the methodological stance of the project.
func TestDemo_Run_OutputContainsClosingNote(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf, datasetPath); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Note on this articulation") {
		t.Error("output does not contain closing note section")
	}
}

// --- Group 2: Error path ---

// TestDemo_Run_InvalidPath_ReturnsError verifies that run() returns a non-nil
// error when given a path that does not exist. This ensures the error path
// through loader.Load is handled and propagated rather than silently ignored.
func TestDemo_Run_InvalidPath_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	err := run(&buf, "/nonexistent/path/dataset.json")
	if err == nil {
		t.Error("run() with invalid path: want non-nil error, got nil")
	}
}
