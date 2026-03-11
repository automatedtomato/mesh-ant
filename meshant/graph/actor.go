// Package graph — graph-as-actor identity functions.
//
// M5 makes MeshAnt's observation apparatus traceable inside the mesh it observes.
// An identified MeshGraph or GraphDiff can appear as a Source or Target in
// subsequent traces, following the ANT principle of generalised symmetry: a graph
// that influenced an outcome is an actant like any other.
//
// Design decisions:
//   - Explicit opt-in: Articulate and Diff return graphs with empty IDs. The caller
//     calls IdentifyGraph or IdentifyDiff when they intend to use the result as an
//     actor — not automatically, because not every articulation needs to be one.
//   - Immutability: IdentifyGraph and IdentifyDiff return new values; inputs are not
//     modified.
//   - No registry: callers retain the graphs they identify. There is no package-level
//     map of ID → MeshGraph. Lookup is the caller's responsibility.
//   - No external dependencies: UUID generation uses crypto/rand only.
package graph

import (
	"crypto/rand"
	"fmt"
)

// Graph-reference prefix constants.
// These are the authoritative definitions; the schema package carries matching
// unexported copies for use in its string-level predicates (schema.IsGraphRef etc.)
// without creating an import cycle.
const (
	graphRefPrefixGraph = "meshgraph:"
	graphRefPrefixDiff  = "meshdiff:"
)

// IdentifyGraph assigns a fresh, stable UUID to g.ID and returns the updated
// MeshGraph. The input g is not modified (immutable pattern).
//
// Call IdentifyGraph only when you intend to use the graph as an actor in the
// mesh — i.e., when you plan to reference it in subsequent traces via GraphRef.
// Most articulations produced for analysis do not need to be actors.
func IdentifyGraph(g MeshGraph) MeshGraph {
	g.ID = newUUID4()
	return g
}

// IdentifyDiff assigns a fresh, stable UUID to d.ID and returns the updated
// GraphDiff. The input d is not modified.
func IdentifyDiff(d GraphDiff) GraphDiff {
	d.ID = newUUID4()
	return d
}

// GraphRef returns the graph-reference string for g ("meshgraph:<g.ID>").
// Returns an error if g.ID is empty — call IdentifyGraph first.
//
// The returned string can be placed in Trace.Source or Trace.Target to record
// that this graph acted in the mesh.
//
// Note: the reference string is a stable handle — it carries no positional
// information (no observer positions, time window, or shadow). The Cut is
// held in the MeshGraph struct. If you need to recover the position from
// which the articulation was made after the reference enters the mesh, retain
// the identified MeshGraph alongside the reference string.
func GraphRef(g MeshGraph) (string, error) {
	if g.ID == "" {
		return "", fmt.Errorf("graph.GraphRef: graph has no ID; call IdentifyGraph first")
	}
	return graphRefPrefixGraph + g.ID, nil
}

// DiffRef returns the graph-reference string for d ("meshdiff:<d.ID>").
// Returns an error if d.ID is empty — call IdentifyDiff first.
//
// Same positional note as GraphRef: the string carries no Cut information.
// Retain the identified GraphDiff if you need to access From/To cuts after
// the reference enters the mesh.
func DiffRef(d GraphDiff) (string, error) {
	if d.ID == "" {
		return "", fmt.Errorf("graph.DiffRef: diff has no ID; call IdentifyDiff first")
	}
	return graphRefPrefixDiff + d.ID, nil
}

// newUUID4 generates a random version-4 UUID string in lowercase hyphenated form:
// xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx, where y is 8, 9, a, or b.
//
// Uses crypto/rand for randomness. Panics if the system random source is
// unavailable — this is an unrecoverable environment failure, not a caller error.
func newUUID4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("graph.newUUID4: crypto/rand unavailable: " + err.Error())
	}
	// Set version 4 (bits 12–15 of byte 6).
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant bits (bits 6–7 of byte 8): 10xxxxxx.
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	)
}
