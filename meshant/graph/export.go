// Package graph — export.go provides machine-readable output helpers for
// MeshGraph and GraphDiff: JSON, Graphviz DOT, and Mermaid flowchart formats.
//
// These functions complement the human-readable PrintArticulation and PrintDiff
// functions (graph.go, diff.go) by offering structured output paths suitable
// for piping to jq, dot(1), Mermaid renderers, or storage.
//
// JSON output relies on the M7 codec (TimeWindow null convention, json struct
// tags on all graph types in graph.go, diff.go, serial.go).
//
// DOT output produces a valid Graphviz digraph. Multi-source/multi-target edges
// are rendered as Cartesian-product arcs (one arc per source×target pair).
// Shadow elements appear in a dashed cluster_shadow subgraph, making the
// articulation's blind spots literally visible in the diagram.
//
// Mermaid output produces a flowchart TD. Node IDs are sanitized (non-
// alphanumeric characters replaced with underscores) while original names are
// preserved as display labels. Shadow elements appear in a subgraph Shadow block.
//
// Usage:
//
//	var buf bytes.Buffer
//	if err := graph.PrintGraphJSON(&buf, g); err != nil { ... }
//	if err := graph.PrintGraphDOT(&buf, g); err != nil { ... }
//	if err := graph.PrintGraphMermaid(&buf, g); err != nil { ... }
//	if err := graph.PrintDiffDOT(&buf, d); err != nil { ... }
//	if err := graph.PrintDiffMermaid(&buf, d); err != nil { ... }
package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// maxEdgeLabel is the maximum number of runes to include in a DOT or Mermaid
// edge label. Labels longer than this are truncated with "..." to keep diagrams
// compact without losing the identity of what changed.
const maxEdgeLabel = 28

// PrintGraphJSON writes g as indented JSON to w.
// TimeWindow bounds follow the M7 null convention (serial.go): zero = JSON null.
// Returns any write error. json.MarshalIndent cannot fail for a well-formed MeshGraph.
func PrintGraphJSON(w io.Writer, g MeshGraph) error {
	data, _ := json.MarshalIndent(g, "", "  ") // cannot fail for MeshGraph
	_, err := w.Write(data)
	return err
}

// PrintDiffJSON writes d as indented JSON to w. Same conventions as PrintGraphJSON.
func PrintDiffJSON(w io.Writer, d GraphDiff) error {
	data, _ := json.MarshalIndent(d, "", "  ") // cannot fail for GraphDiff
	_, err := w.Write(data)
	return err
}

