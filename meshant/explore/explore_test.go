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
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/explore"
	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// baseTime is the fixed timestamp used in test trace construction.
var baseTime = time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

// newValidTrace returns a minimal Trace that passes schema.Validate().
// Used by testStoreWithTraces to build pre-populated test stores.
func newValidTrace(id, whatChanged, observer string) schema.Trace {
	return schema.Trace{
		ID:          id,
		Timestamp:   baseTime,
		WhatChanged: whatChanged,
		Observer:    observer,
	}
}

// testStoreWithTraces builds a JSONFileStore pre-populated with the given
// traces by storing them into a temp-dir-backed file.
func testStoreWithTraces(t *testing.T, traces []schema.Trace) store.TraceStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "traces.json")
	ts := store.NewJSONFileStore(path)
	t.Cleanup(func() { _ = ts.Close() })
	if err := ts.Store(context.Background(), traces); err != nil {
		t.Fatalf("testStoreWithTraces: %v", err)
	}
	return ts
}

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
	s := explore.NewSession(ts, "alice", nil)

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
	s := explore.NewSession(testStore(t), "", nil)
	if s.Analyst() != "" {
		t.Errorf("Analyst() = %q, want empty string", s.Analyst())
	}
}

// === Quit and EOF ===

func TestRun_Quit(t *testing.T) {
	s := explore.NewSession(testStore(t), "alice", nil)
	run(t, s, "quit\n")

	// A bare quit records no turns — no analytical act was performed.
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after bare quit, want 0", len(s.Turns()))
	}
}

func TestRun_QuitAlias(t *testing.T) {
	// "q" is accepted as an alias for "quit".
	s := explore.NewSession(testStore(t), "alice", nil)
	run(t, s, "q\n")
	// A bare 'q' records no turns — same contract as 'quit'.
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after 'q', want 0", len(s.Turns()))
	}
}

func TestRun_EOF(t *testing.T) {
	// EOF on the reader is treated identically to quit — consistent with
	// the review/session.go pattern; a closed reader is an unrecoverable state.
	s := explore.NewSession(testStore(t), "alice", nil)
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
	s := explore.NewSession(testStore(t), "alice", nil)
	run(t, s, "\n\n\nquit\n")
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after empty lines + quit, want 0", len(s.Turns()))
	}
}

// === cut command ===

func TestRun_Cut(t *testing.T) {
	s := explore.NewSession(testStore(t), "analyst-1", nil)
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
	s := explore.NewSession(testStore(t), "analyst-1", nil)
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
	s := explore.NewSession(testStore(t), "analyst-1", nil)
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
	s := explore.NewSession(testStore(t), "analyst-1", nil)
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
	s := explore.NewSession(testStore(t), "analyst-1", nil)
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
	s := explore.NewSession(testStore(t), "analyst-1", nil)
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
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "h\nquit\n")
	for _, keyword := range []string{"cut", "help", "quit"} {
		if !strings.Contains(out, keyword) {
			t.Errorf("help alias 'h' output missing %q\nfull output:\n%s", keyword, out)
		}
	}
}

// === unknown command ===

func TestRun_UnknownCommand(t *testing.T) {
	s := explore.NewSession(testStore(t), "analyst-1", nil)
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
	s := explore.NewSession(testStore(t), "analyst-1", nil)
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
	s := explore.NewSession(testStore(t), "alice", nil)
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
	s := explore.NewSession(testStore(t), "alice", nil)
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

// === window command ===

func TestRun_Window_Set(t *testing.T) {
	// window <from> <to> sets the session time window; a subsequent cut
	// turn snapshots the non-zero window.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "window 2026-01-01T00:00:00Z 2026-12-31T23:59:59Z\ncut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn (the cut)")
	}
	if turns[0].Window.IsZero() {
		t.Error("Turn.Window should be non-zero after window command before cut")
	}
}

func TestRun_Window_Clear(t *testing.T) {
	// window clear resets the window; the next cut turn captures zero window.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "window 2026-01-01T00:00:00Z 2026-12-31T23:59:59Z\nwindow clear\ncut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if !turns[0].Window.IsZero() {
		t.Errorf("Turn.Window = %+v after 'window clear'; want zero", turns[0].Window)
	}
}

func TestRun_Window_BareClears(t *testing.T) {
	// bare 'window' (no args) clears the window.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "window 2026-01-01T00:00:00Z 2026-12-31T23:59:59Z\nwindow\ncut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if !turns[0].Window.IsZero() {
		t.Errorf("Turn.Window = %+v after bare 'window'; want zero", turns[0].Window)
	}
}

func TestRun_Window_InvalidFrom(t *testing.T) {
	// Invalid from value prints an inline error; window unchanged; session continues.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "window not-a-date 2026-12-31T23:59:59Z\ncut alice\nquit\n")

	if !strings.Contains(out, "invalid") {
		t.Errorf("expected 'invalid' in output for bad from value, got:\n%s", out)
	}
	// Window was not set — the cut turn should have zero window.
	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn (the cut)")
	}
	if !turns[0].Window.IsZero() {
		t.Errorf("Turn.Window = %+v after failed window set; want zero", turns[0].Window)
	}
}

func TestRun_Window_InvalidTo(t *testing.T) {
	// Invalid to value prints an inline error; window unchanged.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "window 2026-01-01T00:00:00Z not-a-date\nquit\n")

	if !strings.Contains(out, "invalid") {
		t.Errorf("expected 'invalid' in output for bad to value, got:\n%s", out)
	}
}

