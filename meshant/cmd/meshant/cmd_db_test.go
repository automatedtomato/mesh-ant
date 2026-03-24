package main

import (
	"bytes"
	"strings"
	"testing"
)

// cmd_db_test.go covers the --db flag error paths added to analytical commands
// in issue #144. These tests do not require a running Neo4j instance — they
// exercise flag validation and the "not built with Neo4j support" error that
// the !neo4j stub returns.

// ── articulate ────────────────────────────────────────────────────────────────

// TestCmdArticulate_DB_MutualExclusion: passing both --db and a file arg
// is an error. The data source must be unambiguous.
func TestCmdArticulate_DB_MutualExclusion(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdArticulate(&w, []string{
		"--observer", "obs",
		"--db", "bolt://localhost:7687",
		"traces.json",
	})
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusion: %v", err)
	}
}

// TestCmdArticulate_DB_NotBuilt: --db without neo4j build tag returns a clear
// error telling the user to rebuild.
func TestCmdArticulate_DB_NotBuilt(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdArticulate(&w, []string{
		"--observer", "obs",
		"--db", "bolt://localhost:7687",
	})
	if err == nil {
		t.Fatal("expected error for --db without neo4j build tag")
	}
	// The !neo4j stub returns "rebuild with -tags neo4j".
	if !strings.Contains(err.Error(), "neo4j") {
		t.Errorf("error should mention neo4j: %v", err)
	}
}

// ── diff ──────────────────────────────────────────────────────────────────────

func TestCmdDiff_DB_MutualExclusion(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdDiff(&w, []string{
		"--observer-a", "obs-a",
		"--observer-b", "obs-b",
		"--db", "bolt://localhost:7687",
		"traces.json",
	})
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusion: %v", err)
	}
}

func TestCmdDiff_DB_NotBuilt(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdDiff(&w, []string{
		"--observer-a", "obs-a",
		"--observer-b", "obs-b",
		"--db", "bolt://localhost:7687",
	})
	if err == nil {
		t.Fatal("expected error for --db without neo4j build tag")
	}
	if !strings.Contains(err.Error(), "neo4j") {
		t.Errorf("error should mention neo4j: %v", err)
	}
}

// ── shadow ────────────────────────────────────────────────────────────────────

func TestCmdShadow_DB_MutualExclusion(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdShadow(&w, []string{
		"--observer", "obs",
		"--db", "bolt://localhost:7687",
		"traces.json",
	})
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusion: %v", err)
	}
}

func TestCmdShadow_DB_NotBuilt(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdShadow(&w, []string{
		"--observer", "obs",
		"--db", "bolt://localhost:7687",
	})
	if err == nil {
		t.Fatal("expected error for --db without neo4j build tag")
	}
	if !strings.Contains(err.Error(), "neo4j") {
		t.Errorf("error should mention neo4j: %v", err)
	}
}

// ── gaps ──────────────────────────────────────────────────────────────────────

func TestCmdGaps_DB_MutualExclusion(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdGaps(&w, []string{
		"--observer-a", "obs-a",
		"--observer-b", "obs-b",
		"--db", "bolt://localhost:7687",
		"traces.json",
	})
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusion: %v", err)
	}
}

func TestCmdGaps_DB_NotBuilt(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdGaps(&w, []string{
		"--observer-a", "obs-a",
		"--observer-b", "obs-b",
		"--db", "bolt://localhost:7687",
	})
	if err == nil {
		t.Fatal("expected error for --db without neo4j build tag")
	}
	if !strings.Contains(err.Error(), "neo4j") {
		t.Errorf("error should mention neo4j: %v", err)
	}
}

// ── follow ────────────────────────────────────────────────────────────────────

func TestCmdFollow_DB_MutualExclusion(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdFollow(&w, []string{
		"--observer", "obs",
		"--element", "elem",
		"--db", "bolt://localhost:7687",
		"traces.json",
	})
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusion: %v", err)
	}
}

func TestCmdFollow_DB_NotBuilt(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdFollow(&w, []string{
		"--observer", "obs",
		"--element", "elem",
		"--db", "bolt://localhost:7687",
	})
	if err == nil {
		t.Fatal("expected error for --db without neo4j build tag")
	}
	if !strings.Contains(err.Error(), "neo4j") {
		t.Errorf("error should mention neo4j: %v", err)
	}
}

// ── bottleneck ────────────────────────────────────────────────────────────────

func TestCmdBottleneck_DB_MutualExclusion(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdBottleneck(&w, []string{
		"--db", "bolt://localhost:7687",
		"traces.json",
	})
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusion: %v", err)
	}
}

func TestCmdBottleneck_DB_NotBuilt(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdBottleneck(&w, []string{
		"--db", "bolt://localhost:7687",
	})
	if err == nil {
		t.Fatal("expected error for --db without neo4j build tag")
	}
	if !strings.Contains(err.Error(), "neo4j") {
		t.Errorf("error should mention neo4j: %v", err)
	}
}
