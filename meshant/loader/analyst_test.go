// analyst_test.go tests GroupByAnalyst — the function that partitions a
// slice of TraceDraft records by their ExtractedBy field. Each distinct
// ExtractedBy value represents an analyst position; the grouping makes
// cross-analyst disagreement visible as data, not noise.
package loader_test

import (
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// makeDraft constructs a minimal TraceDraft with only the fields GroupByAnalyst
// acts on: SourceSpan (used as a stable identity sentinel in assertions) and
// ExtractedBy (the grouping key).
func makeDraft(sourceSpan, extractedBy string) schema.TraceDraft {
	return schema.TraceDraft{
		SourceSpan:  sourceSpan,
		ExtractedBy: extractedBy,
	}
}

// TestGroupByAnalyst_EmptyInput verifies that an empty input slice produces
// a non-nil empty map. Callers should be able to range over the result
// unconditionally without a nil check.
func TestGroupByAnalyst_EmptyInput(t *testing.T) {
	result := loader.GroupByAnalyst(nil)
	if result == nil {
		t.Fatal("GroupByAnalyst(nil): expected non-nil map, got nil")
	}
	if len(result) != 0 {
		t.Fatalf("GroupByAnalyst(nil): expected empty map, got len=%d", len(result))
	}

	// Also test with explicit empty slice (not just nil).
	result2 := loader.GroupByAnalyst([]schema.TraceDraft{})
	if result2 == nil {
		t.Fatal("GroupByAnalyst([]TraceDraft{}): expected non-nil map, got nil")
	}
	if len(result2) != 0 {
		t.Fatalf("GroupByAnalyst([]TraceDraft{}): expected empty map, got len=%d", len(result2))
	}
}

// TestGroupByAnalyst_SingleAnalyst verifies that when all drafts share one
// ExtractedBy value, the result has exactly one key and all drafts appear
// under it.
func TestGroupByAnalyst_SingleAnalyst(t *testing.T) {
	drafts := []schema.TraceDraft{
		makeDraft("span-a", "human"),
		makeDraft("span-b", "human"),
		makeDraft("span-c", "human"),
	}

	result := loader.GroupByAnalyst(drafts)

	if len(result) != 1 {
		t.Fatalf("expected 1 key, got %d: %v", len(result), result)
	}

	group, ok := result["human"]
	if !ok {
		t.Fatal(`expected key "human" in result map`)
	}
	if len(group) != 3 {
		t.Fatalf(`expected 3 drafts under "human", got %d`, len(group))
	}
}

// TestGroupByAnalyst_TwoAnalysts verifies that two distinct ExtractedBy values
// produce two keys and that each draft is placed under the correct key.
func TestGroupByAnalyst_TwoAnalysts(t *testing.T) {
	drafts := []schema.TraceDraft{
		makeDraft("span-a", "human"),
		makeDraft("span-b", "llm-pass1"),
		makeDraft("span-c", "human"),
		makeDraft("span-d", "llm-pass1"),
	}

	result := loader.GroupByAnalyst(drafts)

	if len(result) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(result))
	}

	humanGroup, ok := result["human"]
	if !ok {
		t.Fatal(`expected key "human" in result map`)
	}
	if len(humanGroup) != 2 {
		t.Fatalf(`expected 2 drafts under "human", got %d`, len(humanGroup))
	}
	if humanGroup[0].SourceSpan != "span-a" || humanGroup[1].SourceSpan != "span-c" {
		t.Errorf(`"human" group order wrong: got spans %q, %q; want "span-a", "span-c"`,
			humanGroup[0].SourceSpan, humanGroup[1].SourceSpan)
	}

	llmGroup, ok := result["llm-pass1"]
	if !ok {
		t.Fatal(`expected key "llm-pass1" in result map`)
	}
	if len(llmGroup) != 2 {
		t.Fatalf(`expected 2 drafts under "llm-pass1", got %d`, len(llmGroup))
	}
	if llmGroup[0].SourceSpan != "span-b" || llmGroup[1].SourceSpan != "span-d" {
		t.Errorf(`"llm-pass1" group order wrong: got spans %q, %q; want "span-b", "span-d"`,
			llmGroup[0].SourceSpan, llmGroup[1].SourceSpan)
	}
}

