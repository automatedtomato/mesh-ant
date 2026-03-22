// cmd_promote_session_test.go tests the cmdPromoteSession subcommand (white-box, package main).
//
// Tests verify flag validation, output JSON correctness, and error propagation.
// No LLM client is involved — promote-session reads a session JSON file from disk.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// writeSessionFile serialises rec to a temp file and returns its path.
func writeSessionFile(t *testing.T, rec llm.SessionRecord) string {
	t.Helper()
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("writeSessionFile: marshal: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writeSessionFile: write: %v", err)
	}
	return path
}

// testSession returns a minimal valid SessionRecord for CLI tests.
func testSession() llm.SessionRecord {
	return llm.SessionRecord{
		ID:      "d4e5f6a7-b8c9-0123-defa-123456789012",
		Command: "extract",
		Conditions: llm.ExtractionConditions{
			ModelID:      "claude-sonnet-4-6",
			SourceDocRef: "data/notes.md",
			Timestamp:    time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC),
		},
		DraftCount: 3,
		Timestamp:  time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC),
	}
}

// TestCmdPromoteSession_success verifies that a valid session file and observer
// produces a JSON array containing one valid promoted trace.
func TestCmdPromoteSession_success(t *testing.T) {
	sessionPath := writeSessionFile(t, testSession())
	var out bytes.Buffer

	err := cmdPromoteSession(&out, []string{
		"--session-file", sessionPath,
		"--observer", "analyst-alice",
	})
	if err != nil {
		t.Fatalf("cmdPromoteSession() want no error, got: %v", err)
	}

	outStr := out.String()
	// stdout should mention the session ID
	if !strings.Contains(outStr, "d4e5f6a7-b8c9-0123-defa-123456789012") {
		t.Errorf("stdout: want session ID in output, got: %q", outStr)
	}
}

// TestCmdPromoteSession_outputFile verifies that --output writes a valid
// []schema.Trace JSON array to the specified file.
func TestCmdPromoteSession_outputFile(t *testing.T) {
	sessionPath := writeSessionFile(t, testSession())
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "traces.json")
	var out bytes.Buffer

	err := cmdPromoteSession(&out, []string{
		"--session-file", sessionPath,
		"--observer", "analyst-alice",
		"--output", outPath,
	})
	if err != nil {
		t.Fatalf("cmdPromoteSession() want no error, got: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}

	var traces []schema.Trace
	if err := json.Unmarshal(data, &traces); err != nil {
		t.Fatalf("output file: want JSON []Trace, parse error: %v", err)
	}
	if len(traces) != 1 {
		t.Errorf("output file: want 1 trace, got %d", len(traces))
	}
	if err := traces[0].Validate(); err != nil {
		t.Errorf("promoted trace fails Validate(): %v", err)
	}
	if traces[0].Observer != "analyst-alice" {
		t.Errorf("Observer: want %q, got %q", "analyst-alice", traces[0].Observer)
	}
}

// TestCmdPromoteSession_missingSessionFile verifies that a non-existent
// --session-file path returns an error.
func TestCmdPromoteSession_missingSessionFile(t *testing.T) {
	var out bytes.Buffer

	err := cmdPromoteSession(&out, []string{
		"--session-file", "/nonexistent/session.json",
		"--observer", "analyst-alice",
	})
	if err == nil {
		t.Fatal("cmdPromoteSession() with missing session file: want error, got nil")
	}
}

// TestCmdPromoteSession_missingObserver verifies that omitting --observer
// returns an error — no trace without an observer.
func TestCmdPromoteSession_missingObserver(t *testing.T) {
	sessionPath := writeSessionFile(t, testSession())
	var out bytes.Buffer

	err := cmdPromoteSession(&out, []string{
		"--session-file", sessionPath,
		// --observer intentionally omitted
	})
	if err == nil {
		t.Fatal("cmdPromoteSession() without --observer: want error, got nil")
	}
}

// TestCmdPromoteSession_missingSessionFileFlag verifies that omitting
// --session-file returns an error with usage information.
func TestCmdPromoteSession_missingSessionFileFlag(t *testing.T) {
	var out bytes.Buffer

	err := cmdPromoteSession(&out, []string{
		"--observer", "analyst-alice",
		// --session-file intentionally omitted
	})
	if err == nil {
		t.Fatal("cmdPromoteSession() without --session-file: want error, got nil")
	}
}

// TestCmdPromoteSession_badJSON verifies that a session file containing
// invalid JSON returns an error.
func TestCmdPromoteSession_badJSON(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write bad JSON: %v", err)
	}
	var out bytes.Buffer

	err := cmdPromoteSession(&out, []string{
		"--session-file", badPath,
		"--observer", "analyst-alice",
	})
	if err == nil {
		t.Fatal("cmdPromoteSession() with bad JSON: want error, got nil")
	}
}

// TestCmdPromoteSession_helpFlag verifies that --help returns flag.ErrHelp
// and makes no filesystem writes.
func TestCmdPromoteSession_helpFlag(t *testing.T) {
	var out bytes.Buffer

	err := cmdPromoteSession(&out, []string{"--help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("--help: want flag.ErrHelp, got %v", err)
	}
}

// TestCmdPromoteSession_outputContainsSessionTag verifies that the promoted
// trace output carries the "session" tag.
func TestCmdPromoteSession_outputContainsSessionTag(t *testing.T) {
	sessionPath := writeSessionFile(t, testSession())
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "traces.json")
	var out bytes.Buffer

	if err := cmdPromoteSession(&out, []string{
		"--session-file", sessionPath,
		"--observer", "analyst-alice",
		"--output", outPath,
	}); err != nil {
		t.Fatalf("cmdPromoteSession() want no error, got: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	var traces []schema.Trace
	if err := json.Unmarshal(data, &traces); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	found := false
	for _, tag := range traces[0].Tags {
		if tag == string(schema.TagValueSession) {
			found = true
		}
	}
	if !found {
		t.Errorf("Tags: want %q in %v", schema.TagValueSession, traces[0].Tags)
	}
}
