// Package serve implements the meshant HTTP server.
//
// Every endpoint enforces the ANT constraint that no graph is returned without
// naming its observer position. The Envelope type makes the cut explicit in
// every response; the server returns 400 when the required observer is absent.
//
// The server holds a TraceStore and queries it per request (no caching). This
// is consistent with the loadTraces design decision (D1 in store-cli-v1.md):
// the full substrate is loaded and the analytical engine applies all cut logic.
//
// See docs/decisions/serve-v1.md for design decisions and ANT tensions.
package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// CutMeta carries the observer-position metadata that accompanies every response.
// Every graph is a positioned reading; this struct makes that position undeniable.
type CutMeta struct {
	// Observer is the position from which this cut was made.
	Observer string `json:"observer"`

	// From is the RFC3339 lower bound of the time window, or null when unbounded.
	From *string `json:"from"`

	// To is the RFC3339 upper bound of the time window, or null when unbounded.
	To *string `json:"to"`

	// Tags lists the tag filter used, or null when no tag filter was applied.
	Tags []string `json:"tags"`

	// TraceCount is the number of traces included in the cut.
	TraceCount int `json:"trace_count"`

	// ShadowCount is the number of shadow elements (for articulation-based cuts)
	// or the approximate count of non-observer traces (for /traces).
	// See ANT tension T2 in serve-v1.md for the /traces approximation.
	ShadowCount int `json:"shadow_count"`
}

// Envelope wraps every response with cut metadata.
// The server never returns data without naming its cut.
type Envelope struct {
	Cut  CutMeta     `json:"cut"`
	Data interface{} `json:"data"`
}

// ErrorBody is the JSON shape for error responses.
type ErrorBody struct {
	Error string `json:"error"`
}

// writeJSON writes v as indented JSON with the given HTTP status code.
// Sets Content-Type: application/json.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// writeError writes a JSON error response with the given status and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorBody{Error: msg})
}

// cutMetaFromGraph builds a CutMeta from an articulated MeshGraph.
// The observer is the first entry in g.Cut.ObserverPositions (empty on a full cut).
// From/To are RFC3339 strings when the time window is non-zero, null otherwise.
func cutMetaFromGraph(g graph.MeshGraph) CutMeta {
	observer := ""
	if len(g.Cut.ObserverPositions) > 0 {
		observer = g.Cut.ObserverPositions[0]
	}

	var fromPtr, toPtr *string
	if !g.Cut.TimeWindow.Start.IsZero() {
		s := g.Cut.TimeWindow.Start.UTC().Format(time.RFC3339)
		fromPtr = &s
	}
	if !g.Cut.TimeWindow.End.IsZero() {
		s := g.Cut.TimeWindow.End.UTC().Format(time.RFC3339)
		toPtr = &s
	}

	return CutMeta{
		Observer:    observer,
		From:        fromPtr,
		To:          toPtr,
		Tags:        g.Cut.Tags,
		TraceCount:  g.Cut.TracesIncluded,
		ShadowCount: len(g.Cut.ShadowElements),
	}
}

// parseTimeParam parses a named RFC3339 query parameter.
// Returns a zero time and nil error when value is empty (parameter absent).
func parseTimeParam(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %q value %q: expected RFC3339 (e.g. 2026-01-01T00:00:00Z)", name, value)
	}
	return t, nil
}

// parseQueryTime parses ?from= and ?to= into a graph.TimeWindow.
// Either or both may be absent (half-open or unbounded window).
func parseQueryTime(fromStr, toStr string) (graph.TimeWindow, error) {
	start, err := parseTimeParam("from", fromStr)
	if err != nil {
		return graph.TimeWindow{}, err
	}
	end, err := parseTimeParam("to", toStr)
	if err != nil {
		return graph.TimeWindow{}, err
	}
	tw := graph.TimeWindow{Start: start, End: end}
	if !tw.Start.IsZero() && !tw.End.IsZero() {
		if err := tw.Validate(); err != nil {
			return graph.TimeWindow{}, err
		}
	}
	return tw, nil
}

// parseLimit parses the ?limit= query parameter.
// Returns 0 (no limit) when the parameter is absent.
// Returns an error for non-integer or negative values.
func parseLimit(limitStr string) (int, error) {
	if limitStr == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(limitStr)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("limit must be a non-negative integer")
	}
	return n, nil
}
