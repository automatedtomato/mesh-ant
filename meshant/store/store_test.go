package store_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// --- Interface compliance ---

// TestJSONFileStore_ImplementsTraceStore is a compile-time check that
// *JSONFileStore satisfies the TraceStore interface. If the interface or
// implementation diverge, this test fails to compile.
func TestJSONFileStore_ImplementsTraceStore(t *testing.T) {
	var _ store.TraceStore = store.NewJSONFileStore(tempPath(t))
}

// --- Query: basic ---

func TestQuery_EmptyOpts_ReturnsAllTraces(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
		validTrace("00000000-0000-0000-0000-000000000003", "change c"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() want no error, got %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 traces, got %d", len(got))
	}
}

func TestQuery_FileDoesNotExist_ReturnsEmptySlice(t *testing.T) {
	s := store.NewJSONFileStore(tempPath(t))
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() on nonexistent file: want no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty slice, got %d traces", len(got))
	}
}

func TestQuery_EmptyFile_ReturnsEmptySlice(t *testing.T) {
	s := store.NewJSONFileStore(writeTempJSON(t, []schema.Trace{}))
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() on empty file: want no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty slice, got %d traces", len(got))
	}
}

// --- Query: Observer filter ---

func TestQuery_Observer_MatchesExact(t *testing.T) {
	traces := []schema.Trace{
		validTraceWithObserver("00000000-0000-0000-0000-000000000001", "change a", "observer/alpha"),
		validTraceWithObserver("00000000-0000-0000-0000-000000000002", "change b", "observer/alpha"),
		validTraceWithObserver("00000000-0000-0000-0000-000000000003", "change c", "observer/beta"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Observer: "observer/alpha"})
	if err != nil {
		t.Fatalf("Query() observer filter: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 traces for observer/alpha, got %d", len(got))
	}
}

func TestQuery_Observer_NoMatch_ReturnsEmpty(t *testing.T) {
	traces := []schema.Trace{
		validTraceWithObserver("00000000-0000-0000-0000-000000000001", "change a", "observer/alpha"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Observer: "observer/nobody"})
	if err != nil {
		t.Fatalf("Query() observer no match: want no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty, got %d", len(got))
	}
}

func TestQuery_Observer_EmptyString_NoFilter(t *testing.T) {
	traces := []schema.Trace{
		validTraceWithObserver("00000000-0000-0000-0000-000000000001", "change a", "observer/alpha"),
		validTraceWithObserver("00000000-0000-0000-0000-000000000002", "change b", "observer/beta"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Observer: ""})
	if err != nil {
		t.Fatalf("Query() empty observer: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("empty observer should return all 2 traces, got %d", len(got))
	}
}

// --- Query: TimeWindow filter ---

func TestQuery_Window_StartOnly_ExcludesEarlier(t *testing.T) {
	day1 := baseTime
	day2 := baseTime.Add(24 * time.Hour)
	day3 := baseTime.Add(48 * time.Hour)
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", day1),
		validTraceAt("00000000-0000-0000-0000-000000000002", "change b", day2),
		validTraceAt("00000000-0000-0000-0000-000000000003", "change c", day3),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{Start: day2},
	})
	if err != nil {
		t.Fatalf("Query() window Start: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 traces (day2, day3), got %d", len(got))
	}
}

func TestQuery_Window_EndOnly_ExcludesLater(t *testing.T) {
	day1 := baseTime
	day2 := baseTime.Add(24 * time.Hour)
	day3 := baseTime.Add(48 * time.Hour)
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", day1),
		validTraceAt("00000000-0000-0000-0000-000000000002", "change b", day2),
		validTraceAt("00000000-0000-0000-0000-000000000003", "change c", day3),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{End: day2},
	})
	if err != nil {
		t.Fatalf("Query() window End: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 traces (day1, day2), got %d", len(got))
	}
}

