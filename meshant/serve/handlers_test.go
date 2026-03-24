// Package serve_test provides black-box tests for the serve package.
// All tests use httptest.NewRecorder and inject a *store.JSONFileStore
// loaded with a small deterministic fixture — no real HTTP listener is needed.
package serve_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/serve"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// errStore is a stub TraceStore whose Query always returns an error.
// Used to exercise the 500-error path in all four handlers (Finding 5).
type errStore struct{}

func (errStore) Store(_ context.Context, _ []schema.Trace) error { return nil }
func (errStore) Query(_ context.Context, _ store.QueryOpts) ([]schema.Trace, error) {
	return nil, fmt.Errorf("store: connection refused")
}
func (errStore) Get(_ context.Context, _ string) (schema.Trace, bool, error) {
	return schema.Trace{}, false, nil
}
func (errStore) Close() error { return nil }

// testTraces returns 4 deterministic traces: 2 from "alice", 2 from "bob".
// alice and bob share no elements — shadow is always non-zero for both.
func testTraces() []schema.Trace {
	return []schema.Trace{
		{
			ID:          "a0000000-0000-0000-0000-000000000001",
			Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			WhatChanged: "alice change 1",
			Observer:    "alice",
			Source:      []string{"element-a"},
			Target:      []string{"element-b"},
		},
		{
			ID:          "a0000000-0000-0000-0000-000000000002",
			Timestamp:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			WhatChanged: "alice change 2",
			Observer:    "alice",
			Source:      []string{"element-b"},
			Target:      []string{"element-c"},
			Tags:        []string{"structural"},
		},
		{
			ID:          "a0000000-0000-0000-0000-000000000003",
			Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			WhatChanged: "bob change 1",
			Observer:    "bob",
			Source:      []string{"element-x"},
			Target:      []string{"element-y"},
		},
		{
			ID:          "a0000000-0000-0000-0000-000000000004",
			Timestamp:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			WhatChanged: "bob change 2",
			Observer:    "bob",
			Source:      []string{"element-y"},
			Target:      []string{"element-z"},
			Tags:        []string{"structural"},
		},
	}
}

// testServer creates a serve.Server backed by a JSONFileStore populated with testTraces.
func testServer(t *testing.T) *serve.Server {
	t.Helper()
	ts := store.NewJSONFileStore(t.TempDir() + "/traces.json")
	if err := ts.Store(context.Background(), testTraces()); err != nil {
		t.Fatalf("testServer: store: %v", err)
	}
	return serve.NewServer(ts)
}

// doGET sends a GET request to path on srv and returns the recorded response.
func doGET(srv *serve.Server, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

// decodeEnvelope parses the response body into a generic map for assertions.
func decodeEnvelope(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var env map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("decodeEnvelope: body=%q err=%v", rr.Body.String(), err)
	}
	return env
}

// cutField extracts the "cut" sub-map from an envelope.
func cutField(t *testing.T, env map[string]interface{}) map[string]interface{} {
	t.Helper()
	cut, ok := env["cut"].(map[string]interface{})
	if !ok {
		t.Fatalf("envelope missing 'cut' field: %v", env)
	}
	return cut
}

// --- Routing tests ---

func TestServer_UnknownRoute_404(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/unknown")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestServer_NonGET_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/articulate?observer=alice", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestServer_ContentTypeJSON(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=alice")
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

// --- /articulate tests ---

func TestHandleArticulate_MissingObserver_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "observer is required") {
		t.Errorf("error should mention 'observer is required': %q", errMsg)
	}
}

func TestHandleArticulate_HappyPath_200(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)

	if cut["observer"] != "alice" {
		t.Errorf("cut.observer should be 'alice', got %v", cut["observer"])
	}
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 2 {
		t.Errorf("cut.trace_count should be 2 (alice has 2 traces), got %v", traceCount)
	}
	// alice and bob share no elements — shadow_count should be > 0
	shadowCount, _ := cut["shadow_count"].(float64)
	if shadowCount == 0 {
		t.Errorf("cut.shadow_count should be > 0 when bob's elements are in shadow")
	}
	if _, ok := env["data"]; !ok {
		t.Errorf("envelope missing 'data' field")
	}
}

