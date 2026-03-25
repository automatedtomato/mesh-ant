// Package mcp_test contains black-box tests for the meshant MCP server.
//
// Test strategy:
//   - All tests use bytes.Buffer for I/O — no goroutines, no network.
//   - The fidelity test (TestMCPServer_Articulate_Fidelity) is the core
//     correctness assertion: the MCP result must equal the direct Go API result
//     wrapped in the same Envelope. If they diverge the MCP layer has introduced
//     an unattributable transformation (D6 in mcp-v1.md).
//   - Protocol tests verify JSON-RPC correctness independently of MeshAnt logic.
package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/mcp"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// --- helpers ---

// baseTime is a deterministic reference timestamp used across tests.
var baseTime = time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

// validTrace returns a minimal Trace that passes schema.Validate().
func validTrace(id, whatChanged, observer string) schema.Trace {
	return schema.Trace{
		ID:          id,
		Timestamp:   baseTime,
		WhatChanged: whatChanged,
		Observer:    observer,
	}
}

// testStore builds a JSONFileStore pre-populated with the given traces by
// writing them to a temp file and returning a store over that path. It also
// returns a cleanup function.
func testStore(t *testing.T, traces []schema.Trace) store.TraceStore {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/traces.json"
	data, err := json.MarshalIndent(traces, "", "  ")
	if err != nil {
		t.Fatalf("testStore: marshal: %v", err)
	}
	if err := writeFile(path, data); err != nil {
		t.Fatalf("testStore: write: %v", err)
	}
	return store.NewJSONFileStore(path)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

// runMCP drives the server with the given newline-delimited JSON-RPC messages
// and returns all response lines. Messages are sent as a single block; the
// server reads until EOF.
//
// Protocol bootstrap: every test must send initialize first, then
// notifications/initialized, then the actual method call. The server returns
// no response for notifications.
func runMCP(t *testing.T, srv *mcp.Server, messages []map[string]interface{}) []map[string]interface{} {
	t.Helper()

	var buf bytes.Buffer
	for _, msg := range messages {
		b, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("runMCP: marshal message: %v", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}

	var out bytes.Buffer
	ctx := context.Background()
	if err := srv.Run(ctx, &buf, &out); err != nil {
		t.Fatalf("runMCP: server error: %v", err)
	}

	// Parse newline-delimited JSON responses.
	var responses []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("runMCP: unmarshal response line %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	return responses
}

// initMessages returns the standard protocol bootstrap messages.
// The server must see initialize (request) and notifications/initialized
// (notification) before tools/call.
func initMessages(id interface{}) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "initialize",
			"params":  map[string]interface{}{},
		},
		{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		},
	}
}