func TestQuery_Window_BothBounds_IncludesInRange(t *testing.T) {
	day1 := baseTime
	day2 := baseTime.Add(24 * time.Hour)
	day3 := baseTime.Add(48 * time.Hour)
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", day1),
		validTraceAt("00000000-0000-0000-0000-000000000002", "change b", day2),
		validTraceAt("00000000-0000-0000-0000-000000000003", "change c", day3),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{Start: day2, End: day2},
	})
	if err != nil {
		t.Fatalf("Query() window both bounds: want no error, got %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace (day2 only), got %d", len(got))
	}
}

func TestQuery_Window_InclusiveBounds_ExactMatch(t *testing.T) {
	ts := baseTime
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", ts),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	// Trace timestamp exactly equals both Start and End — must be included.
	got, err := s.Query(context.Background(), store.QueryOpts{
		Window: graph.TimeWindow{Start: ts, End: ts},
	})
	if err != nil {
		t.Fatalf("Query() window inclusive exact: want no error, got %v", err)
	}
	if len(got) != 1 {
		t.Errorf("window [ts, ts] must include trace at ts, got %d", len(got))
	}
}

func TestQuery_Window_Zero_NoFilter(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Window: graph.TimeWindow{}})
	if err != nil {
		t.Fatalf("Query() zero window: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("zero window must return all 2 traces, got %d", len(got))
	}
}

// --- Query: Tags filter (ALL/AND semantics) ---

func TestQuery_Tags_SingleTag_Matches(t *testing.T) {
	traces := []schema.Trace{
		validTraceWithTags("00000000-0000-0000-0000-000000000001", "change a", []string{"delay", "threshold"}),
		validTraceWithTags("00000000-0000-0000-0000-000000000002", "change b", []string{"other"}),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Tags: []string{"delay"}})
	if err != nil {
		t.Fatalf("Query() single tag: want no error, got %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace with tag 'delay', got %d", len(got))
	}
}

func TestQuery_Tags_AllRequired_BothPresent(t *testing.T) {
	traces := []schema.Trace{
		validTraceWithTags("00000000-0000-0000-0000-000000000001", "change a", []string{"delay", "threshold"}),
		validTraceWithTags("00000000-0000-0000-0000-000000000002", "change b", []string{"delay"}),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Tags: []string{"delay", "threshold"}})
	if err != nil {
		t.Fatalf("Query() all tags: want no error, got %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace with both 'delay' and 'threshold', got %d", len(got))
	}
}

func TestQuery_Tags_PartialMatch_Excluded(t *testing.T) {
	// Trace has only one of the required tags — must be excluded (AND semantics).
	traces := []schema.Trace{
		validTraceWithTags("00000000-0000-0000-0000-000000000001", "change a", []string{"delay"}),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Tags: []string{"delay", "threshold"}})
	if err != nil {
		t.Fatalf("Query() partial tags: want no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("trace with only 'delay' should be excluded when both required, got %d", len(got))
	}
}

func TestQuery_Tags_Empty_NoFilter(t *testing.T) {
	traces := []schema.Trace{
		validTraceWithTags("00000000-0000-0000-0000-000000000001", "change a", []string{"delay"}),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Tags: nil})
	if err != nil {
		t.Fatalf("Query() empty tags: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("empty tags must return all 2 traces, got %d", len(got))
	}
}

// --- Query: Limit ---

func TestQuery_Limit_CapsResults(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
		validTrace("00000000-0000-0000-0000-000000000003", "change c"),
		validTrace("00000000-0000-0000-0000-000000000004", "change d"),
		validTrace("00000000-0000-0000-0000-000000000005", "change e"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Limit: 3})
	if err != nil {
		t.Fatalf("Query() limit: want no error, got %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 traces (limit), got %d", len(got))
	}
}

func TestQuery_Limit_ZeroMeansNoLimit(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Limit: 0})
	if err != nil {
		t.Fatalf("Query() limit=0: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("limit=0 must return all 2 traces, got %d", len(got))
	}
}

