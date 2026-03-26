// tools.go registers MCP tool handlers for the meshant analytical engine.
//
// Batch 1 (issues #176 + #177): meshant_articulate, meshant_shadow,
// meshant_follow, meshant_bottleneck, meshant_summarize, meshant_validate.
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

// Input validation limits — applied in every handler before use.
// These protect against storage pollution and memory amplification from
// a crafted MCP client sending pathological parameter values.
const (
	// maxObserverLen caps the observer and element name strings. No legitimate
	// ANT actant name approaches this length.
	maxObserverLen = 500
	// maxTagLen caps individual tag strings.
	maxTagLen = 200
	// maxTagCount caps the number of tags per request.
	maxTagCount = 50
	// maxFollowDepth caps the MaxDepth parameter for meshant_follow.
	// Zero means "unlimited" (the documented default); any positive value is
	// bounded here. A realistic trace substrate has far fewer than 1000 steps.
	maxFollowDepth = 1000
)

// validateObserver returns an error if the observer value is empty or too long.
func validateObserver(obs string) error {
	if obs == "" {
		return fmt.Errorf("observer is required — every cut is a positioned reading")
	}
	if len(obs) > maxObserverLen {
		return fmt.Errorf("observer exceeds maximum length %d", maxObserverLen)
	}
	return nil
}

// validateTags returns an error if any tag is too long or the slice is too large.
func validateTags(tags []string) error {
	if len(tags) > maxTagCount {
		return fmt.Errorf("too many tags: %d exceeds maximum %d", len(tags), maxTagCount)
	}
	for _, tag := range tags {
		if len(tag) > maxTagLen {
			return fmt.Errorf("tag %q exceeds maximum length %d", tag, maxTagLen)
		}
	}
	return nil
}