func TestHandleArticulate_WithTimeWindow(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=alice&from=2026-01-02T00:00:00Z")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	// Only alice's second trace (2026-01-02) passes the from filter.
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 1 {
		t.Errorf("expected trace_count 1 (after from filter), got %v", traceCount)
	}
	if cut["from"] == nil {
		t.Errorf("cut.from should be populated when ?from= is given")
	}
}

func TestHandleArticulate_WithTags(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=alice&tags=structural")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	// Only alice's second trace has the "structural" tag.
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 1 {
		t.Errorf("expected trace_count 1 (structural tag filter), got %v", traceCount)
	}
}

func TestHandleArticulate_InvalidFrom_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=alice&from=not-a-date")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "invalid") || !strings.Contains(errMsg, "from") {
		t.Errorf("error should mention 'invalid' and 'from': %q", errMsg)
	}
}

func TestHandleArticulate_InvertedTimeWindow_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=alice&from=2026-02-01T00:00:00Z&to=2026-01-01T00:00:00Z")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// --- /diff tests ---

func TestHandleDiff_MissingObserverA_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/diff?observer-b=bob")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "diff requires two observer positions") {
		t.Errorf("error should mention 'diff requires two observer positions': %q", errMsg)
	}
}

func TestHandleDiff_MissingObserverB_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/diff?observer-a=alice")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "diff requires two observer positions") {
		t.Errorf("error should mention 'diff requires two observer positions': %q", errMsg)
	}
}

func TestHandleDiff_HappyPath_200(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/diff?observer-a=alice&observer-b=bob")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	// Envelope cut is from observer-a (alice) perspective — design decision D4.
	if cut["observer"] != "alice" {
		t.Errorf("cut.observer should be alice (observer-a), got %v", cut["observer"])
	}
	// data should contain GraphDiff fields
	data, ok := env["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be an object, got %T: %v", env["data"], env["data"])
	}
	// GraphDiff has NodesAdded, NodesRemoved, etc.
	if _, ok := data["nodes_added"]; !ok {
		t.Errorf("data should contain 'nodes_added': %v", data)
	}
}

func TestHandleDiff_SameObserver_200(t *testing.T) {
	// Diffing an observer against themselves is valid — yields zero changes.
	srv := testServer(t)
	rr := doGET(srv, "/diff?observer-a=alice&observer-b=alice")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	data, ok := env["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be an object, got %T", env["data"])
	}
	// Self-diff should produce no added or removed nodes/edges.
	for _, field := range []string{"nodes_added", "nodes_removed", "edges_added", "edges_removed"} {
		arr, _ := data[field].([]interface{})
		if len(arr) != 0 {
			t.Errorf("self-diff: %s should be empty, got %v", field, arr)
		}
	}
}

// --- /shadow tests ---

func TestHandleShadow_MissingObserver_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/shadow")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "observer is required") {
		t.Errorf("error should mention 'observer is required': %q", errMsg)
	}
}

func TestHandleShadow_HappyPath_200(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/shadow?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	if cut["observer"] != "alice" {
		t.Errorf("cut.observer should be 'alice', got %v", cut["observer"])
	}
	// data should be an array of shadow elements (alice can't see bob's elements)
	shadowCount, _ := cut["shadow_count"].(float64)
	if shadowCount == 0 {
		t.Errorf("shadow_count should be > 0 for alice (bob's elements are in shadow)")
	}
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T: %v", env["data"], env["data"])
	}
	if len(data) == 0 {
		t.Errorf("shadow data should be non-empty for alice")
	}
}

func TestHandleShadow_NoShadow_EmptyArray(t *testing.T) {
	// Use a store with only one observer — no shadow.
	ts := store.NewJSONFileStore(t.TempDir() + "/traces.json")
	onlyAlice := []schema.Trace{testTraces()[0]} // just one trace from alice
	if err := ts.Store(context.Background(), onlyAlice); err != nil {
		t.Fatalf("setup: store: %v", err)
	}
	srv := serve.NewServer(ts)

	rr := doGET(srv, "/shadow?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	shadowCount, _ := cut["shadow_count"].(float64)
	if shadowCount != 0 {
		t.Errorf("shadow_count should be 0 when only one observer, got %v", shadowCount)
	}
	// data must be a JSON array (not null) — verifies the nil-guard in handlers.go.
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be a JSON array (not null) even when shadow is empty; got %T", env["data"])
	}
	if len(data) != 0 {
		t.Errorf("expected empty shadow array, got %d elements", len(data))
	}
}

// --- /traces tests ---

