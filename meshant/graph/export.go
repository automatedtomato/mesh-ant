// Package graph — export.go provides JSON serialisation helpers for MeshGraph
// and GraphDiff.
//
// These functions complement the human-readable PrintArticulation and PrintDiff
// functions (graph.go, diff.go) by offering a machine-readable JSON output path.
// The JSON representation relies entirely on the struct tags declared on all
// graph types (graph.go, diff.go) and the custom TimeWindow codec (serial.go).
//
// Usage:
//
//	var buf bytes.Buffer
//	if err := graph.PrintGraphJSON(&buf, g); err != nil { ... }
//	// buf now holds indented JSON for g
package graph

import (
	"encoding/json"
	"io"
)

// PrintGraphJSON writes g as indented JSON to w.
//
// The output is a complete, self-contained JSON object that can be stored,
// transmitted, or re-ingested by any JSON consumer. Indentation uses two spaces
// per level ("  ") for readability and diff-friendliness.
//
// TimeWindow bounds follow the M7 null convention defined in serial.go: a zero
// Start or End is serialised as JSON null rather than the RFC3339 zero-time
// string "0001-01-01T00:00:00Z". This makes unbounded windows unambiguous.
//
// The caller is responsible for any surrounding JSON structure (e.g. wrapping
// the object in an array or adding envelope fields). PrintGraphJSON writes only
// the MeshGraph object itself.
//
// Returns any write error from w. MeshGraph contains only JSON-safe types
// (strings, ints, slices, maps, and time.Time via a custom marshaler that always
// succeeds), so json.MarshalIndent will not fail for a well-formed MeshGraph.
func PrintGraphJSON(w io.Writer, g MeshGraph) error {
	// MarshalIndent cannot fail for MeshGraph: all fields are basic types or
	// time.Time via a custom codec that always succeeds. The error return from
	// MarshalIndent is intentionally ignored here; only write errors are returned.
	data, _ := json.MarshalIndent(g, "", "  ")
	_, err := w.Write(data)
	return err
}

// PrintDiffJSON writes d as indented JSON to w.
//
// The output follows the same conventions as PrintGraphJSON: two-space
// indentation, TimeWindow null convention for zero bounds, and no surrounding
// envelope. The caller is responsible for any wrapping structure.
//
// Returns any write error from w. GraphDiff contains only JSON-safe types,
// so json.MarshalIndent will not fail for a well-formed GraphDiff.
func PrintDiffJSON(w io.Writer, d GraphDiff) error {
	// MarshalIndent cannot fail for GraphDiff — see PrintGraphJSON for rationale.
	data, _ := json.MarshalIndent(d, "", "  ")
	_, err := w.Write(data)
	return err
}