func TestQuery_Limit_LargerThanDataset_ReturnsAll(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{Limit: 100})
	if err != nil {
		t.Fatalf("Query() limit larger than dataset: want no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("limit 100 with 2 traces must return all 2, got %d", len(got))
	}
}

// --- Query: Combined filters (AND semantics) ---

func TestQuery_ObserverAndWindow_AND(t *testing.T) {
	day1 := baseTime
	day2 := baseTime.Add(24 * time.Hour)
	traces := []schema.Trace{
		{
			ID: "00000000-0000-0000-0000-000000000001", Timestamp: day1,
			WhatChanged: "change a", Observer: "observer/alpha",
		},
		{
			ID: "00000000-0000-0000-0000-000000000002", Timestamp: day2,
			WhatChanged: "change b", Observer: "observer/alpha",
		},
		{
			ID: "00000000-0000-0000-0000-000000000003", Timestamp: day1,
			WhatChanged: "change c", Observer: "observer/beta",
		},
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{
		Observer: "observer/alpha",
		Window:   graph.TimeWindow{End: day1},
	})
	if err != nil {
		t.Fatalf("Query() observer+window: want no error, got %v", err)
	}
	// Only trace 1: observer/alpha AND timestamp<=day1.
	if len(got) != 1 {
		t.Errorf("want 1 trace (observer/alpha + End=day1), got %d", len(got))
	}
}

func TestQuery_ObserverAndTags_AND(t *testing.T) {
	traces := []schema.Trace{
		{
			ID: "00000000-0000-0000-0000-000000000001", Timestamp: baseTime,
			WhatChanged: "change a", Observer: "observer/alpha", Tags: []string{"delay"},
		},
		{
			ID: "00000000-0000-0000-0000-000000000002", Timestamp: baseTime,
			WhatChanged: "change b", Observer: "observer/alpha",
		},
		{
			ID: "00000000-0000-0000-0000-000000000003", Timestamp: baseTime,
			WhatChanged: "change c", Observer: "observer/beta", Tags: []string{"delay"},
		},
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{
		Observer: "observer/alpha",
		Tags:     []string{"delay"},
	})
	if err != nil {
		t.Fatalf("Query() observer+tags: want no error, got %v", err)
	}
	// Only trace 1: observer/alpha AND has tag "delay".
	if len(got) != 1 {
		t.Errorf("want 1 trace (observer/alpha + tag delay), got %d", len(got))
	}
}

func TestQuery_AllFilters_AND(t *testing.T) {
	day1 := baseTime
	day2 := baseTime.Add(24 * time.Hour)
	traces := []schema.Trace{
		// Matches all filters
		{
			ID: "00000000-0000-0000-0000-000000000001", Timestamp: day1,
			WhatChanged: "change a", Observer: "observer/alpha", Tags: []string{"delay"},
		},
		// Wrong observer
		{
			ID: "00000000-0000-0000-0000-000000000002", Timestamp: day1,
			WhatChanged: "change b", Observer: "observer/beta", Tags: []string{"delay"},
		},
		// Outside window
		{
			ID: "00000000-0000-0000-0000-000000000003", Timestamp: day2,
			WhatChanged: "change c", Observer: "observer/alpha", Tags: []string{"delay"},
		},
		// Missing tag
		{
			ID: "00000000-0000-0000-0000-000000000004", Timestamp: day1,
			WhatChanged: "change d", Observer: "observer/alpha",
		},
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, err := s.Query(context.Background(), store.QueryOpts{
		Observer: "observer/alpha",
		Window:   graph.TimeWindow{End: day1},
		Tags:     []string{"delay"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Query() all filters: want no error, got %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trace matching all filters, got %d", len(got))
	}
	if got[0].ID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("want trace 001, got %q", got[0].ID)
	}
}

// --- Get ---

func TestGet_Found_ReturnsTrace(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
		validTrace("00000000-0000-0000-0000-000000000003", "change c"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	got, found, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000002")
	if err != nil {
		t.Fatalf("Get() found: want no error, got %v", err)
	}
	if !found {
		t.Fatal("Get() found: want found=true, got false")
	}
	if got.ID != "00000000-0000-0000-0000-000000000002" {
		t.Errorf("Get() found: want ID 002, got %q", got.ID)
	}
}

func TestGet_NotFound_ReturnsFalse(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	_, found, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000099")
	if err != nil {
		t.Fatalf("Get() not found: want no error, got %v", err)
	}
	if found {
		t.Fatal("Get() not found: want found=false, got true")
	}
}

func TestGet_FileDoesNotExist_ReturnsFalse(t *testing.T) {
	s := store.NewJSONFileStore(tempPath(t))
	_, found, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("Get() on nonexistent file: want no error, got %v", err)
	}
	if found {
		t.Fatal("Get() on nonexistent file: want found=false, got true")
	}
}

// --- Store ---

func TestStore_WritesNewFile(t *testing.T) {
	path := tempPath(t)
	s := store.NewJSONFileStore(path)
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
	}
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store() new file: want no error, got %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after Store: %v", err)
	}
	var got []schema.Trace
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal after Store: %v", err)
	}
	if len(got) != 1 || got[0].ID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("Store() new file: want 1 trace, got %v", got)
	}
}