// filterByTagsOR returns traces that carry at least one of the requested tags
// (OR semantics). If filter is empty, all traces are returned unchanged.
// This matches the semantics described in the meshant_validate tool schema and
// the graph.ArticulationOptions.Tags field — distinct from QueryOpts.Tags which
// uses AND semantics.
func filterByTagsOR(traces []schema.Trace, filter []string) []schema.Trace {
	if len(filter) == 0 {
		return traces
	}
	want := make(map[string]bool, len(filter))
	for _, tag := range filter {
		want[tag] = true
	}
	out := traces[:0:0] // reuse backing array type but start empty
	for _, t := range traces {
		for _, tag := range t.Tags {
			if want[tag] {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

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
					Items:       &property{Type: "string"},
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

	// Step 1: observer is required and must be within bounds.
	if err := validateObserver(args.Observer); err != nil {
		return nil, err
	}
	if err := validateTags(args.Tags); err != nil {
		return nil, fmt.Errorf("meshant_articulate: %w", err)
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

// =============================================================================
// Batch 1 — #177: shadow, follow, bottleneck, summarize, validate
// =============================================================================

// --- meshant_shadow ---

// shadowArgs is the input shape for meshant_shadow.
type shadowArgs struct {
	Observer string   `json:"observer"`
	From     string   `json:"from,omitempty"`
	To       string   `json:"to,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// registerShadow registers the meshant_shadow tool on the server.
func (s *Server) registerShadow() {
	sc := toolSchema{
		Name: "meshant_shadow",
		Description: "Return the shadow elements from a positioned articulation — " +
			"elements visible from other observer positions but not from this one. " +
			"Every cut names what it excludes; the shadow is that name. " +
			"Returns a graph.Envelope with CutMeta and a ShadowSummary.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"observer": {
					Type:        "string",
					Description: "The ANT observer position. Required — shadow is always positioned.",
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
					Description: "Tag filter: include only traces carrying at least one of these tags. Optional.",
					Items:       &property{Type: "string"},
				},
			},
			Required: []string{"observer"},
		},
	}
	s.registerTool(sc, s.handleShadow)
}

// handleShadow is the tool handler for meshant_shadow.
func (s *Server) handleShadow(ctx context.Context, rawParams json.RawMessage) (interface{}, error) {
	var args shadowArgs
	if err := json.Unmarshal(rawParams, &args); err != nil {
		return nil, fmt.Errorf("meshant_shadow: invalid params: %w", err)
	}
	if err := validateObserver(args.Observer); err != nil {
		return nil, err
	}
	if err := validateTags(args.Tags); err != nil {
		return nil, fmt.Errorf("meshant_shadow: %w", err)
	}
	tw, err := parseTimeWindow(args.From, args.To)
	if err != nil {
		return nil, fmt.Errorf("meshant_shadow: %w", err)
	}
	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		return nil, fmt.Errorf("meshant_shadow: query store: %w", err)
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{args.Observer},
		TimeWindow:        tw,
		Tags:              args.Tags,
	})
	meta := graph.CutMetaFromGraph(g)
	meta.Analyst = s.analyst
	shadow := graph.SummariseShadow(g)
	env := graph.Envelope{Cut: meta, Data: shadow}
	s.recordInvocation(ctx, "meshant_shadow", args.Observer)
	return env, nil
}

// --- meshant_follow ---

// followArgs is the input shape for meshant_follow.
type followArgs struct {
	Observer  string   `json:"observer"`
	Element   string   `json:"element"`
	Direction string   `json:"direction,omitempty"`
	MaxDepth  int      `json:"max_depth,omitempty"`
	From      string   `json:"from,omitempty"`
	To        string   `json:"to,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// registerFollow registers the meshant_follow tool on the server.
func (s *Server) registerFollow() {
	sc := toolSchema{
		Name: "meshant_follow",
		Description: "Follow a translation chain through the positioned graph from a named element. " +
			"Each step is classified as intermediary-like, mediator-like, or translation. " +
			"The chain is itself a cut — the analyst declares where to start and which direction to follow. " +
			"Returns a graph.Envelope with CutMeta and a ClassifiedChain.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"observer": {
					Type:        "string",
					Description: "The ANT observer position. Required — every chain traversal is a positioned reading.",
				},
				"element": {
					Type:        "string",
					Description: "The starting actant name. Required.",
				},
				"direction": {
					Type:        "string",
					Description: `Traversal direction: "forward" (source→target) or "backward" (target→source). Defaults to "forward".`,
				},
				"max_depth": {
					Type:        "integer",
					Description: "Maximum traversal steps. 0 means unlimited. Optional.",
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
					Description: "Tag filter: include only traces carrying at least one of these tags. Optional.",
					Items:       &property{Type: "string"},
				},
			},
			Required: []string{"observer", "element"},
		},
	}
	s.registerTool(sc, s.handleFollow)
}

// handleFollow is the tool handler for meshant_follow.
func (s *Server) handleFollow(ctx context.Context, rawParams json.RawMessage) (interface{}, error) {
	var args followArgs
	if err := json.Unmarshal(rawParams, &args); err != nil {
		return nil, fmt.Errorf("meshant_follow: invalid params: %w", err)
	}
	if err := validateObserver(args.Observer); err != nil {
		return nil, err
	}
	if args.Element == "" {
		return nil, fmt.Errorf("element is required — a chain must start somewhere")
	}
	if len(args.Element) > maxObserverLen {
		return nil, fmt.Errorf("meshant_follow: element exceeds maximum length %d", maxObserverLen)
	}
	if err := validateTags(args.Tags); err != nil {
		return nil, fmt.Errorf("meshant_follow: %w", err)
	}
	if args.MaxDepth < 0 {
		return nil, fmt.Errorf("meshant_follow: max_depth must be >= 0")
	}
	if args.MaxDepth > maxFollowDepth {
		return nil, fmt.Errorf("meshant_follow: max_depth %d exceeds maximum %d", args.MaxDepth, maxFollowDepth)
	}

	// Validate and map direction. Default to forward.
	var dir graph.Direction
	switch args.Direction {
	case "", "forward":
		dir = graph.DirectionForward
	case "backward":
		dir = graph.DirectionBackward
	default:
		return nil, fmt.Errorf("meshant_follow: direction must be %q or %q, got %q",
			"forward", "backward", args.Direction)
	}

	tw, err := parseTimeWindow(args.From, args.To)
	if err != nil {
		return nil, fmt.Errorf("meshant_follow: %w", err)
	}
	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		return nil, fmt.Errorf("meshant_follow: query store: %w", err)
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{args.Observer},
		TimeWindow:        tw,
		Tags:              args.Tags,
	})
	chain := graph.FollowTranslation(g, args.Element, graph.FollowOptions{
		Direction: dir,
		MaxDepth:  args.MaxDepth,
	})
	classified := graph.ClassifyChain(chain, graph.ClassifyOptions{})
	meta := graph.CutMetaFromGraph(g)
	meta.Analyst = s.analyst
	env := graph.Envelope{Cut: meta, Data: classified}
	s.recordInvocation(ctx, "meshant_follow", args.Observer)
	return env, nil
}

// --- meshant_bottleneck ---

// bottleneckArgs is the input shape for meshant_bottleneck.
type bottleneckArgs struct {
	Observer string   `json:"observer"`
	From     string   `json:"from,omitempty"`
	To       string   `json:"to,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// registerBottleneck registers the meshant_bottleneck tool on the server.
func (s *Server) registerBottleneck() {
	sc := toolSchema{
		Name: "meshant_bottleneck",
		Description: "Identify bottleneck actants — elements with high appearance count, " +
			"mediation count, or shadow count in the positioned graph. " +
			"These are provisional readings from one cut; a different observer position " +
			"would produce different notes. " +
			"Returns a graph.Envelope with CutMeta and a []BottleneckNote.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"observer": {
					Type:        "string",
					Description: "The ANT observer position. Required.",
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
					Description: "Tag filter: include only traces carrying at least one of these tags. Optional.",
					Items:       &property{Type: "string"},
				},
			},
			Required: []string{"observer"},
		},
	}
	s.registerTool(sc, s.handleBottleneck)
}

// handleBottleneck is the tool handler for meshant_bottleneck.
func (s *Server) handleBottleneck(ctx context.Context, rawParams json.RawMessage) (interface{}, error) {
	var args bottleneckArgs
	if err := json.Unmarshal(rawParams, &args); err != nil {
		return nil, fmt.Errorf("meshant_bottleneck: invalid params: %w", err)
	}
	if err := validateObserver(args.Observer); err != nil {
		return nil, err
	}
	if err := validateTags(args.Tags); err != nil {
		return nil, fmt.Errorf("meshant_bottleneck: %w", err)
	}
	tw, err := parseTimeWindow(args.From, args.To)
	if err != nil {
		return nil, fmt.Errorf("meshant_bottleneck: %w", err)
	}
	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		return nil, fmt.Errorf("meshant_bottleneck: query store: %w", err)
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{args.Observer},
		TimeWindow:        tw,
		Tags:              args.Tags,
	})
	notes := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})
	meta := graph.CutMetaFromGraph(g)
	meta.Analyst = s.analyst
	env := graph.Envelope{Cut: meta, Data: notes}
	s.recordInvocation(ctx, "meshant_bottleneck", args.Observer)
	return env, nil
}

// --- meshant_summarize ---

// summarizeArgs is the input shape for meshant_summarize.
type summarizeArgs struct {
	Observer string   `json:"observer"`
	From     string   `json:"from,omitempty"`
	To       string   `json:"to,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// registerSummarize registers the meshant_summarize tool on the server.
func (s *Server) registerSummarize() {
	sc := toolSchema{
		Name: "meshant_summarize",
		Description: "Return a provisional narrative summary of the positioned graph — " +
			"actant count, trace count, shadow count, top elements by appearance, " +
			"mediations observed. This is a positioned reading, not a complete account. " +
			"Returns a graph.Envelope with CutMeta and a NarrativeDraft.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"observer": {
					Type:        "string",
					Description: "The ANT observer position. Required — summaries are always positioned.",
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
					Description: "Tag filter: include only traces carrying at least one of these tags. Optional.",
					Items:       &property{Type: "string"},
				},
			},
			Required: []string{"observer"},
		},
	}
	s.registerTool(sc, s.handleSummarize)
}

