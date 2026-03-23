//go:build neo4j

// neo4j_store_test.go contains integration tests for Neo4jStore. All tests
// require a live Neo4j instance. Set MESHANT_NEO4J_TEST_URL to enable them:
//
//	MESHANT_NEO4J_TEST_URL=bolt://localhost:7687 go test -tags neo4j ./store/
//
// The database is cleared before and after each test. Do not run these tests
// against a production database.
package store_test

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	neo4jdriver "github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// --- test infrastructure ---

// neo4jTestEnv reads Neo4j connection parameters from environment variables.
// Skips the test if MESHANT_NEO4J_TEST_URL is not set.
func neo4jTestEnv(t *testing.T) (url, user, pass string) {
	t.Helper()
	url = os.Getenv("MESHANT_NEO4J_TEST_URL")
	if url == "" {
		t.Skip("MESHANT_NEO4J_TEST_URL not set — skipping Neo4j integration test")
	}
	user = os.Getenv("MESHANT_NEO4J_TEST_USER")
	if user == "" {
		user = "neo4j"
	}
	pass = os.Getenv("MESHANT_NEO4J_TEST_PASS")
	if pass == "" {
		pass = "neo4j"
	}
	return
}

// neo4jTestStore creates a Neo4jStore, clears the database before and after
// the test, and registers Close in t.Cleanup.
func neo4jTestStore(t *testing.T) *store.Neo4jStore {
	t.Helper()
	url, user, pass := neo4jTestEnv(t)
	ctx := context.Background()

	s, err := store.NewNeo4jStore(ctx, store.Neo4jConfig{
		BoltURL:  url,
		Username: user,
		Password: pass,
	})
	if err != nil {
		t.Fatalf("neo4jTestStore: NewNeo4jStore: %v", err)
	}

	neo4jClearDB(t, url, user, pass) // pre-test cleanup for leftover state
	t.Cleanup(func() {
		neo4jClearDB(t, url, user, pass)
		if err := s.Close(); err != nil {
			t.Logf("cleanup: Close: %v", err)
		}
	})
	return s
}

// neo4jClearDB deletes all nodes and relationships using a dedicated driver
// connection. It does not use the store under test so it cannot mask failures
// in Close or the session lifecycle.
func neo4jClearDB(t *testing.T, url, user, pass string) {
	t.Helper()
	ctx := context.Background()
	driver, err := neo4jdriver.NewDriverWithContext(url, neo4jdriver.BasicAuth(user, pass, ""))
	if err != nil {
		t.Fatalf("neo4jClearDB: create driver: %v", err)
	}
	defer driver.Close(ctx)

	sess := driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer sess.Close(ctx)

	if _, err := sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
		return nil, err
	}); err != nil {
		t.Fatalf("neo4jClearDB: delete all: %v", err)
	}
}

// sortedStringSlicesEqual reports whether two string slices contain the same
// elements, regardless of order. Used for Source, Target, and Tags comparisons
// because Neo4j's collect(DISTINCT) does not guarantee return order.
func sortedStringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := make([]string, len(a))
	bc := make([]string, len(b))
	copy(ac, a)
	copy(bc, b)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

// --- Interface compliance ---

// TestNeo4jStore_ImplementsTraceStore is a compile-time check that
// *Neo4jStore satisfies the TraceStore interface.
func TestNeo4jStore_ImplementsTraceStore(t *testing.T) {
	url, user, pass := neo4jTestEnv(t)
	ctx := context.Background()
	s, err := store.NewNeo4jStore(ctx, store.Neo4jConfig{
		BoltURL:  url,
		Username: user,
		Password: pass,
	})
	if err != nil {
		t.Fatalf("NewNeo4jStore: %v", err)
	}
	defer s.Close()
	var _ store.TraceStore = s
}

// --- Constructor ---

func TestNeo4jStore_EmptyBoltURL_ReturnsError(t *testing.T) {
	neo4jTestEnv(t) // skip if no URL configured (env check only)
	_, err := store.NewNeo4jStore(context.Background(), store.Neo4jConfig{})
	if err == nil {
		t.Fatal("NewNeo4jStore with empty BoltURL: want error, got nil")
	}
}

