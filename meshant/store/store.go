// Package store defines the TraceStore interface — the narrow, swappable
// boundary between the Go analytical engine and trace storage backends.
//
// Two backends implement TraceStore at v1:
//   - JSONFileStore: wraps the existing loader.Load path (JSON file on disk)
//   - Neo4j adapter: connects to a Neo4j-compatible graph DB (Phase 3, issue #143)
//
// The analytical engine (graph.Articulate, graph.Diff, etc.) receives
// []schema.Trace from TraceStore.Query and does not know which backend
// supplied the data. Switching backends requires no changes to the engine.
//
// See docs/decisions/kg-scoping-v1.md §2 for the full design rationale.
package store

import (
	"context"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// TraceStore is the storage interface the Go analytical engine calls.
// It is deliberately narrow: the engine needs traces filtered by cut axes;
// it does not need to know how or where traces are stored.
//
// Every method accepts a context so callers can cancel long-running
// operations (relevant for the Neo4j backend; the JSON backend checks
// context before each operation).
type TraceStore interface {
	// Store persists traces. Idempotent on ID: storing a trace whose ID
	// already exists updates its properties in-place. All traces in the
	// slice must pass schema.Validate(); the call fails atomically if any
	// trace is invalid.
	Store(ctx context.Context, traces []schema.Trace) error

	// Query returns traces matching the given options. Each non-zero
	// field in QueryOpts adds an AND constraint. The returned slice is
	// the pre-filtered substrate the analytical engine cuts from.
	// An empty result is valid (no traces match the criteria).
	Query(ctx context.Context, opts QueryOpts) ([]schema.Trace, error)

	// Get retrieves a single trace by ID.
	// Returns (zero, false, nil) if the trace does not exist.
	// Returns (trace, true, nil) if found.
	Get(ctx context.Context, id string) (schema.Trace, bool, error)

	// Close releases backend resources. Safe to call multiple times.
	Close() error
}

// QueryOpts specifies the pre-filtering criteria for a Query call.
// Each non-zero field adds an AND constraint. The analytical engine applies
// cut logic (shadow assignment, graph construction) on the returned slice;
// the store handles only efficient pre-filtering, not cut semantics.
type QueryOpts struct {
	// Observer filters to traces whose Observer field matches exactly.
	// Empty string means no observer filter (all observers included).
	Observer string

	// Window filters to traces whose Timestamp falls within [Start, End].
	// Zero values on either bound mean no constraint on that side.
	//
	// NOTE: TimeWindow currently lives in the graph package. It will
	// eventually move to schema; see kg-scoping-v1.md §2.1 standing tension.
	Window graph.TimeWindow

	// Tags filters to traces that carry ALL of the listed tags (AND semantics).
	// This is distinct from graph.Articulate's tag filter, which uses OR.
	// Empty slice means no tag filter.
	Tags []string

	// Limit caps the number of returned traces. 0 means no limit.
	Limit int
}
