//go:build neo4j

// neo4j_store.go implements TraceStore against a Neo4j graph database using
// the official neo4j-go-driver/v5.
//
// Build tag: this file is compiled only with -tags neo4j. The default binary
// does not link against the Neo4j driver. To use this adapter:
//
//	go build -tags neo4j ./...
//	go test -tags neo4j ./store/
//
// Note: go mod tidy must also be run with -tags neo4j to retain the driver
// dependency in go.mod:
//
//	go mod tidy -tags neo4j
//
// See docs/decisions/neo4j-adapter-v1.md for the full design rationale.
package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jConfig holds the connection parameters for a Neo4j database.
//
// BoltURL must use one of: bolt://, bolt+s://, neo4j://, neo4j+s://.
// Database is the target database name; empty string uses the driver default
// (typically "neo4j").
//
// Credentials must not be hardcoded — supply them via environment variables
// or a secret manager. The convention used by meshant store is:
//
//	MESHANT_DB_URL      — the Bolt URL
//	MESHANT_DB_USER     — username
//	MESHANT_DB_PASS     — password
type Neo4jConfig struct {
	BoltURL  string // e.g. "bolt://localhost:7687"
	Username string
	Password string
	Database string // empty = driver default ("neo4j")
}

// Neo4jStore implements TraceStore against a Neo4j graph database. Construct
// with NewNeo4jStore; close with Close when done.
//
// Neo4jStore is safe to use from multiple goroutines — the underlying driver
// maintains a connection pool. Store, Query, and Get each open and close their
// own sessions.
type Neo4jStore struct {
	driver    neo4j.DriverWithContext
	database  string
	closeOnce sync.Once
}

// NewNeo4jStore creates a Neo4jStore backed by the given config, verifies
// that the server is reachable, and returns the store. Returns an error if
// BoltURL is empty, the driver cannot be created, or connectivity fails
// (unreachable host, bad credentials, etc.).
//
// Callers should defer s.Close() immediately after a successful return.
func NewNeo4jStore(ctx context.Context, cfg Neo4jConfig) (*Neo4jStore, error) {
	if cfg.BoltURL == "" {
		return nil, fmt.Errorf("store: NewNeo4jStore: BoltURL is required")
	}

	driver, err := neo4j.NewDriverWithContext(
		cfg.BoltURL,
		neo4j.BasicAuth(cfg.Username, cfg.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("store: NewNeo4jStore: create driver: %w", err)
	}

	if err := driver.VerifyConnectivity(ctx); err != nil {
		// Close the driver before returning to avoid a resource leak.
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("store: NewNeo4jStore: verify connectivity: %w", err)
	}

	return &Neo4jStore{
		driver:   driver,
		database: cfg.Database,
	}, nil
}

// Store persists traces to Neo4j. It is idempotent on ID: storing a trace
// whose ID already exists updates its properties in-place (MERGE semantics).
//
// All traces are validated before any write. If any trace is invalid the call
// fails without modifying the database. All writes occur in a single
// transaction; on failure the transaction is rolled back and no traces from
// this call are persisted.
func (s *Neo4jStore) Store(ctx context.Context, traces []schema.Trace) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("store: Store: %w", err)
	}

	// Validate all traces before touching the database.
	for i, t := range traces {
		if err := t.Validate(); err != nil {
			return fmt.Errorf("store: Store: trace %d (id=%q): %w", i, t.ID, err)
		}
	}
	if len(traces) == 0 {
		return nil
	}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, t := range traces {
			cypher, params := storeCypher(t)
			result, err := tx.Run(ctx, cypher, params)
			if err != nil {
				return nil, fmt.Errorf("store trace %q: %w", t.ID, err)
			}
			// Consume the result to detect server-side errors eagerly.
			if _, err := result.Consume(ctx); err != nil {
				return nil, fmt.Errorf("consume store result for trace %q: %w", t.ID, err)
			}
		}
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("store: Store: %w", err)
	}
	return nil
}

// Query returns traces matching the given options. Each non-zero field in opts
// adds an AND constraint. The returned slice is ordered by timestamp ascending.
// An empty result is valid — no traces matched the criteria.
//
// ANT tensions (T1, T2): the observer filter partially commits a cut before
// the analytical engine sees the data; Limit truncates the substrate without
// the engine knowing what was excluded. Both are documented in
// docs/decisions/neo4j-adapter-v1.md.
func (s *Neo4jStore) Query(ctx context.Context, opts QueryOpts) ([]schema.Trace, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("store: Query: %w", err)
	}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	cypher, params := queryCypher(opts)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		records, err := res.Collect(ctx)
		if err != nil {
			return nil, err
		}
		traces := make([]schema.Trace, 0, len(records))
		for _, rec := range records {
			t, err := traceFromRecord(rec)
			if err != nil {
				return nil, err
			}
			traces = append(traces, t)
		}
		return traces, nil
	})
	if err != nil {
		return nil, fmt.Errorf("store: Query: %w", err)
	}
	if result == nil {
		return nil, nil
	}
	return result.([]schema.Trace), nil
}

// Get retrieves a single trace by ID. Returns (zero, false, nil) if the trace
// does not exist; (trace, true, nil) if found.
func (s *Neo4jStore) Get(ctx context.Context, id string) (schema.Trace, bool, error) {
	if err := ctx.Err(); err != nil {
		return schema.Trace{}, false, fmt.Errorf("store: Get: %w", err)
	}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	cypher, params := getCypher(id)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		records, err := res.Collect(ctx)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			return nil, nil // trace not found
		}
		t, err := traceFromRecord(records[0])
		if err != nil {
			return nil, err
		}
		return &t, nil
	})
	if err != nil {
		return schema.Trace{}, false, fmt.Errorf("store: Get: %w", err)
	}
	if result == nil {
		return schema.Trace{}, false, nil
	}
	return *result.(*schema.Trace), true, nil
}

// Close releases the Neo4j driver and its connection pool. Safe to call
// multiple times — only the first call does work.
func (s *Neo4jStore) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		closeErr = s.driver.Close(context.Background())
	})
	return closeErr
}

// traceFromRecord extracts a schema.Trace from a Neo4j query record. The
// record must have columns "t" (:Trace node), "sources" (collect result),
// and "targets" (collect result).
func traceFromRecord(rec *neo4j.Record) (schema.Trace, error) {
	nodeAny, ok := rec.Get("t")
	if !ok {
		return schema.Trace{}, fmt.Errorf("traceFromRecord: missing column 't'")
	}
	node, ok := nodeAny.(neo4j.Node)
	if !ok {
		return schema.Trace{}, fmt.Errorf("traceFromRecord: 't' is not a Node (got %T)", nodeAny)
	}

	var rawSources, rawTargets []any
	if sv, ok := rec.Get("sources"); ok {
		if s, ok := sv.([]any); ok {
			rawSources = s
		}
	}
	if tv, ok := rec.Get("targets"); ok {
		if t, ok := tv.([]any); ok {
			rawTargets = t
		}
	}

	return recordToTrace(node.Props, rawSources, rawTargets)
}
