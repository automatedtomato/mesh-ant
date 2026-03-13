// deforestation_test.go specifies the structure and content requirements for
// the deforestation example dataset (data/examples/deforestation.json).
//
// These tests are written in the RED phase — the dataset does not yet exist.
// All tests here should FAIL until the dataset is created. The tests define
// what the dataset must contain; the dataset must be written to satisfy them.
//
// The deforestation scenario traces a network of actors and non-humans
// involved in deforestation monitoring: satellite systems, NGO coordinators,
// carbon registries, policy enforcers, and more. It is designed to exercise
// the full ANT vocabulary — translation, mediation, threshold, delay,
// blockage, amplification, and redirection — across a richer and larger
// dataset than the vendor registration example.
package loader_test

import (
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// deforestationPath is the relative path from the loader package directory
// to the deforestation dataset. Go test sets the working directory to the
// package directory, so two levels up reaches the module root.
const deforestationPath = "../../data/examples/deforestation.json"

// loadDeforestation is a shared helper that loads the dataset and fails the
// calling test immediately if the file is missing or invalid. Using t.Fatal
// here (via the helper) stops the calling test without cascading: each test
// that calls this helper independently decides whether to continue.
//
// Downstream assertion tests use t.Errorf (not t.Fatalf) so that all failures
// in a single test are collected before the test ends.
func loadDeforestation(t *testing.T) []schema.Trace {
	t.Helper()
	traces, err := loader.Load(deforestationPath)
	if err != nil {
		t.Fatalf("Load(%q): %v", deforestationPath, err)
	}
	return traces
}

// --- Group 1: Load ---

// TestDeforestation_Load_Count verifies the dataset contains exactly 20 traces.
// A count of exactly 20 is required so that percentage-based thresholds in
// later tests (e.g. "at least 5 flagged") are meaningful and not trivially met.
func TestDeforestation_Load_Count(t *testing.T) {
	traces := loadDeforestation(t)
	if len(traces) != 20 {
		t.Errorf("Load: want 20 traces, got %d", len(traces))
	}
}

// TestDeforestation_Load_AllValid confirms that every trace passes
// schema.Validate(). Load() already calls Validate() internally and returns
// an error on the first failure, so a successful Load() implies all traces
// are valid. This test records that expectation explicitly and double-checks
// the count so the name is not misleading.
func TestDeforestation_Load_AllValid(t *testing.T) {
	// A successful Load() means all traces passed Validate(). Confirm the full
	// count is as expected — if this diverges from TestLoad_Count, something
	// has changed in the dataset or validation rules.
	traces := loadDeforestation(t)
	if len(traces) != 20 {
		t.Errorf("Load: want 20 valid traces, got %d", len(traces))
	}
}

// TestDeforestation_Load_AllHaveObserver ensures every trace records the
// observer position. Required by Principle 8: the designer is inside the
// mesh. An empty Observer would hide the cut that made the trace.
func TestDeforestation_Load_AllHaveObserver(t *testing.T) {
	traces := loadDeforestation(t)
	for i, tr := range traces {
		if tr.Observer == "" {
			t.Errorf("trace[%d] (id=%q): Observer is empty", i, tr.ID)
		}
	}
}

// TestDeforestation_Load_AllHaveWhatChanged ensures every trace records the
// difference it captures. WhatChanged is the primary content of a trace;
// an absent value means the trace failed to document what made a difference.
func TestDeforestation_Load_AllHaveWhatChanged(t *testing.T) {
	traces := loadDeforestation(t)
	for i, tr := range traces {
		if tr.WhatChanged == "" {
			t.Errorf("trace[%d] (id=%q): WhatChanged is empty", i, tr.ID)
		}
	}
}

// --- Group 2: Tag coverage ---

// TestDeforestation_Tags_AllTypesPresent verifies that the dataset contains
// at least one trace for each tag in the standard ANT vocabulary. A dataset
// that omits any tag type provides incomplete coverage of the framework's
// conceptual vocabulary and would limit downstream analysis.
func TestDeforestation_Tags_AllTypesPresent(t *testing.T) {
	traces := loadDeforestation(t)

	// required lists all six standard tag values that must appear at least once.
	required := []string{
		"delay",
		"threshold",
		"blockage",
		"amplification",
		"redirection",
		"translation",
	}

	// Build a set of all tags present across the dataset for O(1) lookup.
	present := make(map[string]bool)
	for _, tr := range traces {
		for _, tag := range tr.Tags {
			present[tag] = true
		}
	}

	for _, tag := range required {
		if !present[tag] {
			t.Errorf("tag %q: not found in any trace; at least 1 required", tag)
		}
	}
}

// TestDeforestation_Tags_TranslationCount verifies that "translation" is the
// dominant tag in this dataset. Deforestation monitoring involves many
// translation events: satellite data translated into policy evidence,
// community reports translated into registry entries, etc. At least 5
// translation-tagged traces are required.
func TestDeforestation_Tags_TranslationCount(t *testing.T) {
	traces := loadDeforestation(t)

	count := 0
	for _, tr := range traces {
		for _, tag := range tr.Tags {
			if tag == "translation" {
				count++
				break // count each trace once, regardless of multiple tags
			}
		}
	}

	const minTranslation = 5
	if count < minTranslation {
		t.Errorf("translation-tagged traces: want >= %d, got %d", minTranslation, count)
	}
}

// TestDeforestation_Tags_ThresholdCount verifies that the dataset contains
// enough threshold-tagged traces to make threshold analysis meaningful.
// Thresholds are critical friction points: permit denial thresholds, alert
// thresholds for deforestation rate, funding cut-off conditions. At least 3
// threshold-tagged traces are required.
func TestDeforestation_Tags_ThresholdCount(t *testing.T) {
	traces := loadDeforestation(t)

	count := 0
	for _, tr := range traces {
		for _, tag := range tr.Tags {
			if tag == "threshold" {
				count++
				break // count each trace once
			}
		}
	}

	const minThreshold = 3
	if count < minThreshold {
		t.Errorf("threshold-tagged traces: want >= %d, got %d", minThreshold, count)
	}
}

// --- Group 3: Observer positions ---

// TestDeforestation_Observers_DistinctCount verifies that the dataset
// captures multiple, heterogeneous observer positions. A dataset dominated
// by a single observer would hide the distributed, contested nature of
// deforestation monitoring. At least 8 distinct observer strings are required.
func TestDeforestation_Observers_DistinctCount(t *testing.T) {
	traces := loadDeforestation(t)

	observers := make(map[string]bool)
	for _, tr := range traces {
		if tr.Observer != "" {
			observers[tr.Observer] = true
		}
	}

	const minDistinctObservers = 8
	if len(observers) < minDistinctObservers {
		t.Errorf("distinct observers: want >= %d, got %d (found: %v)",
			minDistinctObservers, len(observers), sortedKeys(observers))
	}
}

// TestDeforestation_Observers_SpecificPresent verifies that four key actor
// positions are represented as observers. These are the positions whose
// perspectives are most distinct in a deforestation monitoring network:
// the satellite operator (technical/remote), the NGO field coordinator
// (on-the-ground), the carbon registry auditor (financial/legal), and the
// policy enforcement officer (governmental). Each anchors a different part
// of the mesh.
func TestDeforestation_Observers_SpecificPresent(t *testing.T) {
	traces := loadDeforestation(t)

	// Build a set of all observer strings in the dataset.
	present := make(map[string]bool)
	for _, tr := range traces {
		present[tr.Observer] = true
	}

	required := []string{
		"satellite-operator",
		"ngo-field-coordinator",
		"carbon-registry-auditor",
		"policy-enforcement-officer",
	}
	for _, obs := range required {
		if !present[obs] {
			t.Errorf("observer %q: not found in dataset; required", obs)
		}
	}
}

// --- Group 4: Structural properties ---

// TestDeforestation_AbsentSource_Count verifies that at least 3 traces have
// a nil or empty Source slice. Absent source is methodologically significant:
// it means attribution was genuinely unknown or unattributable, not that the
// field was forgotten. The dataset should include enough of these to signal
// that missing attribution is a feature, not an error.
func TestDeforestation_AbsentSource_Count(t *testing.T) {
	traces := loadDeforestation(t)

	count := 0
	for _, tr := range traces {
		if len(tr.Source) == 0 {
			count++
		}
	}

	const minAbsentSource = 3
	if count < minAbsentSource {
		t.Errorf("traces with nil/empty Source: want >= %d, got %d", minAbsentSource, count)
	}
}

// TestDeforestation_AbsentSource_Explained verifies that every trace with an
// absent Source field documents why attribution is unavailable in WhatChanged.
// An empty Source is methodologically intentional in ANT — automated systems,
// collective testimony, or unidentifiable origins. The explanation belongs in
// WhatChanged so the trace does not look like an oversight.
func TestDeforestation_AbsentSource_Explained(t *testing.T) {
	traces := loadDeforestation(t)

	const minExplanationLen = 40 // a meaningful sentence is longer than this
	for i, tr := range traces {
		if len(tr.Source) == 0 && len(tr.WhatChanged) < minExplanationLen {
			t.Errorf("trace[%d] (id=%q): absent Source but WhatChanged too short (%d chars) to explain the absence",
				i, tr.ID, len(tr.WhatChanged))
		}
	}
}

// TestDeforestation_MultiTarget_Count verifies that at least 1 trace has two
// or more entries in its Target slice. Multiple targets signal distributed
// effect — the ANT position that the same action can simultaneously affect
// multiple elements of the mesh. The symmetry with TestDeforestation_MultiSource_Count
// confirms the dataset exercises distributed agency on both sides of a trace.
func TestDeforestation_MultiTarget_Count(t *testing.T) {
	traces := loadDeforestation(t)

	count := 0
	for _, tr := range traces {
		if len(tr.Target) >= 2 {
			count++
		}
	}

	const minMultiTarget = 1
	if count < minMultiTarget {
		t.Errorf("traces with len(Target) >= 2: want >= %d, got %d", minMultiTarget, count)
	}
}

// TestDeforestation_MultiSource_Count verifies that at least 3 traces have
// two or more entries in their Source slice. Multiple sources signal
// distributed agency — the ANT position that differences are produced by
// heterogeneous assemblages, not single actors. A dataset with no
// multi-source traces would imply that agency is always singular.
func TestDeforestation_MultiSource_Count(t *testing.T) {
	traces := loadDeforestation(t)

	count := 0
	for _, tr := range traces {
		if len(tr.Source) >= 2 {
			count++
		}
	}

	const minMultiSource = 3
	if count < minMultiSource {
		t.Errorf("traces with len(Source) >= 2: want >= %d, got %d", minMultiSource, count)
	}
}

// TestDeforestation_Mediations_Count verifies that at least 10 traces carry
// a non-empty Mediation field. A mediator transforms what passes through
// it — it is not a neutral conduit. A dataset where most traces lack
// mediation would be underdetermined from an ANT perspective — too many
// black boxes left unopened.
func TestDeforestation_Mediations_Count(t *testing.T) {
	traces := loadDeforestation(t)

	count := 0
	for _, tr := range traces {
		if tr.Mediation != "" {
			count++
		}
	}

	const minMediated = 10
	if count < minMediated {
		t.Errorf("traces with non-empty Mediation: want >= %d, got %d", minMediated, count)
	}
}

// TestDeforestation_IDs_AllUnique verifies that all 20 trace IDs are
// distinct. Duplicate IDs would create ambiguity in any downstream analysis
// that uses ID as a key (lookups, deduplication, references).
func TestDeforestation_IDs_AllUnique(t *testing.T) {
	traces := loadDeforestation(t)

	seen := make(map[string]int) // id → first index where it appeared
	for i, tr := range traces {
		if prev, exists := seen[tr.ID]; exists {
			t.Errorf("trace[%d] (id=%q): duplicate of trace[%d]", i, tr.ID, prev)
		}
		seen[tr.ID] = i
	}
}

// --- Group 5: Summarise ---

// TestDeforestation_Summarise_ElementCount verifies that Summarise finds at
// least 20 distinct elements across all Source and Target slices. With 20
// traces in a heterogeneous deforestation network, fewer than 20 distinct
// elements would imply an unrealistically narrow set of named participants.
func TestDeforestation_Summarise_ElementCount(t *testing.T) {
	traces := loadDeforestation(t)
	s := loader.Summarise(traces)

	const minElements = 20
	if len(s.Elements) < minElements {
		t.Errorf("Summarise: distinct elements: want >= %d, got %d", minElements, len(s.Elements))
	}
}

// TestDeforestation_Summarise_MediatedTraceCount verifies that Summarise
// counts at least 10 mediated traces. This mirrors the structural requirement
// in TestDeforestation_Mediations_Count but exercises the Summarise function
// rather than direct field inspection, confirming that MediatedTraceCount
// is correctly computed.
func TestDeforestation_Summarise_MediatedTraceCount(t *testing.T) {
	traces := loadDeforestation(t)
	s := loader.Summarise(traces)

	const minMediatedTraces = 10
	if s.MediatedTraceCount < minMediatedTraces {
		t.Errorf("Summarise: MediatedTraceCount: want >= %d, got %d",
			minMediatedTraces, s.MediatedTraceCount)
	}
}

// TestDeforestation_Summarise_MediationsCount verifies that Summarise
// produces the expected number of unique mediation strings. The deforestation
// dataset has 18 mediated traces, each with a distinct mediation string —
// dedup should preserve all of them. This confirms that encounter-order
// deduplication works correctly at scale, not just on the smaller
// vendor-registration dataset.
func TestDeforestation_Summarise_MediationsCount(t *testing.T) {
	traces := loadDeforestation(t)
	s := loader.Summarise(traces)

	// Each mediated trace in the deforestation dataset uses a distinct
	// mediation string, so unique count should equal mediated trace count.
	if s.MediatedTraceCount != len(s.Mediations) {
		t.Errorf("Summarise: MediatedTraceCount (%d) != len(Mediations) (%d): "+
			"dedup produced fewer entries than expected — possible duplicate mediation string",
			s.MediatedTraceCount, len(s.Mediations))
	}

	const minUniqueMediations = 15
	if len(s.Mediations) < minUniqueMediations {
		t.Errorf("Summarise: unique mediations: want >= %d, got %d",
			minUniqueMediations, len(s.Mediations))
	}
}

// TestDeforestation_Summarise_FlaggedTraceCount verifies that Summarise
// identifies at least 5 flagged traces (those tagged "delay" or "threshold").
// Delay and threshold are the primary friction signals in this scenario:
// permitting delays, satellite revisit intervals, funding threshold decisions.
// Fewer than 5 would suggest the dataset underrepresents structural friction.
func TestDeforestation_Summarise_FlaggedTraceCount(t *testing.T) {
	traces := loadDeforestation(t)
	s := loader.Summarise(traces)

	const minFlagged = 5
	if len(s.FlaggedTraces) < minFlagged {
		t.Errorf("Summarise: FlaggedTraces: want >= %d, got %d",
			minFlagged, len(s.FlaggedTraces))
	}
}

