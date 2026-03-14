// chain.go provides translation chain traversal — Layer 4 analytical operation.
//
// A translation chain reads *through* an articulated MeshGraph, following
// connected elements along edges. This is the first operation that traces a
// path (temporal, directional) rather than producing a snapshot (spatial,
// simultaneous like Articulate) or comparing snapshots (like Diff).
//
// A chain is itself a cut: the analyst chooses where to start, which direction
// to follow, and how deep to go. Two observers could follow different chains
// through the same graph. The chain carries the Cut it was read from so it
// remains self-situated.
//
// Branching strategy (v1): first-match, deterministic. When multiple edges
// leave a node, the first by dataset order is followed. Alternatives are
// recorded as breaks with reason "branch-not-taken" — consistent with the
// shadow philosophy of naming what was not followed.
package graph

// Direction controls whether FollowTranslation reads forward (source → target)
// or backward (target → source) through graph edges.
type Direction string

const (
	// DirectionForward follows edges from source to target.
	// This is the default when Direction is zero-value.
	DirectionForward Direction = "forward"

	// DirectionBackward follows edges from target to source.
	DirectionBackward Direction = "backward"
)

// FollowOptions parameterises a translation chain traversal.
type FollowOptions struct {
	// Direction controls traversal direction. Zero value means forward.
	Direction Direction

	// MaxDepth limits the number of steps. Zero means unlimited.
	MaxDepth int
}

// ChainStep records one traversal step: crossing a single edge.
type ChainStep struct {
	// Edge is the graph edge that was traversed in this step.
	Edge Edge

	// ElementExited is the node the chain was at before this step.
	// For the first step, this equals TranslationChain.StartElement.
	ElementExited string

	// ElementEntered is the node the chain arrived at via this edge.
	ElementEntered string
}

// ChainBreak records where and why the chain could not continue.
// Breaks are analytically significant — they name what stopped the traversal.
type ChainBreak struct {
	// AtElement is the node where the break occurred.
	AtElement string `json:"at_element"`

	// Reason describes why the chain stopped or why an alternative was not
	// followed. Known values:
	//   "element-not-in-graph" — the start element does not exist in the graph
	//   "no-outgoing-edges"    — no edges leave this node (forward)
	//   "no-incoming-edges"    — no edges enter this node (backward)
	//   "depth-limit"          — MaxDepth reached
	//   "cycle-detected"       — the chain would revisit an already-visited node;
	//                            note: the cycle-closing step IS recorded in
	//                            TranslationChain.Steps (unlike depth-limit or
	//                            no-outgoing-edges). This lets callers see which
	//                            edge closed the loop.
	//   "branch-not-taken"     — an alternative edge was available but not followed
	Reason string `json:"reason"`
}

// TranslationChain is the result of following connected elements through a
// MeshGraph. It records the path taken (Steps), the points where the chain
// could not continue or chose not to follow (Breaks), and the Cut from which
// it was read.
type TranslationChain struct {
	// StartElement is the node where the chain was entered.
	StartElement string

	// Steps is the ordered sequence of edge traversals.
	Steps []ChainStep

	// Breaks records where the chain stopped or where alternatives exist.
	Breaks []ChainBreak

	// Cut is the articulation parameters of the MeshGraph this chain was
	// read from. The chain is situated within this cut.
	Cut Cut
}

// FollowTranslation traverses connected elements through g starting from the
// element named by from. It returns an immutable TranslationChain.
//
// The traversal is deterministic: when multiple edges leave a node, the first
// by dataset order (edge index) is followed. Alternative edges are recorded as
// breaks with reason "branch-not-taken".
//
// The returned chain carries g.Cut so the chain is self-situated.
func FollowTranslation(g MeshGraph, from string, opts FollowOptions) TranslationChain {
	// Resolve default direction.
	dir := opts.Direction
	if dir == "" {
		dir = DirectionForward
	}

	chain := TranslationChain{
		StartElement: from,
		Cut:          g.Cut,
	}

	// Check that the start element exists in the graph.
	if _, ok := g.Nodes[from]; !ok {
		chain.Breaks = append(chain.Breaks, ChainBreak{
			AtElement: from,
			Reason:    "element-not-in-graph",
		})
		return chain
	}

	// Build adjacency: for forward, index edges by each source element;
	// for backward, index edges by each target element.
	type adjacencyEntry struct {
		edgeIdx int
		target  string // the element we'd enter (target for forward, source for backward)
	}
	adjacency := make(map[string][]adjacencyEntry)

	for i, e := range g.Edges {
		if dir == DirectionForward {
			for _, s := range e.Sources {
				for _, t := range e.Targets {
					adjacency[s] = append(adjacency[s], adjacencyEntry{edgeIdx: i, target: t})
				}
			}
		} else {
			for _, t := range e.Targets {
				for _, s := range e.Sources {
					adjacency[t] = append(adjacency[t], adjacencyEntry{edgeIdx: i, target: s})
				}
			}
		}
	}

	// Walk the chain.
	visited := map[string]bool{from: true}
	current := from

	for {
		// Check depth limit.
		if opts.MaxDepth > 0 && len(chain.Steps) >= opts.MaxDepth {
			chain.Breaks = append(chain.Breaks, ChainBreak{
				AtElement: current,
				Reason:    "depth-limit",
			})
			break
		}

		entries := adjacency[current]
		if len(entries) == 0 {
			// No edges from this element in the traversal direction.
			reason := "no-outgoing-edges"
			if dir == DirectionBackward {
				reason = "no-incoming-edges"
			}
			chain.Breaks = append(chain.Breaks, ChainBreak{
				AtElement: current,
				Reason:    reason,
			})
			break
		}

		// First-match: follow the first entry, record others as branch-not-taken.
		// The cycle-closing step IS recorded in Steps so callers can see which
		// edge closed the loop — see ChainBreak.Reason doc for "cycle-detected".
		first := entries[0]
		cycleDetected := false

		// Capture the node before advancing current so that branch-not-taken
		// breaks below can attribute AtElement to the decision point, not the
		// node we entered.
		decisionPoint := current

		chain.Steps = append(chain.Steps, ChainStep{
			Edge:           g.Edges[first.edgeIdx],
			ElementExited:  current,
			ElementEntered: first.target,
		})

		if visited[first.target] {
			chain.Breaks = append(chain.Breaks, ChainBreak{
				AtElement: first.target,
				Reason:    "cycle-detected",
			})
			cycleDetected = true
		} else {
			visited[first.target] = true
			current = first.target
		}

		// Record alternatives not followed as branch-not-taken.
		// AtElement names the node where the branch decision was made (the
		// node we exited), not the node we entered via first-match.
		for range entries[1:] {
			chain.Breaks = append(chain.Breaks, ChainBreak{
				AtElement: decisionPoint,
				Reason:    "branch-not-taken",
			})
		}

		if cycleDetected {
			break
		}
	}

	return chain
}
