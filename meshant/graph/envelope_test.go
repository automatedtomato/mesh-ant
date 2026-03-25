// Package graph_test provides black-box tests for graph/envelope.go.
//
// Tests exercise CutMetaFromGraph directly and verify that:
//   - Observer is populated from g.Cut.ObserverPositions[0]
//   - From/To are RFC3339 strings when the TimeWindow bound is non-zero, nil otherwise
//   - Tags are carried through unmodified
//   - TraceCount matches Cut.TracesIncluded
//   - ShadowCount matches len(Cut.ShadowElements)
//   - Analyst is always empty from CutMetaFromGraph (callers set it explicitly)
//   - Analyst omitempty: absent from JSON when empty, present when non-empty
//   - Envelope JSON round-trips correctly
package graph_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// makeGraphWithCut constructs a minimal MeshGraph populated only from the
// fields that CutMetaFromGraph reads. All other fields are zero values.
func makeGraphWithCut(
	observerPositions []string,
	start, end time.Time,
	tags []string,
	tracesIncluded int,
	shadowElements []graph.ShadowElement,
) graph.MeshGraph {
	return graph.MeshGraph{
		Nodes: map[string]graph.Node{},
		Edges: []graph.Edge{},
		Cut: graph.Cut{
			ObserverPositions: observerPositions,
			TimeWindow:        graph.TimeWindow{Start: start, End: end},
			Tags:              tags,
			TracesIncluded:    tracesIncluded,
			ShadowElements:    shadowElements,
		},
	}
}

// ---- Observer field -----------------------------------------------------------

func TestCutMetaFromGraph_ObserverPopulated(t *testing.T) {
	g := makeGraphWithCut([]string{"alice"}, time.Time{}, time.Time{}, nil, 0, nil)
	meta := graph.CutMetaFromGraph(g)
	if meta.Observer != "alice" {
		t.Errorf("Observer = %q, want %q", meta.Observer, "alice")
	}
}

func TestCutMetaFromGraph_ObserverMultiplePositions(t *testing.T) {
	// When ObserverPositions has more than one entry, only the first is used.
	// This truncation is deliberate: the single-observer HTTP API always
	// produces exactly one position. The contract is documented in the godoc.
	g := makeGraphWithCut([]string{"alice", "bob"}, time.Time{}, time.Time{}, nil, 0, nil)
	meta := graph.CutMetaFromGraph(g)
	if meta.Observer != "alice" {
		t.Errorf("Observer = %q, want %q (only first position used)", meta.Observer, "alice")
	}
}

func TestCutMetaFromGraph_ObserverEmpty(t *testing.T) {
	// No ObserverPositions — full cut, no observer filter.
	g := makeGraphWithCut(nil, time.Time{}, time.Time{}, nil, 0, nil)
	meta := graph.CutMetaFromGraph(g)
	if meta.Observer != "" {
		t.Errorf("Observer = %q, want empty string", meta.Observer)
	}
}

// ---- TimeWindow / From / To --------------------------------------------------

func TestCutMetaFromGraph_TimeWindowBothBounds(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	g := makeGraphWithCut([]string{"alice"}, start, end, nil, 0, nil)
	meta := graph.CutMetaFromGraph(g)

	if meta.From == nil {
		t.Fatal("From is nil, want RFC3339 string")
	}
	if *meta.From != start.Format(time.RFC3339) {
		t.Errorf("From = %q, want %q", *meta.From, start.Format(time.RFC3339))
	}
	if meta.To == nil {
		t.Fatal("To is nil, want RFC3339 string")
	}
	if *meta.To != end.Format(time.RFC3339) {
		t.Errorf("To = %q, want %q", *meta.To, end.Format(time.RFC3339))
	}
}

func TestCutMetaFromGraph_TimeWindowZero(t *testing.T) {
	// Zero TimeWindow — both bounds should be nil in the output.
	g := makeGraphWithCut([]string{"alice"}, time.Time{}, time.Time{}, nil, 0, nil)
	meta := graph.CutMetaFromGraph(g)

	if meta.From != nil {
		t.Errorf("From = %v, want nil (zero time window)", meta.From)
	}
	if meta.To != nil {
		t.Errorf("To = %v, want nil (zero time window)", meta.To)
	}
}

func TestCutMetaFromGraph_TimeWindowHalfOpenStart(t *testing.T) {
	// Only Start set — From should be non-nil, To should be nil.
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	g := makeGraphWithCut([]string{"alice"}, start, time.Time{}, nil, 0, nil)
	meta := graph.CutMetaFromGraph(g)

	if meta.From == nil {
		t.Fatal("From is nil, want RFC3339 string")
	}
	if *meta.From != start.Format(time.RFC3339) {
		t.Errorf("From = %q, want %q", *meta.From, start.Format(time.RFC3339))
	}
	if meta.To != nil {
		t.Errorf("To = %v, want nil (End is zero)", meta.To)
	}
}

func TestCutMetaFromGraph_TimeWindowHalfOpenEnd(t *testing.T) {
	// Only End set — From should be nil, To should be non-nil.
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	g := makeGraphWithCut([]string{"alice"}, time.Time{}, end, nil, 0, nil)
	meta := graph.CutMetaFromGraph(g)

	if meta.From != nil {
		t.Errorf("From = %v, want nil (Start is zero)", meta.From)
	}
	if meta.To == nil {
		t.Fatal("To is nil, want RFC3339 string")
	}
	if *meta.To != end.Format(time.RFC3339) {
		t.Errorf("To = %q, want %q", *meta.To, end.Format(time.RFC3339))
	}
}

// ---- Tags -------------------------------------------------------------------