// --- TestMCPServer_Articulate_Fidelity ---
//
// D6 (mcp-v1.md): MCP tool result must equal the direct Go API result wrapped
// in the same Envelope. Call meshant_articulate via MCP, call graph.Articulate
// directly, compare Envelope.Data. Also verifies Envelope.Cut.Analyst and
// Envelope.Cut.Observer.
func TestMCPServer_Articulate_Fidelity(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-4000-8000-000000000001", "A deployed B", "alice"),
		validTrace("00000000-0000-4000-8000-000000000002", "B reported to C", "alice"),
		validTrace("00000000-0000-4000-8000-000000000003", "D blocked E", "bob"),
	}
	ts := testStore(t, traces)
	defer ts.Close()

	srv := mcp.NewServer(ts, "test-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "meshant_articulate",
			"arguments": map[string]interface{}{
				"observer": "alice",
			},
		},
	})

	responses := runMCP(t, srv, msgs)

	// Expect 2 responses: one for initialize, one for tools/call.
	// notifications/initialized produces no response.
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d: %v", len(responses), responses)
	}
	toolResp := responses[1]

	// Extract the result content.
	result, ok := toolResp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result object, got: %v", toolResp)
	}
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("want content array, got: %v", result)
	}
	item, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("want content item object, got: %v", content[0])
	}
	text, ok := item["text"].(string)
	if !ok {
		t.Fatalf("want text string in content item, got: %v", item)
	}

	// Unmarshal the envelope from the text field.
	var env graph.Envelope
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	// Verify cut metadata.
	if env.Cut.Analyst != "test-analyst" {
		t.Errorf("Cut.Analyst: want %q, got %q", "test-analyst", env.Cut.Analyst)
	}
	if env.Cut.Observer != "alice" {
		t.Errorf("Cut.Observer: want %q, got %q", "alice", env.Cut.Observer)
	}

	// Fidelity: compare Envelope.Data to direct graph.Articulate result.
	//
	// Strategy: marshal both envelopes to JSON, then unmarshal into
	// map[string]interface{} for comparison. This normalizes field ordering
	// differences (struct tags vs map key ordering) and focuses comparison
	// on the values that matter, not serialization order. Both paths go
	// through the same normalization so neither has an advantage.
	//
	// Timing note: handleArticulate calls recordInvocation *after* building
	// the envelope but *before* returning. recordInvocation writes an
	// "mcp-invocation" trace back to the same JSONFileStore. This does NOT
	// affect the fidelity comparison because directG is built from the same
	// three seed traces that were in the store *before* Run was called.
	// Both sides operate on the same substrate snapshot.
	directG := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{"alice"},
	})
	directMeta := graph.CutMetaFromGraph(directG)
	directMeta.Analyst = "test-analyst"
	directEnv := graph.Envelope{Cut: directMeta, Data: directG}

	// Normalize MCP envelope through map round-trip.
	mcpJSON, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal MCP envelope: %v", err)
	}
	var mcpNorm map[string]interface{}
	if err := json.Unmarshal(mcpJSON, &mcpNorm); err != nil {
		t.Fatalf("normalize MCP envelope: %v", err)
	}
	mcpFinal, err := json.Marshal(mcpNorm)
	if err != nil {
		t.Fatalf("re-marshal MCP envelope: %v", err)
	}

	// Normalize direct envelope through map round-trip.
	directJSON, err := json.Marshal(directEnv)
	if err != nil {
		t.Fatalf("marshal direct envelope: %v", err)
	}
	var directNorm map[string]interface{}
	if err := json.Unmarshal(directJSON, &directNorm); err != nil {
		t.Fatalf("normalize direct envelope: %v", err)
	}
	directFinal, err := json.Marshal(directNorm)
	if err != nil {
		t.Fatalf("re-marshal direct envelope: %v", err)
	}

	if string(mcpFinal) != string(directFinal) {
		t.Errorf("fidelity mismatch:\nMCP:    %s\nDirect: %s", mcpFinal, directFinal)
	}
}

// --- TestMCPServer_UnknownMethod_Returns32601 ---
//
// JSON-RPC 2.0: unknown method must return error code -32601.
func TestMCPServer_UnknownMethod_Returns32601(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "nonexistent/method",
		"params":  map[string]interface{}{},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	resp := responses[1]

	rpcErr, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("want error field, got: %v", resp)
	}
	code, ok := rpcErr["code"].(float64)
	if !ok {
		t.Fatalf("want numeric code, got: %v", rpcErr["code"])
	}
	if int(code) != -32601 {
		t.Errorf("want code -32601, got %d", int(code))
	}
}

// --- TestMCPServer_ToolsList_ContainsArticulate ---
//
// tools/list must return at least meshant_articulate in the tools array.
func TestMCPServer_ToolsList_ContainsArticulate(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	resp := responses[1]

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result, got: %v", resp)
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("want tools array, got: %v", result["tools"])
	}

	found := false
	for _, toolRaw := range tools {
		tool, ok := toolRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if tool["name"] == "meshant_articulate" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tools/list: meshant_articulate not found in tools: %v", tools)
	}
}

