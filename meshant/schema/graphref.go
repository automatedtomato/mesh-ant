// Package schema — graph-reference string predicates.
//
// When a MeshGraph or GraphDiff is identified as an actor (via graph.IdentifyGraph
// or graph.IdentifyDiff), it is assigned a stable string identifier that can appear
// in Trace.Source or Trace.Target just like any other element name. Following the
// ANT principle of generalised symmetry, graphs enter the mesh through the same
// structural positions as any other actant — no privileged field is needed.
//
// The string convention (defined authoritatively in the graph package) is:
//   - "meshgraph:<uuid>" for an identified MeshGraph
//   - "meshdiff:<uuid>"  for an identified GraphDiff
//
// This file provides predicate functions that let any package inspect whether a
// source/target string is a graph-reference, without importing the graph package
// (which would create an import cycle: graph imports schema; schema must not
// import graph).
package schema

import "strings"

// Graph-reference prefix constants.
// These are unexported; the schema layer only needs to recognise the prefix.
// The authoritative definition (which also uses these literals to produce
// reference strings) lives in the graph package.
const (
	graphRefPrefixGraph = "meshgraph:"
	graphRefPrefixDiff  = "meshdiff:"
)

// IsGraphRef reports whether s is a graph-reference string — i.e., whether it
// begins with "meshgraph:" or "meshdiff:". It does not validate the UUID portion.
func IsGraphRef(s string) bool {
	return strings.HasPrefix(s, graphRefPrefixGraph) || strings.HasPrefix(s, graphRefPrefixDiff)
}

// GraphRefKind returns the kind prefix of a graph-reference string:
//   - "meshgraph" if s begins with "meshgraph:"
//   - "meshdiff"  if s begins with "meshdiff:"
//   - ""          if s is not a graph-reference
func GraphRefKind(s string) string {
	switch {
	case strings.HasPrefix(s, graphRefPrefixGraph):
		return "meshgraph"
	case strings.HasPrefix(s, graphRefPrefixDiff):
		return "meshdiff"
	default:
		return ""
	}
}

// GraphRefID returns the UUID portion of a graph-reference string — the part
// after the "meshgraph:" or "meshdiff:" prefix. Returns "" if s is not a
// graph-reference, or if the ID portion is empty (e.g. "meshgraph:" with nothing
// after the colon).
//
// Note: both cases return "". Callers who need to distinguish "not a graph-ref"
// from "graph-ref with empty ID" should call [IsGraphRef] first.
func GraphRefID(s string) string {
	var id string
	switch {
	case strings.HasPrefix(s, graphRefPrefixGraph):
		id = strings.TrimPrefix(s, graphRefPrefixGraph)
	case strings.HasPrefix(s, graphRefPrefixDiff):
		id = strings.TrimPrefix(s, graphRefPrefixDiff)
	}
	return id
}