func TestRun_Window_Inverted(t *testing.T) {
	// End before Start: TimeWindow.Validate() fires; inline error printed; window unchanged.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "window 2026-12-31T00:00:00Z 2026-01-01T00:00:00Z\ncut alice\nquit\n")

	// Validate() error message contains "before" describing the inverted bounds.
	if !strings.Contains(out, "before") && !strings.Contains(out, "inverted") {
		t.Errorf("expected window validation error in output, got:\n%s", out)
	}
	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn (the cut)")
	}
	if !turns[0].Window.IsZero() {
		t.Errorf("Turn.Window = %+v after inverted window; want zero", turns[0].Window)
	}
}

func TestRun_Window_WrongArgCount(t *testing.T) {
	// One argument that is not "clear" is a usage error.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "window 2026-01-01T00:00:00Z\nquit\n")

	if !strings.Contains(out, "usage") && !strings.Contains(out, "window") {
		t.Errorf("expected usage hint in output for wrong arg count, got:\n%s", out)
	}
}

func TestRun_Window_NoTurnRecorded(t *testing.T) {
	// window command does not record a turn — it is a filter setter, not an analytical act.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "window 2026-01-01T00:00:00Z 2026-12-31T23:59:59Z\nquit\n")

	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after window command, want 0", len(s.Turns()))
	}
}

func TestRun_Window_PrintsConfirmation(t *testing.T) {
	// window command prints a confirmation showing the active bounds.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "window 2026-01-01T00:00:00Z 2026-12-31T23:59:59Z\nquit\n")

	if !strings.Contains(out, "2026") {
		t.Errorf("window confirmation should contain year '2026', got:\n%s", out)
	}
}

// === tags command ===

func TestRun_Tags_Set(t *testing.T) {
	// tags <t1> <t2...> sets the session tag filter; the next cut turn snapshots it.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "tags delay threshold\ncut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if len(turns[0].Tags) != 2 {
		t.Fatalf("Turn.Tags = %v, want 2 elements", turns[0].Tags)
	}
	if turns[0].Tags[0] != "delay" || turns[0].Tags[1] != "threshold" {
		t.Errorf("Turn.Tags = %v, want [delay threshold]", turns[0].Tags)
	}
}

func TestRun_Tags_Replace(t *testing.T) {
	// A second tags command replaces (not appends) the tag filter.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "tags delay\ntags amplification\ncut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if len(turns[0].Tags) != 1 || turns[0].Tags[0] != "amplification" {
		t.Errorf("Turn.Tags = %v, want [amplification] (replace, not append)", turns[0].Tags)
	}
}

func TestRun_Tags_Clear(t *testing.T) {
	// tags clear resets the filter to nil.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "tags delay\ntags clear\ncut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if turns[0].Tags != nil {
		t.Errorf("Turn.Tags = %v after 'tags clear', want nil", turns[0].Tags)
	}
}

func TestRun_Tags_BareClears(t *testing.T) {
	// bare 'tags' (no args) clears the filter.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "tags delay\ntags\ncut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if turns[0].Tags != nil {
		t.Errorf("Turn.Tags = %v after bare 'tags', want nil", turns[0].Tags)
	}
}

func TestRun_Tags_NoTurnRecorded(t *testing.T) {
	// tags command does not record a turn — it is a filter setter.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "tags delay threshold\nquit\n")

	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d after tags command, want 0", len(s.Turns()))
	}
}

func TestRun_Tags_PrintsConfirmation(t *testing.T) {
	// tags command prints a confirmation showing the active tags.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "tags delay threshold\nquit\n")

	if !strings.Contains(out, "delay") {
		t.Errorf("tags confirmation should contain 'delay', got:\n%s", out)
	}
	if !strings.Contains(out, "threshold") {
		t.Errorf("tags confirmation should contain 'threshold', got:\n%s", out)
	}
}

// === articulate command ===

func TestRun_Articulate_NilStore(t *testing.T) {
	// With a nil store, articulate prints an inline error and the session continues.
	s := explore.NewSession(nil, "alice", nil)
	out := run(t, s, "cut alice\narticulate\nquit\n")

	if !strings.Contains(out, "no trace substrate") {
		t.Errorf("expected 'no trace substrate' error, got:\n%s", out)
	}
	// The cut turn recorded, but articulate did not add another.
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Articulate_NoObserver(t *testing.T) {
	// Without a prior cut, articulate prints an inline error about missing observer.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "articulate\nquit\n")

	if !strings.Contains(out, "observer") {
		t.Errorf("expected 'observer' error, got:\n%s", out)
	}
	// No turn recorded for a failed analytical command.
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d, want 0", len(s.Turns()))
	}
}

func TestRun_Articulate_HappyPath(t *testing.T) {
	// cut then articulate: output contains the articulation header; turn is recorded.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee02", "relay B dropped packet", "security-team"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\narticulate\nquit\n")

	if !strings.Contains(out, "=== Mesh Articulation") {
		t.Errorf("expected articulation header in output, got:\n%s", out)
	}
}

func TestRun_Articulate_RecordsTurn(t *testing.T) {
	// articulate records a turn with the correct Command and Observer fields.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	run(t, s, "cut ops-engineer\narticulate\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + articulate)", len(turns))
	}
	artTurn := turns[1]
	if artTurn.Command != "articulate" {
		t.Errorf("Turn.Command = %q, want %q", artTurn.Command, "articulate")
	}
	if artTurn.Observer != "ops-engineer" {
		t.Errorf("Turn.Observer = %q, want %q", artTurn.Observer, "ops-engineer")
	}
	if artTurn.ExecutedAt.IsZero() {
		t.Error("Turn.ExecutedAt should be non-zero")
	}
	if artTurn.Reading == nil {
		t.Error("Turn.Reading should be non-nil (graph.MeshGraph)")
	}
}

