// analyst.go provides GroupByAnalyst — a function that partitions a slice of
// TraceDraft records by their ExtractedBy field. Each distinct ExtractedBy
// value represents an analyst position on the source material; grouping by it
// makes cross-analyst disagreement visible as data rather than noise.
//
// This is a pure data-partitioning function. It does not validate, filter, or
// reorder drafts — it preserves encounter order within each group.
package loader

import "github.com/automatedtomato/mesh-ant/meshant/schema"

// GroupByAnalyst partitions drafts by their ExtractedBy field and returns a
// map from analyst-position string to the drafts attributed to that position.
//
// Rules:
//   - Drafts with an empty ExtractedBy are grouped under key "" (not discarded).
//     An undeclared analyst position is still a position.
//   - The returned map is never nil; an empty input produces a non-nil empty map.
//   - Drafts within each group appear in the same order as in the input slice.
//   - Returned slices do not alias the input — callers may append to a group
//     without affecting the original drafts slice.
//
// Two drafts with different ExtractedBy values for the same SourceSpan
// represent two analyst positions on the same material. Their disagreement is
// data, not error — it is the raw material of comparative analysis.
func GroupByAnalyst(drafts []schema.TraceDraft) map[string][]schema.TraceDraft {
	// Allocate the result map eagerly so the return value is never nil.
	result := make(map[string][]schema.TraceDraft)

	for _, d := range drafts {
		// Append to a new or existing group. Because we use append on a
		// per-key slice rather than slicing into the input, each group's
		// backing array is independent of the input slice.
		result[d.ExtractedBy] = append(result[d.ExtractedBy], d)
	}

	return result
}
