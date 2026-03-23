//go:build neo4j

// neo4j_cypher.go builds Cypher queries and maps between Go types and Neo4j
// property maps. All functions here are pure — no driver dependency, no I/O.
// Separating query construction from driver calls makes both easier to reason
// about and keeps neo4j_store.go focused on session/transaction management.
package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// storeCypher returns the Cypher statement and parameter map for upserting a
// single trace. Uses MERGE on :Trace.id for idempotent upsert.
//
// Element relationships use FOREACH rather than UNWIND. FOREACH is safe on
// empty slices — UNWIND on an empty list produces zero rows and would silently
// drop the trace from further processing in the same statement.
//
// Graph schema per docs/decisions/kg-scoping-v1.md §1.2:
//
//	(:Element {name})-[:SOURCE_OF]->(:Trace)
//	(:Trace)-[:TARGETS]->(:Element {name})
//
// ANT note (T3): MERGE on :Element.name commits to string equality as the
// provisional equivalence criterion for elements — see kg-scoping-v1.md §1.3.
func storeCypher(t schema.Trace) (string, map[string]any) {
	const cypher = `MERGE (t:Trace {id: $id})
SET t.timestamp    = $timestamp,
    t.what_changed = $what_changed,
    t.observer     = $observer,
    t.mediation    = $mediation,
    t.tags         = $tags
WITH t
FOREACH (srcName IN $sources |
  MERGE (src:Element {name: srcName})
  MERGE (src)-[:SOURCE_OF]->(t)
)
WITH t
FOREACH (tgtName IN $targets |
  MERGE (tgt:Element {name: tgtName})
  MERGE (t)-[:TARGETS]->(tgt)
)`

	sources := t.Source
	if sources == nil {
		sources = []string{}
	}
	targets := t.Target
	if targets == nil {
		targets = []string{}
	}
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}

	return cypher, map[string]any{
		"id":           t.ID,
		"timestamp":    t.Timestamp.UTC().Format(time.RFC3339Nano),
		"what_changed": t.WhatChanged,
		"observer":     t.Observer,
		"mediation":    t.Mediation,
		"tags":         tags,
		"sources":      sources,
		"targets":      targets,
	}
}

// queryCypher returns Cypher and parameters for a Query call. Non-zero opts
// fields add AND WHERE clauses. The two-stage OPTIONAL MATCH (with an
// intermediate WITH to aggregate sources before matching targets) avoids the
// Cartesian product that would arise from joining all sources × all targets.
//
// Timestamp comparisons use lexicographic ordering on RFC3339Nano UTC strings,
// which is safe because all timestamps are stored in UTC and ISO 8601 UTC
// strings sort chronologically.
func queryCypher(opts QueryOpts) (string, map[string]any) {
	var clauses []string
	params := make(map[string]any)

	if opts.Observer != "" {
		clauses = append(clauses, "t.observer = $observer")
		params["observer"] = opts.Observer
	}
	if !opts.Window.Start.IsZero() {
		clauses = append(clauses, "t.timestamp >= $from")
		params["from"] = opts.Window.Start.UTC().Format(time.RFC3339Nano)
	}
	if !opts.Window.End.IsZero() {
		clauses = append(clauses, "t.timestamp <= $to")
		params["to"] = opts.Window.End.UTC().Format(time.RFC3339Nano)
	}
	if len(opts.Tags) > 0 {
		clauses = append(clauses, "ALL(tag IN $tags WHERE tag IN t.tags)")
		params["tags"] = opts.Tags
	}

	var b strings.Builder
	b.WriteString("MATCH (t:Trace)\n")
	if len(clauses) > 0 {
		b.WriteString("WHERE ")
		b.WriteString(strings.Join(clauses, "\n  AND "))
		b.WriteString("\n")
	}
	// Two-stage OPTIONAL MATCH avoids source × target Cartesian product.
	b.WriteString("OPTIONAL MATCH (src:Element)-[:SOURCE_OF]->(t)\n")
	b.WriteString("WITH t, collect(DISTINCT src.name) AS sources\n")
	b.WriteString("OPTIONAL MATCH (t)-[:TARGETS]->(tgt:Element)\n")
	b.WriteString("RETURN t, sources, collect(DISTINCT tgt.name) AS targets\n")
	b.WriteString("ORDER BY t.timestamp ASC")

	if opts.Limit > 0 {
		b.WriteString("\nLIMIT $limit")
		params["limit"] = opts.Limit
	}

	return b.String(), params
}

// getCypher returns Cypher and parameters to retrieve a single trace by ID.
func getCypher(id string) (string, map[string]any) {
	const cypher = `MATCH (t:Trace {id: $id})
OPTIONAL MATCH (src:Element)-[:SOURCE_OF]->(t)
WITH t, collect(DISTINCT src.name) AS sources
OPTIONAL MATCH (t)-[:TARGETS]->(tgt:Element)
RETURN t, sources, collect(DISTINCT tgt.name) AS targets`
	return cypher, map[string]any{"id": id}
}

// recordToTrace converts a Neo4j node property map and collected source/target
// name slices into a schema.Trace. Returns an error if the timestamp cannot be
// parsed. Falls back from RFC3339Nano to RFC3339 for timestamps without
// sub-second precision.
func recordToTrace(props map[string]any, rawSources, rawTargets []any) (schema.Trace, error) {
	tsStr := asString(props["timestamp"])
	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		ts, err = time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return schema.Trace{}, fmt.Errorf("recordToTrace: parse timestamp %q: %w", tsStr, err)
		}
	}
	return schema.Trace{
		ID:          asString(props["id"]),
		Timestamp:   ts.UTC(),
		WhatChanged: asString(props["what_changed"]),
		Observer:    asString(props["observer"]),
		Mediation:   asString(props["mediation"]),
		Tags:        anyListToStrings(props["tags"]),
		Source:      anySliceToStrings(rawSources),
		Target:      anySliceToStrings(rawTargets),
	}, nil
}

// asString safely converts an any to string. Returns "" on nil or wrong type.
func asString(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// anyListToStrings converts a Neo4j list property (stored as []any of strings)
// to []string. Returns nil for nil, empty, or non-list input.
func anyListToStrings(v any) []string {
	if v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	return anySliceToStrings(list)
}

// anySliceToStrings converts a []any of strings (e.g. from collect(DISTINCT))
// to []string, skipping non-string and empty values. Returns nil when the
// result would be empty, preserving Trace.Source/Target nil-means-absent
// semantics.
func anySliceToStrings(vs []any) []string {
	if len(vs) == 0 {
		return nil
	}
	result := make([]string, 0, len(vs))
	for _, v := range vs {
		if s, ok := v.(string); ok && s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