func TestRun_Articulate_WithWindowAndTags(t *testing.T) {
	// window and tags set before articulate are snapshotted in the turn.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	run(t, s, "window 2026-01-01T00:00:00Z 2026-12-31T23:59:59Z\ntags delay\ncut ops-engineer\narticulate\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + articulate)", len(turns))
	}
	artTurn := turns[1]
	if artTurn.Window.IsZero() {
		t.Error("Turn.Window should be non-zero (window was set before articulate)")
	}
	if len(artTurn.Tags) != 1 || artTurn.Tags[0] != "delay" {
		t.Errorf("Turn.Tags = %v, want [delay]", artTurn.Tags)
	}
}

// === shadow command ===

func TestRun_Shadow_NilStore(t *testing.T) {
	// With a nil store, shadow prints an inline error and the session continues.
	s := explore.NewSession(nil, "alice", nil)
	out := run(t, s, "cut alice\nshadow\nquit\n")

	if !strings.Contains(out, "no trace substrate") {
		t.Errorf("expected 'no trace substrate' error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Shadow_NoObserver(t *testing.T) {
	// Without a prior cut, shadow prints an inline error about missing observer.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "shadow\nquit\n")

	if !strings.Contains(out, "observer") {
		t.Errorf("expected 'observer' error, got:\n%s", out)
	}
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d, want 0", len(s.Turns()))
	}
}

func TestRun_Shadow_HappyPath(t *testing.T) {
	// cut then shadow: output contains the shadow summary header; turn is recorded.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee02", "relay B dropped packet", "security-team"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nshadow\nquit\n")

	if !strings.Contains(out, "=== Shadow Summary") {
		t.Errorf("expected shadow summary header in output, got:\n%s", out)
	}
}

func TestRun_Shadow_RecordsTurn(t *testing.T) {
	// shadow records a turn with the correct Command and Reading fields.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	run(t, s, "cut ops-engineer\nshadow\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + shadow)", len(turns))
	}
	shadowTurn := turns[1]
	if shadowTurn.Command != "shadow" {
		t.Errorf("Turn.Command = %q, want %q", shadowTurn.Command, "shadow")
	}
	if shadowTurn.Observer != "ops-engineer" {
		t.Errorf("Turn.Observer = %q, want %q", shadowTurn.Observer, "ops-engineer")
	}
	if shadowTurn.Reading == nil {
		t.Error("Turn.Reading should be non-nil (graph.ShadowSummary)")
	}
}

// === articulate — filter application ===

func TestRun_Articulate_WindowFilterApplied(t *testing.T) {
	// window filter is passed to graph.Articulate — not merely snapshotted.
	// Trace 1 falls inside the window; trace 2 falls outside. The articulation
	// must include only 1 of the 2 traces; MeshGraph.Cut.TracesIncluded == 1.
	inside := schema.Trace{
		ID:          "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee03",
		Timestamp:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		WhatChanged: "relay A routed packet",
		Observer:    "ops-engineer",
	}
	outside := schema.Trace{
		ID:          "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee04",
		Timestamp:   time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), // before window start
		WhatChanged: "relay B dropped packet",
		Observer:    "ops-engineer",
	}
	s := explore.NewSession(testStoreWithTraces(t, []schema.Trace{inside, outside}), "analyst-1", nil)
	run(t, s, "window 2026-01-01T00:00:00Z 2026-12-31T23:59:59Z\ncut ops-engineer\narticulate\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + articulate)", len(turns))
	}
	g, ok := turns[1].Reading.(graph.MeshGraph)
	if !ok {
		t.Fatalf("Turn.Reading type = %T, want graph.MeshGraph", turns[1].Reading)
	}
	// Only the in-window trace should be included; the other should be shadowed.
	if g.Cut.TracesIncluded != 1 {
		t.Errorf("Cut.TracesIncluded = %d, want 1 (only the in-window trace)", g.Cut.TracesIncluded)
	}
}

func TestRun_Articulate_EmptyStore(t *testing.T) {
	// articulate against a non-nil but empty store is a valid analytical act:
	// the cut produced zero traces; the graph has no nodes and no shadow.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\narticulate\nquit\n")

	// The articulation header must still appear — an empty graph is a valid reading.
	if !strings.Contains(out, "=== Mesh Articulation") {
		t.Errorf("expected articulation header even for empty store, got:\n%s", out)
	}
	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + articulate)", len(turns))
	}
	g, ok := turns[1].Reading.(graph.MeshGraph)
	if !ok {
		t.Fatalf("Turn.Reading type = %T, want graph.MeshGraph", turns[1].Reading)
	}
	if g.Cut.TracesIncluded != 0 {
		t.Errorf("Cut.TracesIncluded = %d for empty store, want 0", g.Cut.TracesIncluded)
	}
}

func TestRun_Shadow_EmptyStore(t *testing.T) {
	// shadow against an empty store: zero shadow elements; summary header present.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nshadow\nquit\n")

	if !strings.Contains(out, "=== Shadow Summary") {
		t.Errorf("expected shadow header even for empty store, got:\n%s", out)
	}
	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + shadow)", len(turns))
	}
	ss, ok := turns[1].Reading.(graph.ShadowSummary)
	if !ok {
		t.Fatalf("Turn.Reading type = %T, want graph.ShadowSummary", turns[1].Reading)
	}
	if ss.TotalShadowed != 0 {
		t.Errorf("ShadowSummary.TotalShadowed = %d for empty store, want 0", ss.TotalShadowed)
	}
}

// === articulate/shadow — query error path ===

// errStore is a minimal TraceStore stub that returns a fixed error on Query.
// Used exclusively to test the query-error branches in cmdArticulate and
// cmdShadow, which cannot be reliably triggered with a real JSONFileStore.
type errStore struct{}

