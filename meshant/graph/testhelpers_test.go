// testhelpers_test.go provides shared test helpers for the graph_test package.
// Helpers defined here are available to all _test.go files in this directory.
package graph_test

import (
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// mustParseTime parses an RFC3339 string and fatals the test on failure.
// A parse failure means the test was authored with a bad literal — it is
// not a runtime condition of the code under test. Using t.Fatalf integrates
// cleanly with the testing framework and produces a clear source line reference.
func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("mustParseTime: parse %q: %v", s, err)
	}
	return ts
}

// validTraceWithElements builds a valid schema.Trace with the given id,
// observer, source, and target slices. Used in Diff tests that need traces
// which produce graph nodes, without repeating construction inline.
// The id must be a lowercase hyphenated UUID string.
func validTraceWithElements(id, observer string, sources, targets []string) schema.Trace {
	t := schema.Trace{
		ID:          id,
		Timestamp:   time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
		WhatChanged: "something changed",
		Observer:    observer,
		Source:      sources,
		Target:      targets,
	}
	return t
}
