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
	json.NewDecoder(rr.Body).Decode(&body)
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
