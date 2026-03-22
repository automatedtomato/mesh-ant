// analyst.go provides GroupByAnalyst — a function that partitions a slice of
// TraceDraft records by their ExtractedBy field. Each distinct ExtractedBy
// value represents an analyst position on the source material; grouping by it
// makes cross-analyst disagreement visible as data rather than noise.
//
// This is a pure data-partitioning function. It does not validate, filter, or
// reorder drafts — it preserves encounter order within each group.
package loader

import "github.com/automatedtomato/mesh-ant/meshant/schema"

// GroupByAnalyst partitions drafts by ExtractedBy and returns a map from
// analyst-position to drafts. Empty ExtractedBy is grouped under key "" —
// an undeclared position is still a position. Returned map is never nil.
// Returned slices do not alias the input.
func GroupByAnalyst(drafts []schema.TraceDraft) map[string][]schema.TraceDraft {
	result := make(map[string][]schema.TraceDraft)

	for _, d := range drafts {
		result[d.ExtractedBy] = append(result[d.ExtractedBy], d)
	}

	return result
}
