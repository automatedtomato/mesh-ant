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

// Graph-reference prefix constants — authoritative definitions.
// The schema package carries matching unexported copies to avoid an import cycle.
const (
	graphRefPrefixGraph  = "meshgraph:"
	graphRefPrefixDiff   = "meshdiff:"
	graphRefPrefixChain  = "meshchain:"
)

// IdentifyGraph assigns a fresh UUID to g.ID and returns the updated MeshGraph.
// The input is not modified. Call only when you intend to use the graph as an
// actor referenced via GraphRef — most articulations do not need to be actors.
func IdentifyGraph(g MeshGraph) MeshGraph {
	g.ID = newUUID4()
	return g
}

// IdentifyDiff assigns a fresh UUID to d.ID and returns the updated GraphDiff.
// The input is not modified.
func IdentifyDiff(d GraphDiff) GraphDiff {
	d.ID = newUUID4()
	return d
}

// GraphRef returns "meshgraph:<g.ID>". Errors if g.ID is empty (call IdentifyGraph first).
// The string carries no positional information — retain the MeshGraph alongside the
// reference if you need to recover the Cut after it enters the mesh.
func GraphRef(g MeshGraph) (string, error) {
	if g.ID == "" {
		return "", fmt.Errorf("graph.GraphRef: graph has no ID; call IdentifyGraph first")
	}
	return graphRefPrefixGraph + g.ID, nil
}

// DiffRef returns "meshdiff:<d.ID>". Errors if d.ID is empty (call IdentifyDiff first).
// Same positional note as GraphRef: retain the GraphDiff to access From/To cuts.
func DiffRef(d GraphDiff) (string, error) {
	if d.ID == "" {
		return "", fmt.Errorf("graph.DiffRef: diff has no ID; call IdentifyDiff first")
	}
	return graphRefPrefixDiff + d.ID, nil
}

// IdentifyChain assigns a fresh UUID to c.ID and returns the updated TranslationChain.
// The input is not modified. Call only when you intend to reference the chain
// via ChainRef — most chains do not need to be actors.
func IdentifyChain(c TranslationChain) TranslationChain {
	c.ID = newUUID4()
	return c
}

// ChainRef returns "meshchain:<c.ID>". Errors if c.ID is empty (call IdentifyChain first).
// The string can appear in Trace.Source or Trace.Target, consistent with generalised symmetry.
func ChainRef(c TranslationChain) (string, error) {
	if c.ID == "" {
		return "", fmt.Errorf("graph.ChainRef: chain has no ID; call IdentifyChain first")
	}
	return graphRefPrefixChain + c.ID, nil
}

// newUUID4 generates a random version-4 UUID in lowercase hyphenated form.
// Panics if crypto/rand is unavailable — that is an unrecoverable environment failure.
func newUUID4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("graph.newUUID4: crypto/rand unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	)
}