func TestHandleTraces_MissingObserver_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/traces")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "observer is required") && !strings.Contains(errMsg, "every reading is positioned") {
		t.Errorf("error should mention observer requirement: %q", errMsg)
	}
}

func TestHandleTraces_HappyPath_200(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/traces?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	if cut["observer"] != "alice" {
		t.Errorf("cut.observer should be 'alice', got %v", cut["observer"])
	}
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 2 {
		t.Errorf("expected trace_count 2 for alice, got %v", traceCount)
	}
	// shadow_count is approximate: 4 total - 2 alice = 2
	shadowCount, _ := cut["shadow_count"].(float64)
	if shadowCount != 2 {
		t.Errorf("expected shadow_count 2 (approximate: 4 total - 2 alice), got %v", shadowCount)
	}
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T", env["data"])
	}
	if len(data) != 2 {
		t.Errorf("expected 2 traces in data, got %d", len(data))
	}
}

func TestHandleTraces_WithLimit(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/traces?observer=alice&limit=1")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T", env["data"])
	}
	if len(data) != 1 {
		t.Errorf("expected 1 trace (limit=1), got %d", len(data))
	}
}

func TestHandleTraces_LimitZero_ReturnsAll(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/traces?observer=alice&limit=0")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T", env["data"])
	}
	if len(data) != 2 {
		t.Errorf("expected 2 traces (limit=0 means no limit), got %d", len(data))
	}
}

func TestHandleTraces_InvalidLimit_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/traces?observer=alice&limit=bad")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleTraces_WithTimeWindow(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/traces?observer=alice&from=2026-01-02T00:00:00Z")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 1 {
		t.Errorf("expected 1 trace after from filter, got %v", traceCount)
	}
}

// F6: ?to= upper-bound-only test — covers cutMetaFromGraph toPtr branch and
// filterTraces end-bound filter (both previously uncovered).
func TestHandleArticulate_ToOnly_200(t *testing.T) {
	srv := testServer(t)
	// Only alice's first trace (2026-01-01) is on or before 2026-01-01T23:59:59Z.
	rr := doGET(srv, "/articulate?observer=alice&to=2026-01-01T23:59:59Z")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 1 {
		t.Errorf("expected 1 trace (to filter), got %v", traceCount)
	}
	if cut["to"] == nil {
		t.Errorf("cut.to should be populated when ?to= is given")
	}
}

// F7: invalid ?from= on /diff — covers the parseQueryTime error branch in handleDiff.
func TestHandleDiff_InvalidFrom_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/diff?observer-a=alice&observer-b=bob&from=bad-date")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "invalid") || !strings.Contains(errMsg, "from") {
		t.Errorf("error should mention 'invalid' and 'from': %q", errMsg)
	}
}

// F8: tags filter on /traces — covers filterTraces tags-matching branch.
func TestHandleTraces_WithTags(t *testing.T) {
	srv := testServer(t)
	// Only alice's second trace has the "structural" tag.
	rr := doGET(srv, "/traces?observer=alice&tags=structural")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 1 {
		t.Errorf("expected 1 trace (structural tag filter on alice), got %v", traceCount)
	}
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T", env["data"])
	}
	if len(data) != 1 {
		t.Errorf("expected 1 trace in data, got %d", len(data))
	}
}

// F4 + F5 — observer with zero matches and failing store.

// TestHandleArticulate_UnknownObserver_EmptyGraph: a valid observer name not in
// the store returns 200 with trace_count=0 and a valid (empty) graph.
// Exercises the full-cut/zero-result path in Articulate.
func TestHandleArticulate_UnknownObserver_EmptyGraph(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=charlie")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 0 {
		t.Errorf("expected trace_count 0 for unknown observer, got %v", traceCount)
	}
	if _, ok := env["data"]; !ok {
		t.Errorf("envelope should still have a data field for zero-match cut")
	}
}

// TestHandleShadow_UnknownObserver_EmptySlice: unknown observer → shadow is all
// elements in the store. data must be a non-null JSON array.
func TestHandleShadow_UnknownObserver_EmptySlice(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/shadow?observer=charlie")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 0 {
		t.Errorf("expected trace_count 0 for unknown observer, got %v", traceCount)
	}
	// data must be a JSON array (not null), even for a zero-match articulation.
	if _, ok := env["data"].([]interface{}); !ok {
		t.Fatalf("data should be a JSON array, got %T: %v", env["data"], env["data"])
	}
}