func TestNeo4jStore_BadURL_ReturnsError(t *testing.T) {
	neo4jTestEnv(t)
	_, err := store.NewNeo4jStore(context.Background(), store.Neo4jConfig{
		BoltURL:  "bolt://localhost:9999", // unreachable port
		Username: "neo4j",
		Password: "neo4j",
	})
	if err == nil {
		t.Fatal("NewNeo4jStore with unreachable host: want connectivity error, got nil")
	}
}

// --- Query: basic ---

func TestNeo4jQuery_EmptyOpts_ReturnsAllTraces(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
		validTrace("00000000-0000-0000-0000-000000000003", "change c"),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() want no error, got %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 traces, got %d", len(got))
	}
}

func TestNeo4jQuery_EmptyDB_ReturnsEmptySlice(t *testing.T) {
	s := neo4jTestStore(t)
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() on empty DB: want no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty slice, got %d traces", len(got))
	}
}

// --- Query: Observer filter ---

func TestNeo4jQuery_Observer_MatchesExact(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTraceWithObserver("00000000-0000-0000-0000-000000000001", "change a", "observer/alpha"),
		validTraceWithObserver("00000000-0000-0000-0000-000000000002", "change b", "observer/alpha"),
		validTraceWithObserver("00000000-0000-0000-0000-000000000003", "change c", "observer/beta"),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Observer: "observer/alpha"})
	if err != nil {
		t.Fatalf("Query() observer filter: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 traces for observer/alpha, got %d", len(got))
	}
}

func TestNeo4jQuery_Observer_NoMatch_ReturnsEmpty(t *testing.T) {
	s := neo4jTestStore(t)
	if err := s.Store(context.Background(), []schema.Trace{
		validTraceWithObserver("00000000-0000-0000-0000-000000000001", "change a", "observer/alpha"),
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Observer: "observer/nobody"})
	if err != nil {
		t.Fatalf("Query() observer no match: want no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty, got %d", len(got))
	}
}

func TestNeo4jQuery_Observer_EmptyString_NoFilter(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTraceWithObserver("00000000-0000-0000-0000-000000000001", "change a", "observer/alpha"),
		validTraceWithObserver("00000000-0000-0000-0000-000000000002", "change b", "observer/beta"),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Observer: ""})
	if err != nil {
		t.Fatalf("Query() empty observer: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("empty observer should return all 2 traces, got %d", len(got))
	}
}

// --- Query: TimeWindow filter ---

func TestNeo4jQuery_Window_StartOnly_ExcludesEarlier(t *testing.T) {
	s := neo4jTestStore(t)
	day1, day2, day3 := baseTime, baseTime.Add(24*time.Hour), baseTime.Add(48*time.Hour)
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", day1),
		validTraceAt("00000000-0000-0000-0000-000000000002", "change b", day2),
		validTraceAt("00000000-0000-0000-0000-000000000003", "change c", day3),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{Start: day2},
	})
	if err != nil {
		t.Fatalf("Query() window Start: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 traces (day2, day3), got %d", len(got))
	}
}

func TestNeo4jQuery_Window_EndOnly_ExcludesLater(t *testing.T) {
	s := neo4jTestStore(t)
	day1, day2, day3 := baseTime, baseTime.Add(24*time.Hour), baseTime.Add(48*time.Hour)
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", day1),
		validTraceAt("00000000-0000-0000-0000-000000000002", "change b", day2),
		validTraceAt("00000000-0000-0000-0000-000000000003", "change c", day3),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{End: day2},
	})
	if err != nil {
		t.Fatalf("Query() window End: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 traces (day1, day2), got %d", len(got))
	}
}

func TestNeo4jQuery_Window_BothBounds_IncludesInRange(t *testing.T) {
	s := neo4jTestStore(t)
	day1, day2, day3 := baseTime, baseTime.Add(24*time.Hour), baseTime.Add(48*time.Hour)
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", day1),
		validTraceAt("00000000-0000-0000-0000-000000000002", "change b", day2),
		validTraceAt("00000000-0000-0000-0000-000000000003", "change c", day3),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{Start: day2, End: day2},
	})
	if err != nil {
		t.Fatalf("Query() window both bounds: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace (day2 only), got %d", len(got))
	}
}

