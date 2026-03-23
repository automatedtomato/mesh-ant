// testhelpers_test.go provides shared test helpers for the store_test package.
// Helpers here are available to all _test.go files in this directory.
package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// baseTime is a fixed reference timestamp used across helpers.
// Using a non-zero, deterministic time prevents accidental unbounded-window matches.
var baseTime = time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

// validTrace returns a minimal Trace that passes schema.Validate().
// Uses baseTime as Timestamp. Tests that need other field values can override
// fields on the returned struct directly.
func validTrace(id, whatChanged string) schema.Trace {
	return schema.Trace{
		ID:          id,
		Timestamp:   baseTime,
		WhatChanged: whatChanged,
		Observer:    "test-observer/position-a",
	}
}

// validTraceAt returns a valid Trace with a custom timestamp.
func validTraceAt(id, whatChanged string, ts time.Time) schema.Trace {
	t := validTrace(id, whatChanged)
	t.Timestamp = ts
	return t
}

// validTraceWithObserver returns a valid Trace with a custom Observer field.
func validTraceWithObserver(id, whatChanged, observer string) schema.Trace {
	t := validTrace(id, whatChanged)
	t.Observer = observer
	return t
}

// validTraceWithTags returns a valid Trace with the given Tags.
func validTraceWithTags(id, whatChanged string, tags []string) schema.Trace {
	t := validTrace(id, whatChanged)
	t.Tags = tags
	return t
}

// writeTempJSON marshals traces to a temp file and returns the file path.
// The file is automatically removed at end of test.
func writeTempJSON(t *testing.T, traces []schema.Trace) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "traces.json")
	data, err := json.MarshalIndent(traces, "", "  ")
	if err != nil {
		t.Fatalf("writeTempJSON: marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeTempJSON: write: %v", err)
	}
	return path
}

// tempPath returns a path in a temp dir that does not yet exist.
// Useful for testing Store() creating a new file.
func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "traces.json")
}