func (e errStore) Store(_ context.Context, _ []schema.Trace) error { return nil }
func (e errStore) Query(_ context.Context, _ store.QueryOpts) ([]schema.Trace, error) {
	return nil, fmt.Errorf("stub query error")
}
func (e errStore) Get(_ context.Context, _ string) (schema.Trace, bool, error) {
	return schema.Trace{}, false, nil
}
func (e errStore) Close() error { return nil }

func TestRun_Articulate_QueryError(t *testing.T) {
	// When the store returns a query error, articulate prints an inline error
	// and the session continues without recording a turn.
	s := explore.NewSession(errStore{}, "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\narticulate\nquit\n")

	if !strings.Contains(out, "failed to load traces") {
		t.Errorf("expected query error message, got:\n%s", out)
	}
	// cut recorded one turn; articulate recorded none (it failed).
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Shadow_QueryError(t *testing.T) {
	// When the store returns a query error, shadow prints an inline error
	// and the session continues without recording a turn.
	s := explore.NewSession(errStore{}, "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nshadow\nquit\n")

	if !strings.Contains(out, "failed to load traces") {
		t.Errorf("expected query error message, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

// === tags — "clear" as non-sole argument ===

func TestRun_Tags_ClearWithOtherArgs_TreatedAsLiteral(t *testing.T) {
	// "tags clear foo" treats "clear" as a literal tag, not a keyword —
	// the keyword only fires when "clear" is the sole argument.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "tags clear foo\ncut alice\nquit\n")

	turns := s.Turns()
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	if len(turns[0].Tags) != 2 {
		t.Fatalf("Turn.Tags = %v, want 2 elements (clear, foo)", turns[0].Tags)
	}
	if turns[0].Tags[0] != "clear" || turns[0].Tags[1] != "foo" {
		t.Errorf("Turn.Tags = %v, want [clear foo]", turns[0].Tags)
	}
}

// === help command — updated for batch 1 commands ===

func TestRun_Help_ShowsNewCommands(t *testing.T) {
	// help output lists all batch-1 commands.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "help\nquit\n")

	for _, keyword := range []string{"articulate", "shadow", "window", "tags"} {
		if !strings.Contains(out, keyword) {
			t.Errorf("help output missing %q\nfull output:\n%s", keyword, out)
		}
	}
}

// === follow command ===

func TestRun_Follow_NilStore(t *testing.T) {
	// nil store prints inline error; no turn recorded beyond the cut.
	s := explore.NewSession(nil, "alice", nil)
	out := run(t, s, "cut alice\nfollow relay-a\nquit\n")

	if !strings.Contains(out, "no trace substrate") {
		t.Errorf("expected 'no trace substrate' error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Follow_NoObserver(t *testing.T) {
	// Without a prior cut, follow prints an inline error.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "follow relay-a\nquit\n")

	if !strings.Contains(out, "observer") {
		t.Errorf("expected 'observer' error, got:\n%s", out)
	}
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d, want 0", len(s.Turns()))
	}
}

func TestRun_Follow_NoElement(t *testing.T) {
	// Missing element argument prints inline error; session continues.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "cut alice\nfollow\nquit\n")

	if !strings.Contains(out, "element") {
		t.Errorf("expected 'element' in error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Follow_HappyPath(t *testing.T) {
	// cut then follow: output contains the chain header; turn is recorded.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nfollow relay\nquit\n")

	if !strings.Contains(out, "=== Translation Chain (provisional reading) ===") {
		t.Errorf("expected chain header in output, got:\n%s", out)
	}
	if len(s.Turns()) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + follow)", len(s.Turns()))
	}
}

func TestRun_Follow_RecordsTurn(t *testing.T) {
	// follow records a turn with the correct Command, Observer, and Reading type.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	run(t, s, "cut ops-engineer\nfollow relay\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + follow)", len(turns))
	}
	ft := turns[1]
	if ft.Command != "follow relay" {
		t.Errorf("Turn.Command = %q, want %q", ft.Command, "follow relay")
	}
	if ft.Observer != "ops-engineer" {
		t.Errorf("Turn.Observer = %q, want %q", ft.Observer, "ops-engineer")
	}
	if _, ok := ft.Reading.(graph.ClassifiedChain); !ok {
		t.Errorf("Turn.Reading type = %T, want graph.ClassifiedChain", ft.Reading)
	}
}

func TestRun_Follow_WithMaxDepth(t *testing.T) {
	// follow element 2 parses max_depth=2 without error; turn recorded.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	run(t, s, "cut ops-engineer\nfollow relay 2\nquit\n")

	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d after follow with max_depth, want 2", len(s.Turns()))
	}
}

func TestRun_Follow_ZeroMaxDepth(t *testing.T) {
	// follow relay 0 is valid; 0 means unlimited in the graph engine.
	// The command should complete, produce output, and record a turn.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nfollow relay 0\nquit\n")

	if !strings.Contains(out, "=== Translation Chain (provisional reading) ===") {
		t.Errorf("expected chain header in output, got:\n%s", out)
	}
	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + follow)", len(turns))
	}
	if turns[1].Command != "follow relay 0" {
		t.Errorf("Turn.Command = %q, want %q", turns[1].Command, "follow relay 0")
	}
}

func TestRun_Follow_NegativeMaxDepth(t *testing.T) {
	// follow relay -1 is treated as unlimited (graph engine: MaxDepth <= 0 means unlimited).
	// The command should complete without error and record a turn.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nfollow relay -1\nquit\n")

	if !strings.Contains(out, "=== Translation Chain (provisional reading) ===") {
		t.Errorf("expected chain header for negative max_depth (treated as unlimited), got:\n%s", out)
	}
	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d, want 2 (negative max_depth treated as unlimited)", len(s.Turns()))
	}
}