func TestNeo4jQuery_Window_InclusiveBounds_ExactMatch(t *testing.T) {
	s := neo4jTestStore(t)
	ts := baseTime
	if err := s.Store(context.Background(), []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", ts),
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{Start: ts, End: ts},
	})
	if err != nil {
		t.Fatalf("Query() window inclusive exact: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("window [ts, ts] must include trace at ts, got %d", len(got))
	}
}

// --- Query: Tags filter ---

func TestNeo4jQuery_Tags_ANDSemantics_AllRequired(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTraceWithTags("00000000-0000-0000-0000-000000000001", "change a", []string{"delay", "blockage"}),
		validTraceWithTags("00000000-0000-0000-0000-000000000002", "change b", []string{"delay"}),
		validTraceWithTags("00000000-0000-0000-0000-000000000003", "change c", []string{"blockage"}),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Tags: []string{"delay", "blockage"}})
	if err != nil {
		t.Fatalf("Query() tags AND: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace with both tags, got %d", len(got))
	}
}

func TestNeo4jQuery_Tags_SingleTag_MatchesSubset(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTraceWithTags("00000000-0000-0000-0000-000000000001", "change a", []string{"delay"}),
		validTraceWithTags("00000000-0000-0000-0000-000000000002", "change b", []string{"blockage"}),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Tags: []string{"delay"}})
	if err != nil {
		t.Fatalf("Query() single tag: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace with delay tag, got %d", len(got))
	}
}

func TestNeo4jQuery_Tags_EmptySlice_NoFilter(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Tags: []string{}})
	if err != nil {
		t.Fatalf("Query() empty tags: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("empty tags should return all 2 traces, got %d", len(got))
	}
}

// --- Query: Limit ---

func TestNeo4jQuery_Limit_CapsResults(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", baseTime),
		validTraceAt("00000000-0000-0000-0000-000000000002", "change b", baseTime.Add(time.Second)),
		validTraceAt("00000000-0000-0000-0000-000000000003", "change c", baseTime.Add(2*time.Second)),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Limit: 2})
	if err != nil {
		t.Fatalf("Query() Limit: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 (Limit=2), got %d", len(got))
	}
}

func TestNeo4jQuery_Limit_Zero_NoLimit(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
		validTrace("00000000-0000-0000-0000-000000000003", "change c"),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Limit: 0})
	if err != nil {
		t.Fatalf("Query() Limit=0: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Limit=0 should return all 3 traces, got %d", len(got))
	}
}

// --- Query: ordered by timestamp ---

func TestNeo4jQuery_OrderByTimestamp_Ascending(t *testing.T) {
	s := neo4jTestStore(t)
	// Store out of order to verify DB sorts them.
	t3 := validTraceAt("00000000-0000-0000-0000-000000000003", "third", baseTime.Add(2*time.Hour))
	t1 := validTraceAt("00000000-0000-0000-0000-000000000001", "first", baseTime)
	t2 := validTraceAt("00000000-0000-0000-0000-000000000002", "second", baseTime.Add(time.Hour))
	if err := s.Store(context.Background(), []schema.Trace{t3, t1, t2}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 traces, got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Timestamp.Before(got[i-1].Timestamp) {
			t.Errorf("trace %d timestamp %v is before trace %d timestamp %v — not ascending",
				i, got[i].Timestamp, i-1, got[i-1].Timestamp)
		}
	}
}

// --- Get ---

func TestNeo4jGet_Found_ReturnsTrace(t *testing.T) {
	s := neo4jTestStore(t)
	tr := validTrace("00000000-0000-0000-0000-000000000001", "change a")
	if err := s.Store(context.Background(), []schema.Trace{tr}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, found, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("Get() want no error, got %v", err)
	}
	if !found {
		t.Fatal("Get() want found=true, got false")
	}
	if got.ID != tr.ID {
		t.Errorf("Get() ID: want %q, got %q", tr.ID, got.ID)
	}
}