// TestGroupByAnalyst_EmptyExtractedBy verifies that drafts with an empty
// ExtractedBy field are grouped under key "" (the empty string), not
// discarded. An analyst who didn't declare a position is still an analyst.
func TestGroupByAnalyst_EmptyExtractedBy(t *testing.T) {
	drafts := []schema.TraceDraft{
		makeDraft("span-a", ""),
		makeDraft("span-b", "human"),
		makeDraft("span-c", ""),
	}

	result := loader.GroupByAnalyst(drafts)

	if len(result) != 2 {
		t.Fatalf("expected 2 keys (\"\" and \"human\"), got %d", len(result))
	}

	emptyGroup, ok := result[""]
	if !ok {
		t.Fatal(`expected key "" in result map`)
	}
	if len(emptyGroup) != 2 {
		t.Fatalf(`expected 2 drafts under "", got %d`, len(emptyGroup))
	}

	humanGroup, ok := result["human"]
	if !ok {
		t.Fatal(`expected key "human" in result map`)
	}
	if len(humanGroup) != 1 {
		t.Fatalf(`expected 1 draft under "human", got %d`, len(humanGroup))
	}
}

// TestGroupByAnalyst_OrderPreservation verifies that drafts within each group
// appear in the same order as in the input slice. Encounter order is data:
// the sequence of drafts from a given analyst position may be meaningful.
func TestGroupByAnalyst_OrderPreservation(t *testing.T) {
	drafts := []schema.TraceDraft{
		makeDraft("first", "human"),
		makeDraft("second", "llm-pass1"),
		makeDraft("third", "human"),
		makeDraft("fourth", "llm-pass1"),
		makeDraft("fifth", "human"),
	}

	result := loader.GroupByAnalyst(drafts)

	humanGroup := result["human"]
	wantHuman := []string{"first", "third", "fifth"}
	if len(humanGroup) != len(wantHuman) {
		t.Fatalf("human group: expected %d drafts, got %d", len(wantHuman), len(humanGroup))
	}
	for i, want := range wantHuman {
		if humanGroup[i].SourceSpan != want {
			t.Errorf("human group[%d]: got %q, want %q", i, humanGroup[i].SourceSpan, want)
		}
	}

	llmGroup := result["llm-pass1"]
	wantLLM := []string{"second", "fourth"}
	if len(llmGroup) != len(wantLLM) {
		t.Fatalf("llm-pass1 group: expected %d drafts, got %d", len(wantLLM), len(llmGroup))
	}
	for i, want := range wantLLM {
		if llmGroup[i].SourceSpan != want {
			t.Errorf("llm-pass1 group[%d]: got %q, want %q", i, llmGroup[i].SourceSpan, want)
		}
	}
}

// TestGroupByAnalyst_ThreeAnalysts verifies that N>2 distinct ExtractedBy
// values all produce separate keys. Guards against implementations that could
// silently drop keys beyond the first two (e.g., a hardcoded two-bucket path).
func TestGroupByAnalyst_ThreeAnalysts(t *testing.T) {
	drafts := []schema.TraceDraft{
		makeDraft("span-a", "analyst-a"),
		makeDraft("span-b", "analyst-b"),
		makeDraft("span-c", "analyst-c"),
	}

	result := loader.GroupByAnalyst(drafts)

	if len(result) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(result), result)
	}
	for _, label := range []string{"analyst-a", "analyst-b", "analyst-c"} {
		if len(result[label]) != 1 {
			t.Errorf("key %q: expected 1 draft, got %d", label, len(result[label]))
		}
	}
}

// TestGroupByAnalyst_NoAliasing verifies that returned slices do not alias
// the input. Two properties are checked:
//
//  1. Length invariance: appending to a returned group slice does not change
//     len(drafts) — the returned slice does not share backing memory with the
//     input slice header.
//
//  2. Write isolation: mutating an element in a returned group does not affect
//     the corresponding element in the original input. Because TraceDraft is a
//     value type, this should hold by construction (append copies values), but
//     the test guards against future implementation changes (e.g., sub-slicing
//     an internal buffer) that would break the promise.
func TestGroupByAnalyst_NoAliasing(t *testing.T) {
	drafts := []schema.TraceDraft{
		makeDraft("span-a", "human"),
		makeDraft("span-b", "human"),
	}
	originalLen := len(drafts)
	originalSpan0 := drafts[0].SourceSpan

	result := loader.GroupByAnalyst(drafts)
	group := result["human"]

	// 1. Length invariance: append to returned slice must not grow input.
	_ = append(group, makeDraft("span-extra", "human"))
	if len(drafts) != originalLen {
		t.Fatalf("appending to returned group modified input slice: len(drafts) went from %d to %d",
			originalLen, len(drafts))
	}

	// 2. Write isolation: mutating a returned element must not affect input.
	group[0].SourceSpan = "mutated"
	if drafts[0].SourceSpan != originalSpan0 {
		t.Fatalf("mutating returned group element affected input: drafts[0].SourceSpan = %q, want %q",
			drafts[0].SourceSpan, originalSpan0)
	}

	// Re-call to confirm the internally stored slice is also unaffected.
	result2 := loader.GroupByAnalyst(drafts)
	if len(result2["human"]) != 2 {
		t.Fatalf("expected 2 drafts in re-called result, got %d", len(result2["human"]))
	}
}
