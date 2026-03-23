package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// storeValidTrace returns a minimal valid schema.Trace for cmdStore tests.
// IDs must be valid lowercase-hyphenated UUIDs; the two hard-coded values
// below satisfy that constraint.
func storeValidTrace(id string) schema.Trace {
	return schema.Trace{
		ID:          id,
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		WhatChanged: "test change",
		Observer:    "test-observer",
		Source:      []string{"element-a"},
		Target:      []string{"element-b"},
	}
}

const (
	storeTraceID1  = "a1000000-0000-0000-0000-000000000001"
	storeTraceID2  = "a1000000-0000-0000-0000-000000000002"
	storeTraceIDem = "a1000000-0000-0000-0000-000000000003"
	storeTraceIDb  = "a1000000-0000-0000-0000-000000000004"
)

// writeTracesJSON writes traces to a temporary JSON file and returns its path.
func writeTracesJSON(t *testing.T, traces []schema.Trace) string {
	t.Helper()
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "traces-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if err := enc.Encode(traces); err != nil {
		t.Fatalf("encode traces: %v", err)
	}
	return f.Name()
}

// storeOutputPath returns a path for a JSONFileStore output within TempDir.
func storeOutputPath(t *testing.T) string {
	t.Helper()
	return t.TempDir() + "/out.json"
}

// TestCmdStore_MissingDB_Error: no --db flag and MESHANT_DB_URL unset → error.
func TestCmdStore_MissingDB_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	path := writeTracesJSON(t, []schema.Trace{storeValidTrace(storeTraceID1)})
	var w bytes.Buffer
	err := cmdStore(&w, nil, []string{path})
	if err == nil {
		t.Fatal("expected error when --db and env var both absent")
	}
	if !strings.Contains(err.Error(), "--db") {
		t.Errorf("error should mention --db: %v", err)
	}
}

// TestCmdStore_MissingFile_Error: --db set, no positional arg → error.
func TestCmdStore_MissingFile_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	var w bytes.Buffer
	err := cmdStore(&w, nil, []string{"--db", "bolt://localhost:7687"})
	if err == nil {
		t.Fatal("expected error when positional arg absent")
	}
	if !strings.Contains(err.Error(), "traces.json") {
		t.Errorf("error should mention traces.json: %v", err)
	}
}

// TestCmdStore_HappyPath_InjectedStore: inject a JSONFileStore, verify count
// is printed and traces are persisted idempotently.
func TestCmdStore_HappyPath_InjectedStore(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	traces := []schema.Trace{
		storeValidTrace(storeTraceID1),
		storeValidTrace(storeTraceID2),
	}
	inputPath := writeTracesJSON(t, traces)
	ts := store.NewJSONFileStore(storeOutputPath(t))

	var w bytes.Buffer
	if err := cmdStore(&w, ts, []string{inputPath}); err != nil {
		t.Fatalf("cmdStore: %v", err)
	}

	got := w.String()
	if !strings.Contains(got, "stored 2 trace(s)") {
		t.Errorf("output should contain %q: %q", "stored 2 trace(s)", got)
	}

	// Verify persistence: query back all traces.
	stored, err := ts.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(stored) != 2 {
		t.Errorf("expected 2 stored traces, got %d", len(stored))
	}
}

// TestCmdStore_Idempotent: storing the same file twice produces no duplicates.
func TestCmdStore_Idempotent(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	traces := []schema.Trace{storeValidTrace(storeTraceIDem)}
	inputPath := writeTracesJSON(t, traces)
	ts := store.NewJSONFileStore(storeOutputPath(t))

	// First store.
	if err := cmdStore(&bytes.Buffer{}, ts, []string{inputPath}); err != nil {
		t.Fatalf("first store: %v", err)
	}
	// Second store of same data.
	if err := cmdStore(&bytes.Buffer{}, ts, []string{inputPath}); err != nil {
		t.Fatalf("second store: %v", err)
	}

	stored, err := ts.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(stored) != 1 {
		t.Errorf("expected 1 trace (idempotent), got %d", len(stored))
	}
}

// TestCmdStore_EmptyFile_NoError: empty JSON array → 0 stored, no error.
func TestCmdStore_EmptyFile_NoError(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	inputPath := writeTracesJSON(t, []schema.Trace{})
	ts := store.NewJSONFileStore(storeOutputPath(t))

	var w bytes.Buffer
	if err := cmdStore(&w, ts, []string{inputPath}); err != nil {
		t.Fatalf("cmdStore: %v", err)
	}
	got := w.String()
	if !strings.Contains(got, "stored 0 trace(s)") {
		t.Errorf("output should contain %q: %q", "stored 0 trace(s)", got)
	}
}

// TestCmdStore_InvalidTrace_Error: a trace failing Validate() → error, store
// not written. Validation occurs inside TraceStore.Store; this test verifies
// the error surfaces correctly through cmdStore.
func TestCmdStore_InvalidTrace_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	// A trace missing the required WhatChanged field.
	bad := schema.Trace{
		ID:        storeTraceIDb,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Observer:  "obs",
		// WhatChanged intentionally absent — fails Validate().
	}
	inputPath := writeTracesJSON(t, []schema.Trace{bad})
	ts := store.NewJSONFileStore(storeOutputPath(t))

	err := cmdStore(&bytes.Buffer{}, ts, []string{inputPath})
	if err == nil {
		t.Fatal("expected error for invalid trace")
	}
	// Confirm the validation error is surfaced (not some unrelated I/O error).
	if !strings.Contains(err.Error(), "store:") {
		t.Errorf("error should be wrapped under store: prefix: %v", err)
	}
	if !strings.Contains(err.Error(), storeTraceIDb) {
		t.Errorf("error should name the failing trace ID %q: %v", storeTraceIDb, err)
	}
}

// TestCmdStore_FileNotFound_Error: loader.Load failure for a nonexistent path
// surfaces as a store-prefixed error.
func TestCmdStore_FileNotFound_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	ts := store.NewJSONFileStore(storeOutputPath(t))
	nonexistent := t.TempDir() + "/does-not-exist.json"

	err := cmdStore(&bytes.Buffer{}, ts, []string{nonexistent})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "store:") {
		t.Errorf("error should be wrapped under store: prefix: %v", err)
	}
}

// TestCmdStore_MalformedJSON_Error: malformed JSON input surfaces as a
// store-prefixed error. Covers the loader.Load error path in cmd_store.go.
func TestCmdStore_MalformedJSON_Error(t *testing.T) {
	t.Setenv("MESHANT_DB_URL", "")
	dir := t.TempDir()
	path := dir + "/bad.json"
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write bad json: %v", err)
	}
	ts := store.NewJSONFileStore(storeOutputPath(t))

	err := cmdStore(&bytes.Buffer{}, ts, []string{path})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "store:") {
		t.Errorf("error should be wrapped under store: prefix: %v", err)
	}
}