func TestNeo4jGet_NotFound_ReturnsFalse(t *testing.T) {
	s := neo4jTestStore(t)
	_, found, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000099")
	if err != nil {
		t.Fatalf("Get() missing: want no error, got %v", err)
	}
	if found {
		t.Fatal("Get() missing: want found=false, got true")
	}
}

// --- Store: upsert and validation ---

func TestNeo4jStore_Upsert_UpdatesExistingTrace(t *testing.T) {
	s := neo4jTestStore(t)
	tr := validTrace("00000000-0000-0000-0000-000000000001", "original")
	if err := s.Store(context.Background(), []schema.Trace{tr}); err != nil {
		t.Fatalf("Store initial: %v", err)
	}
	updated := tr
	updated.WhatChanged = "updated"
	if err := s.Store(context.Background(), []schema.Trace{updated}); err != nil {
		t.Fatalf("Store update: %v", err)
	}
	// Should still be one trace.
	all, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("want 1 trace after upsert, got %d", len(all))
	}
	if all[0].WhatChanged != "updated" {
		t.Errorf("want updated WhatChanged, got %q", all[0].WhatChanged)
	}
}

func TestNeo4jStore_InvalidTrace_ReturnsError_NoWrite(t *testing.T) {
	s := neo4jTestStore(t)
	bad := schema.Trace{} // missing required fields
	err := s.Store(context.Background(), []schema.Trace{bad})
	if err == nil {
		t.Fatal("Store with invalid trace: want error, got nil")
	}
	// No traces should be in the DB.
	all, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query after failed store: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("want 0 traces after failed store, got %d", len(all))
	}
}

func TestNeo4jStore_InvalidBatch_AtomicFailure(t *testing.T) {
	s := neo4jTestStore(t)
	// Store one valid trace first.
	pre := validTrace("00000000-0000-0000-0000-000000000001", "pre-existing")
	if err := s.Store(context.Background(), []schema.Trace{pre}); err != nil {
		t.Fatalf("Store pre-existing: %v", err)
	}
	// Now try to store a valid + invalid batch.
	valid := validTrace("00000000-0000-0000-0000-000000000002", "valid in batch")
	invalid := schema.Trace{}
	err := s.Store(context.Background(), []schema.Trace{valid, invalid})
	if err == nil {
		t.Fatal("Store invalid batch: want error, got nil")
	}
	// Only the pre-existing trace should be in the DB — batch was rejected.
	all, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query after invalid batch: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("want 1 trace (pre-existing only) after rejected batch, got %d", len(all))
	}
}

func TestNeo4jStore_EmptySlice_NoWrite(t *testing.T) {
	s := neo4jTestStore(t)
	err := s.Store(context.Background(), []schema.Trace{})
	if err != nil {
		t.Fatalf("Store empty slice: want no error, got %v", err)
	}
	all, err2 := s.Query(context.Background(), store.QueryOpts{})
	if err2 != nil {
		t.Fatalf("Query after empty store: %v", err2)
	}
	if len(all) != 0 {
		t.Errorf("want 0 traces after empty store, got %d", len(all))
	}
}

// --- Close ---