// handleSummarize is the tool handler for meshant_summarize.
func (s *Server) handleSummarize(ctx context.Context, rawParams json.RawMessage) (interface{}, error) {
	var args summarizeArgs
	if err := json.Unmarshal(rawParams, &args); err != nil {
		return nil, fmt.Errorf("meshant_summarize: invalid params: %w", err)
	}
	if err := validateObserver(args.Observer); err != nil {
		return nil, err
	}
	if err := validateTags(args.Tags); err != nil {
		return nil, fmt.Errorf("meshant_summarize: %w", err)
	}
	tw, err := parseTimeWindow(args.From, args.To)
	if err != nil {
		return nil, fmt.Errorf("meshant_summarize: %w", err)
	}
	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		return nil, fmt.Errorf("meshant_summarize: query store: %w", err)
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{args.Observer},
		TimeWindow:        tw,
		Tags:              args.Tags,
	})
	narrative := graph.DraftNarrative(g)
	meta := graph.CutMetaFromGraph(g)
	meta.Analyst = s.analyst
	env := graph.Envelope{Cut: meta, Data: narrative}
	s.recordInvocation(ctx, "meshant_summarize", args.Observer)
	return env, nil
}

// --- meshant_validate ---

// validateArgs is the input shape for meshant_validate.
// No observer is required — validate is not a cut-producing operation (D5 exemption).
type validateArgs struct {
	Tags []string `json:"tags,omitempty"`
}

