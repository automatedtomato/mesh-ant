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
//   - "meshchain:<uuid>" for an identified TranslationChain
//
// This file provides predicate functions that let any package inspect whether a
// source/target string is a graph-reference, without importing the graph package
// (which would create an import cycle: graph imports schema; schema must not
// import graph).
package schema

import "strings"

// Graph-reference prefix constants. Unexported — schema only needs to recognise
// them. Authoritative definitions live in the graph package.
const (
	graphRefPrefixGraph  = "meshgraph:"
	graphRefPrefixDiff   = "meshdiff:"
	graphRefPrefixChain  = "meshchain:"
)

// parseGraphRef splits s on the first colon and validates the prefix.
// Returns (kind, id) or ("", "") if s is not a graph-reference.
// Adding a new prefix requires editing only this function.
func parseGraphRef(s string) (kind, id string) {
	before, after, ok := strings.Cut(s, ":")
	if !ok {
		return "", ""
	}
	if before != "meshgraph" && before != "meshdiff" && before != "meshchain" {
		return "", ""
	}
	return before, after
}

// IsGraphRef reports whether s is a graph-reference string — i.e., whether it
// begins with "meshgraph:", "meshdiff:", or "meshchain:". It does not validate
// the UUID portion.
func IsGraphRef(s string) bool {
	kind, _ := parseGraphRef(s)
	return kind != ""
}

// GraphRefKind returns the kind prefix of a graph-reference string:
//   - "meshgraph" if s begins with "meshgraph:"
//   - "meshdiff"  if s begins with "meshdiff:"
//   - "meshchain" if s begins with "meshchain:"
//   - ""          if s is not a graph-reference
func GraphRefKind(s string) string {
	kind, _ := parseGraphRef(s)
	return kind
}

// GraphRefID returns the UUID portion of a graph-reference string.
// Returns "" if s is not a graph-reference or the ID portion is empty.
// Call [IsGraphRef] first to distinguish "not a ref" from "ref with empty ID".
func GraphRefID(s string) string {
	_, id := parseGraphRef(s)
	return id
}