func TestStore_AppendsToExisting(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	path := writeTempJSON(t, traces)
	s := store.NewJSONFileStore(path)
	newTraces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000003", "change c"),
		validTrace("00000000-0000-0000-0000-000000000004", "change d"),
	}
	if err := s.Store(context.Background(), newTraces); err != nil {
		t.Fatalf("Store() append: want no error, got %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() after append: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("Store() append: want 4 traces total, got %d", len(got))
	}
}

func TestStore_UpsertOnID_UpdatesExisting(t *testing.T) {
	original := validTrace("00000000-0000-0000-0000-000000000001", "original change")
	path := writeTempJSON(t, []schema.Trace{original})
	s := store.NewJSONFileStore(path)

	updated := validTrace("00000000-0000-0000-0000-000000000001", "updated change")
	if err := s.Store(context.Background(), []schema.Trace{updated}); err != nil {
		t.Fatalf("Store() upsert: want no error, got %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() after upsert: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Store() upsert: want 1 trace (no duplicate), got %d", len(got))
	}
	if got[0].WhatChanged != "updated change" {
		t.Errorf("Store() upsert: want WhatChanged='updated change', got %q", got[0].WhatChanged)
	}
}

func TestStore_InvalidTrace_ReturnsError(t *testing.T) {
	s := store.NewJSONFileStore(tempPath(t))
	bad := schema.Trace{} // empty ID — fails Validate()
	if err := s.Store(context.Background(), []schema.Trace{bad}); err == nil {
		t.Fatal("Store() with invalid trace: want error, got nil")
	}
}

// TestStore_InvalidBatch_FileUnmodified verifies the atomicity guarantee:
// if any trace in a batch is invalid, the pre-existing file is not changed.
func TestStore_InvalidBatch_FileUnmodified(t *testing.T) {
	existing := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "original"),
	}
	path := writeTempJSON(t, existing)
	originalData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile before Store: %v", err)
	}

	s := store.NewJSONFileStore(path)
	// Batch with one valid and one invalid trace.
	bad := schema.Trace{} // empty ID — fails Validate()
	storeErr := s.Store(context.Background(), []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000002", "new"),
		bad,
	})
	if storeErr == nil {
		t.Fatal("Store() with invalid batch: want error, got nil")
	}
	afterData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after failed Store: %v", err)
	}
	if string(afterData) != string(originalData) {
		t.Error("Store() with invalid batch: file was modified; atomicity guarantee violated")
	}
}

func TestStore_EmptySlice_DoesNotCreateFile(t *testing.T) {
	path := tempPath(t)
	s := store.NewJSONFileStore(path)
	if err := s.Store(context.Background(), []schema.Trace{}); err != nil {
		t.Fatalf("Store() empty slice: want no error, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Store() with empty slice should not create the file")
	}
}

