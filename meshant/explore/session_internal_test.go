// session_internal_test.go — white-box tests for AnalysisSession internals.
//
// Uses package explore (not explore_test) to access unexported fields
// that cannot be reached from the black-box test suite. Kept minimal: only
// tests that genuinely require internal access belong here.
package explore

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// testStoreInternal creates a real JSONFileStore backed by a temp file.
// Duplicates explore_test.testStore — a deliberate copy to keep the two
// test packages independent (no shared test helper across package boundaries).
func testStoreInternal(t *testing.T) store.TraceStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "traces.json")
	ts := store.NewJSONFileStore(path)
	t.Cleanup(func() { _ = ts.Close() })
	return ts
}

// TestTurns_TagsDeepCopy verifies that Turns() deep-copies the Tags slice
// within each turn so that callers cannot mutate the session's internal record
// by modifying returned tag elements.
//
// This test sets s.tags directly (white-box access) before recording a turn,
// so the Tags branch in both recordTurn and Turns() is exercised. The test
// will be superseded by a black-box equivalent when the `tags` command is
// wired in #183 — at that point this internal test may be removed.
//
// TODO(#183): replace with black-box test once `tags` command is implemented.
func TestTurns_TagsDeepCopy(t *testing.T) {
	s := NewSession(testStoreInternal(t), "analyst-1")

	// Directly set unexported s.tags to a non-empty slice so recordTurn
	// records a turn with non-nil Tags, exercising both copy branches.
	s.tags = []string{"incident", "2026"}

	// Record a turn by executing a cut (the only skeleton command that records).
	in := strings.NewReader("cut alice\nquit\n")
	if err := s.Run(context.Background(), in, &nopWriter{}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn after cut")
	}

	// Verify the turn recorded the tags.
	if len(turns[0].Tags) != 2 {
		t.Fatalf("Turn.Tags = %v, want 2 elements", turns[0].Tags)
	}

	// Mutate the returned Tags slice — the session's internal record must not change.
	turns[0].Tags[0] = "mutated"

	turns2 := s.Turns()
	if turns2[0].Tags[0] == "mutated" {
		t.Error("Turns() returned a reference to internal Tags; mutation propagated")
	}
	if turns2[0].Tags[0] != "incident" {
		t.Errorf("turns2[0].Tags[0] = %q, want %q", turns2[0].Tags[0], "incident")
	}
}

// TestRecordTurn_TagsDeepCopy verifies that recordTurn deep-copies s.tags
// so that a subsequent change to s.tags does not retroactively alter a
// completed turn's recorded conditions (D3 in explore-v1.md).
func TestRecordTurn_TagsDeepCopy(t *testing.T) {
	s := NewSession(testStoreInternal(t), "analyst-1")
	s.tags = []string{"alpha"}

	// Record a turn with tags = ["alpha"].
	s.recordTurn("cut alice", nil, nil)
	s.observer = "alice"

	// Mutate s.tags after recording.
	s.tags[0] = "mutated"
	s.tags = append(s.tags, "beta")

	// The recorded turn must still carry the original tags.
	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if len(turns[0].Tags) != 1 {
		t.Fatalf("Turn.Tags = %v, want 1 element (snapshot at record time)", turns[0].Tags)
	}
	if turns[0].Tags[0] != "alpha" {
		t.Errorf("Turn.Tags[0] = %q, want %q (snapshot must not reflect post-record mutation)", turns[0].Tags[0], "alpha")
	}
}

// nopWriter discards all output. Used in internal tests to satisfy io.Writer
// without importing bytes.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