func TestRun_Follow_InvalidMaxDepth(t *testing.T) {
	// Non-integer max_depth prints inline error; no turn beyond cut.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "cut alice\nfollow relay abc\nquit\n")

	if !strings.Contains(out, "invalid") || !strings.Contains(out, "max_depth") {
		t.Errorf("expected 'invalid max_depth' error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Follow_QueryError(t *testing.T) {
	// Store query error prints inline error; no turn beyond cut.
	s := explore.NewSession(errStore{}, "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nfollow relay\nquit\n")

	if !strings.Contains(out, "failed to load traces") {
		t.Errorf("expected query error message, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

// === bottleneck command ===

func TestRun_Bottleneck_NilStore(t *testing.T) {
	// nil store prints inline error; no turn beyond cut.
	s := explore.NewSession(nil, "alice", nil)
	out := run(t, s, "cut alice\nbottleneck\nquit\n")

	if !strings.Contains(out, "no trace substrate") {
		t.Errorf("expected 'no trace substrate' error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Bottleneck_NoObserver(t *testing.T) {
	// Without a prior cut, bottleneck prints an inline error.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "bottleneck\nquit\n")

	if !strings.Contains(out, "observer") {
		t.Errorf("expected 'observer' error, got:\n%s", out)
	}
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d, want 0", len(s.Turns()))
	}
}

func TestRun_Bottleneck_HappyPath(t *testing.T) {
	// cut then bottleneck: output produced; turn recorded.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nbottleneck\nquit\n")

	// PrintBottleneckNotes always emits this exact header.
	if !strings.Contains(out, "=== Bottleneck Notes (provisional reading from this cut) ===") {
		t.Errorf("expected bottleneck header in output, got:\n%s", out)
	}
	if len(s.Turns()) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + bottleneck)", len(s.Turns()))
	}
}

func TestRun_Bottleneck_RecordsTurn(t *testing.T) {
	// bottleneck records a turn with the correct Command, Observer, Reading type.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	run(t, s, "cut ops-engineer\nbottleneck\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + bottleneck)", len(turns))
	}
	bt := turns[1]
	if bt.Command != "bottleneck" {
		t.Errorf("Turn.Command = %q, want %q", bt.Command, "bottleneck")
	}
	if bt.Observer != "ops-engineer" {
		t.Errorf("Turn.Observer = %q, want %q", bt.Observer, "ops-engineer")
	}
	if _, ok := bt.Reading.([]graph.BottleneckNote); !ok {
		t.Errorf("Turn.Reading type = %T, want []graph.BottleneckNote", bt.Reading)
	}
}

func TestRun_Bottleneck_EmptyStore(t *testing.T) {
	// bottleneck against an empty store is valid: no notes; turn still recorded.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	run(t, s, "cut ops-engineer\nbottleneck\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2", len(turns))
	}
	notes, ok := turns[1].Reading.([]graph.BottleneckNote)
	if !ok {
		t.Fatalf("Turn.Reading type = %T, want []graph.BottleneckNote", turns[1].Reading)
	}
	if len(notes) != 0 {
		t.Errorf("notes = %d for empty store, want 0", len(notes))
	}
}

func TestRun_Bottleneck_QueryError(t *testing.T) {
	// Store query error prints inline error; no turn beyond cut.
	s := explore.NewSession(errStore{}, "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nbottleneck\nquit\n")

	if !strings.Contains(out, "failed to load traces") {
		t.Errorf("expected query error message, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

// === diff command ===

func TestRun_Diff_NilStore(t *testing.T) {
	// nil store prints inline error; no turn beyond cut.
	s := explore.NewSession(nil, "alice", nil)
	out := run(t, s, "cut alice\ndiff bob\nquit\n")

	if !strings.Contains(out, "no trace substrate") {
		t.Errorf("expected 'no trace substrate' error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Diff_NoObserver(t *testing.T) {
	// Without a prior cut, diff prints an inline error.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "diff bob\nquit\n")

	if !strings.Contains(out, "observer") {
		t.Errorf("expected 'observer' error, got:\n%s", out)
	}
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d, want 0", len(s.Turns()))
	}
}

func TestRun_Diff_NoArg(t *testing.T) {
	// diff without observer-b prints a usage error; no turn beyond cut.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "cut alice\ndiff\nquit\n")

	if !strings.Contains(out, "observer-b") {
		t.Errorf("expected 'observer-b' in error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Diff_HappyPath(t *testing.T) {
	// cut obs-a then diff obs-b: output contains diff header; turn recorded.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee02", "relay B dropped packet", "security-team"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\ndiff security-team\nquit\n")

	// PrintDiff always emits this exact header.
	if !strings.Contains(out, "=== Mesh Diff (situated comparison) ===") {
		t.Errorf("expected diff header in output, got:\n%s", out)
	}
	if len(s.Turns()) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + diff)", len(s.Turns()))
	}
}

func TestRun_Diff_RecordsTurn(t *testing.T) {
	// diff records a turn with correct Command, Observer, and Reading type.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee02", "relay B dropped packet", "security-team"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	run(t, s, "cut ops-engineer\ndiff security-team\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2", len(turns))
	}
	dt := turns[1]
	if dt.Command != "diff security-team" {
		t.Errorf("Turn.Command = %q, want %q", dt.Command, "diff security-team")
	}
	if dt.Observer != "ops-engineer" {
		t.Errorf("Turn.Observer = %q, want %q", dt.Observer, "ops-engineer")
	}
	if _, ok := dt.Reading.(graph.GraphDiff); !ok {
		t.Errorf("Turn.Reading type = %T, want graph.GraphDiff", dt.Reading)
	}
}

func TestRun_Diff_QueryError(t *testing.T) {
	// Store query error prints inline error; no turn beyond cut.
	s := explore.NewSession(errStore{}, "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\ndiff security-team\nquit\n")

	if !strings.Contains(out, "failed to load traces") {
		t.Errorf("expected query error message, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Diff_SameObserver(t *testing.T) {
	// diff when observer A == observer B produces a zero-diff (no error).
	// The command should complete and record a turn.
	// Note: a same-observer guard ("diff: observer-b must differ") is deferred to v2.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\ndiff ops-engineer\nquit\n")

	if !strings.Contains(out, "=== Mesh Diff (situated comparison) ===") {
		t.Errorf("expected diff header even for same-observer diff, got:\n%s", out)
	}
	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d, want 2 (cut + diff)", len(s.Turns()))
	}
}

// === gaps command ===

func TestRun_Gaps_NilStore(t *testing.T) {
	// nil store prints inline error; no turn beyond cut.
	s := explore.NewSession(nil, "alice", nil)
	out := run(t, s, "cut alice\ngaps bob\nquit\n")

	if !strings.Contains(out, "no trace substrate") {
		t.Errorf("expected 'no trace substrate' error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Gaps_NoObserver(t *testing.T) {
	// Without a prior cut, gaps prints an inline error.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "gaps bob\nquit\n")

	if !strings.Contains(out, "observer") {
		t.Errorf("expected 'observer' error, got:\n%s", out)
	}
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d, want 0", len(s.Turns()))
	}
}

func TestRun_Gaps_NoArg(t *testing.T) {
	// gaps without observer-b prints a usage error; no turn beyond cut.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "cut alice\ngaps\nquit\n")

	if !strings.Contains(out, "observer-b") {
		t.Errorf("expected 'observer-b' in error, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Gaps_HappyPath(t *testing.T) {
	// cut obs-a then gaps obs-b: output contains gap analysis; turn recorded.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee02", "relay B dropped packet", "security-team"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\ngaps security-team\nquit\n")

	// PrintObserverGap always emits this exact header.
	if !strings.Contains(out, "=== Observer Gap ===") {
		t.Errorf("expected gap header in output, got:\n%s", out)
	}
	if len(s.Turns()) != 2 {
		t.Fatalf("Turns() = %d, want 2 (cut + gaps)", len(s.Turns()))
	}
}

func TestRun_Gaps_RecordsTurn(t *testing.T) {
	// gaps records a turn with correct Command, Observer, and Reading type.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee02", "relay B dropped packet", "security-team"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	run(t, s, "cut ops-engineer\ngaps security-team\nquit\n")

	turns := s.Turns()
	if len(turns) != 2 {
		t.Fatalf("Turns() = %d, want 2", len(turns))
	}
	gt := turns[1]
	if gt.Command != "gaps security-team" {
		t.Errorf("Turn.Command = %q, want %q", gt.Command, "gaps security-team")
	}
	if gt.Observer != "ops-engineer" {
		t.Errorf("Turn.Observer = %q, want %q", gt.Observer, "ops-engineer")
	}
	if _, ok := gt.Reading.(graph.ObserverGap); !ok {
		t.Errorf("Turn.Reading type = %T, want graph.ObserverGap", gt.Reading)
	}
}

func TestRun_Gaps_QueryError(t *testing.T) {
	// Store query error prints inline error; no turn beyond cut.
	s := explore.NewSession(errStore{}, "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\ngaps security-team\nquit\n")

	if !strings.Contains(out, "failed to load traces") {
		t.Errorf("expected query error message, got:\n%s", out)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only the cut)", len(s.Turns()))
	}
}

func TestRun_Gaps_SameObserver(t *testing.T) {
	// gaps when observer A == observer B produces an empty-gap result (no error).
	// The command should complete and record a turn.
	// Note: a same-observer guard is deferred to v2.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\ngaps ops-engineer\nquit\n")

	if !strings.Contains(out, "=== Observer Gap ===") {
		t.Errorf("expected gap header even for same-observer gaps, got:\n%s", out)
	}
	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d, want 2 (cut + gaps)", len(s.Turns()))
	}
}

// === help — batch 2 commands ===

func TestRun_Help_ShowsBatch2Commands(t *testing.T) {
	// help output lists all batch-2 commands.
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "help\nquit\n")

	for _, keyword := range []string{"diff", "gaps", "follow", "bottleneck"} {
		if !strings.Contains(out, keyword) {
			t.Errorf("help output missing %q\nfull output:\n%s", keyword, out)
		}
	}
}

// === suggest command ===

// mockSuggestClient is a test double for explore.SuggestClient.
type mockSuggestClient struct {
	response string
	err      error
	// callCount records how many times Complete was called; used in guard tests
	// to confirm the LLM is never contacted when a guard fires.
	callCount  int
	lastSystem string
	lastPrompt string
}

func (m *mockSuggestClient) Complete(_ context.Context, system, prompt string) (string, error) {
	m.callCount++
	m.lastSystem = system
	m.lastPrompt = prompt
	return m.response, m.err
}

func TestRun_Suggest_NoClient(t *testing.T) {
	// suggest with nil client prints inline error; session continues.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", nil)
	out := run(t, s, "cut ops-engineer\nshadow\nsuggest\nquit\n")

	if !strings.Contains(out, "no LLM client") {
		t.Errorf("expected 'no LLM client' error, got:\n%s", out)
	}
	// shadow + cut recorded, but not suggest (guard fired)
	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d, want 2 (cut + shadow)", len(s.Turns()))
	}
}

func TestRun_Suggest_NoAnalyst(t *testing.T) {
	// suggest with empty analyst prints attribution error; LLM is not contacted.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{response: "Try following relay"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "", mc)
	out := run(t, s, "cut ops-engineer\nshadow\nsuggest\nquit\n")

	if !strings.Contains(out, "analyst not set") {
		t.Errorf("expected 'analyst not set' error, got:\n%s", out)
	}
	if mc.callCount != 0 {
		t.Errorf("LLM was contacted %d time(s), want 0 (guard should have fired)", mc.callCount)
	}
	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d, want 2 (cut + shadow only)", len(s.Turns()))
	}
}

func TestRun_Suggest_NilStore(t *testing.T) {
	// suggest with nil store prints inline error; session continues.
	mc := &mockSuggestClient{response: "Try following relay"}
	s := explore.NewSession(nil, "analyst-1", mc)
	out := run(t, s, "cut alice\nsuggest\nquit\n")

	if !strings.Contains(out, "no trace substrate") {
		t.Errorf("expected 'no trace substrate' error, got:\n%s", out)
	}
}

func TestRun_Suggest_NoObserver(t *testing.T) {
	// suggest without prior cut prints observer error; session continues.
	mc := &mockSuggestClient{response: "Try following relay"}
	s := explore.NewSession(testStore(t), "analyst-1", mc)
	out := run(t, s, "suggest\nquit\n")

	if !strings.Contains(out, "observer not set") {
		t.Errorf("expected 'observer not set' error, got:\n%s", out)
	}
	if len(s.Turns()) != 0 {
		t.Errorf("Turns() = %d, want 0", len(s.Turns()))
	}
}

func TestRun_Suggest_NoPriorReading(t *testing.T) {
	// suggest after cut but no shadow/bottleneck/gaps prints error; LLM not contacted.
	mc := &mockSuggestClient{response: "Try following relay"}
	s := explore.NewSession(testStore(t), "analyst-1", mc)
	out := run(t, s, "cut alice\nsuggest\nquit\n")

	if !strings.Contains(out, "no prior") {
		t.Errorf("expected 'no prior' error, got:\n%s", out)
	}
	if mc.callCount != 0 {
		t.Errorf("LLM was contacted %d time(s), want 0 (guard should have fired)", mc.callCount)
	}
	if len(s.Turns()) != 1 {
		t.Errorf("Turns() = %d, want 1 (only cut)", len(s.Turns()))
	}
}

func TestRun_Suggest_HappyPath(t *testing.T) {
	// Full happy path: cut → shadow → suggest. Output contains LLM response;
	// turn recorded with SuggestionMeta fully populated.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{response: "Follow element relay for translation chain"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	out := run(t, s, "cut ops-engineer\nshadow\nsuggest\nquit\n")

	// LLM response should appear in output.
	if !strings.Contains(out, "Follow element relay") {
		t.Errorf("expected LLM response in output, got:\n%s", out)
	}
	// Header should appear.
	if !strings.Contains(out, "=== Suggestion") {
		t.Errorf("expected suggestion header in output, got:\n%s", out)
	}

	turns := s.Turns()
	if len(turns) != 3 {
		t.Fatalf("Turns() = %d, want 3 (cut + shadow + suggest)", len(turns))
	}
	st := turns[2]
	if st.Command != "suggest" {
		t.Errorf("Turn.Command = %q, want %q", st.Command, "suggest")
	}
	// Reading carries the LLM response string.
	reading, ok := st.Reading.(string)
	if !ok {
		t.Fatalf("Turn.Reading type = %T, want string", st.Reading)
	}
	if reading != "Follow element relay for translation chain" {
		t.Errorf("Turn.Reading = %q, want LLM response", reading)
	}
	// SuggestionMeta must be populated.
	if st.Suggestion == nil {
		t.Fatal("Turn.Suggestion = nil, want non-nil SuggestionMeta")
	}
	if st.Suggestion.Analyst != "analyst-1" {
		t.Errorf("Suggestion.Analyst = %q, want %q", st.Suggestion.Analyst, "analyst-1")
	}
	if st.Suggestion.Basis != "shadow" {
		t.Errorf("Suggestion.Basis = %q, want %q", st.Suggestion.Basis, "shadow")
	}
	if st.Suggestion.TraceCount != 1 {
		t.Errorf("Suggestion.TraceCount = %d, want 1 (one trace loaded)", st.Suggestion.TraceCount)
	}
	if st.Suggestion.GeneratedAt.IsZero() {
		t.Error("Suggestion.GeneratedAt is zero")
	}
}

func TestRun_Suggest_ExplicitBasis_Shadow(t *testing.T) {
	// suggest shadow finds the shadow turn even when bottleneck is more recent.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{response: "Check shadow elements"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	run(t, s, "cut ops-engineer\nshadow\nbottleneck\nsuggest shadow\nquit\n")

	turns := s.Turns()
	if len(turns) != 4 {
		t.Fatalf("Turns() = %d, want 4", len(turns))
	}
	if turns[3].Suggestion == nil {
		t.Fatal("suggest turn has nil Suggestion")
	}
	if turns[3].Suggestion.Basis != "shadow" {
		t.Errorf("Basis = %q, want %q — should use shadow, not more-recent bottleneck", turns[3].Suggestion.Basis, "shadow")
	}
}

func TestRun_Suggest_ExplicitBasis_Bottleneck(t *testing.T) {
	// suggest bottleneck selects the bottleneck turn's reading.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{response: "Relay is provisionally central"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	run(t, s, "cut ops-engineer\nbottleneck\nsuggest bottleneck\nquit\n")

	turns := s.Turns()
	if len(turns) != 3 {
		t.Fatalf("Turns() = %d, want 3", len(turns))
	}
	if turns[2].Suggestion == nil || turns[2].Suggestion.Basis != "bottleneck" {
		t.Errorf("Basis = %v, want %q", turns[2].Suggestion, "bottleneck")
	}
}

func TestRun_Suggest_ExplicitBasis_Gaps(t *testing.T) {
	// suggest gaps selects the gaps turn's reading.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee02", "relay B dropped packet", "security-team"),
	}
	mc := &mockSuggestClient{response: "Investigate gap elements"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	run(t, s, "cut ops-engineer\ngaps security-team\nsuggest gaps\nquit\n")

	turns := s.Turns()
	if len(turns) != 3 {
		t.Fatalf("Turns() = %d, want 3", len(turns))
	}
	if turns[2].Suggestion == nil || turns[2].Suggestion.Basis != "gaps" {
		t.Errorf("Basis = %v, want %q", turns[2].Suggestion, "gaps")
	}
}

func TestRun_Suggest_AutoDetect_SkipsFollow(t *testing.T) {
	// Auto-detect skips follow turns (ClassifiedChain is not a valid basis).
	// With shadow then follow, suggest should pick shadow, not the more-recent follow.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{response: "Follow the shadow elements"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	run(t, s, "cut ops-engineer\nshadow\nfollow relay\nsuggest\nquit\n")

	turns := s.Turns()
	if len(turns) != 4 {
		t.Fatalf("Turns() = %d, want 4 (cut + shadow + follow + suggest)", len(turns))
	}
	if turns[3].Suggestion == nil {
		t.Fatal("suggest turn has nil Suggestion")
	}
	if turns[3].Suggestion.Basis != "shadow" {
		t.Errorf("Basis = %q, want %q — auto-detect should skip follow and find shadow", turns[3].Suggestion.Basis, "shadow")
	}
}

func TestRun_Suggest_InvalidBasis(t *testing.T) {
	// suggest with unknown basis prints error; no turn recorded.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{response: "ignored"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	out := run(t, s, "cut ops-engineer\nshadow\nsuggest articulate\nquit\n")

	if !strings.Contains(out, "unknown basis") {
		t.Errorf("expected 'unknown basis' error, got:\n%s", out)
	}
	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d, want 2 (cut + shadow)", len(s.Turns()))
	}
}

func TestRun_Suggest_LLMError(t *testing.T) {
	// LLM call failure prints inline error; no turn recorded for the failed suggest.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{err: fmt.Errorf("API rate limit exceeded")}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	out := run(t, s, "cut ops-engineer\nshadow\nsuggest\nquit\n")

	if !strings.Contains(out, "LLM call failed") {
		t.Errorf("expected 'LLM call failed' error, got:\n%s", out)
	}
	// shadow turn recorded; suggest turn not recorded (LLM failed)
	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d, want 2 (cut + shadow only)", len(s.Turns()))
	}
}

func TestRun_Suggest_AutoDetect_Gaps(t *testing.T) {
	// suggest with no basis arg auto-detects gaps as the most recent valid reading.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee02", "relay B dropped packet", "security-team"),
	}
	mc := &mockSuggestClient{response: "Investigate gap elements"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	run(t, s, "cut ops-engineer\nshadow\ngaps security-team\nsuggest\nquit\n")

	turns := s.Turns()
	if len(turns) != 4 {
		t.Fatalf("Turns() = %d, want 4", len(turns))
	}
	if turns[3].Suggestion == nil {
		t.Fatal("suggest turn has nil Suggestion")
	}
	// Auto-detect should pick gaps (most recent valid reading)
	if turns[3].Suggestion.Basis != "gaps" {
		t.Errorf("Basis = %q, want %q — auto-detect should pick most recent (gaps)", turns[3].Suggestion.Basis, "gaps")
	}
}

func TestRun_Suggest_SuggestionMeta_CutUsed(t *testing.T) {
	// SuggestionMeta.CutUsed carries the correct observer position.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{response: "Navigate the network"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	run(t, s, "cut ops-engineer\nshadow\nsuggest\nquit\n")

	turns := s.Turns()
	if len(turns) != 3 {
		t.Fatalf("Turns() = %d, want 3", len(turns))
	}
	meta := turns[2].Suggestion
	if meta == nil {
		t.Fatal("Suggestion is nil")
	}
	if meta.CutUsed.Observer != "ops-engineer" {
		t.Errorf("CutUsed.Observer = %q, want %q", meta.CutUsed.Observer, "ops-engineer")
	}
	if meta.CutUsed.Analyst != "analyst-1" {
		t.Errorf("CutUsed.Analyst = %q, want %q", meta.CutUsed.Analyst, "analyst-1")
	}
	if meta.TraceCount != 1 {
		t.Errorf("TraceCount = %d, want 1", meta.TraceCount)
	}
}

func TestRun_Suggest_NoPriorBasis_AfterArticulate(t *testing.T) {
	// articulate alone is not a valid basis for suggest.
	// suggest must refuse even after articulate — only shadow/bottleneck/gaps qualify.
	traces := []schema.Trace{
		newValidTrace("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee01", "relay A routed packet", "ops-engineer"),
	}
	mc := &mockSuggestClient{response: "Should not be called"}
	s := explore.NewSession(testStoreWithTraces(t, traces), "analyst-1", mc)
	out := run(t, s, "cut ops-engineer\narticulate\nsuggest\nquit\n")

	if !strings.Contains(out, "no prior") {
		t.Errorf("expected 'no prior' error after articulate-only, got:\n%s", out)
	}
	// articulate does not qualify as a basis
	if len(s.Turns()) != 2 {
		t.Errorf("Turns() = %d, want 2 (cut + articulate)", len(s.Turns()))
	}
}

// === help — batch 3 (suggest) ===

func TestRun_Help_ShowsSuggest(t *testing.T) {
	s := explore.NewSession(testStore(t), "analyst-1", nil)
	out := run(t, s, "help\nquit\n")

	if !strings.Contains(out, "suggest") {
		t.Errorf("help output missing 'suggest'\nfull output:\n%s", out)
	}
}
