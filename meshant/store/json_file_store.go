package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// JSONFileStore is a TraceStore backed by a JSON file on disk.
//
// It wraps the existing loader.Load logic and adds Store/Get operations.
// All traces are held in a single JSON array at the given path.
//
// JSONFileStore is appropriate for small-to-medium trace datasets during
// local development and before a graph DB backend is available. It is not
// designed for concurrent writes — callers must coordinate externally if
// multiple goroutines or processes write to the same file.
//
// The JSON file backend implements the same TraceStore interface as the
// Neo4j adapter (#143), so switching backends requires no changes to the
// analytical engine.
type JSONFileStore struct {
	path string
}

// NewJSONFileStore creates a JSONFileStore backed by the JSON file at path.
// The file need not exist yet; Store will create it on first write.
// The path is fixed for the lifetime of the store.
func NewJSONFileStore(path string) *JSONFileStore {
	return &JSONFileStore{path: path}
}

// Store persists traces to the JSON file. It is idempotent on ID: storing
// a trace whose ID already exists updates its properties in-place.
//
// All traces in the slice are validated before any write. If any trace is
// invalid the call fails without modifying the file.
//
// Writes are atomic: the new file is written to a temp file in the same
// directory, then renamed over the target. This prevents partial writes
// from corrupting the dataset.
func (s *JSONFileStore) Store(ctx context.Context, traces []schema.Trace) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("store: Store: %w", err)
	}

	// Validate all incoming traces before touching the file.
	for i, t := range traces {
		if err := t.Validate(); err != nil {
			return fmt.Errorf("store: Store: trace %d (id=%q): %w", i, t.ID, err)
		}
	}

	// Short-circuit for empty slice: no file modification needed.
	if len(traces) == 0 {
		return nil
	}

	// Load existing traces. Missing file is treated as an empty dataset.
	existing, err := s.loadOrEmpty()
	if err != nil {
		return fmt.Errorf("store: Store: load existing: %w", err)
	}

	// Build an ID-keyed map from existing traces, then upsert incoming ones.
	byID := make(map[string]schema.Trace, len(existing))
	for _, t := range existing {
		byID[t.ID] = t
	}
	for _, t := range traces {
		byID[t.ID] = t
	}

	// Collect and sort by timestamp for deterministic, readable output.
	merged := make([]schema.Trace, 0, len(byID))
	for _, t := range byID {
		merged = append(merged, t)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp.Before(merged[j].Timestamp)
	})

	return s.writeAtomic(merged)
}

// Query returns traces matching the given options. Each non-zero field in
// opts adds an AND constraint. The analytical engine applies cut logic
// (shadow assignment, graph construction) on the returned slice.
//
// A missing or empty file returns an empty slice without error.
func (s *JSONFileStore) Query(ctx context.Context, opts QueryOpts) ([]schema.Trace, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("store: Query: %w", err)
	}

	all, err := s.loadOrEmpty()
	if err != nil {
		return nil, fmt.Errorf("store: Query: %w", err)
	}

	result := make([]schema.Trace, 0, len(all))
	for _, t := range all {
		if !matchesOpts(t, opts) {
			continue
		}
		result = append(result, t)
	}

	// Apply limit after all other filters.
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result, nil
}

// Get retrieves a single trace by ID. Returns (zero, false, nil) if the
// trace does not exist or the file is missing. Returns (trace, true, nil)
// if found.
func (s *JSONFileStore) Get(ctx context.Context, id string) (schema.Trace, bool, error) {
	if err := ctx.Err(); err != nil {
		return schema.Trace{}, false, fmt.Errorf("store: Get: %w", err)
	}

	all, err := s.loadOrEmpty()
	if err != nil {
		return schema.Trace{}, false, fmt.Errorf("store: Get: %w", err)
	}

	for _, t := range all {
		if t.ID == id {
			return t, true, nil
		}
	}
	return schema.Trace{}, false, nil
}

// Close is a no-op for the JSON file store — there are no persistent
// resources to release. Safe to call multiple times.
func (s *JSONFileStore) Close() error {
	return nil
}

// --- private helpers ---

// loadOrEmpty calls loader.Load but converts a "file not found" error into
// an empty slice. All other errors (malformed JSON, validation failure) are
// returned as-is.
//
// Coupling note: the os.ErrNotExist check relies on loader.Load wrapping the
// os.Open error with %w (see loader/loader.go). If loader.Load ever changes
// to use fmt.Errorf without %w at that layer, this check will silently stop
// working and a missing file will be returned as an error instead of an empty
// slice. The test TestQuery_FileDoesNotExist_ReturnsEmptySlice guards this.
func (s *JSONFileStore) loadOrEmpty() ([]schema.Trace, error) {
	traces, err := loader.Load(s.path)
	if err == nil {
		return traces, nil
	}
	// Treat missing file as empty dataset (not an error condition for a store).
	if errors.Is(err, os.ErrNotExist) {
		return []schema.Trace{}, nil
	}
	return nil, err
}

// matchesOpts reports whether trace t passes all non-zero filters in opts.
// All constraints are AND: a trace must satisfy every specified criterion.
func matchesOpts(t schema.Trace, opts QueryOpts) bool {
	// Observer: exact match when non-empty.
	if opts.Observer != "" && t.Observer != opts.Observer {
		return false
	}

	// TimeWindow: inclusive bounds, zero bound means unbounded.
	if !opts.Window.Start.IsZero() && t.Timestamp.Before(opts.Window.Start) {
		return false
	}
	if !opts.Window.End.IsZero() && t.Timestamp.After(opts.Window.End) {
		return false
	}

	// Tags: ALL required tags must be present (AND semantics).
	// This differs from graph.Articulate's tag filter, which uses OR.
	if len(opts.Tags) > 0 {
		tagSet := make(map[string]bool, len(t.Tags))
		for _, tag := range t.Tags {
			tagSet[tag] = true
		}
		for _, required := range opts.Tags {
			if !tagSet[required] {
				return false
			}
		}
	}

	return true
}

// writeAtomic serialises traces as an indented JSON array and writes it
// atomically: a temp file in the same directory is written first, then
// renamed over the target path to avoid partial-write corruption.
//
// The Write, Sync, and Close error branches inside this function are not
// covered by tests — they require OS-level fault injection (e.g., writing
// to a full filesystem or a kernel-simulated write failure). These are
// known, deliberate gaps, not untested-by-accident code. The Rename error
// branch can be exercised by pointing the store at a read-only directory.
func (s *JSONFileStore) writeAtomic(traces []schema.Trace) error {
	data, err := json.MarshalIndent(traces, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "meshant-store-*.json.tmp")
	if err != nil {
		return fmt.Errorf("store: create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Write data; fsync before close to ensure the data reaches stable storage
	// before the rename. Without fsync, close(2) does not guarantee dirty pages
	// are flushed — the rename could succeed while the data remains in the page
	// cache, corrupting the file on a crash.
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("store: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("store: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("store: close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("store: rename temp to %q: %w", s.path, err)
	}

	return nil
}