func TestNeo4jStore_Close_Idempotent(t *testing.T) {
	url, user, pass := neo4jTestEnv(t)
	s, err := store.NewNeo4jStore(context.Background(), store.Neo4jConfig{
		BoltURL: url, Username: user, Password: pass,
	})
	if err != nil {
		t.Fatalf("NewNeo4jStore: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// --- Context cancellation ---

func TestNeo4jStore_Store_CancelledContext(t *testing.T) {
	s := neo4jTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Store(ctx, []schema.Trace{validTrace("00000000-0000-0000-0000-000000000001", "c")})
	if err == nil {
		t.Fatal("Store with cancelled context: want error, got nil")
	}
}

func TestNeo4jStore_Query_CancelledContext(t *testing.T) {
	s := neo4jTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Query(ctx, store.QueryOpts{})
	if err == nil {
		t.Fatal("Query with cancelled context: want error, got nil")
	}
}

func TestNeo4jStore_Get_CancelledContext(t *testing.T) {
	s := neo4jTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := s.Get(ctx, "00000000-0000-0000-0000-000000000001")
	if err == nil {
		t.Fatal("Get with cancelled context: want error, got nil")
	}
}

// --- Round-trip: field preservation ---

func TestNeo4jStore_RoundTrip_AllFields(t *testing.T) {
	s := neo4jTestStore(t)
	tr := schema.Trace{
		ID:          "00000000-0000-0000-0000-000000000001",
		Timestamp:   baseTime,
		WhatChanged: "the system rerouted the request",
		Observer:    "analyst/position-a",
		Mediation:   "load-balancer",
		Source:      []string{"service-a", "service-b"},
		Target:      []string{"service-c"},
		Tags:        []string{"redirection", "delay"},
	}
	if err := s.Store(context.Background(), []schema.Trace{tr}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, found, err := s.Get(context.Background(), tr.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("Get: trace not found")
	}
	if got.ID != tr.ID {
		t.Errorf("ID: want %q, got %q", tr.ID, got.ID)
	}
	if !got.Timestamp.Equal(tr.Timestamp) {
		t.Errorf("Timestamp: want %v, got %v", tr.Timestamp, got.Timestamp)
	}
	if got.WhatChanged != tr.WhatChanged {
		t.Errorf("WhatChanged: want %q, got %q", tr.WhatChanged, got.WhatChanged)
	}
	if got.Observer != tr.Observer {
		t.Errorf("Observer: want %q, got %q", tr.Observer, got.Observer)
	}
	if got.Mediation != tr.Mediation {
		t.Errorf("Mediation: want %q, got %q", tr.Mediation, got.Mediation)
	}
	// collect(DISTINCT) order is non-deterministic; compare sorted.
	if !sortedStringSlicesEqual(got.Source, tr.Source) {
		t.Errorf("Source: want %v, got %v", tr.Source, got.Source)
	}
	if !sortedStringSlicesEqual(got.Target, tr.Target) {
		t.Errorf("Target: want %v, got %v", tr.Target, got.Target)
	}
	if !sortedStringSlicesEqual(got.Tags, tr.Tags) {
		t.Errorf("Tags: want %v, got %v", tr.Tags, got.Tags)
	}
}

func TestNeo4jStore_RoundTrip_NilSourceTarget(t *testing.T) {
	s := neo4jTestStore(t)
	tr := validTrace("00000000-0000-0000-0000-000000000001", "no source or target")
	// Source and Target are nil by default in validTrace.
	if err := s.Store(context.Background(), []schema.Trace{tr}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, found, err := s.Get(context.Background(), tr.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("Get: trace not found")
	}
	if got.Source != nil {
		t.Errorf("Source: want nil, got %v", got.Source)
	}
	if got.Target != nil {
		t.Errorf("Target: want nil, got %v", got.Target)
	}
}

// --- Neo4j-specific: element deduplication ---

// TestNeo4jStore_ElementDeduplication verifies that two traces sharing a
// source string result in one :Element node, not two. This tests the MERGE
// semantics on :Element.name — the provisional equivalence criterion from
// kg-scoping-v1.md §1.3.
func TestNeo4jStore_ElementDeduplication(t *testing.T) {
	url, user, pass := neo4jTestEnv(t)
	s := neo4jTestStore(t)

	tr1 := validTrace("00000000-0000-0000-0000-000000000001", "change a")
	tr1.Source = []string{"shared-element"}
	tr2 := validTrace("00000000-0000-0000-0000-000000000002", "change b")
	tr2.Source = []string{"shared-element"}

	if err := s.Store(context.Background(), []schema.Trace{tr1, tr2}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Count :Element nodes via raw driver.
	ctx := context.Background()
	driver, err := neo4jdriver.NewDriverWithContext(url, neo4jdriver.BasicAuth(user, pass, ""))
	if err != nil {
		t.Fatalf("create driver: %v", err)
	}
	defer driver.Close(ctx)

	sess := driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer sess.Close(ctx)

	result, err := sess.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			"MATCH (e:Element {name: $name}) RETURN count(e) AS cnt",
			map[string]any{"name": "shared-element"})
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, err
		}
		cnt, _ := record.Get("cnt")
		return cnt, nil
	})
	if err != nil {
		t.Fatalf("count :Element nodes: %v", err)
	}
	count, _ := result.(int64)
	if count != 1 {
		t.Errorf("want 1 :Element node for shared-element (MERGE dedup), got %d", count)
	}
}

// TestNeo4jStore_RelationshipDirections verifies that SOURCE_OF goes
// Element→Trace and TARGETS goes Trace→Element, as specified in
// kg-scoping-v1.md §1.2.
func TestNeo4jStore_RelationshipDirections(t *testing.T) {
	url, user, pass := neo4jTestEnv(t)
	s := neo4jTestStore(t)

	tr := validTrace("00000000-0000-0000-0000-000000000001", "directed relationship test")
	tr.Source = []string{"source-element"}
	tr.Target = []string{"target-element"}
	if err := s.Store(context.Background(), []schema.Trace{tr}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	ctx := context.Background()
	driver, err := neo4jdriver.NewDriverWithContext(url, neo4jdriver.BasicAuth(user, pass, ""))
	if err != nil {
		t.Fatalf("create driver: %v", err)
	}
	defer driver.Close(ctx)
	sess := driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer sess.Close(ctx)

	// Verify SOURCE_OF direction: Element → Trace.
	sourceResult, err := sess.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			"MATCH (e:Element {name: 'source-element'})-[:SOURCE_OF]->(t:Trace {id: $id}) RETURN count(t) AS cnt",
			map[string]any{"id": tr.ID})
		if err != nil {
			return nil, err
		}
		rec, err := res.Single(ctx)
		if err != nil {
			return nil, err
		}
		cnt, _ := rec.Get("cnt")
		return cnt, nil
	})
	if err != nil {
		t.Fatalf("check SOURCE_OF direction: %v", err)
	}
	if c, _ := sourceResult.(int64); c != 1 {
		t.Errorf("SOURCE_OF: want Element→Trace, direction wrong (count=%d)", c)
	}

	// Verify TARGETS direction: Trace → Element.
	targetResult, err := sess.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			"MATCH (t:Trace {id: $id})-[:TARGETS]->(e:Element {name: 'target-element'}) RETURN count(e) AS cnt",
			map[string]any{"id": tr.ID})
		if err != nil {
			return nil, err
		}
		rec, err := res.Single(ctx)
		if err != nil {
			return nil, err
		}
		cnt, _ := rec.Get("cnt")
		return cnt, nil
	})
	if err != nil {
		t.Fatalf("check TARGETS direction: %v", err)
	}
	if c, _ := targetResult.(int64); c != 1 {
		t.Errorf("TARGETS: want Trace→Element, direction wrong (count=%d)", c)
	}
}

