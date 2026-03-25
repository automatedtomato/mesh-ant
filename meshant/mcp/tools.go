// tools.go registers MCP tool handlers for the meshant analytical engine.
//
// Batch 1 (issue #176): meshant_articulate — build a positioned mesh graph.
// Batch 1 remaining (#177): shadow, follow, bottleneck, summarize, validate.
// Batch 2 (#178): diff, gaps (dual-observer).
//
// Every cut-producing tool:
//  1. Validates required parameters.
//  2. Queries the full substrate from the TraceStore (no pre-filtering —
//     cut logic lives in graph.Articulate, not in the store).
//  3. Calls the relevant graph.* function.
//  4. Builds a graph.Envelope with CutMeta.Analyst set to s.analyst.
//  5. Writes a reflexive invocation trace (Principle 8, D5 in mcp-v1.md).
//
// Reflexivity note (T171.2): heavy use accumulates invocation traces tagged
// "mcp-invocation". These are filterable by tag and are named, not hidden.
package mcp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// articulateArgs is the input shape for meshant_articulate.
type articulateArgs struct {
	// Observer is the ANT position from which to articulate the graph.
	// Required: every graph is a positioned reading. Empty string is rejected
	// with an explicit error naming the epistemic obligation (D1, mcp-v1.md).
	Observer string `json:"observer"`

	// From is the RFC3339 lower bound of the time window. Optional.
	From string `json:"from,omitempty"`

	// To is the RFC3339 upper bound of the time window. Optional.
	To string `json:"to,omitempty"`

	// Tags restricts the cut to traces carrying at least one matching tag
	// (OR semantics, consistent with graph.ArticulationOptions). Optional.
	Tags []string `json:"tags,omitempty"`
}

// registerArticulate registers the meshant_articulate tool on the server.
// Called from NewServer during construction.
func (s *Server) registerArticulate() {
	schema := toolSchema{
		Name: "meshant_articulate",
		Description: "Articulate a positioned mesh graph from the trace substrate. " +
			"Every graph is a cut from a named observer position — there is no god's-eye view. " +
			"Returns a graph.Envelope with cut metadata (observer, analyst, shadow elements) and " +
			"the full MeshGraph. The analyst field names who requested this reading.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"observer": {
					Type:        "string",
					Description: "The ANT observer position from which to articulate the graph. Required — every graph is a positioned reading.",
				},
				"from": {
					Type:        "string",
					Description: "RFC3339 lower bound of the time window (inclusive). Optional.",
				},
				"to": {
					Type:        "string",
					Description: "RFC3339 upper bound of the time window (inclusive). Optional.",
				},
				"tags": {
					Type:        "array",
					Description: "Tag filter: include only traces carrying at least one of these tags (OR semantics). Optional.",
				},
			},
			Required: []string{"observer"},
		},
	}
	s.registerTool(schema, s.handleArticulate)
}

// handleArticulate is the tool handler for meshant_articulate.
//
// Steps (per plan in mcp-v1.md):
//  1. Validate observer non-empty.
//  2. Parse from/to as RFC3339 → TimeWindow.
//  3. ts.Query(ctx, QueryOpts{}) — full substrate, no pre-filter.
//  4. graph.Articulate(traces, opts).
//  5. meta := graph.CutMetaFromGraph(g); meta.Analyst = s.analyst.
//  6. return graph.Envelope{Cut: meta, Data: g}.
//  7. Record reflexive invocation trace (D5, Principle 8).
func (s *Server) handleArticulate(ctx context.Context, rawParams json.RawMessage) (interface{}, error) {
	var args articulateArgs
	if err := json.Unmarshal(rawParams, &args); err != nil {
		return nil, fmt.Errorf("meshant_articulate: invalid params: %w", err)
	}

	// Step 1: observer is required — every graph is a positioned reading.
	if args.Observer == "" {
		return nil, fmt.Errorf("observer is required — every graph is a positioned reading")
	}

	// Step 2: parse optional time window.
	tw, err := parseTimeWindow(args.From, args.To)
	if err != nil {
		return nil, fmt.Errorf("meshant_articulate: %w", err)
	}

	// Step 3: query full substrate.
	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		return nil, fmt.Errorf("meshant_articulate: query store: %w", err)
	}

	// Step 4: articulate.
	opts := graph.ArticulationOptions{
		ObserverPositions: []string{args.Observer},
		TimeWindow:        tw,
		Tags:              args.Tags,
	}
	g := graph.Articulate(traces, opts)

	// Step 5: build envelope with analyst stamped.
	meta := graph.CutMetaFromGraph(g)
	meta.Analyst = s.analyst
	env := graph.Envelope{Cut: meta, Data: g}

	// Step 7: reflexive invocation trace (D5, Principle 8).
	// Failure is logged but does not abort the tool response — the analytical
	// result is always returned. The failure to record is itself an absence
	// worth naming; it is logged, not silenced.
	s.recordInvocation(ctx, "meshant_articulate", args.Observer)

	return env, nil
}