// --- TestMCPServer_Articulate_MissingObserver_ReturnsError ---
//
// meshant_articulate without observer must return an error result,
// not a JSON-RPC error code — the tool was called successfully but
// the argument was invalid.
func TestMCPServer_Articulate_MissingObserver_ReturnsError(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "meshant_articulate",
			"arguments": map[string]interface{}{},
		},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	resp := responses[1]

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result (not error), got: %v", resp)
	}
	// isError flag must be true.
	isErr, ok := result["isError"].(bool)
	if !ok || !isErr {
		t.Errorf("want isError=true in result, got: %v", result)
	}
	// Content should mention "observer is required".
	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		t.Fatalf("want content, got none")
	}
	item, _ := content[0].(map[string]interface{})
	text, _ := item["text"].(string)
	if !strings.Contains(text, "observer is required") {
		t.Errorf("error text should contain 'observer is required', got: %q", text)
	}
}

// --- TestMCPServer_Articulate_WithTimeWindow ---
//
// meshant_articulate with from/to restricts to traces within the window.
func TestMCPServer_Articulate_WithTimeWindow(t *testing.T) {
	early := baseTime.Add(-2 * time.Hour)
	late := baseTime.Add(2 * time.Hour)

	traces := []schema.Trace{
		{
			ID:          "00000000-0000-4000-8000-000000000001",
			Timestamp:   early,
			WhatChanged: "early event",
			Observer:    "alice",
		},
		{
			ID:          "00000000-0000-4000-8000-000000000002",
			Timestamp:   late,
			WhatChanged: "late event",
			Observer:    "alice",
		},
	}
	ts := testStore(t, traces)
	defer ts.Close()

	srv := mcp.NewServer(ts, "analyst-x")

	// from/to window that includes only the early trace.
	fromStr := early.UTC().Format(time.RFC3339)
	toStr := baseTime.UTC().Format(time.RFC3339)

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "meshant_articulate",
			"arguments": map[string]interface{}{
				"observer": "alice",
				"from":     fromStr,
				"to":       toStr,
			},
		},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}

	result, ok := responses[1]["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result: %v", responses[1])
	}
	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		t.Fatalf("want content")
	}
	item, _ := content[0].(map[string]interface{})
	text, _ := item["text"].(string)

	var env graph.Envelope
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	// Only the early trace should be included.
	if env.Cut.TraceCount != 1 {
		t.Errorf("want trace_count=1, got %d", env.Cut.TraceCount)
	}
}

// --- TestMCPServer_Notification_NoResponse ---
//
// notifications/initialized (and initialized) are notifications: no response
// line should be produced. The server must not write a response for them.
func TestMCPServer_Notification_NoResponse(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	// Send only a notification — no request.
	msgs := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
			// No "id" field — this is a notification.
		},
	}

	var buf bytes.Buffer
	for _, msg := range msgs {
		b, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}

	var out bytes.Buffer
	if err := srv.Run(context.Background(), &buf, &out); err != nil {
		t.Fatalf("server error: %v", err)
	}

	// No output should have been written.
	if out.Len() != 0 {
		t.Errorf("notification must produce no response, got: %q", out.String())
	}
}

// --- TestMCPServer_MalformedJSON_Returns32700 ---
//
// Malformed JSON on the wire must return parse error code -32700.
func TestMCPServer_MalformedJSON_Returns32700(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	// Send a malformed JSON line.
	var buf bytes.Buffer
	buf.WriteString("{this is not json}\n")

	var out bytes.Buffer
	if err := srv.Run(context.Background(), &buf, &out); err != nil {
		t.Fatalf("server error: %v", err)
	}

	outStr := strings.TrimSpace(out.String())
	if outStr == "" {
		t.Fatal("expected a response for malformed JSON, got empty output")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(outStr), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	rpcErr, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("want error field: %v", resp)
	}
	code, _ := rpcErr["code"].(float64)
	if int(code) != -32700 {
		t.Errorf("want code -32700, got %d", int(code))
	}
}