// --- Query: combined filters (AND across multiple opts fields) ---

func TestNeo4jQuery_ObserverAndWindow_AND(t *testing.T) {
	s := neo4jTestStore(t)
	day1, day2, day3 := baseTime, baseTime.Add(24*time.Hour), baseTime.Add(48*time.Hour)
	traces := []schema.Trace{
		validTraceWithObserver("00000000-0000-0000-0000-000000000001", "a", "observer/alpha"),
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000002", "b", "observer/alpha")
			tr.Timestamp = day2
			return tr
		}(),
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000003", "c", "observer/beta")
			tr.Timestamp = day2
			return tr
		}(),
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000004", "d", "observer/alpha")
			tr.Timestamp = day3
			return tr
		}(),
	}
	_ = day1 // not used in filter
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{
		Observer: "observer/alpha",
		Window:   graph.TimeWindow{Start: day2, End: day2},
	})
	if err != nil {
		t.Fatalf("Query() observer+window: %v", err)
	}
	// Only trace 2: observer/alpha AND in day2 window.
	if len(got) != 1 {
		t.Errorf("want 1 trace (observer/alpha, day2), got %d", len(got))
	}
}

func TestNeo4jQuery_ObserverAndTags_AND(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		// Matches both filters.
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000001", "a", "observer/alpha")
			tr.Tags = []string{"delay"}
			return tr
		}(),
		// Wrong observer.
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000002", "b", "observer/beta")
			tr.Tags = []string{"delay"}
			return tr
		}(),
		// Correct observer, wrong tag.
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000003", "c", "observer/alpha")
			tr.Tags = []string{"blockage"}
			return tr
		}(),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{
		Observer: "observer/alpha",
		Tags:     []string{"delay"},
	})
	if err != nil {
		t.Fatalf("Query() observer+tags: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace (observer/alpha AND delay tag), got %d", len(got))
	}
}