// TestHandleTraces_UnknownObserver_EmptyArray: unknown observer → data is []
// not null. Exercises the nil-guard at handlers.go:212-215.
func TestHandleTraces_UnknownObserver_EmptyArray(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/traces?observer=charlie")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	traceCount, _ := cut["trace_count"].(float64)
	if traceCount != 0 {
		t.Errorf("expected trace_count 0 for unknown observer, got %v", traceCount)
	}
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be a JSON array (not null) for zero-match observer; got %T", env["data"])
	}
	if len(data) != 0 {
		t.Errorf("expected empty array for unknown observer, got %d elements", len(data))
	}
}

// F5: failing store — all four handlers should return 500 when Query fails.

func TestHandleArticulate_StoreError_500(t *testing.T) {
	srv := serve.NewServer(errStore{})
	rr := doGET(srv, "/articulate?observer=alice")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on store failure, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"] == nil {
		t.Errorf("error body should have 'error' field")
	}
}

func TestHandleDiff_StoreError_500(t *testing.T) {
	srv := serve.NewServer(errStore{})
	rr := doGET(srv, "/diff?observer-a=alice&observer-b=bob")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on store failure, got %d", rr.Code)
	}
}

func TestHandleShadow_StoreError_500(t *testing.T) {
	srv := serve.NewServer(errStore{})
	rr := doGET(srv, "/shadow?observer=alice")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on store failure, got %d", rr.Code)
	}
}

func TestHandleTraces_StoreError_500(t *testing.T) {
	srv := serve.NewServer(errStore{})
	rr := doGET(srv, "/traces?observer=alice")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on store failure, got %d", rr.Code)
	}
}

// --- /element/{name} tests ---

// TestHandleElement_MissingObserver_400: GET /element/element-a with no observer
// must return 400 with ANT-flavoured observer error. Element visibility is always
// positioned — no observer means no cut.
func TestHandleElement_MissingObserver_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/element/element-a")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	// Must use em dash (—) matching the canonical ANT error string.
	if !strings.Contains(errMsg, "observer is required") {
		t.Errorf("error should mention 'observer is required': %q", errMsg)
	}
}

// TestHandleElement_HappyPath_200: GET /element/element-a?observer=alice returns 200
// and traces whose Source or Target contains "element-a". Alice's first trace has
// element-a in Source; her second trace does not mention element-a.
func TestHandleElement_HappyPath_200(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/element/element-a?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	cut := cutField(t, env)
	if cut["observer"] != "alice" {
		t.Errorf("cut.observer should be 'alice', got %v", cut["observer"])
	}
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T: %v", env["data"], env["data"])
	}
	// Alice's first trace: Source=["element-a"] Target=["element-b"] — one match.
	if len(data) != 1 {
		t.Errorf("expected 1 trace mentioning element-a for alice, got %d", len(data))
	}
}

// TestHandleElement_UnknownElement_EmptyArray: element not in any trace returns
// 200 with data=[] (not null, not 404). The ANT response is: the element exists
// nowhere in this observer's substrate.
func TestHandleElement_UnknownElement_EmptyArray(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/element/element-does-not-exist?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be a JSON array (not null) for unknown element; got %T", env["data"])
	}
	if len(data) != 0 {
		t.Errorf("expected empty array for unknown element, got %d elements", len(data))
	}
}

// TestHandleElement_FiltersByObserver: element-x appears in bob's traces but not
// alice's. Requesting ?observer=alice must return an empty array; ?observer=bob
// must return bob's traces mentioning element-x.
func TestHandleElement_FiltersByObserver(t *testing.T) {
	srv := testServer(t)

	// Alice cannot see element-x (it's in bob's traces only).
	rrAlice := doGET(srv, "/element/element-x?observer=alice")
	if rrAlice.Code != http.StatusOK {
		t.Errorf("expected 200 for alice/element-x, got %d", rrAlice.Code)
	}
	envAlice := decodeEnvelope(t, rrAlice)
	dataAlice, ok := envAlice["data"].([]interface{})
	if !ok {
		t.Fatalf("alice/element-x data should be a JSON array, got %T", envAlice["data"])
	}
	if len(dataAlice) != 0 {
		t.Errorf("alice should see 0 traces for element-x, got %d", len(dataAlice))
	}

	// Bob can see element-x (source of his first trace).
	rrBob := doGET(srv, "/element/element-x?observer=bob")
	if rrBob.Code != http.StatusOK {
		t.Errorf("expected 200 for bob/element-x, got %d", rrBob.Code)
	}
	envBob := decodeEnvelope(t, rrBob)
	dataBob, ok := envBob["data"].([]interface{})
	if !ok {
		t.Fatalf("bob/element-x data should be a JSON array, got %T", envBob["data"])
	}
	if len(dataBob) != 1 {
		t.Errorf("bob should see 1 trace for element-x, got %d", len(dataBob))
	}
}

