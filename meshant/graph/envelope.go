// Package graph — envelope.go defines the cut-metadata and response envelope
// types that make every analytical reading explicitly positioned.
//
// CutMeta and Envelope were originally defined in the serve package
// (serve/response.go). Moving them here lets any package — not only the HTTP
// server — build a positioned response without importing serve.
//
// Design notes:
//   - Analyst is an optional identifier for the human or agent that requested
//     the cut. It is deliberately separate from Observer (the ANT position from
//     which the graph was articulated) because the analyst may not be an actant
//     inside the network they are reading. Callers set Analyst explicitly;
//     CutMetaFromGraph always returns an empty Analyst.
//   - CutMetaFromGraph reads only the first ObserverPositions entry. Multi-
//     observer cuts are valid in graph.Articulate but the single-observer
//     HTTP API always populates exactly one position.
//   - From and To are *string (nullable) so the JSON serialisation is null
//     rather than the zero-time string "0001-01-01T00:00:00Z".
package graph

import "time"

// CutMeta carries the observer-position metadata that accompanies every
// response. Every graph is a positioned reading; this struct makes that
// position undeniable and carries it through every layer that wraps the data.
type CutMeta struct {
	// Observer is the ANT position from which this cut was made.
	Observer string `json:"observer"`

	// Analyst is the human or agent that requested the cut. It may be empty
	// when no attribution is needed. Omitted from JSON when empty (omitempty).
	// Callers set this field explicitly; CutMetaFromGraph always returns "".
	Analyst string `json:"analyst,omitempty"`

	// From is the RFC3339 lower bound of the time window, or null when unbounded.
	From *string `json:"from"`

	// To is the RFC3339 upper bound of the time window, or null when unbounded.
	To *string `json:"to"`

	// Tags lists the tag filter used. Omitted from JSON when no tag filter was
	// applied (nil slice). Use omitempty so consumers can distinguish
	// "unfiltered cut" (key absent) from "filtered on zero tags" (empty array).
	Tags []string `json:"tags,omitempty"`

	// TraceCount is the number of traces included in the cut.
	TraceCount int `json:"trace_count"`

	// ShadowCount is the number of shadow elements (for articulation-based cuts)
	// or an approximate count of non-observer traces (for raw-trace endpoints).
	// See ANT tension T2 in serve-v1.md for the /traces approximation.
	ShadowCount int `json:"shadow_count"`
}

// Envelope wraps every response with cut metadata.
// The server and any other analytical output layer never return data without
// naming the cut from which it was produced.
type Envelope struct {
	Cut  CutMeta     `json:"cut"`
	Data interface{} `json:"data"`
}

// CutMetaFromGraph builds a CutMeta from an articulated MeshGraph.
//
// Observer is the first entry in g.Cut.ObserverPositions; empty when no
// observer filter was applied (full cut).
//
// From and To are RFC3339 strings when the corresponding TimeWindow bound is
// non-zero; nil when unbounded.
//
// Analyst is always empty — callers are responsible for setting it to name the
// human or agent that requested the cut.
func CutMetaFromGraph(g MeshGraph) CutMeta {
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
		// Analyst is intentionally left empty — callers set it explicitly.
		Observer:    observer,
		From:        fromPtr,
		To:          toPtr,
		Tags:        g.Cut.Tags,
		TraceCount:  g.Cut.TracesIncluded,
		ShadowCount: len(g.Cut.ShadowElements),
	}
}
