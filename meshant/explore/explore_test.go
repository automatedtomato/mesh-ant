// Package explore_test verifies the AnalysisSession REPL using black-box tests.
//
// Tests use injected io.Reader / io.Writer (strings.NewReader + bytes.Buffer)
// so no terminal is required. A real JSONFileStore backed by a temp file is used
// instead of a mock — consistency with the actual store interface is preferred
// over isolation from it (see review/session.go for the same pattern).
package explore_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/explore"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// testStore returns a JSONFileStore backed by a temp file with no traces.
// The store is closed automatically when the test ends via t.Cleanup.
func testStore(t *testing.T) store.TraceStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "traces.json")
	ts := store.NewJSONFileStore(path)
	t.Cleanup(func() { _ = ts.Close() })
	return ts
}

// run is a helper that runs the session with the given input and returns output.
func run(t *testing.T, s *explore.AnalysisSession, input string) string {
	t.Helper()
	in := strings.NewReader(input)
	var out bytes.Buffer
	if err := s.Run(context.Background(), in, &out); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	return out.String()
}

// === NewSession ===

func TestNewSession(t *testing.T) {
	ts := testStore(t)
	s := explore.NewSession(ts, "alice")

	if s.Analyst() != "alice" {
		t.Errorf("Analyst() = %q, want %q", s.Analyst(), "alice")
	}
	if s.Observer() != "" {
		t.Errorf("Observer() = %q, want empty string before any cut", s.Observer())
	}
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() len = %d, want 0 before any command", len(s.Turns()))
	}
}

func TestNewSession_EmptyAnalyst(t *testing.T) {
	// An empty analyst is permitted — the session works without a named conductor.
	s := explore.NewSession(testStore(t), "")
	if s.Analyst() != "" {
		t.Errorf("Analyst() = %q, want empty string", s.Analyst())
	}
}

// === Quit and EOF ===

func TestRun_Quit(t *testing.T) {
	s := explore.NewSession(testStore(t), "alice")
	run(t, s, "quit\n")

	// A bare quit records no turns — no analytical act was performed.
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after bare quit, want 0", len(s.Turns()))
	}
}

func TestRun_QuitAlias(t *testing.T) {
	// "q" is accepted as an alias for "quit".
	s := explore.NewSession(testStore(t), "alice")
	run(t, s, "q\n")
	// A bare 'q' records no turns — same contract as 'quit'.
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after 'q', want 0", len(s.Turns()))
	}
}

func TestRun_EOF(t *testing.T) {
	// EOF on the reader is treated identically to quit — consistent with
	// the review/session.go pattern; a closed reader is an unrecoverable state.
	s := explore.NewSession(testStore(t), "alice")
	run(t, s, "")
	// EOF records no turns and leaves state unchanged.
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after EOF, want 0", len(s.Turns()))
	}
	if s.Observer() != "" {
		t.Errorf("Observer() = %q after EOF, want empty", s.Observer())
	}
}

func TestRun_EmptyLines(t *testing.T) {
	// Empty lines are silently skipped; session continues to quit.
	s := explore.NewSession(testStore(t), "alice")
	run(t, s, "\n\n\nquit\n")
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after empty lines + quit, want 0", len(s.Turns()))
	}
}

// === cut command ===

func TestRun_Cut(t *testing.T) {
	s := explore.NewSession(testStore(t), "analyst-1")
	out := run(t, s, "cut on-call-engineer\nquit\n")

	if s.Observer() != "on-call-engineer" {
		t.Errorf("Observer() = %q, want %q", s.Observer(), "on-call-engineer")
	}

	// cut prints a confirmation message containing the observer name.
	if !strings.Contains(out, "on-call-engineer") {
		t.Errorf("cut output should contain the observer name, got:\n%s", out)
	}

	// cut records exactly one turn.
	turns := s.Turns()
	if len(turns) != 1 {
		t.Fatalf("Turns() = %d, want 1", len(turns))
	}

	// The turn records the observer that became active as a result of the cut.
	if turns[0].Command != "cut on-call-engineer" {
		t.Errorf("Turn.Command = %q, want %q", turns[0].Command, "cut on-call-engineer")
	}
	if turns[0].Observer != "on-call-engineer" {
		t.Errorf("Turn.Observer = %q, want %q", turns[0].Observer, "on-call-engineer")
	}
	if turns[0].ExecutedAt.IsZero() {
		t.Error("Turn.ExecutedAt should be non-zero")
	}
}