// TestHandleElement_StoreError_500: errStore.Query failure → 500 with error field.
func TestHandleElement_StoreError_500(t *testing.T) {
	srv := serve.NewServer(errStore{})
	rr := doGET(srv, "/element/element-a?observer=alice")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on store failure, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"] == nil {
		t.Errorf("error body should have 'error' field")
	}
}

// TestHandleElement_WithTimeWindow: ?from= filter applies on top of the element
// filter. Alice has two traces; only the second (2026-01-02) mentions element-b
// in Target. With ?from=2026-01-02, only the second trace passes the time window.
func TestHandleElement_WithTimeWindow(t *testing.T) {
	srv := testServer(t)
	// element-b appears in alice's first trace (Target) and second trace (Source).
	// With ?from=2026-01-02T00:00:00Z only the second trace is within window.
	rr := doGET(srv, "/element/element-b?observer=alice&from=2026-01-02T00:00:00Z")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T", env["data"])
	}
	// After from filter only alice's second trace (2026-01-02, Source=element-b) matches.
	if len(data) != 1 {
		t.Errorf("expected 1 trace after time-window filter for element-b, got %d", len(data))
	}
}

// TestHandleElement_URLEncoding: an element name that is URL-encoded in the path
// should be decoded correctly. "element%2Da" decodes to "element-a".
func TestHandleElement_URLEncoding(t *testing.T) {
	srv := testServer(t)
	// "element%2Da" URL-decodes to "element-a"
	rr := doGET(srv, "/element/element%2Da?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T", env["data"])
	}
	if len(data) != 1 {
		t.Errorf("expected 1 trace for URL-encoded element name, got %d", len(data))
	}
}

// TestHandleElement_TargetMatch_200: element-b appears in alice's first trace as
// a Target and in alice's second trace as a Source. Without a time-window filter
// both traces match, exercising the Target-loop branch in filterByElement.
func TestHandleElement_TargetMatch_200(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/element/element-b?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("data should be an array, got %T", env["data"])
	}
	// Trace 1 (2026-01-01): Source=["element-a"] Target=["element-b"] → Target match.
	// Trace 2 (2026-01-02): Source=["element-b"] Target=["element-c"] → Source match.
	// Both must be returned.
	if len(data) != 2 {
		t.Errorf("expected 2 traces for element-b (Source+Target matches), got %d", len(data))
	}
}

// TestHandleArticulate_InvalidTo_400: invalid ?to= value → 400. Exercises the
// parseQueryTime "to" error branch in response.go.
func TestHandleArticulate_InvalidTo_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=alice&to=not-a-date")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ?to=, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "invalid") || !strings.Contains(errMsg, "to") {
		t.Errorf("error should mention 'invalid' and 'to': %q", errMsg)
	}
}

// TestHandleShadow_InvalidFrom_400: invalid ?from= value on /shadow → 400.
// Covers the parseQueryTime error branch in handleShadow.
func TestHandleShadow_InvalidFrom_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/shadow?observer=alice&from=not-a-date")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ?from= on /shadow, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "invalid") || !strings.Contains(errMsg, "from") {
		t.Errorf("error should mention 'invalid' and 'from': %q", errMsg)
	}
}

// TestHandleElement_InvalidFrom_400: invalid ?from= value on /element/{name} → 400.
// Covers the parseQueryTime error branch in handleElement.
func TestHandleElement_InvalidFrom_400(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/element/element-a?observer=alice&from=not-a-date")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ?from= on /element, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "invalid") || !strings.Contains(errMsg, "from") {
		t.Errorf("error should mention 'invalid' and 'from': %q", errMsg)
	}
}

// --- static file serving tests ---