// --- TestMCPServer_ToolsCall_InvalidParams ---
//
// tools/call with invalid JSON params (not an object with "name" key) must
// return a JSON-RPC -32602 invalid params error.
func TestMCPServer_ToolsCall_InvalidParams(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params":  "not-an-object",
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	resp := responses[1]
	rpcErr, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("want error field: %v", resp)
	}
	code, _ := rpcErr["code"].(float64)
	if int(code) != -32602 {
		t.Errorf("want code -32602, got %d", int(code))
	}
}

// --- TestMCPServer_ToolsCall_UnknownTool ---
//
// tools/call with an unrecognized tool name must return -32601.
func TestMCPServer_ToolsCall_UnknownTool(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "no_such_tool",
			"arguments": map[string]interface{}{},
		},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	resp := responses[1]
	rpcErr, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("want error field: %v", resp)
	}
	code, _ := rpcErr["code"].(float64)
	if int(code) != -32601 {
		t.Errorf("want code -32601 (tool not found), got %d", int(code))
	}
}

// --- TestMCPServer_Initialize_ReturnsServerInfo ---
//
// initialize must return serverInfo with name="meshant" and a non-empty version.
func TestMCPServer_Initialize_ReturnsServerInfo(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	msgs := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]interface{}{},
		},
	}

	responses := runMCP(t, srv, msgs)
	if len(responses) != 1 {
		t.Fatalf("want 1 response, got %d", len(responses))
	}
	result, ok := responses[0]["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result: %v", responses[0])
	}
	info, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("want serverInfo: %v", result)
	}
	if info["name"] != "meshant" {
		t.Errorf("serverInfo.name: want 'meshant', got %v", info["name"])
	}
}

// --- TestMCPServer_Articulate_InvalidFrom ---
//
// meshant_articulate with an invalid RFC3339 from value must return an
// isError=true result describing the parse failure.
func TestMCPServer_Articulate_InvalidFrom(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "meshant_articulate",
			"arguments": map[string]interface{}{
				"observer": "alice",
				"from":     "not-a-timestamp",
			},
		},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	result, ok := responses[1]["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result: %v", responses[1])
	}
	isErr, _ := result["isError"].(bool)
	if !isErr {
		t.Errorf("want isError=true for invalid from, got: %v", result)
	}
}

// --- TestMCPServer_Articulate_RecordsInvocationTrace ---
//
// D5 (mcp-v1.md): every cut-producing tool call must write a reflexive
// invocation trace. After calling meshant_articulate, the store must contain
// a trace tagged "mcp-invocation" with the correct observer.
//
// Implementation note: the fidelity test (TestMCPServer_Articulate_Fidelity)
// uses a JSONFileStore backed by a temp file. recordInvocation writes back to
// that same file atomically. This test queries the store after Run returns to
// confirm the side effect landed.
func TestMCPServer_Articulate_RecordsInvocationTrace(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-4000-8000-000000000010", "X changed Y", "alice"),
	}
	ts := testStore(t, traces)
	defer ts.Close()

	srv := mcp.NewServer(ts, "recorder-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "meshant_articulate",
			"arguments": map[string]interface{}{
				"observer": "alice",
			},
		},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}

	// Confirm the tool call succeeded (no isError).
	result, ok := responses[1]["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result: %v", responses[1])
	}
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("meshant_articulate returned isError=true: %v", result)
	}

	// Query the store for traces tagged "mcp-invocation".
	// The invocation trace is written after articulation, before Run returns.
	ctx := context.Background()
	all, err := ts.Query(ctx, store.QueryOpts{Tags: []string{"mcp-invocation"}})
	if err != nil {
		t.Fatalf("query store: %v", err)
	}
	if len(all) == 0 {
		t.Fatalf("want at least 1 mcp-invocation trace, got 0")
	}

	inv := all[0]
	// Observer must match the tool call argument.
	if inv.Observer != "alice" {
		t.Errorf("invocation trace observer: want %q, got %q", "alice", inv.Observer)
	}
	// Must carry the tool name tag.
	foundToolTag := false
	for _, tag := range inv.Tags {
		if tag == "meshant_articulate" {
			foundToolTag = true
			break
		}
	}
	if !foundToolTag {
		t.Errorf("invocation trace missing tool-name tag %q: tags=%v", "meshant_articulate", inv.Tags)
	}
}

