package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// lineageNode holds a draft and its subsequent readings in the DerivedFrom chain.
// Used internally by cmdLineage to build and render chains.
type lineageNode struct {
	draft      schema.TraceDraft
	subsequent []*lineageNode
}

// lineageResult holds the parsed chain structure returned by buildLineage.
// anchors are drafts that start a reading sequence (no DerivedFrom, or prior
// not in dataset). Chain order is positional — earlier readings are not more
// authentic than later ones; they simply came first in the production sequence.
type lineageResult struct {
	anchors    []*lineageNode // drafts starting a reading sequence
	standalone int            // count of anchors with no subsequent readings
}

// buildLineage walks DerivedFrom links in the dataset and constructs a tree.
// Returns an error if a cycle is detected. A cycle is detected using DFS with
// a "currently visiting" set (grey set in standard DFS cycle detection).
func buildLineage(drafts []schema.TraceDraft) (lineageResult, error) {
	// Index drafts by ID for O(1) prior lookup.
	byID := make(map[string]*lineageNode, len(drafts))
	nodes := make([]*lineageNode, len(drafts))
	for i := range drafts {
		n := &lineageNode{draft: drafts[i]}
		nodes[i] = n
		byID[drafts[i].ID] = n
	}

	// Link subsequent readings to their prior readings.
	for _, n := range nodes {
		if n.draft.DerivedFrom == "" {
			continue
		}
		prior, ok := byID[n.draft.DerivedFrom]
		if !ok {
			// Prior not in dataset — treat this draft as a chain anchor.
			continue
		}
		prior.subsequent = append(prior.subsequent, n)
	}

	// Identify anchors: drafts with no DerivedFrom, or whose DerivedFrom is not
	// present in the dataset.
	var anchors []*lineageNode
	for _, n := range nodes {
		if n.draft.DerivedFrom == "" {
			anchors = append(anchors, n)
		} else if _, ok := byID[n.draft.DerivedFrom]; !ok {
			anchors = append(anchors, n)
		}
	}

	// Cycle detection: DFS from every anchor. If we reach a node already in the
	// current path (grey set), a cycle exists. Cycles involving nodes that have
	// no path from any anchor are detected separately via the "visited" set.
	visited := make(map[string]bool, len(drafts))
	for _, root := range anchors {
		if err := detectCycleDFS(root, byID, visited, make(map[string]bool)); err != nil {
			return lineageResult{}, err
		}
	}

	// Check for cycles among unreachable nodes (orphaned cycles: A→B→A with no
	// external root). Any unvisited node is part of a cycle or orphaned cycle.
	for _, n := range nodes {
		if !visited[n.draft.ID] {
			// Attempt DFS from this node to detect and name the cycle.
			if err := detectCycleDFS(n, byID, visited, make(map[string]bool)); err != nil {
				return lineageResult{}, err
			}
		}
	}

	// Count standalone anchors (no subsequent readings).
	standalone := 0
	for _, r := range anchors {
		if len(r.subsequent) == 0 {
			standalone++
		}
	}

	return lineageResult{anchors: anchors, standalone: standalone}, nil
}

// detectCycleDFS performs a depth-first search from node, using the grey set
// (inPath) to detect back edges (cycles). Visited nodes are marked in the
// shared visited map so that each node is processed at most once across all
// DFS calls. byID is used to follow DerivedFrom links not already wired into
// the tree (handles orphaned cycles not reachable from any root).
func detectCycleDFS(n *lineageNode, byID map[string]*lineageNode, visited, inPath map[string]bool) error {
	if inPath[n.draft.ID] {
		return fmt.Errorf("lineage: cycle detected involving draft id %q", n.draft.ID)
	}
	if visited[n.draft.ID] {
		return nil
	}
	visited[n.draft.ID] = true
	inPath[n.draft.ID] = true

	for _, child := range n.subsequent {
		if err := detectCycleDFS(child, byID, visited, inPath); err != nil {
			return err
		}
	}

	// Also follow DerivedFrom links to catch orphaned cycles (A→B→A where
	// neither A nor B is a root). This handles the case where inPath contains
	// an orphaned cycle node reached via DerivedFrom from an unvisited node.
	if n.draft.DerivedFrom != "" {
		if prior, ok := byID[n.draft.DerivedFrom]; ok && !visited[prior.draft.ID] {
			if err := detectCycleDFS(prior, byID, visited, inPath); err != nil {
				return err
			}
		}
	}

	delete(inPath, n.draft.ID)
	return nil
}

