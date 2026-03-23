package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCmdServe_NoSource_Error: no --db and no positional arg → error.
func TestCmdServe_NoSource_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdServe(&w, []string{})
	if err == nil {
		t.Fatal("expected error when no source given")
	}
	if !strings.Contains(err.Error(), "traces.json") {
		t.Errorf("error should mention traces.json: %v", err)
	}
}

// TestCmdServe_MutualExclusion: --db and <file> are mutually exclusive.
func TestCmdServe_MutualExclusion(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdServe(&w, []string{"--db", "bolt://localhost:7687", "/some/file.json"})
	if err == nil {
		t.Fatal("expected error when --db and file arg both given")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutually exclusive: %v", err)
	}
}

// TestCmdServe_InvalidPort: --port must be parseable integer.
func TestCmdServe_InvalidPort(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdServe(&w, []string{"--port", "notaport", "/some/file.json"})
	if err == nil {
		t.Fatal("expected error for invalid --port")
	}
}

// TestCmdServe_MissingDB_Error: no --db, no file — clear message mentioning --db.
func TestCmdServe_MissingDB_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdServe(&w, []string{})
	if err == nil {
		t.Fatal("expected error")
	}
	// Error should point toward the --db flag or traces.json.
	if !strings.Contains(err.Error(), "--db") && !strings.Contains(err.Error(), "traces.json") {
		t.Errorf("error should mention --db or traces.json: %v", err)
	}
}
