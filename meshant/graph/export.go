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

	// Metadata comment: name the articulation position.
	b.WriteString("// ")
	b.WriteString(dotCutComment(g.Cut))
	b.WriteString("\ndigraph {\n")
	b.WriteString("  rankdir=TB\n")
	b.WriteString("  node [shape=box]\n")

	// Sort node names for deterministic output.
	names := make([]string, 0, len(g.Nodes))
	for name := range g.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	// Emit nodes with appearance count label.
	for _, name := range names {
		node := g.Nodes[name]
		fmt.Fprintf(&b, "  %s [label=%s]\n",
			dotQuote(name),
			dotQuote(fmt.Sprintf("%s (%d)", name, node.AppearanceCount)),
		)
	}

	// Emit edges as Cartesian product of sources × targets.
	for _, edge := range g.Edges {
		label := dotQuote(truncateLabel(edge.WhatChanged))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s -> %s [label=%s]\n",
					dotQuote(src), dotQuote(tgt), label)
			}
		}
	}

	// Shadow subgraph — only emitted if there are shadow elements.
	if len(g.Cut.ShadowElements) > 0 {
		b.WriteString("  subgraph cluster_shadow {\n")
		b.WriteString("    label=\"shadow\"\n")
		b.WriteString("    style=dashed\n")
		b.WriteString("    color=grey\n")
		for _, se := range g.Cut.ShadowElements {
			fmt.Fprintf(&b, "    %s [style=dashed, color=grey]\n", dotQuote(se.Name))
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

	// Metadata comment.
	b.WriteString("%% ")
	b.WriteString(dotCutComment(g.Cut))
	b.WriteString("\nflowchart TD\n")

	// Build a sanitized-ID map for all names that appear in nodes or edges.
	// This ensures edges referencing elements not in g.Nodes still get valid IDs.
	allNames := collectAllNames(g)
	idMap := buildMermaidIDMap(allNames)

	// Sort node names for deterministic output.
	names := make([]string, 0, len(g.Nodes))
	for name := range g.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	// Emit node declarations with sanitized IDs and original labels.
	for _, name := range names {
		node := g.Nodes[name]
		fmt.Fprintf(&b, "  %s[\"%s (%d)\"]\n",
			idMap[name],
			mermaidLabel(name),
			node.AppearanceCount,
		)
	}

	// Emit edges as Cartesian product of sources × targets.
	for _, edge := range g.Edges {
		label := mermaidLabel(truncateLabel(edge.WhatChanged))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s --> |\"%s\"| %s\n",
					idMap[src], label, idMap[tgt])
			}
		}
	}

	// Shadow subgraph — only emitted if there are shadow elements.
	if len(g.Cut.ShadowElements) > 0 {
		b.WriteString("  subgraph Shadow\n")
		for _, se := range g.Cut.ShadowElements {
			fmt.Fprintf(&b, "    %s[\"%s\"]\n", idMap[se.Name], mermaidLabel(se.Name))
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

	// Two-line comment: From and To cuts.
	b.WriteString("// From: ")
	b.WriteString(dotCutComment(d.From))
	b.WriteString("\n// To: ")
	b.WriteString(dotCutComment(d.To))
	b.WriteString("\ndigraph {\n")
	b.WriteString("  rankdir=TB\n")
	b.WriteString("  node [shape=box]\n")

	// Sort added nodes for deterministic output.
	addedNodes := make([]string, len(d.NodesAdded))
	copy(addedNodes, d.NodesAdded)
	sort.Strings(addedNodes)

	// Emit added nodes: green/bold with "(added)" label.
	for _, name := range addedNodes {
		fmt.Fprintf(&b, "  %s [label=%s, color=green, style=bold]\n",
			dotQuote(stripNewlines(name)),
			dotQuote(fmt.Sprintf("%s (added)", stripNewlines(name))),
		)
	}

	// Sort removed nodes for deterministic output.
	removedNodes := make([]string, len(d.NodesRemoved))
	copy(removedNodes, d.NodesRemoved)
	sort.Strings(removedNodes)

	// Emit removed nodes: red/dashed with "(removed)" label.
	for _, name := range removedNodes {
		fmt.Fprintf(&b, "  %s [label=%s, color=red, style=dashed]\n",
			dotQuote(stripNewlines(name)),
			dotQuote(fmt.Sprintf("%s (removed)", stripNewlines(name))),
		)
	}

	// Sort persisted nodes for deterministic output.
	persistedNodes := make([]PersistedNode, len(d.NodesPersisted))
	copy(persistedNodes, d.NodesPersisted)
	sort.Slice(persistedNodes, func(i, j int) bool { return persistedNodes[i].Name < persistedNodes[j].Name })

	// Emit persisted nodes with appearance count label "name (N→M)".
	for _, p := range persistedNodes {
		fmt.Fprintf(&b, "  %s [label=%s]\n",
			dotQuote(stripNewlines(p.Name)),
			dotQuote(fmt.Sprintf("%s (%d→%d)", stripNewlines(p.Name), p.CountFrom, p.CountTo)),
		)
	}

	// Emit added edges as Cartesian product with green/bold style.
	for _, edge := range d.EdgesAdded {
		label := dotQuote(truncateLabel(stripNewlines(edge.WhatChanged)))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s -> %s [label=%s, color=green, style=bold]\n",
					dotQuote(stripNewlines(src)), dotQuote(stripNewlines(tgt)), label)
			}
		}
	}

	// Emit removed edges as Cartesian product with red/dashed style.
	for _, edge := range d.EdgesRemoved {
		label := dotQuote(truncateLabel(stripNewlines(edge.WhatChanged)))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s -> %s [label=%s, color=red, style=dashed]\n",
					dotQuote(stripNewlines(src)), dotQuote(stripNewlines(tgt)), label)
			}
		}
	}

	// Shadow shifts subgraph — only emitted if there are shifts.
	// Colors per kind: emerged=green (now visible — consistent with added-node convention),
	// submerged=red (now hidden — consistent with removed-node convention),
	// reason-changed=orange (shifted meaning but still in shadow).
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

	// Two-line comment: From and To cuts.
	b.WriteString("%% From: ")
	b.WriteString(dotCutComment(d.From))
	b.WriteString("\n%% To: ")
	b.WriteString(dotCutComment(d.To))
	b.WriteString("\nflowchart TD\n")

	// Build a sanitized-ID map for all names in the diff.
	allNames := collectAllDiffNames(d)
	idMap := buildMermaidIDMap(allNames)

	// Emit added node declarations with "(added)" label.
	addedNodes := make([]string, len(d.NodesAdded))
	copy(addedNodes, d.NodesAdded)
	sort.Strings(addedNodes)
	for _, name := range addedNodes {
		fmt.Fprintf(&b, "  %s[\"%s (added)\"]\n",
			idMap[name],
			mermaidLabel(name),
		)
	}

	// Emit removed node declarations with "(removed)" label.
	removedNodes := make([]string, len(d.NodesRemoved))
	copy(removedNodes, d.NodesRemoved)
	sort.Strings(removedNodes)
	for _, name := range removedNodes {
		fmt.Fprintf(&b, "  %s[\"%s (removed)\"]\n",
			idMap[name],
			mermaidLabel(name),
		)
	}

	// Emit persisted node declarations with "(N→M)" count label.
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

	// Emit style directives for added nodes (green stroke) and removed nodes
	// (red dashed stroke). These follow all node declarations.
	for _, name := range addedNodes {
		fmt.Fprintf(&b, "  style %s stroke:green,stroke-width:3px\n", idMap[name])
	}
	for _, name := range removedNodes {
		fmt.Fprintf(&b, "  style %s stroke:red,stroke-dasharray:5\n", idMap[name])
	}

	// Emit added edges as solid --> arrows (Cartesian product).
	for _, edge := range d.EdgesAdded {
		label := mermaidLabel(truncateLabel(edge.WhatChanged))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s --> |\"%s\"| %s\n",
					idMap[src], label, idMap[tgt])
			}
		}
	}

	// Emit removed edges as dashed -.-> arrows (Cartesian product).
	for _, edge := range d.EdgesRemoved {
		label := mermaidLabel(truncateLabel(edge.WhatChanged))
		for _, src := range edge.Sources {
			for _, tgt := range edge.Targets {
				fmt.Fprintf(&b, "  %s -.-> |\"%s\"| %s\n",
					idMap[src], label, idMap[tgt])
			}
		}
	}

	// Shadow shifts subgraph — only emitted if there are shifts.
	// Per-node style directives mirror the DOT color convention:
	// emerged=green, submerged=red, reason-changed=orange.
	if len(d.ShadowShifts) > 0 {
		b.WriteString("  subgraph ShadowShifts\n")
		for _, ss := range d.ShadowShifts {
			// Label describes both the name and shift kind for readability.
			fmt.Fprintf(&b, "    %s[\"%s (%s)\"]\n",
				idMap[ss.Name],
				mermaidLabel(ss.Name),
				mermaidLabel(string(ss.Kind)),
			)
		}
		b.WriteString("  end\n")
		// Style directives for shadow shift nodes (after subgraph block).
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

// collectAllDiffNames returns all element names that appear in a GraphDiff:
// added/removed node names, persisted node names, edge sources/targets, and
// shadow shift names. Returns sorted, deduplicated list.
// This ensures every name referenced in the diagram gets a sanitized Mermaid ID.
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

// --- helpers ---

// dotCutComment returns a short human-readable summary of a Cut for use as
// a comment in DOT and Mermaid output. Example:
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
	// "full temporal cut" names the zero TimeWindow as a deliberate choice —
	// the full temporal extent of the dataset — rather than implying a neutral
	// absence. Mirrors the "(all — full cut)" observer convention.
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
	// "full tag cut" names the empty Tags slice as a deliberate choice —
	// the full tag extent of the dataset — rather than implying a neutral absence.
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

// dotQuote wraps s in double quotes and escapes any double quotes within s.
// Required for DOT node IDs and labels that may contain hyphens or spaces.
func dotQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// truncateLabel truncates s to maxEdgeLabel runes, appending "..." if truncated.
// This keeps DOT and Mermaid edge labels readable without losing identity.
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

// collectAllNames returns all element names that appear in a MeshGraph:
// node names, edge sources/targets, and shadow element names.
// This ensures every name referenced in the diagram gets a sanitized ID.
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

// buildMermaidIDMap builds a map from original name → unique sanitized Mermaid ID.
// Collisions (two names that sanitize to the same ID) are resolved by appending
// "_2", "_3", etc. to the later-sorted name.
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