// PrintGraphDOT writes g as a Graphviz DOT digraph to w.
//
// Nodes appear with their appearance count as a label. Edges are rendered as
// directed arcs; multi-source/multi-target edges expand to one arc per
// source×target pair (Cartesian product) — a visualization simplification that
// preserves meaning for digram consumers. The JSON export (PrintGraphJSON)
// remains fully lossless.
//
// Shadow elements are rendered in a separate cluster_shadow subgraph with
// dashed style, making the articulation's blind spots visible in the diagram
// (consistent with Principle 5: preserve shadow, do not hide the cut).
//
// A comment block at the top records the observer positions and time window
// from g.Cut, naming the position from which this graph was articulated.
//
// The output is valid DOT syntax accepted by Graphviz dot(1).
func PrintGraphDOT(w io.Writer, g MeshGraph) error {
	var b strings.Builder

	b.WriteString("// ")
	b.WriteString(dotCutComment(g.Cut))
	b.WriteString("\ndigraph {\n")
	b.WriteString("  rankdir=TB\n")
	b.WriteString("  node [shape=box]\n")

	names := make([]string, 0, len(g.Nodes)) // sorted for deterministic output
	for name := range g.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		node := g.Nodes[name]
		fmt.Fprintf(&b, "  %s [label=%s]\n",
			dotQuote(name),
			dotQuote(fmt.Sprintf("%s (%d)", name, node.AppearanceCount)),
		)
	}

	for _, edge := range g.Edges {
		label := dotQuote(truncateLabel(edge.WhatChanged))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s -> %s [label=%s]\n",
					dotQuote(src), dotQuote(tgt), label)
			}
		}
	}

	if len(g.Cut.ShadowElements) > 0 {
		b.WriteString("  subgraph cluster_shadow {\n")
		b.WriteString("    label=\"shadow\"\n")
		b.WriteString("    style=dashed\n")
		b.WriteString("    color=grey\n")
		for _, se := range g.Cut.ShadowElements {
			fmt.Fprintf(&b, "    %s [style=dashed, color=grey]\n", dotQuote(se.Name))
		}
		for i := 1; i < len(g.Cut.ShadowElements); i++ { // invisible edges force vertical layout
			fmt.Fprintf(&b, "    %s -> %s [style=invis]\n",
				dotQuote(g.Cut.ShadowElements[i-1].Name),
				dotQuote(g.Cut.ShadowElements[i].Name))
		}
		b.WriteString("  }\n")
	}

	b.WriteString("}\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// PrintGraphMermaid writes g as a Mermaid flowchart (TD direction) to w.
//
// Node IDs are sanitized for Mermaid compatibility: non-alphanumeric characters
// are replaced with underscores, and a leading digit is prefixed with "n_".
// Original names are preserved as display labels. Collisions after sanitization
// are resolved by appending "_2", "_3", etc.
//
// Edges expand to one arrow per source×target pair (same Cartesian product
// convention as PrintGraphDOT). Shadow elements appear in a subgraph Shadow
// block, making the blind spots of the articulation visible.
//
// A %% comment at the top records the observer positions and time window.
//
// The output is valid Mermaid flowchart syntax.
func PrintGraphMermaid(w io.Writer, g MeshGraph) error {
	var b strings.Builder

	b.WriteString("%% ")
	b.WriteString(dotCutComment(g.Cut))
	b.WriteString("\nflowchart TD\n")

	allNames := collectAllNames(g)
	idMap := buildMermaidIDMap(allNames)

	names := make([]string, 0, len(g.Nodes))
	for name := range g.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		node := g.Nodes[name]
		fmt.Fprintf(&b, "  %s[\"%s (%d)\"]\n",
			idMap[name],
			mermaidLabel(name),
			node.AppearanceCount,
		)
	}

	for _, edge := range g.Edges {
		label := mermaidLabel(truncateLabel(edge.WhatChanged))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s --> |\"%s\"| %s\n",
					idMap[src], label, idMap[tgt])
			}
		}
	}

	if len(g.Cut.ShadowElements) > 0 {
		b.WriteString("  subgraph Shadow\n")
		for _, se := range g.Cut.ShadowElements {
			fmt.Fprintf(&b, "    %s[\"%s\"]\n", idMap[se.Name], mermaidLabel(se.Name))
		}
		for i := 1; i < len(g.Cut.ShadowElements); i++ { // invisible links force vertical layout
			fmt.Fprintf(&b, "    %s ~~~ %s\n",
				idMap[g.Cut.ShadowElements[i-1].Name],
				idMap[g.Cut.ShadowElements[i].Name])
		}
		b.WriteString("  end\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// PrintDiffDOT writes d as a Graphviz DOT digraph to w.
//
// The output records what changed between two situated cuts:
//   - Added nodes in green/bold
//   - Removed nodes in red/dashed
//   - Persisted nodes with a "name (N→M)" appearance count label
//   - Added edges as Cartesian-product arcs with green color
//   - Removed edges as Cartesian-product arcs with red/dashed color
//   - Shadow shifts in a cluster_shadow_shifts subgraph (omitted if empty)
//     with per-kind colors: emerged=red, submerged=green, reason-changed=orange
//
// Two comment lines at the top record the From and To cuts using dotCutComment.
// All user-derived strings (names, labels) are sanitized to prevent injection.
func PrintDiffDOT(w io.Writer, d GraphDiff) error {
	var b strings.Builder

	b.WriteString("// From: ")
	b.WriteString(dotCutComment(d.From))
	b.WriteString("\n// To: ")
	b.WriteString(dotCutComment(d.To))
	b.WriteString("\ndigraph {\n")
	b.WriteString("  rankdir=TB\n")
	b.WriteString("  node [shape=box]\n")

	addedNodes := make([]string, len(d.NodesAdded))
	copy(addedNodes, d.NodesAdded)
	sort.Strings(addedNodes)
	for _, name := range addedNodes {
		fmt.Fprintf(&b, "  %s [label=%s, color=green, style=bold]\n",
			dotQuote(stripNewlines(name)),
			dotQuote(fmt.Sprintf("%s (added)", stripNewlines(name))),
		)
	}

	removedNodes := make([]string, len(d.NodesRemoved))
	copy(removedNodes, d.NodesRemoved)
	sort.Strings(removedNodes)
	for _, name := range removedNodes {
		fmt.Fprintf(&b, "  %s [label=%s, color=red, style=dashed]\n",
			dotQuote(stripNewlines(name)),
			dotQuote(fmt.Sprintf("%s (removed)", stripNewlines(name))),
		)
	}

	persistedNodes := make([]PersistedNode, len(d.NodesPersisted))
	copy(persistedNodes, d.NodesPersisted)
	sort.Slice(persistedNodes, func(i, j int) bool { return persistedNodes[i].Name < persistedNodes[j].Name })
	for _, p := range persistedNodes {
		fmt.Fprintf(&b, "  %s [label=%s]\n",
			dotQuote(stripNewlines(p.Name)),
			dotQuote(fmt.Sprintf("%s (%d→%d)", stripNewlines(p.Name), p.CountFrom, p.CountTo)),
		)
	}

	for _, edge := range d.EdgesAdded {
		label := dotQuote(truncateLabel(stripNewlines(edge.WhatChanged)))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s -> %s [label=%s, color=green, style=bold]\n",
					dotQuote(stripNewlines(src)), dotQuote(stripNewlines(tgt)), label)
			}
		}
	}

	for _, edge := range d.EdgesRemoved {
		label := dotQuote(truncateLabel(stripNewlines(edge.WhatChanged)))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s -> %s [label=%s, color=red, style=dashed]\n",
					dotQuote(stripNewlines(src)), dotQuote(stripNewlines(tgt)), label)
			}
		}
	}

	// Shadow shifts: emerged=green, submerged=red, reason-changed=orange.
	if len(d.ShadowShifts) > 0 {
		b.WriteString("  subgraph cluster_shadow_shifts {\n")
		b.WriteString("    label=\"shadow shifts\"\n")
		b.WriteString("    style=dashed\n")
		b.WriteString("    color=grey\n")
		for _, ss := range d.ShadowShifts {
			color := "orange"
			switch ss.Kind {
			case ShadowShiftEmerged:
				color = "green"
			case ShadowShiftSubmerged:
				color = "red"
			}
			fmt.Fprintf(&b, "    %s [label=%s, color=%s]\n",
				dotQuote(stripNewlines(ss.Name)),
				dotQuote(fmt.Sprintf("%s (%s)", stripNewlines(ss.Name), stripNewlines(string(ss.Kind)))),
				color,
			)
		}
		for i := 1; i < len(d.ShadowShifts); i++ { // invisible edges force vertical layout
			fmt.Fprintf(&b, "    %s -> %s [style=invis]\n",
				dotQuote(stripNewlines(d.ShadowShifts[i-1].Name)),
				dotQuote(stripNewlines(d.ShadowShifts[i].Name)))
		}
		b.WriteString("  }\n")
	}

	b.WriteString("}\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// PrintDiffMermaid writes d as a Mermaid flowchart (TD direction) to w.
//
// Node IDs are sanitized for Mermaid compatibility (same rules as
// PrintGraphMermaid). The output encodes diff semantics through:
//   - Added node declarations with "(added)" label + green stroke style
//   - Removed node declarations with "(removed)" label + red dashed style
//   - Persisted node declarations with "(N→M)" count label
//   - Added edges as --> solid arrows with labels
//   - Removed edges as -.-> dashed arrows with labels
//   - Shadow shifts in a ShadowShifts subgraph (omitted if empty)
//
// Two %% comment lines at the top record the From and To cuts.
// All user-derived strings are sanitized to prevent click-directive injection.
func PrintDiffMermaid(w io.Writer, d GraphDiff) error {
	var b strings.Builder

	b.WriteString("%% From: ")
	b.WriteString(dotCutComment(d.From))
	b.WriteString("\n%% To: ")
	b.WriteString(dotCutComment(d.To))
	b.WriteString("\nflowchart TD\n")

	allNames := collectAllDiffNames(d)
	idMap := buildMermaidIDMap(allNames)

	addedNodes := make([]string, len(d.NodesAdded))
	copy(addedNodes, d.NodesAdded)
	sort.Strings(addedNodes)
	for _, name := range addedNodes {
		fmt.Fprintf(&b, "  %s[\"%s (added)\"]\n", idMap[name], mermaidLabel(name))
	}

	removedNodes := make([]string, len(d.NodesRemoved))
	copy(removedNodes, d.NodesRemoved)
	sort.Strings(removedNodes)
	for _, name := range removedNodes {
		fmt.Fprintf(&b, "  %s[\"%s (removed)\"]\n", idMap[name], mermaidLabel(name))
	}

	persistedNodes := make([]PersistedNode, len(d.NodesPersisted))
	copy(persistedNodes, d.NodesPersisted)
	sort.Slice(persistedNodes, func(i, j int) bool { return persistedNodes[i].Name < persistedNodes[j].Name })
	for _, p := range persistedNodes {
		fmt.Fprintf(&b, "  %s[\"%s (%d→%d)\"]\n",
			idMap[p.Name],
			mermaidLabel(p.Name),
			p.CountFrom,
			p.CountTo,
		)
	}

	for _, name := range addedNodes {
		fmt.Fprintf(&b, "  style %s stroke:green,stroke-width:3px\n", idMap[name])
	}
	for _, name := range removedNodes {
		fmt.Fprintf(&b, "  style %s stroke:red,stroke-dasharray:5\n", idMap[name])
	}

	for _, edge := range d.EdgesAdded {
		label := mermaidLabel(truncateLabel(edge.WhatChanged))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s --> |\"%s\"| %s\n",
					idMap[src], label, idMap[tgt])
			}
		}
	}

	for _, edge := range d.EdgesRemoved {
		label := mermaidLabel(truncateLabel(edge.WhatChanged))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s -.-> |\"%s\"| %s\n",
					idMap[src], label, idMap[tgt])
			}
		}
	}

	// Shadow shifts: emerged=green, submerged=red, reason-changed=orange.
	if len(d.ShadowShifts) > 0 {
		b.WriteString("  subgraph ShadowShifts\n")
		for _, ss := range d.ShadowShifts {
			fmt.Fprintf(&b, "    %s[\"%s (%s)\"]\n",
				idMap[ss.Name],
				mermaidLabel(ss.Name),
				mermaidLabel(string(ss.Kind)),
			)
		}
		for i := 1; i < len(d.ShadowShifts); i++ { // invisible links force vertical layout
			fmt.Fprintf(&b, "    %s ~~~ %s\n",
				idMap[d.ShadowShifts[i-1].Name],
				idMap[d.ShadowShifts[i].Name])
		}
		b.WriteString("  end\n")
		for _, ss := range d.ShadowShifts {
			color := "orange"
			switch ss.Kind {
			case ShadowShiftEmerged:
				color = "green"
			case ShadowShiftSubmerged:
				color = "red"
			}
			fmt.Fprintf(&b, "  style %s stroke:%s,stroke-dasharray:5\n", idMap[ss.Name], color)
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// collectAllDiffNames returns all element names in a GraphDiff (sorted, deduplicated).
func collectAllDiffNames(d GraphDiff) []string {
	seen := make(map[string]bool)
	for _, name := range d.NodesAdded {
		seen[name] = true
	}
	for _, name := range d.NodesRemoved {
		seen[name] = true
	}
	for _, p := range d.NodesPersisted {
		seen[p.Name] = true
	}
	for _, edge := range d.EdgesAdded {
		for _, s := range edge.Sources {
			seen[s] = true
		}
		for _, t := range edge.Targets {
			seen[t] = true
		}
	}
	for _, edge := range d.EdgesRemoved {
		for _, s := range edge.Sources {
			seen[s] = true
		}
		for _, t := range edge.Targets {
			seen[t] = true
		}
	}
	for _, ss := range d.ShadowShifts {
		seen[ss.Name] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// dotCutComment returns a short Cut summary for DOT/Mermaid comment lines. Example:
//
//	"observer: meteorological-analyst | window: 2026-04-14T00:00:00Z–2026-04-14T23:59:59Z | tags: critical, mediated"
//
// Observer position strings are newline-stripped before joining to prevent a
// crafted observer value from breaking out of the comment line into raw DOT syntax.
func dotCutComment(c Cut) string {
	obs := "full cut"
	if len(c.ObserverPositions) > 0 {
		sanitized := make([]string, len(c.ObserverPositions))
		for i, p := range c.ObserverPositions {
			sanitized[i] = stripNewlines(p)
		}
		obs = strings.Join(sanitized, ", ")
	}
	win := "full temporal cut"
	if !c.TimeWindow.IsZero() {
		start := "(unbounded)"
		end := "(unbounded)"
		if !c.TimeWindow.Start.IsZero() {
			start = c.TimeWindow.Start.UTC().Format("2006-01-02")
		}
		if !c.TimeWindow.End.IsZero() {
			end = c.TimeWindow.End.UTC().Format("2006-01-02")
		}
		win = start + "–" + end
	}
	tags := "full tag cut"
	if len(c.Tags) > 0 {
		sanitized := make([]string, len(c.Tags))
		for i, tag := range c.Tags {
			sanitized[i] = stripNewlines(tag)
		}
		tags = strings.Join(sanitized, ", ")
	}
	return fmt.Sprintf("observer: %s | window: %s | tags: %s", obs, win, tags)
}

// dotQuote wraps s in double quotes and escapes internal double quotes.
func dotQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// truncateLabel truncates s to maxEdgeLabel runes, appending "..." if truncated.
func truncateLabel(s string) string {
	runes := []rune(s)
	if len(runes) <= maxEdgeLabel {
		return s
	}
	return string(runes[:maxEdgeLabel]) + "..."
}

// mermaidLabel sanitizes s for safe embedding inside a Mermaid label string.
// It strips newlines and carriage returns (which would break out of the label
// and allow injection of arbitrary Mermaid directives, including click handlers
// with javascript: URIs when rendered in a browser) and replaces double quotes
// with single quotes (Mermaid does not support \" escaping).
func mermaidLabel(s string) string {
	s = stripNewlines(s)
	return strings.ReplaceAll(s, `"`, `'`)
}

// stripNewlines removes newline and carriage-return characters from s.
// Used to prevent multi-line injection into single-line DOT comments and
// Mermaid label strings derived from user-controlled trace field values.
func stripNewlines(s string) string {
	return strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
}

// nonAlphanumRe matches any character that is not a letter, digit, or underscore.
var nonAlphanumRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// sanitizeMermaidID converts a node name to a valid Mermaid node ID:
//   - replaces non-alphanumeric characters with underscores
//   - prefixes with "n_" if the result starts with a digit
func sanitizeMermaidID(name string) string {
	id := nonAlphanumRe.ReplaceAllString(name, "_")
	if len(id) > 0 && id[0] >= '0' && id[0] <= '9' {
		id = "n_" + id
	}
	if id == "" {
		id = "n_empty"
	}
	return id
}

// collectAllNames returns all element names in a MeshGraph (sorted, deduplicated).
func collectAllNames(g MeshGraph) []string {
	seen := make(map[string]bool)
	for name := range g.Nodes {
		seen[name] = true
	}
	for _, edge := range g.Edges {
		for _, s := range edge.Sources {
			seen[s] = true
		}
		for _, t := range edge.Targets {
			seen[t] = true
		}
	}
	for _, se := range g.Cut.ShadowElements {
		seen[se.Name] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// buildMermaidIDMap builds original name → sanitized Mermaid ID.
// Collisions are resolved by appending "_2", "_3", etc.
func buildMermaidIDMap(names []string) map[string]string {
	idMap := make(map[string]string, len(names))
	usedIDs := make(map[string]int) // base ID → collision count

	for _, name := range names {
		base := sanitizeMermaidID(name)
		count := usedIDs[base]
		usedIDs[base]++
		if count == 0 {
			idMap[name] = base
		} else {
			idMap[name] = fmt.Sprintf("%s_%d", base, count+1)
		}
	}
	return idMap
}