// --- TestMCPServer_InitializedRequest_NoError ---
//
// Some MCP clients send "notifications/initialized" or "initialized" as a
// request (with an id field) rather than a notification. The server must
// respond with an empty result, not a -32601 error.
func TestMCPServer_InitializedRequest_NoError(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	msgs := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]interface{}{},
		},
		{
			// "notifications/initialized" sent as a request (has an id).
			// Non-conformant but some clients do this.
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "notifications/initialized",
		},
	}

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	// Second response must be a result, not an error.
	resp := responses[1]
	if _, hasErr := resp["error"]; hasErr {
		t.Errorf("notifications/initialized as request must not return error, got: %v", resp)
	}
	if _, hasResult := resp["result"]; !hasResult {
		t.Errorf("notifications/initialized as request must return result, got: %v", resp)
	}
}

// --- TestMCPServer_Articulate_TagsFilter ---
//
// meshant_articulate with a tags filter must restrict the graph to traces that
// carry at least one matching tag. An implementation that silently drops the
// tags argument would return TraceCount == 2 instead of 1.
func TestMCPServer_Articulate_TagsFilter(t *testing.T) {
	traces := []schema.Trace{
		{
			ID:          "00000000-0000-4000-8000-000000000021",
			Timestamp:   baseTime,
			WhatChanged: "tagged event",
			Observer:    "alice",
			Tags:        []string{"tag-a"},
		},
		{
			ID:          "00000000-0000-4000-8000-000000000022",
			Timestamp:   baseTime,
			WhatChanged: "untagged event",
			Observer:    "alice",
		},
	}
	ts := testStore(t, traces)
	defer ts.Close()

	srv := mcp.NewServer(ts, "test-analyst")

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "meshant_articulate",
			"arguments": map[string]interface{}{
				"observer": "alice",
				"tags":     []interface{}{"tag-a"},
			},
		},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}

	result, ok := responses[1]["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result: %v", responses[1])
	}
	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		t.Fatalf("want content")
	}
	item, _ := content[0].(map[string]interface{})
	text, _ := item["text"].(string)

	var env graph.Envelope
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	// Only the "tag-a" trace should be included.
	if env.Cut.TraceCount != 1 {
		t.Errorf("want trace_count=1 (only tagged trace), got %d", env.Cut.TraceCount)
	}
}

// --- TestMCPServer_Articulate_InvertedTimeWindow ---
//
// meshant_articulate with from > to (inverted window) must return isError=true.
// parseTimeWindow calls tw.Validate() when both bounds are set; this test
// exercises that error branch.
func TestMCPServer_Articulate_InvertedTimeWindow(t *testing.T) {
	ts := testStore(t, nil)
	defer ts.Close()
	srv := mcp.NewServer(ts, "test-analyst")

	// from is later than to — invalid window.
	fromStr := baseTime.Add(2 * time.Hour).UTC().Format(time.RFC3339)
	toStr := baseTime.UTC().Format(time.RFC3339)

	msgs := append(initMessages(1), map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "meshant_articulate",
			"arguments": map[string]interface{}{
				"observer": "alice",
				"from":     fromStr,
				"to":       toStr,
			},
		},
	})

	responses := runMCP(t, srv, msgs)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}

	result, ok := responses[1]["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("want result (not protocol error): %v", responses[1])
	}
	isErr, _ := result["isError"].(bool)
	if !isErr {
		t.Errorf("want isError=true for inverted time window (from > to), got: %v", result)
	}
}
