// testhelpers_test.go provides shared test helpers for the graph_test package.
// Helpers defined here are available to all _test.go files in this directory.
package graph_test

import (
	"testing"
	"time"
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