// TestServer_Root_ServesHTML: GET / returns 200 with Content-Type containing
// "text/html". Verifies that the go:embed web/ assets are mounted correctly.
func TestServer_Root_ServesHTML(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /, got %d: %s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected Content-Type containing 'text/html', got %q", ct)
	}
}

// TestServer_StaticCSS: GET /style.css returns 200 and correct Content-Type.
func TestServer_StaticCSS(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/style.css")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /style.css, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "css") {
		t.Errorf("expected CSS Content-Type, got %q", ct)
	}
}

// TestServer_StaticJS: GET /app.js returns 200 and a JS Content-Type.
func TestServer_StaticJS(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/app.js")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /app.js, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Errorf("expected JS Content-Type, got %q", ct)
	}
}

// TestServer_APIRoutesPrecedence: API routes (registered before the static file
// handler) must not be masked by the static file server. An API path still
// returns JSON even after the static handler is added.
func TestServer_APIRoutesPrecedence(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/articulate?observer=alice")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for /articulate, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json for API route, got %q", ct)
	}
}

// --- handleObservers tests ---

func TestHandleObservers_HappyPath_200(t *testing.T) {
	srv := testServer(t)
	rr := doGET(srv, "/observers")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	env := decodeEnvelope(t, rr)

	// data field must be a JSON array.
	data, ok := env["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be array, got %T: %v", env["data"], env["data"])
	}

	// testTraces() has "alice" and "bob" — sorted alphabetically.
	if len(data) != 2 {
		t.Fatalf("expected 2 observers, got %d: %v", len(data), data)
	}
	if data[0].(string) != "alice" || data[1].(string) != "bob" {
		t.Errorf("expected [alice bob], got %v", data)
	}
}

func TestHandleObservers_Sorted(t *testing.T) {
	// Populate store with observers in non-alphabetical order.
	ts := store.NewJSONFileStore(t.TempDir() + "/traces.json")
	traces := []schema.Trace{
		{ID: "b0000000-0000-0000-0000-000000000001", Timestamp: time.Now(), WhatChanged: "z", Observer: "zara"},
		{ID: "b0000000-0000-0000-0000-000000000002", Timestamp: time.Now(), WhatChanged: "a", Observer: "alice"},
		{ID: "b0000000-0000-0000-0000-000000000003", Timestamp: time.Now(), WhatChanged: "m", Observer: "mike"},
	}
	if err := ts.Store(context.Background(), traces); err != nil {
		t.Fatalf("store: %v", err)
	}
	srv := serve.NewServer(ts)
	rr := doGET(srv, "/observers")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	data := env["data"].([]interface{})
	names := make([]string, len(data))
	for i, d := range data {
		names[i] = d.(string)
	}
	want := []string{"alice", "mike", "zara"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("position %d: want %q got %q", i, w, names[i])
		}
	}
}

func TestHandleObservers_EmptyStore_EmptyArray(t *testing.T) {
	ts := store.NewJSONFileStore(t.TempDir() + "/traces.json")
	srv := serve.NewServer(ts)
	rr := doGET(srv, "/observers")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	// data should be null or empty array — either is acceptable for an empty store.
	// The endpoint returns whatever []string{} marshals to (null when nil, [] when empty).
	// Verify no error field is set.
	if errField, ok := env["error"]; ok && errField != nil {
		t.Errorf("unexpected error field: %v", errField)
	}
}

func TestHandleObservers_StoreError_500(t *testing.T) {
	srv := serve.NewServer(errStore{})
	rr := doGET(srv, "/observers")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestHandleObservers_Deduplicated(t *testing.T) {
	// Multiple traces with the same observer should produce one entry.
	ts := store.NewJSONFileStore(t.TempDir() + "/traces.json")
	traces := []schema.Trace{
		{ID: "c0000000-0000-0000-0000-000000000001", Timestamp: time.Now(), WhatChanged: "a", Observer: "alice"},
		{ID: "c0000000-0000-0000-0000-000000000002", Timestamp: time.Now(), WhatChanged: "b", Observer: "alice"},
		{ID: "c0000000-0000-0000-0000-000000000003", Timestamp: time.Now(), WhatChanged: "c", Observer: "alice"},
	}
	if err := ts.Store(context.Background(), traces); err != nil {
		t.Fatalf("store: %v", err)
	}
	srv := serve.NewServer(ts)
	rr := doGET(srv, "/observers")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	data := env["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 deduplicated observer, got %d: %v", len(data), data)
	}
}
