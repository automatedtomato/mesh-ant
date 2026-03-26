// Package mcp implements a Model Context Protocol (MCP) server for MeshAnt.
//
// The server exposes MeshAnt's analytical engine as MCP tools callable by any
// compliant LLM client (Claude Code, Cursor, Cline) without shell invocation.
//
// # Design
//
// Transport: stdio (one client per process). SSE is deferred — see D7 in
// docs/decisions/mcp-v1.md for the rationale.
//
// Protocol: JSON-RPC 2.0 over newline-delimited messages. No external SDK —
// the protocol surface is small enough to implement inline.
//
// Observer discipline: every tool requires an observer parameter. Analyst
// identity is injected at construction (--analyst flag) and stamped on every
// CutMeta response. Observer (ANT position) and Analyst (who is reading) are
// kept distinct per D1 in mcp-v1.md.
//
// Reflexivity (Principle 8): every cut-producing tool call writes a trace back
// to the TraceStore. The MCP server is a mediator — its actions must be visible
// in the mesh, not hidden behind a service facade.
//
// ANT tensions documented in mcp-v1.md: T171.1–T171.5.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// rpcRequest is an inbound JSON-RPC 2.0 message.
// id is *json.RawMessage so it can be null (notification) or any JSON value.
// A nil id field means the message is a notification — no response is sent.
type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// rpcResponse is an outbound JSON-RPC 2.0 message.
// Result and Error are mutually exclusive.
type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

// rpcError carries a JSON-RPC error code and message.
// Standard codes:
//
//	-32700 Parse error
//	-32601 Method not found
//	-32602 Invalid params
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolHandler is the function signature for an MCP tool handler.
// It receives the raw params JSON and a context, and returns either a result
// value or an error. A non-nil error is returned to the client as a tool-level
// error (isError=true in the content), not a JSON-RPC protocol error.
type ToolHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)

// Server is the MeshAnt MCP server.
// It listens on stdio, dispatches JSON-RPC requests to tool handlers, and
// stamps every cut response with the injected analyst identity.
//
// Create with NewServer; start with Run.
type Server struct {
	ts      store.TraceStore
	analyst string
	tools   map[string]ToolHandler
	schemas []toolSchema
}

// toolSchema holds the MCP tool descriptor returned by tools/list.
type toolSchema struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

// inputSchema is a minimal JSON Schema descriptor for tool inputs.
type inputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// property is a single JSON Schema property descriptor.
// Items is used for array properties to describe the element type
// (e.g., Items: &property{Type: "string"} for a []string field).
type property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Items       *property `json:"items,omitempty"`
}

// NewServer constructs an MCP server backed by ts with the given analyst
// identity. The analyst value is set on every CutMeta response — it is the
// declared reader position for the entire server lifetime.
//
// Tools are registered in this constructor. Batch 1 (issues #176 + #177)
// registers six tools: articulate, shadow, follow, bottleneck, summarize,
// validate. Batch 2 (issue #178) adds diff and gaps (dual-observer).
func NewServer(ts store.TraceStore, analyst string) *Server {
	s := &Server{
		ts:      ts,
		analyst: analyst,
		tools:   make(map[string]ToolHandler),
	}
	// Register batch-1 tools (#176 + #177).
	s.registerArticulate()
	s.registerShadow()
	s.registerFollow()
	s.registerBottleneck()
	s.registerSummarize()
	s.registerValidate()
	// Register batch-2 tools (#178).
	s.registerDiff()
	s.registerGaps()
	return s
}

// registerTool adds a named handler and its schema to the server.
// Called during construction; not safe for concurrent use after Run begins.
func (s *Server) registerTool(schema toolSchema, handler ToolHandler) {
	s.tools[schema.Name] = handler
	s.schemas = append(s.schemas, schema)
}

// Run reads newline-delimited JSON-RPC messages from in, dispatches them, and
// writes responses to out. It returns when in reaches EOF or the context is
// cancelled.
//
// Each message is processed synchronously. Run is single-threaded by design —
// stdio transport requires no concurrency (D7 in mcp-v1.md).
func (s *Server) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	enc := json.NewEncoder(out)
	scanner := bufio.NewScanner(in)
	// Expand the scanner buffer to 4 MiB. The default 64 KiB limit is a
	// time bomb for large graph payloads: a single tools/call response from
	// a dense substrate can easily exceed it, causing the scanner to return
	// bufio.ErrTooLong and the server to exit mid-session.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Check context cancellation between messages.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Malformed JSON — return parse error (-32700).
			// ID is unknown; use null per JSON-RPC spec.
			resp := rpcResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			}
			if encErr := enc.Encode(resp); encErr != nil {
				return fmt.Errorf("mcp: encode parse error response: %w", encErr)
			}
			continue
		}

		// Notifications have no id — do not produce a response.
		if req.ID == nil {
			continue
		}

		resp := s.dispatch(ctx, req)
		if encErr := enc.Encode(resp); encErr != nil {
			return fmt.Errorf("mcp: encode response: %w", encErr)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("mcp: read: %w", err)
	}
	return nil
}

// dispatch routes a request to the appropriate handler and builds the response.
func (s *Server) dispatch(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized", "notifications/initialized":
		// Both "initialized" and "notifications/initialized" may arrive as
		// requests (with an id field) from non-conformant clients. The canonical
		// form is a notification (no id), handled above by the nil-id guard.
		// When they do arrive with an id, treat them as no-ops and respond with
		// an empty result rather than a -32601 error, which would confuse clients.
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %q", req.Method)},
		}
	}
}

// handleInitialize responds to the MCP initialize handshake.
// Returns server info and an empty capabilities object.
func (s *Server) handleInitialize(req rpcRequest) rpcResponse {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "meshant",
			"version": "4.0.0",
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}
	return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

// handleToolsList returns the list of registered tool schemas.
func (s *Server) handleToolsList(req rpcRequest) rpcResponse {
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": s.schemas},
	}
}

// toolsCallParams is the params shape for a tools/call request.
type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// toolContent is a single item in a tool result content array.
type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// handleToolsCall dispatches a tools/call request to the registered handler.
// On success, returns a content array with one text item containing the
// JSON-serialised result. On handler error, returns isError=true.
func (s *Server) handleToolsCall(ctx context.Context, req rpcRequest) rpcResponse {
	var p toolsCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "invalid params: " + err.Error()},
		}
	}

	handler, ok := s.tools[p.Name]
	if !ok {
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("tool not found: %q", p.Name)},
		}
	}

	result, err := handler(ctx, p.Arguments)
	if err != nil {
		// Tool-level error: return as content with isError=true, not as a
		// JSON-RPC protocol error. This follows the MCP spec for tool errors.
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"isError": true,
				"content": []toolContent{{Type: "text", Text: err.Error()}},
			},
		}
	}

	// Marshal result to JSON; embed in text content item.
	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"isError": true,
				"content": []toolContent{{Type: "text", Text: "mcp: internal error: marshal result: " + marshalErr.Error()}},
			},
		}
	}

	return rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []toolContent{{Type: "text", Text: string(resultJSON)}},
		},
	}
}