func TestNeo4jQuery_AllFilters_AND(t *testing.T) {
	s := neo4jTestStore(t)
	day2 := baseTime.Add(24 * time.Hour)
	traces := []schema.Trace{
		// Matches all filters.
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000001", "match", "observer/alpha")
			tr.Timestamp = day2
			tr.Tags = []string{"delay"}
			return tr
		}(),
		// Wrong observer.
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000002", "b", "observer/beta")
			tr.Timestamp = day2
			tr.Tags = []string{"delay"}
			return tr
		}(),
		// Outside window.
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000003", "c", "observer/alpha")
			tr.Timestamp = baseTime
			tr.Tags = []string{"delay"}
			return tr
		}(),
		// Missing tag.
		func() schema.Trace {
			tr := validTraceWithObserver("00000000-0000-0000-0000-000000000004", "d", "observer/alpha")
			tr.Timestamp = day2
			tr.Tags = []string{"blockage"}
			return tr
		}(),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{
		Observer: "observer/alpha",
		Window:   graph.TimeWindow{Start: day2, End: day2},
		Tags:     []string{"delay"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Query() all filters: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace matching all filters, got %d", len(got))
	}
	if len(got) > 0 && got[0].WhatChanged != "match" {
		t.Errorf("want WhatChanged=match, got %q", got[0].WhatChanged)
	}
}

// --- Query: zero TimeWindow ---

func TestNeo4jQuery_Window_Zero_NoFilter(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{}, // zero window: no constraint
	})
	if err != nil {
		t.Fatalf("Query() zero window: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("zero TimeWindow should return all 2 traces, got %d", len(got))
	}
}

// --- Query: Limit edge cases ---

func TestNeo4jQuery_Limit_LargerThanDataset_ReturnsAll(t *testing.T) {
	s := neo4jTestStore(t)
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{Limit: 100})
	if err != nil {
		t.Fatalf("Query() Limit larger than dataset: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Limit=100 with 2 traces: want 2, got %d", len(got))
	}
}

// --- Round-trip: nil/empty slice edge cases ---

// TestNeo4jStore_RoundTrip_EmptySourceSlice documents the normalization
// contract: storeCypher converts nil→[]string{} before sending to Neo4j;
// anySliceToStrings converts empty collect result back to nil. A caller
// storing Source: []string{} should receive Source: nil back.
func TestNeo4jStore_RoundTrip_EmptySourceSlice(t *testing.T) {
	s := neo4jTestStore(t)
	tr := validTrace("00000000-0000-0000-0000-000000000001", "empty source")
	tr.Source = []string{} // non-nil empty slice
	if err := s.Store(context.Background(), []schema.Trace{tr}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, found, err := s.Get(context.Background(), tr.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("Get: trace not found")
	}
	// storeCypher normalises []string{} to an empty FOREACH; collect(DISTINCT)
	// returns []; anySliceToStrings returns nil. This is the documented contract.
	if got.Source != nil {
		t.Errorf("empty Source slice normalises to nil on round-trip, got %v", got.Source)
	}
}

// TestNeo4jStore_RoundTrip_NilTags confirms that nil Tags is preserved
// through the nil→[]string{} normalisation and empty-list→nil retrieval path.
func TestNeo4jStore_RoundTrip_NilTags(t *testing.T) {
	s := neo4jTestStore(t)
	tr := validTrace("00000000-0000-0000-0000-000000000001", "nil tags")
	// validTrace does not set Tags; it remains nil.
	if err := s.Store(context.Background(), []schema.Trace{tr}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, found, err := s.Get(context.Background(), tr.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("Get: trace not found")
	}
	if got.Tags != nil {
		t.Errorf("nil Tags should round-trip to nil, got %v", got.Tags)
	}
}