func TestRun_Cut_NoArg(t *testing.T) {
	// cut without an observer name shows an inline error; observer unchanged.
	s := explore.NewSession(testStore(t), "analyst-1")
	out := run(t, s, "cut\nquit\n")

	if s.Observer() != "" {
		t.Errorf("Observer() = %q after failed cut, want empty", s.Observer())
	}
	if !strings.Contains(out, "observer name required") {
		t.Errorf("expected 'observer name required' in output, got:\n%s", out)
	}
	// No turn recorded for a failed cut.
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after failed cut, want 0", len(s.Turns()))
	}
}

func TestRun_MultipleCuts(t *testing.T) {
	// Each cut records a turn; each turn snapshots the observer that became active.
	s := explore.NewSession(testStore(t), "analyst-1")
	run(t, s, "cut alice\ncut bob\ncut alice\nquit\n")

	if s.Observer() != "alice" {
		t.Errorf("Observer() = %q after final cut, want %q", s.Observer(), "alice")
	}

	turns := s.Turns()
	if len(turns) != 3 {
		t.Fatalf("Turns() = %d, want 3", len(turns))
	}

	// The positional trajectory: alice → bob → alice
	wantObservers := []string{"alice", "bob", "alice"}
	for i, want := range wantObservers {
		if turns[i].Observer != want {
			t.Errorf("turns[%d].Observer = %q, want %q", i, turns[i].Observer, want)
		}
	}
}

func TestRun_Cut_SnapshotsWindow(t *testing.T) {
	// At session start the window is zero-value; the turn records that.
	s := explore.NewSession(testStore(t), "analyst-1")
	run(t, s, "cut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if !turns[0].Window.IsZero() {
		t.Errorf("Turn.Window should be zero before any window command, got %+v", turns[0].Window)
	}
}

func TestRun_Cut_SnapshotsTags(t *testing.T) {
	// At session start tags are nil; the turn records that.
	s := explore.NewSession(testStore(t), "analyst-1")
	run(t, s, "cut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if turns[0].Tags != nil {
		t.Errorf("Turn.Tags = %v, want nil before any tags command", turns[0].Tags)
	}
}

// === help command ===

func TestRun_Help(t *testing.T) {
	s := explore.NewSession(testStore(t), "analyst-1")
	out := run(t, s, "help\nquit\n")

	// help output must mention all skeleton commands.
	for _, keyword := range []string{"cut", "help", "quit"} {
		if !strings.Contains(out, keyword) {
			t.Errorf("help output missing %q\nfull output:\n%s", keyword, out)
		}
	}
}

func TestRun_HelpAlias(t *testing.T) {
	// "h" produces the same output as "help" — all skeleton keywords present.
	s := explore.NewSession(testStore(t), "analyst-1")
	out := run(t, s, "h\nquit\n")
	for _, keyword := range []string{"cut", "help", "quit"} {
		if !strings.Contains(out, keyword) {
			t.Errorf("help alias 'h' output missing %q\nfull output:\n%s", keyword, out)
		}
	}
}

// === unknown command ===

func TestRun_UnknownCommand(t *testing.T) {
	s := explore.NewSession(testStore(t), "analyst-1")
	out := run(t, s, "foobar\nquit\n")

	// Unknown command shows an inline error; session continues.
	if !strings.Contains(out, "unknown command") {
		t.Errorf("expected 'unknown command' in output, got:\n%s", out)
	}
	// No turn recorded for an unknown command.
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after unknown command, want 0", len(s.Turns()))
	}
}

func TestRun_UnknownThenCut(t *testing.T) {
	// An unknown command does not terminate the session; subsequent commands work.
	s := explore.NewSession(testStore(t), "analyst-1")
	run(t, s, "foobar\ncut alice\nquit\n")

	if s.Observer() != "alice" {
		t.Errorf("Observer() = %q after unknown+cut, want %q", s.Observer(), "alice")
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

// === AnalysisTurn field access ===

func TestAnalysisTurn_SuggestionNilForCut(t *testing.T) {
	// Suggestion is non-nil only for the 'suggest' command (wired in #185).
	s := explore.NewSession(testStore(t), "alice")
	run(t, s, "cut on-call-engineer\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if turns[0].Suggestion != nil {
		t.Errorf("Turn.Suggestion should be nil for cut command, got %+v", turns[0].Suggestion)
	}
}

// === Turns returns a copy ===

func TestTurns_ReturnsCopy(t *testing.T) {
	// Mutations to the slice returned by Turns() must not affect the session.
	s := explore.NewSession(testStore(t), "alice")
	run(t, s, "cut alice\nquit\n")

	copy1 := s.Turns()
	if len(copy1) == 0 {
		t.Fatal("expected at least one turn")
	}
	copy1[0].Command = "mutated"

	copy2 := s.Turns()
	if copy2[0].Command == "mutated" {
		t.Error("Turns() returned a reference to internal state; mutation propagated")
	}
}