func TestStore_PreservesExistingOnUpsert(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
		validTrace("00000000-0000-0000-0000-000000000003", "change c"),
	}
	path := writeTempJSON(t, traces)
	s := store.NewJSONFileStore(path)

	// Upsert trace 002 only.
	updated := validTrace("00000000-0000-0000-0000-000000000002", "change b updated")
	if err := s.Store(context.Background(), []schema.Trace{updated}); err != nil {
		t.Fatalf("Store() preserve: want no error, got %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() after upsert preserve: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Store() preserve: want 3 traces, got %d", len(got))
	}
}

func TestStore_SortsByTimestamp(t *testing.T) {
	// Store traces out of time order, verify they come back sorted.
	day3 := baseTime.Add(48 * time.Hour)
	day1 := baseTime
	day2 := baseTime.Add(24 * time.Hour)
	traces := []schema.Trace{
		validTraceAt("00000000-0000-0000-0000-000000000003", "change c", day3),
		validTraceAt("00000000-0000-0000-0000-000000000001", "change a", day1),
		validTraceAt("00000000-0000-0000-0000-000000000002", "change b", day2),
	}
	path := tempPath(t)
	s := store.NewJSONFileStore(path)
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store() sort: want no error, got %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() after sort store: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 traces, got %d", len(got))
	}
	// Verify ascending order by timestamp.
	if !got[0].Timestamp.Equal(day1) || !got[1].Timestamp.Equal(day2) || !got[2].Timestamp.Equal(day3) {
		t.Errorf("Store() sort: want chronological order, got timestamps: %v, %v, %v",
			got[0].Timestamp, got[1].Timestamp, got[2].Timestamp)
	}
}

// --- Close ---

func TestClose_NoOp_NoError(t *testing.T) {
	s := store.NewJSONFileStore(tempPath(t))
	if err := s.Close(); err != nil {
		t.Fatalf("Close(): want no error, got %v", err)
	}
}

func TestClose_MultipleCallsSafe(t *testing.T) {
	s := store.NewJSONFileStore(tempPath(t))
	if err := s.Close(); err != nil {
		t.Fatalf("Close() first: want no error, got %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() second: want no error, got %v", err)
	}
}

// --- Malformed file ---