func TestCutMetaFromGraph_TagsPopulated(t *testing.T) {
	tags := []string{"structural", "conflict"}
	g := makeGraphWithCut([]string{"alice"}, time.Time{}, time.Time{}, tags, 0, nil)
	meta := graph.CutMetaFromGraph(g)

	if len(meta.Tags) != 2 {
		t.Fatalf("len(Tags) = %d, want 2", len(meta.Tags))
	}
	if meta.Tags[0] != "structural" || meta.Tags[1] != "conflict" {
		t.Errorf("Tags = %v, want [structural conflict]", meta.Tags)
	}
}

func TestCutMetaFromGraph_TagsNil(t *testing.T) {
	g := makeGraphWithCut([]string{"alice"}, time.Time{}, time.Time{}, nil, 0, nil)
	meta := graph.CutMetaFromGraph(g)

	if meta.Tags != nil {
		t.Errorf("Tags = %v, want nil", meta.Tags)
	}
}

func TestCutMeta_TagsOmitempty_NilIsAbsent(t *testing.T) {
	// When Tags is nil (no filter), the JSON key must be absent.
	meta := graph.CutMeta{Observer: "alice", Tags: nil}
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := raw["tags"]; ok {
		t.Errorf("JSON contains 'tags' key, want absent when Tags is nil")
	}
}

func TestCutMeta_TagsOmitempty_EmptySliceIsAbsent(t *testing.T) {
	// omitempty on []string also silences a non-nil empty slice, so both nil
	// and []string{} produce no "tags" key in JSON. This test locks in that
	// contract so callers know not to rely on nil vs. empty distinction in JSON.
	meta := graph.CutMeta{Observer: "alice", Tags: []string{}}
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := raw["tags"]; ok {
		t.Errorf("JSON contains 'tags' key, want absent when Tags is empty slice")
	}
}

// ---- TraceCount / ShadowCount -----------------------------------------------

func TestCutMetaFromGraph_TraceCount(t *testing.T) {
	g := makeGraphWithCut([]string{"alice"}, time.Time{}, time.Time{}, nil, 17, nil)
	meta := graph.CutMetaFromGraph(g)

	if meta.TraceCount != 17 {
		t.Errorf("TraceCount = %d, want 17", meta.TraceCount)
	}
}

func TestCutMetaFromGraph_ShadowCount(t *testing.T) {
	shadows := []graph.ShadowElement{
		{Name: "elem-x", SeenFrom: []string{"bob"}, Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
		{Name: "elem-y", SeenFrom: []string{"carol"}, Reasons: []graph.ShadowReason{graph.ShadowReasonObserver}},
	}
	g := makeGraphWithCut([]string{"alice"}, time.Time{}, time.Time{}, nil, 5, shadows)
	meta := graph.CutMetaFromGraph(g)

	if meta.ShadowCount != 2 {
		t.Errorf("ShadowCount = %d, want 2", meta.ShadowCount)
	}
}

// ---- Analyst field ----------------------------------------------------------

func TestCutMetaFromGraph_AnalystAlwaysEmpty(t *testing.T) {
	// CutMetaFromGraph must never set Analyst; callers set it explicitly.
	g := makeGraphWithCut([]string{"alice"}, time.Time{}, time.Time{}, nil, 3, nil)
	meta := graph.CutMetaFromGraph(g)

	if meta.Analyst != "" {
		t.Errorf("Analyst = %q, want empty (callers set it explicitly)", meta.Analyst)
	}
}

func TestCutMeta_AnalystOmitempty_EmptyIsAbsent(t *testing.T) {
	// When Analyst is empty the JSON key must be absent (omitempty).
	meta := graph.CutMeta{
		Observer:    "alice",
		Analyst:     "",
		TraceCount:  1,
		ShadowCount: 0,
	}
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := raw["analyst"]; ok {
		t.Errorf("JSON contains 'analyst' key, want absent when Analyst is empty")
	}
}

func TestCutMeta_AnalystOmitempty_NonEmptyIsPresent(t *testing.T) {
	// When Analyst is non-empty the JSON key must appear.
	meta := graph.CutMeta{
		Observer:    "alice",
		Analyst:     "dr-smith",
		TraceCount:  1,
		ShadowCount: 0,
	}
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	v, ok := raw["analyst"]
	if !ok {
		t.Fatal("JSON missing 'analyst' key, want present when Analyst is non-empty")
	}
	if v != "dr-smith" {
		t.Errorf("analyst = %v, want %q", v, "dr-smith")
	}
}

// ---- Envelope JSON round-trip -----------------------------------------------

func TestEnvelope_JSONRoundTrip(t *testing.T) {
	// Verify that Envelope marshals and unmarshals faithfully.
	env := graph.Envelope{
		Cut: graph.CutMeta{
			Observer:    "alice",
			Analyst:     "dr-smith",
			TraceCount:  3,
			ShadowCount: 1,
		},
		Data: "payload",
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Decode into a generic map to inspect top-level structure.
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if _, ok := raw["cut"]; !ok {
		t.Error("JSON missing 'cut' key")
	}
	if _, ok := raw["data"]; !ok {
		t.Error("JSON missing 'data' key")
	}

	// Check that cut fields are nested correctly.
	cutMap, ok := raw["cut"].(map[string]interface{})
	if !ok {
		t.Fatalf("cut is not a JSON object: %T", raw["cut"])
	}
	if cutMap["observer"] != "alice" {
		t.Errorf("cut.observer = %v, want alice", cutMap["observer"])
	}
	if cutMap["analyst"] != "dr-smith" {
		t.Errorf("cut.analyst = %v, want dr-smith", cutMap["analyst"])
	}

	// Verify data round-trips.
	if raw["data"] != "payload" {
		t.Errorf("data = %v, want %q", raw["data"], "payload")
	}
}
