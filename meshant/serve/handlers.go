package serve

import (
	"net/http"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// handleArticulate handles GET /articulate.
//
// Required: ?observer=<string>
// Optional: ?from=RFC3339 ?to=RFC3339 ?tags=foo&tags=bar (repeatable, OR semantics)
//
// Loads the full trace substrate, articulates an observer-situated graph, and
// wraps it in an Envelope with cut metadata.
func (s *Server) handleArticulate(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	observer := q.Get("observer")
	if observer == "" {
		writeError(w, http.StatusBadRequest, "observer is required — every graph is a positioned reading")
		return
	}

	tw, err := parseQueryTime(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tags := q["tags"]

	traces, err := s.ts.Query(r.Context(), store.QueryOpts{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{observer},
		TimeWindow:        tw,
		Tags:              tags,
	}
	g := graph.Articulate(traces, opts)
	meta := cutMetaFromGraph(g)

	writeJSON(w, http.StatusOK, Envelope{Cut: meta, Data: g})
}

// handleDiff handles GET /diff.
//
// Required: ?observer-a=<string> ?observer-b=<string>
// Optional: ?from=RFC3339 ?to=RFC3339 ?tags=foo&tags=bar
//
// Articulates two cuts and returns their GraphDiff. The envelope cut is
// populated from observer-a's perspective (design decision D4 in serve-v1.md).
// Both full cuts are available inside the data payload.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	observerA := q.Get("observer-a")
	observerB := q.Get("observer-b")
	if observerA == "" || observerB == "" {
		writeError(w, http.StatusBadRequest, "diff requires two observer positions")
		return
	}

	tw, err := parseQueryTime(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tags := q["tags"]

	traces, err := s.ts.Query(r.Context(), store.QueryOpts{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	optsA := graph.ArticulationOptions{ObserverPositions: []string{observerA}, TimeWindow: tw, Tags: tags}
	optsB := graph.ArticulationOptions{ObserverPositions: []string{observerB}, TimeWindow: tw, Tags: tags}
	gA := graph.Articulate(traces, optsA)
	gB := graph.Articulate(traces, optsB)
	d := graph.Diff(gA, gB)

	// Envelope cut is observer-a's position (D4): the diff is read from A toward B.
	meta := cutMetaFromGraph(gA)

	writeJSON(w, http.StatusOK, Envelope{Cut: meta, Data: d})
}

// handleShadow handles GET /shadow.
//
// Required: ?observer=<string>
// Optional: ?from=RFC3339 ?to=RFC3339 ?tags=foo&tags=bar
//
// Returns the shadow elements for the given cut — elements visible from other
// observer positions but absent from this one. Shadow is never suppressed.
func (s *Server) handleShadow(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	observer := q.Get("observer")
	if observer == "" {
		writeError(w, http.StatusBadRequest, "observer is required — every graph is a positioned reading")
		return
	}

	tw, err := parseQueryTime(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tags := q["tags"]

	traces, err := s.ts.Query(r.Context(), store.QueryOpts{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{observer},
		TimeWindow:        tw,
		Tags:              tags,
	}
	g := graph.Articulate(traces, opts)
	meta := cutMetaFromGraph(g)

	// Return shadow elements as the data payload. Use an empty slice (not nil)
	// so the JSON response is always an array, never null.
	shadowElems := g.Cut.ShadowElements
	if shadowElems == nil {
		shadowElems = []graph.ShadowElement{}
	}

	writeJSON(w, http.StatusOK, Envelope{Cut: meta, Data: shadowElems})
}

// handleTraces handles GET /traces.
//
// Required: ?observer=<string>
// Optional: ?from=RFC3339 ?to=RFC3339 ?tags=foo&tags=bar ?limit=N
//
// Returns the raw traces recorded by the named observer. Does NOT call
// graph.Articulate — this is a substrate view, not a cut. The shadow_count
// in the envelope is approximate: total traces minus observer-filtered traces.
// This counts traces, not elements (ANT tension T2 in serve-v1.md).
func (s *Server) handleTraces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	observer := q.Get("observer")
	if observer == "" {
		writeError(w, http.StatusBadRequest, "observer is required — every reading is positioned")
		return
	}

	tw, err := parseQueryTime(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tags := q["tags"]

	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	allTraces, err := s.ts.Query(r.Context(), store.QueryOpts{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Filter by observer, time window, and tags in-memory (consistent with the
	// full-substrate design: no pre-filtering at the store layer).
	filtered := filterTraces(allTraces, observer, tw, tags)

	// shadow_count is an approximation: total traces minus observer-matching traces.
	// This counts traces, not elements; it differs from articulation shadow (T2).
	shadowCount := len(allTraces) - len(filtered)

	// Apply limit after filtering.
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	// Build cut metadata for the envelope.
	var fromPtr, toPtr *string
	if !tw.Start.IsZero() {
		fs := tw.Start.UTC().Format(time.RFC3339)
		fromPtr = &fs
	}
	if !tw.End.IsZero() {
		ts := tw.End.UTC().Format(time.RFC3339)
		toPtr = &ts
	}

	meta := CutMeta{
		Observer:    observer,
		From:        fromPtr,
		To:          toPtr,
		Tags:        tags,
		TraceCount:  len(filtered),
		ShadowCount: shadowCount,
	}

	// Use an empty slice (not nil) so the JSON response is always an array.
	if filtered == nil {
		filtered = []schema.Trace{}
	}

	writeJSON(w, http.StatusOK, Envelope{Cut: meta, Data: filtered})
}

// filterByElement returns traces where elementName appears in tr.Source or tr.Target.
// Matching is exact string equality on individual slice entries. Both Source and
// Target are checked so that any trace mentioning the named element is included
// regardless of the direction of the relationship.
func filterByElement(traces []schema.Trace, elementName string) []schema.Trace {
	var out []schema.Trace
	for _, tr := range traces {
		found := false
		for _, s := range tr.Source {
			if s == elementName {
				found = true
				break
			}
		}
		if !found {
			for _, t := range tr.Target {
				if t == elementName {
					found = true
					break
				}
			}
		}
		if found {
			out = append(out, tr)
		}
	}
	return out
}

// handleElement handles GET /element/{name}.
//
// Required: ?observer=<string>  (path variable: r.PathValue("name"))
// Optional: ?from=RFC3339 ?to=RFC3339 ?tags=foo&tags=bar
//
// Returns all traces in which the named element appears as source or target,
// filtered by the given observer position and optional time/tag constraints.
// Observer is required — element visibility is always positioned. A named element
// that is not found in any trace returns 200 with an empty array, not 404.
func (s *Server) handleElement(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	observer := q.Get("observer")
	if observer == "" {
		writeError(w, http.StatusBadRequest, "observer is required — every graph is a positioned reading")
		return
	}

	// Decode the path variable; net/http 1.22+ does percent-decoding automatically
	// via PathValue, so "element%2Da" correctly yields "element-a".
	elementName := r.PathValue("name")

	tw, err := parseQueryTime(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tags := q["tags"]

	allTraces, err := s.ts.Query(r.Context(), store.QueryOpts{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Apply observer, time window, and tag filters first (consistent with the
	// full-substrate design: all filtering is in-memory, no pre-filtering at store).
	observerFiltered := filterTraces(allTraces, observer, tw, tags)

	// Then narrow to traces that mention the named element.
	result := filterByElement(observerFiltered, elementName)

	// Nil-guard: always return a JSON array, never null.
	if result == nil {
		result = []schema.Trace{}
	}

	// Build cut metadata. shadow_count is approximate: total minus observer-filtered
	// (mirrors the /traces approximation — T2 in serve-v1.md applies here too).
	shadowCount := len(allTraces) - len(observerFiltered)
	var fromPtr, toPtr *string
	if !tw.Start.IsZero() {
		fs := tw.Start.UTC().Format(time.RFC3339)
		fromPtr = &fs
	}
	if !tw.End.IsZero() {
		ts := tw.End.UTC().Format(time.RFC3339)
		toPtr = &ts
	}

	meta := CutMeta{
		Observer:    observer,
		From:        fromPtr,
		To:          toPtr,
		Tags:        tags,
		TraceCount:  len(result),
		ShadowCount: shadowCount,
	}

	writeJSON(w, http.StatusOK, Envelope{Cut: meta, Data: result})
}

// filterTraces returns traces matching the given observer, time window, and tags.
// Tags use OR semantics (any tag matches) — consistent with graph.ArticulationOptions.
// Time window bounds are inclusive.
func filterTraces(traces []schema.Trace, observer string, tw graph.TimeWindow, tags []string) []schema.Trace {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	var out []schema.Trace
	for _, tr := range traces {
		if tr.Observer != observer {
			continue
		}
		if !tw.Start.IsZero() && tr.Timestamp.Before(tw.Start) {
			continue
		}
		if !tw.End.IsZero() && tr.Timestamp.After(tw.End) {
			continue
		}
		if len(tags) > 0 {
			matched := false
			for _, t := range tr.Tags {
				if tagSet[t] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, tr)
	}
	return out
}