// writeMalformedJSON creates a file with invalid JSON content and returns its path.
func writeMalformedJSON(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{broken`), 0o644); err != nil {
		t.Fatalf("writeMalformedJSON: %v", err)
	}
	return path
}

// TestQuery_MalformedFile_ReturnsError verifies that a malformed JSON file
// causes Query to return an error rather than an empty slice.
func TestQuery_MalformedFile_ReturnsError(t *testing.T) {
	s := store.NewJSONFileStore(writeMalformedJSON(t))
	_, err := s.Query(context.Background(), store.QueryOpts{})
	if err == nil {
		t.Fatal("Query() on malformed JSON file: want error, got nil")
	}
}

// TestGet_MalformedFile_ReturnsError verifies Get returns an error on a malformed file.
func TestGet_MalformedFile_ReturnsError(t *testing.T) {
	s := store.NewJSONFileStore(writeMalformedJSON(t))
	_, _, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err == nil {
		t.Fatal("Get() on malformed JSON file: want error, got nil")
	}
}

// TestStore_MalformedExistingFile_ReturnsError verifies Store returns an error
// when the existing file is malformed (cannot load for upsert merge).
func TestStore_MalformedExistingFile_ReturnsError(t *testing.T) {
	s := store.NewJSONFileStore(writeMalformedJSON(t))
	if err := s.Store(context.Background(), []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
	}); err == nil {
		t.Fatal("Store() with malformed existing file: want error, got nil")
	}
}

// --- Round-trip ---

func TestStoreAndQuery_RoundTrip(t *testing.T) {
	original := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
		validTrace("00000000-0000-0000-0000-000000000002", "change b"),
	}
	path := tempPath(t)
	s := store.NewJSONFileStore(path)
	if err := s.Store(context.Background(), original); err != nil {
		t.Fatalf("Store() round-trip: %v", err)
	}
	got, err := s.Query(context.Background(), store.QueryOpts{})
	if err != nil {
		t.Fatalf("Query() round-trip: %v", err)
	}
	if len(got) != len(original) {
		t.Fatalf("round-trip: want %d traces, got %d", len(original), len(got))
	}
	for i, tr := range original {
		if got[i].ID != tr.ID || got[i].WhatChanged != tr.WhatChanged {
			t.Errorf("round-trip trace %d: want {%s,%s}, got {%s,%s}",
				i, tr.ID, tr.WhatChanged, got[i].ID, got[i].WhatChanged)
		}
	}
}

func TestStoreAndGet_RoundTrip(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
	}
	path := tempPath(t)
	s := store.NewJSONFileStore(path)
	if err := s.Store(context.Background(), traces); err != nil {
		t.Fatalf("Store() Get round-trip: %v", err)
	}
	got, found, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("Get() round-trip: %v", err)
	}
	if !found {
		t.Fatal("Get() round-trip: want found, got false")
	}
	if got.WhatChanged != "change a" {
		t.Errorf("Get() round-trip: want 'change a', got %q", got.WhatChanged)
	}
}

// TestStoreAndQuery_RoundTrip_PreservesAllFields verifies that Tags and Observer
// survive the JSON marshal/unmarshal cycle. A missing json struct tag on either
// field would cause this test to fail.
func TestStoreAndQuery_RoundTrip_PreservesAllFields(t *testing.T) {
	original := schema.Trace{
		ID:          "00000000-0000-0000-0000-000000000001",
		Timestamp:   baseTime,
		WhatChanged: "change with tags",
		Observer:    "analyst/field-position",
		Tags:        []string{"delay", "threshold"},
	}
	path := tempPath(t)
	s := store.NewJSONFileStore(path)
	if err := s.Store(context.Background(), []schema.Trace{original}); err != nil {
		t.Fatalf("Store() full-fields round-trip: %v", err)
	}
	got, found, err := s.Get(context.Background(), original.ID)
	if err != nil {
		t.Fatalf("Get() full-fields round-trip: %v", err)
	}
	if !found {
		t.Fatal("Get() full-fields round-trip: want found, got false")
	}
	if got.Observer != original.Observer {
		t.Errorf("Observer not preserved: want %q, got %q", original.Observer, got.Observer)
	}
	if len(got.Tags) != len(original.Tags) {
		t.Fatalf("Tags length not preserved: want %d, got %d", len(original.Tags), len(got.Tags))
	}
	for i, tag := range original.Tags {
		if got.Tags[i] != tag {
			t.Errorf("Tags[%d] not preserved: want %q, got %q", i, tag, got.Tags[i])
		}
	}
}

// --- Context cancellation ---

func TestQuery_CancelledContext_ReturnsError(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before call
	_, err := s.Query(ctx, store.QueryOpts{})
	if err == nil {
		t.Fatal("Query() with cancelled context: want error, got nil")
	}
}

func TestStore_CancelledContext_ReturnsError(t *testing.T) {
	s := store.NewJSONFileStore(tempPath(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Store(ctx, []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
	})
	if err == nil {
		t.Fatal("Store() with cancelled context: want error, got nil")
	}
}

func TestGet_CancelledContext_ReturnsError(t *testing.T) {
	traces := []schema.Trace{
		validTrace("00000000-0000-0000-0000-000000000001", "change a"),
	}
	s := store.NewJSONFileStore(writeTempJSON(t, traces))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := s.Get(ctx, "00000000-0000-0000-0000-000000000001")
	if err == nil {
		t.Fatal("Get() with cancelled context: want error, got nil")
	}
}
