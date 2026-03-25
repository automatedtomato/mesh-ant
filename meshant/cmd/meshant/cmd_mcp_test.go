package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCmdMcp_MissingAnalyst_Error verifies that cmdMcp refuses to start when
// --analyst is not provided. Per D2 in mcp-v1.md the error message must:
//   - contain "analyst is required"
//   - contain "hiding the cut"
//
// This is not a soft warning — observer-position discipline is enforced at the
// schema level. The CLI refuses to start without a named analyst.
func TestCmdMcp_MissingAnalyst_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdMcp(&w, []string{"/some/traces.json"})
	if err == nil {
		t.Fatal("expected error when --analyst is not provided")
	}
	msg := err.Error()
	if !strings.Contains(msg, "analyst is required") {
		t.Errorf("error should contain 'analyst is required': %v", msg)
	}
	if !strings.Contains(msg, "hiding the cut") {
		t.Errorf("error should contain 'hiding the cut': %v", msg)
	}
	// D2 (mcp-v1.md): the error must name the epistemic obligation, not just
	// refuse. "analyst is required" and "hiding the cut" are the contractual
	// phrases from the spec; the longer explanation is implementation detail
	// and not asserted here to avoid coupling to exact phrasing.
}

// TestCmdMcp_NoSource_Error verifies that cmdMcp returns an error when neither
// --db nor a positional traces file is provided.
func TestCmdMcp_NoSource_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdMcp(&w, []string{"--analyst", "alice"})
	if err == nil {
		t.Fatal("expected error when no source given")
	}
	msg := err.Error()
	if !strings.Contains(msg, "traces.json") && !strings.Contains(msg, "--db") {
		t.Errorf("error should mention traces.json or --db: %v", msg)
	}
}

// TestCmdMcp_DBAndFile_MutuallyExclusive verifies that --db and a positional
// traces file cannot be provided simultaneously.
func TestCmdMcp_DBAndFile_MutuallyExclusive(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdMcp(&w, []string{"--analyst", "alice", "--db", "bolt://localhost:7687", "/some/traces.json"})
	if err == nil {
		t.Fatal("expected error when --db and file arg both given")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutually exclusive: %v", err)
	}
}