// validateResult is the result shape for meshant_validate.
type validateResult struct {
	TotalTraces  int             `json:"total_traces"`
	ValidCount   int             `json:"valid_count"`
	InvalidCount int             `json:"invalid_count"`
	Errors       []validateError `json:"errors"`
}

// validateError records a single validation failure.
type validateError struct {
	TraceID string `json:"trace_id"`
	Error   string `json:"error"`
}

// registerValidate registers the meshant_validate tool on the server.
func (s *Server) registerValidate() {
	sc := toolSchema{
		Name: "meshant_validate",
		Description: "Validate traces in the substrate against schema.Validate. " +
			"Returns counts of valid and invalid traces and any validation errors found. " +
			"This is not a cut-producing operation — no observer position is taken " +
			"and no invocation trace is recorded (D5 exemption in mcp-v1.md).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"tags": {
					Type:        "array",
					Description: "Optionally restrict validation to traces carrying at least one of these tags. Optional.",
					Items:       &property{Type: "string"},
				},
			},
		},
	}
	s.registerTool(sc, s.handleValidate)
}

// handleValidate is the tool handler for meshant_validate.
//
// D5 exemption (mcp-v1.md): validate is not a cut-producing operation.
// No observer is required and no invocation trace is written.
//
// Tag filtering uses OR semantics ("at least one of these tags"), consistent
// with graph.Articulate and the tool schema description. The store's QueryOpts.Tags
// uses AND semantics, so we query the full substrate and filter in-memory.
func (s *Server) handleValidate(ctx context.Context, rawParams json.RawMessage) (interface{}, error) {
	var args validateArgs
	if err := json.Unmarshal(rawParams, &args); err != nil {
		return nil, fmt.Errorf("meshant_validate: invalid params: %w", err)
	}
	if err := validateTags(args.Tags); err != nil {
		return nil, fmt.Errorf("meshant_validate: %w", err)
	}
	// Query the full substrate — tag filtering happens in-memory below (OR semantics).
	all, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		return nil, fmt.Errorf("meshant_validate: query store: %w", err)
	}
	// Apply OR tag filter: keep traces that carry at least one of the requested tags.
	traces := filterByTagsOR(all, args.Tags)
	result := validateResult{
		TotalTraces: len(traces),
		Errors:      []validateError{},
	}
	for _, t := range traces {
		if err := t.Validate(); err != nil {
			result.InvalidCount++
			result.Errors = append(result.Errors, validateError{
				TraceID: t.ID,
				Error:   err.Error(),
			})
		} else {
			result.ValidCount++
		}
	}
	// No recordInvocation — D5 exemption: validate is not a cut-producing operation.
	return result, nil
}