// newUUID4 generates a random version-4 UUID in lowercase hyphenated form.
// Panics if crypto/rand is unavailable — that is an unrecoverable environment failure.
// Mirrors graph.newUUID4 without the package-level export.
func newUUID4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("mcp.newUUID4: crypto/rand unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	)
}

// parseTimeWindow converts RFC3339 from/to strings into a graph.TimeWindow.
// Either end may be empty (half-open window). Returns an error if either
// non-empty value cannot be parsed as RFC3339.
func parseTimeWindow(fromStr, toStr string) (graph.TimeWindow, error) {
	var tw graph.TimeWindow
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return graph.TimeWindow{}, fmt.Errorf("invalid from value %q: expected RFC3339: %w", fromStr, err)
		}
		tw.Start = t
	}
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return graph.TimeWindow{}, fmt.Errorf("invalid to value %q: expected RFC3339: %w", toStr, err)
		}
		tw.End = t
	}
	if !tw.Start.IsZero() && !tw.End.IsZero() {
		if err := tw.Validate(); err != nil {
			return graph.TimeWindow{}, err
		}
	}
	return tw, nil
}

// recordInvocation writes a reflexive invocation trace to the TraceStore.
//
// D5 (mcp-v1.md): every cut-producing tool call writes a reflexive trace.
// Tags: ["mcp-invocation", toolName] — filterable so invocation traces do not
// obscure the original network structure during analysis (T171.2).
//
// Failure is logged (not returned) per the soft-fail policy in D5: the
// analytical result is always returned; the failure to record is named.
//
// Observer attribution (T1 tension — see mcp-v1.md): Observer is set to the
// tool-call's observer argument, not to a fixed "meshant-mcp" identity.
// This choice places the invocation trace within the same observer cut that
// caused it — articulating from Alice's position makes the MCP call visible
// from Alice's position. An alternative (Observer = "meshant-mcp") would
// place calls in a separate position, requiring a separate cut to see them.
func (s *Server) recordInvocation(ctx context.Context, toolName, observer string) {
	id := newUUID4()

	t := schema.Trace{
		ID:          id,
		Timestamp:   time.Now().UTC(),
		WhatChanged: fmt.Sprintf("MCP server mediated call to %q; analyst %q reading from observer position %q", toolName, s.analyst, observer),
		Observer:    observer,
		Tags:        []string{"mcp-invocation", toolName},
	}

	// Validate before storing — t.Validate() ensures required fields are
	// present so a bad invocation trace is caught here rather than silently
	// written as a malformed record into the substrate.
	if err := t.Validate(); err != nil {
		log.Printf("mcp: recordInvocation: validate trace: %v (tool=%s, observer=%s)", err, toolName, observer)
		return
	}

	if err := s.ts.Store(ctx, []schema.Trace{t}); err != nil {
		log.Printf("mcp: recordInvocation: store trace: %v (tool=%s, observer=%s)", err, toolName, observer)
	}
}