// idPrefix returns the first 8 characters of a draft ID for display purposes.
// Returns the full ID if it is shorter than 8 characters.
func idPrefix(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// spanPreview returns the first 60 characters of a source span for display,
// truncating with "..." if longer.
func spanPreview(span string) string {
	// Replace newlines with spaces for single-line display.
	s := strings.ReplaceAll(span, "\n", " ")
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

// printLineageText renders the lineage tree in text format to w.
// Chains are rendered as indented trees with └── connectors.
// Standalone drafts are counted at the end.
func printLineageText(w io.Writer, result lineageResult) error {
	// Chain order is positional (production sequence), not hierarchical.
	// Earlier readings are not more authentic than later ones.
	if _, err := fmt.Fprintln(w, "=== DerivedFrom Chains (positional sequence) ==="); err != nil {
		return err
	}

	// Print chains (anchors with subsequent readings).
	for _, root := range result.anchors {
		if len(root.subsequent) == 0 {
			continue // standalone — printed in summary
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		line := fmt.Sprintf("[%s] %s / %s",
			idPrefix(root.draft.ID),
			root.draft.ExtractionStage,
			root.draft.ExtractedBy,
		)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %q\n", spanPreview(root.draft.SourceSpan)); err != nil {
			return err
		}
		for _, child := range root.subsequent {
			if err := printLineageStep(w, child, "  "); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "Standalone drafts (no DerivedFrom, no subsequent readings): %d\n", result.standalone)
	return err
}

// printLineageStep recursively renders a child node with indentation.
func printLineageStep(w io.Writer, n *lineageNode, indent string) error {
	line := fmt.Sprintf("%s└── [%s] %s / %s",
		indent,
		idPrefix(n.draft.ID),
		n.draft.ExtractionStage,
		n.draft.ExtractedBy,
	)
	if _, err := fmt.Fprintln(w, line); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s      %q\n", indent, spanPreview(n.draft.SourceSpan)); err != nil {
		return err
	}
	for _, child := range n.subsequent {
		if err := printLineageStep(w, child, indent+"  "); err != nil {
			return err
		}
	}
	return nil
}

// lineageJSONChain is the JSON representation of a single chain for --format json.
type lineageJSONChain struct {
	AnchorID string   `json:"anchor_id"`
	Members  []string `json:"members"`
}

// collectMembers recursively appends the IDs of n and all its descendants
// to members in depth-first order. Used by printLineageJSON to produce a
// complete flat list of all chain members regardless of chain depth.
func collectMembers(n *lineageNode, members *[]string) {
	*members = append(*members, n.draft.ID)
	for _, child := range n.subsequent {
		collectMembers(child, members)
	}
}

// printLineageJSON renders the lineage result as a JSON object with "chains"
// and "standalone" keys.
//
// Each chain entry lists all members (anchor + all descendants at every depth)
// in depth-first order via collectMembers. A shallow loop over root.subsequent
// would silently drop grandchildren and deeper nodes.
func printLineageJSON(w io.Writer, result lineageResult) error {
	type output struct {
		Chains     []lineageJSONChain `json:"chains"`
		Standalone int                `json:"standalone"`
	}

	var chains []lineageJSONChain
	for _, root := range result.anchors {
		if len(root.subsequent) == 0 {
			continue
		}
		var members []string
		collectMembers(root, &members)
		chains = append(chains, lineageJSONChain{
			AnchorID: root.draft.ID,
			Members:  members,
		})
	}
	if chains == nil {
		chains = []lineageJSONChain{}
	}

	out := output{
		Chains:     chains,
		Standalone: result.standalone,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// cmdLineage implements the "lineage" subcommand.
//
// It reads a TraceDraft JSON file, walks the DerivedFrom links to build chains,
// and prints the structure in text or JSON format. The lineage reader is a
// chain reader, not a diff tool — it shows structure, not differences between
// chain members (P5 in plan_m12.md, design rule 3).
//
// Cycle detection: if DerivedFrom forms a cycle, cmdLineage returns an error
// naming the cycle rather than silently looping.
//
// Flags:
//   - --id <id>          show lineage for a single draft (root or any member)
//   - --format text|json output format (default: text)
func cmdLineage(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("lineage", flag.ContinueOnError)

	var idFilter string
	fs.StringVar(&idFilter, "id", "", "show lineage for a single draft by ID")

	var format string
	fs.StringVar(&format, "format", "text", "output format: text|json")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate format before file I/O so the error is immediate.
	switch format {
	case "text", "json":
		// valid
	default:
		return fmt.Errorf("lineage: unknown --format %q (text|json)", format)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("lineage: path to drafts.json required\n\nUsage: meshant lineage [--id <id>] [--format text|json] <drafts.json>")
	}
	path := remaining[0]

	drafts, err := loader.LoadDrafts(path)
	if err != nil {
		return fmt.Errorf("lineage: %w", err)
	}

	// Build full lineage to detect cycles before applying --id filter.
	// This ensures cycles in the complete dataset are always caught.
	result, err := buildLineage(drafts)
	if err != nil {
		return fmt.Errorf("lineage: %w", err)
	}

	// Apply --id filter: restrict output to the chain containing the specified ID.
	if idFilter != "" {
		filtered, err := filterLineageByID(result, idFilter)
		if err != nil {
			return fmt.Errorf("lineage: %w", err)
		}
		result = filtered
	}

	switch format {
	case "json":
		return printLineageJSON(w, result)
	default: // "text"
		return printLineageText(w, result)
	}
}

// filterLineageByID restricts the lineage result to the chain(s) containing
// the draft with the given ID. Returns an error if no chain contains the ID.
func filterLineageByID(result lineageResult, id string) (lineageResult, error) {
	// Check if the ID is a chain anchor or a subsequent reading in any chain.
	for _, root := range result.anchors {
		if root.draft.ID == id {
			standalone := 0
			if len(root.subsequent) == 0 {
				standalone = 1
			}
			return lineageResult{anchors: []*lineageNode{root}, standalone: standalone}, nil
		}
		// Check if the ID appears in any subsequent reading of this anchor.
		if chainContainsID(root, id) {
			standalone := 0
			return lineageResult{anchors: []*lineageNode{root}, standalone: standalone}, nil
		}
	}
	return lineageResult{}, fmt.Errorf("draft with id %q not found in any chain", id)
}

// chainContainsID reports whether any subsequent reading in the chain starting
// at n has the given ID (not including n itself).
func chainContainsID(n *lineageNode, id string) bool {
	for _, child := range n.subsequent {
		if child.draft.ID == id {
			return true
		}
		if chainContainsID(child, id) {
			return true
		}
	}
	return false
}
