// Package main contains the test suite for the demo entry point.
//
// Tests call run() and printClosingNote() directly. Using package main
// (internal) is the standard Go approach for testing main packages, since
// main packages cannot be imported by external test packages.
//
// Coverage note: main() is 0% (untestable in any Go main package). Excluding
// main(), the testable surface (run() + printClosingNote()) achieves ~86%.
// The remaining uncovered branches in run() are two defensive
// TimeWindow.Validate() guards that cannot be triggered by the hardcoded
// correct dates — by design, not by omission.
package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// errWriter is an io.Writer that always returns an error. Used to exercise
// write-error propagation in run() and printClosingNote().
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write error") }

// runDemo executes the full demo pipeline once and returns the output.
// It is the shared helper for all happy-path assertions, ensuring the
// pipeline runs exactly once per test group. Using t.Helper() + t.Fatalf
// aborts the calling test immediately on pipeline failure rather than
// cascading into misleading assertion failures.
func runDemo(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := run(&buf, defaultDatasetPath); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	return buf.String()
}

// --- Group 1: Happy path ---

// TestDemo_Run_HappyPath verifies the full pipeline output against the
// evacuation order dataset. All sub-tests share a single run() invocation.
func TestDemo_Run_HappyPath(t *testing.T) {
	output := runDemo(t)

	// Mesh summary must appear first in the output.
	t.Run("ContainsMeshSummary", func(t *testing.T) {
		if !strings.Contains(output, "=== Mesh Summary") {
			t.Error("output does not contain \"=== Mesh Summary\"")
		}
	})

	// Both articulations must be present; one would mean a cut was skipped.
	t.Run("ContainsBothArticulations", func(t *testing.T) {
		if count := strings.Count(output, "=== Mesh Articulation"); count < 2 {
			t.Errorf("output contains %d articulation block(s); want >= 2", count)
		}
	})

	// The diff is the analytical centrepiece; its absence means the
	// comparison was not performed.
	t.Run("ContainsDiff", func(t *testing.T) {
		if !strings.Contains(output, "=== Mesh Diff") {
			t.Error("output does not contain \"=== Mesh Diff\"")
		}
	})

	// Both observer positions must be named; their absence would indicate
	// the ArticulationOptions were not passed correctly.
	t.Run("NamesObservers", func(t *testing.T) {
		for _, obs := range []string{"meteorological-analyst", "local-mayor"} {
			if !strings.Contains(output, obs) {
				t.Errorf("output does not contain observer %q", obs)
			}
		}
	})

	// Both articulations must produce shadow elements given near-disjoint
	// observer positions.
	t.Run("ContainsShadow", func(t *testing.T) {
		if !strings.Contains(output, "Shadow") {
			t.Error("output does not contain \"Shadow\"")
		}
	})

	// The closing note names the Principle 8 gap; its absence would mean
	// the demo does not name its own shadow.
	t.Run("ContainsClosingNote", func(t *testing.T) {
		if !strings.Contains(output, "Note on this articulation") {
			t.Error("output does not contain closing note section")
		}
	})
}

// --- Group 2: Error paths ---

// TestDemo_Run_InvalidPath_ReturnsError verifies that run() propagates
// the error from loader.Load when the dataset path does not exist.
func TestDemo_Run_InvalidPath_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf, "/nonexistent/path/dataset.json"); err == nil {
		t.Error("run() with invalid path: want non-nil error, got nil")
	}
}

// TestDemo_Run_WriterError_ReturnsError verifies that run() propagates
// write errors. errWriter fails on the first write, which short-circuits
// at loader.PrintSummary — only that branch is directly exercised here.
func TestDemo_Run_WriterError_ReturnsError(t *testing.T) {
	if err := run(errWriter{}, defaultDatasetPath); err == nil {
		t.Error("run() with failing writer: want non-nil error, got nil")
	}
}

// TestDemo_PrintClosingNote_WriterError_ReturnsError verifies that
// printClosingNote() propagates write errors independently of the pipeline.
func TestDemo_PrintClosingNote_WriterError_ReturnsError(t *testing.T) {
	if err := printClosingNote(errWriter{}); err == nil {
		t.Error("printClosingNote() with failing writer: want non-nil error, got nil")
	}
}
