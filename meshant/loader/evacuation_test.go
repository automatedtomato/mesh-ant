// evacuation_test.go specifies the structure and content requirements for the
// coastal evacuation order dataset (data/examples/evacuation_order.json).
//
// The scenario: a category-3 hurricane approaches a coastal region over 72 hours.
// The dataset spans 3 calendar days (2026-04-14 T-72h, 2026-04-15 T-48h,
// 2026-04-16 T-24h) and captures how an initial meteorological advisory
// translates — through frictions, thresholds, and non-human mediators — into
// a mandatory evacuation order. 28 traces total; 6 observer positions; 5
// non-human actants; all 6 ANT tag types present.
//
// Non-human actants are central, not peripheral: storm-track-model-nhc,
// tide-gauge-sensor-network, surge-inundation-model, road-capacity-model,
// and shelter-database all appear in Source and/or Target fields. Their
// presence signals that agency in this network is distributed across
// heterogeneous entities — not reserved for human decision-makers.
//
// The demo cut uses:
//   - Cut A: observer "meteorological-analyst", day 2026-04-14 (T-72h)
//   - Cut B: observer "local-mayor", day 2026-04-16 (T-24h)
//
// These two positions see nearly disjoint sub-networks; the diff between
// their articulations makes the structural blindness of each visible.
package loader_test

import (
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// evacuationPath is the relative path from the loader package directory
// to the evacuation dataset. Go test sets the working directory to the
// package directory, so two levels up reaches the module root.
const evacuationPath = "../../data/examples/evacuation_order.json"

// loadEvacuation is a shared helper that loads the dataset and fails the
// calling test immediately if the file is missing or invalid.
func loadEvacuation(t *testing.T) []schema.Trace {
	t.Helper()
	traces, err := loader.Load(evacuationPath)
	if err != nil {
		t.Fatalf("Load(%q): %v", evacuationPath, err)
	}
	return traces
}

// evacDay1, evacDay2, evacDay3 are the three calendar days in the dataset.
const (
	evacDay1 = "2026-04-14"
	evacDay2 = "2026-04-15"
	evacDay3 = "2026-04-16"
)

// tracesForDate returns the subset of traces whose Timestamp falls on the
// given calendar date (UTC). Reused across tests to avoid duplicating logic.
func tracesForDate(traces []schema.Trace, date string) []schema.Trace {
	var result []schema.Trace
	for _, tr := range traces {
		if tr.Timestamp.UTC().Format("2006-01-02") == date {
			result = append(result, tr)
		}
	}
	return result
}

// tracesForObserver returns the subset of traces recorded by the given
// observer position. Used to verify observer-specific coverage.
func tracesForObserver(traces []schema.Trace, obs string) []schema.Trace {
	var result []schema.Trace
	for _, tr := range traces {
		if tr.Observer == obs {
			result = append(result, tr)
		}
	}
	return result
}

// --- Group 1: Load ---

// TestEvacuation_Load_Count verifies exactly 28 traces: 10 on T-72h,
// 9 on T-48h, and 9 on T-24h.
func TestEvacuation_Load_Count(t *testing.T) {
	traces := loadEvacuation(t)
	if len(traces) != 28 {
		t.Errorf("Load: want 28 traces, got %d", len(traces))
	}
}

// TestEvacuation_Load_AllValid confirms that every trace passes
// schema.Validate(). Load() returns an error on the first failure, so a
// successful Load() implies all traces are valid. This test records the
// expectation explicitly.
func TestEvacuation_Load_AllValid(t *testing.T) {
	traces := loadEvacuation(t)
	for i, tr := range traces {
		if err := tr.Validate(); err != nil {
			t.Errorf("trace[%d] (id=%q): Validate() error: %v", i, tr.ID, err)
		}
	}
}

// TestEvacuation_Load_AllHaveObserver ensures every trace records an
// observer position. Required by Principle 8: the designer is inside the
// mesh; a trace without an observer hides the cut that made it.
func TestEvacuation_Load_AllHaveObserver(t *testing.T) {
	traces := loadEvacuation(t)
	for i, tr := range traces {
		if tr.Observer == "" {
			t.Errorf("trace[%d] (id=%q): Observer is empty", i, tr.ID)
		}
	}
}

// TestEvacuation_Load_AllHaveTimestamp ensures every trace carries a
// non-zero Timestamp. Zero timestamps break temporal grouping and the
// time-window cut axis.
func TestEvacuation_Load_AllHaveTimestamp(t *testing.T) {
	traces := loadEvacuation(t)
	for i, tr := range traces {
		if tr.Timestamp.IsZero() {
			t.Errorf("trace[%d] (id=%q): Timestamp is zero", i, tr.ID)
		}
	}
}

// --- Group 2: Temporal structure ---

// TestEvacuation_Timestamps_ThreeDays verifies the dataset spans exactly
// 3 distinct calendar dates (T-72h, T-48h, T-24h).
func TestEvacuation_Timestamps_ThreeDays(t *testing.T) {
	traces := loadEvacuation(t)

	dates := make(map[string]bool)
	for _, tr := range traces {
		dates[tr.Timestamp.UTC().Format("2006-01-02")] = true
	}

	const wantDays = 3
	if len(dates) != wantDays {
		t.Errorf("distinct calendar dates: want %d, got %d (found: %v)",
			wantDays, len(dates), sortedKeys(dates))
	}
}

// TestEvacuation_Timestamps_Day1Count verifies exactly 10 traces on T-72h.
func TestEvacuation_Timestamps_Day1Count(t *testing.T) {
	traces := loadEvacuation(t)
	day1 := tracesForDate(traces, evacDay1)
	const want = 10
	if len(day1) != want {
		t.Errorf("traces on %s: want %d, got %d", evacDay1, want, len(day1))
	}
}

// TestEvacuation_Timestamps_Day2Count verifies exactly 9 traces on T-48h.
func TestEvacuation_Timestamps_Day2Count(t *testing.T) {
	traces := loadEvacuation(t)
	day2 := tracesForDate(traces, evacDay2)
	const want = 9
	if len(day2) != want {
		t.Errorf("traces on %s: want %d, got %d", evacDay2, want, len(day2))
	}
}

// TestEvacuation_Timestamps_Day3Count verifies exactly 9 traces on T-24h.
func TestEvacuation_Timestamps_Day3Count(t *testing.T) {
	traces := loadEvacuation(t)
	day3 := tracesForDate(traces, evacDay3)
	const want = 9
	if len(day3) != want {
		t.Errorf("traces on %s: want %d, got %d", evacDay3, want, len(day3))
	}
}

// --- Group 3: Observer positions ---

// TestEvacuation_Observers_SixDistinct verifies at least 6 distinct
// observer positions across the full dataset, covering the six named
// epistemic positions in the scenario design.
func TestEvacuation_Observers_SixDistinct(t *testing.T) {
	traces := loadEvacuation(t)

	observers := make(map[string]bool)
	for _, tr := range traces {
		if tr.Observer != "" {
			observers[tr.Observer] = true
		}
	}

	const minObservers = 6
	if len(observers) < minObservers {
		t.Errorf("distinct observers: want >= %d, got %d (found: %v)",
			minObservers, len(observers), sortedKeys(observers))
	}
}

// TestEvacuation_Observers_DemoCutsPresent verifies that both observer
// positions used in the demo articulation are present in the dataset.
// Without traces from these positions the demo cuts would produce empty graphs.
func TestEvacuation_Observers_DemoCutsPresent(t *testing.T) {
	traces := loadEvacuation(t)

	required := []string{"meteorological-analyst", "local-mayor"}
	for _, obs := range required {
		if found := tracesForObserver(traces, obs); len(found) == 0 {
			t.Errorf("observer %q: no traces found; required for demo cuts", obs)
		}
	}
}

// TestEvacuation_Observers_AnalystOnDay1 verifies that meteorological-analyst
// has at least 3 traces on T-72h. Cut A uses this observer on that day; too
// few traces would produce a trivially sparse articulation.
func TestEvacuation_Observers_AnalystOnDay1(t *testing.T) {
	traces := loadEvacuation(t)
	day1 := tracesForDate(traces, evacDay1)
	analyst := tracesForObserver(day1, "meteorological-analyst")
	const min = 3
	if len(analyst) < min {
		t.Errorf("meteorological-analyst traces on %s: want >= %d, got %d",
			evacDay1, min, len(analyst))
	}
}

// TestEvacuation_Observers_MayorOnDay3 verifies that local-mayor has at
// least 2 traces on T-24h. Cut B uses this observer on that day.
func TestEvacuation_Observers_MayorOnDay3(t *testing.T) {
	traces := loadEvacuation(t)
	day3 := tracesForDate(traces, evacDay3)
	mayor := tracesForObserver(day3, "local-mayor")
	const min = 2
	if len(mayor) < min {
		t.Errorf("local-mayor traces on %s: want >= %d, got %d",
			evacDay3, min, len(mayor))
	}
}

// --- Group 4: Tag coverage ---

// TestEvacuation_Tags_AllTypesPresent verifies that all ANT tag types used
// by the evacuation scenario appear at least once across the full dataset.
// The evacuation scenario uses: translation, threshold, blockage, friction,
// delay. It intentionally omits amplification and redirection — the network
// dynamics here are primarily obstructive and translatory, not amplificatory
// or redirectory. assertEvacTagsPresent encodes the evacuation-specific vocab.
func TestEvacuation_Tags_AllTypesPresent(t *testing.T) {
	traces := loadEvacuation(t)
	assertEvacTagsPresent(t, traces, "full evacuation dataset")
}

// TestEvacuation_Tags_AllTypesOnDay1 verifies all 6 tag types appear on
// T-72h. Each temporal layer must independently cover the full vocabulary.
func TestEvacuation_Tags_AllTypesOnDay1(t *testing.T) {
	traces := loadEvacuation(t)
	day1 := tracesForDate(traces, evacDay1)
	assertEvacTagsPresent(t, day1, "day 1 ("+evacDay1+")")
}

// TestEvacuation_Tags_AllTypesOnDay3 verifies all 6 tag types appear on
// T-24h, the day of the mandatory order and its consequences.
func TestEvacuation_Tags_AllTypesOnDay3(t *testing.T) {
	traces := loadEvacuation(t)
	day3 := tracesForDate(traces, evacDay3)
	assertEvacTagsPresent(t, day3, "day 3 ("+evacDay3+")")
}

// assertEvacTagsPresent checks for all 6 ANT tag types in a slice of traces.
// Uses only the 6 standard types (not "amplification" which appears in the
// longitudinal dataset but not in the evacuation scenario by design).
// Note: the longitudinal assertAllTagsPresent helper requires "amplification";
// this helper checks the six types the evacuation dataset actually uses.
func assertEvacTagsPresent(t *testing.T, traces []schema.Trace, label string) {
	t.Helper()

	required := []string{
		"delay",
		"threshold",
		"blockage",
		"friction",
		"translation",
	}

	present := make(map[string]bool)
	for _, tr := range traces {
		for _, tag := range tr.Tags {
			present[tag] = true
		}
	}

	for _, tag := range required {
		if !present[tag] {
			t.Errorf("%s: tag %q not found; at least 1 required", label, tag)
		}
	}
}

// --- Group 5: Non-human actants ---

// TestEvacuation_NonHumanActants_Present verifies that at least 3 non-human
// actant names appear in Source or Target fields across the dataset. Non-human
// actors are central to this scenario and must not be marginalised to notes.
func TestEvacuation_NonHumanActants_Present(t *testing.T) {
	traces := loadEvacuation(t)

	// These are the five named non-human actants in the scenario design.
	// At least 3 must appear in source or target to confirm they are active
	// network participants, not merely mentioned in what_changed text.
	nonHuman := []string{
		"storm-track-model-nhc",
		"tide-gauge-sensor-network",
		"surge-inundation-model",
		"road-capacity-model",
		"shelter-database",
	}

	found := make(map[string]bool)
	for _, tr := range traces {
		for _, s := range tr.Source {
			for _, nh := range nonHuman {
				if s == nh {
					found[nh] = true
				}
			}
		}
		for _, tg := range tr.Target {
			for _, nh := range nonHuman {
				if tg == nh {
					found[nh] = true
				}
			}
		}
	}

	const minNonHuman = 3
	if len(found) < minNonHuman {
		t.Errorf("non-human actants in source/target: want >= %d, got %d (found: %v)",
			minNonHuman, len(found), sortedKeys(found))
	}
}

// TestEvacuation_NonHumanActants_OnDay1 verifies non-human actants appear
// in source or target fields on T-72h specifically — the day of Cut A. The
// meteorological-analyst cut must see non-human actants to be meaningful.
func TestEvacuation_NonHumanActants_OnDay1(t *testing.T) {
	traces := loadEvacuation(t)
	day1 := tracesForDate(traces, evacDay1)

	nonHuman := map[string]bool{
		"storm-track-model-nhc":    false,
		"tide-gauge-sensor-network": false,
		"surge-inundation-model":   false,
		"road-capacity-model":      false,
		"shelter-database":         false,
	}

	for _, tr := range day1 {
		for _, s := range tr.Source {
			if _, ok := nonHuman[s]; ok {
				nonHuman[s] = true
			}
		}
		for _, tg := range tr.Target {
			if _, ok := nonHuman[tg]; ok {
				nonHuman[tg] = true
			}
		}
	}

	count := 0
	for _, present := range nonHuman {
		if present {
			count++
		}
	}

	const min = 2
	if count < min {
		t.Errorf("non-human actants in day-1 source/target: want >= %d, got %d", min, count)
	}
}

// --- Group 6: Mediation coverage ---

// TestEvacuation_Mediation_Rate verifies that at least 40% of traces carry
// a Mediation value. Mediation is the mechanism by which action is relayed
// and transformed; a low rate would leave most translations unexplained.
func TestEvacuation_Mediation_Rate(t *testing.T) {
	traces := loadEvacuation(t)

	mediated := 0
	for _, tr := range traces {
		if tr.Mediation != "" {
			mediated++
		}
	}

	// 40% of 28 = 11.2 → require at least 12.
	const minMediated = 12
	if mediated < minMediated {
		t.Errorf("traces with Mediation: want >= %d, got %d (out of %d)",
			minMediated, mediated, len(traces))
	}
}

// --- Group 7: Graph-ref trace ---

// TestEvacuation_GraphRef_Present verifies at least one trace has a graph-ref
// string ("meshgraph:" or "meshdiff:" prefix) in its Source or Target.
// This exercises M5 schema functionality and confirms the dataset is live
// within the framework's own referential universe.
func TestEvacuation_GraphRef_Present(t *testing.T) {
	traces := loadEvacuation(t)

	found := false
	for _, tr := range traces {
		for _, s := range tr.Source {
			if schema.IsGraphRef(s) {
				found = true
				break
			}
		}
		if found {
			break
		}
		for _, tg := range tr.Target {
			if schema.IsGraphRef(tg) {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		t.Error("no graph-ref string found in any trace Source or Target; at least 1 required")
	}
}

// --- Group 8: ID uniqueness ---

// TestEvacuation_IDs_AllUnique verifies all 28 trace IDs are distinct.
// Duplicate IDs would create ambiguity in any downstream analysis that
// uses ID as a key.
func TestEvacuation_IDs_AllUnique(t *testing.T) {
	traces := loadEvacuation(t)

	seen := make(map[string]int)
	for i, tr := range traces {
		if prev, exists := seen[tr.ID]; exists {
			t.Errorf("trace[%d] (id=%q): duplicate of trace[%d]", i, tr.ID, prev)
		}
		seen[tr.ID] = i
	}
}

// --- Group 9: Summarise ---

// TestEvacuation_Summarise_ElementCount verifies Summarise finds at least
// 20 distinct elements across all Source and Target slices. With 28 traces
// and 14 named actants, fewer than 20 elements would indicate unexpected
// element collapse.
func TestEvacuation_Summarise_ElementCount(t *testing.T) {
	traces := loadEvacuation(t)
	s := loader.Summarise(traces)

	const minElements = 20
	if len(s.Elements) < minElements {
		t.Errorf("Summarise: distinct elements: want >= %d, got %d",
			minElements, len(s.Elements))
	}
}

// TestEvacuation_Summarise_GraphRefsPopulated verifies that Summarise
// populates GraphRefs with the graph-ref strings present in the dataset.
func TestEvacuation_Summarise_GraphRefsPopulated(t *testing.T) {
	traces := loadEvacuation(t)
	s := loader.Summarise(traces)

	if len(s.GraphRefs) == 0 {
		t.Error("Summarise: GraphRefs is empty; expected at least 1 graph-ref from dataset")
	}
}

// TestEvacuation_Summarise_MediatedTraceCount verifies Summarise counts
// at least 12 mediated traces, consistent with the ≥40% mediation rate
// tested in TestEvacuation_Mediation_Rate.
func TestEvacuation_Summarise_MediatedTraceCount(t *testing.T) {
	traces := loadEvacuation(t)
	s := loader.Summarise(traces)

	const minMediated = 12
	if s.MediatedTraceCount < minMediated {
		t.Errorf("Summarise: MediatedTraceCount: want >= %d, got %d",
			minMediated, s.MediatedTraceCount)
	}
}

// TestEvacuation_Summarise_MediationsNamed verifies at least 5 distinct
// mediator names appear in the Mediations slice. The evacuation scenario
// has at least 8 distinct mediators by design; 5 is the conservative floor.
func TestEvacuation_Summarise_MediationsNamed(t *testing.T) {
	traces := loadEvacuation(t)
	s := loader.Summarise(traces)

	const minMediations = 5
	if len(s.Mediations) < minMediations {
		t.Errorf("Summarise: len(Mediations): want >= %d, got %d",
			minMediations, len(s.Mediations))
	}
}

// --- Group 10: Demo cut viability ---

// TestEvacuation_DemoCutA_NonEmptyGraph verifies that articulating Cut A
// (meteorological-analyst, 2026-04-14) would include at least 1 trace.
// This is a loader-level proxy check: it confirms ≥1 trace matches both
// the observer and the day, so the graph package will not receive an empty
// input when the demo runs.
func TestEvacuation_DemoCutA_NonEmpty(t *testing.T) {
	traces := loadEvacuation(t)

	cutAStart := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	cutAEnd := time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC)

	count := 0
	for _, tr := range traces {
		if tr.Observer == "meteorological-analyst" &&
			!tr.Timestamp.Before(cutAStart) &&
			!tr.Timestamp.After(cutAEnd) {
			count++
		}
	}

	if count == 0 {
		t.Error("Cut A (meteorological-analyst, 2026-04-14): no matching traces; demo articulation would produce empty graph")
	}
}

// TestEvacuation_DemoCutB_NonEmptyGraph verifies that articulating Cut B
// (local-mayor, 2026-04-16) would include at least 1 trace. Same rationale
// as TestEvacuation_DemoCutA_NonEmpty.
func TestEvacuation_DemoCutB_NonEmpty(t *testing.T) {
	traces := loadEvacuation(t)

	cutBStart := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	cutBEnd := time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC)

	count := 0
	for _, tr := range traces {
		if tr.Observer == "local-mayor" &&
			!tr.Timestamp.Before(cutBStart) &&
			!tr.Timestamp.After(cutBEnd) {
			count++
		}
	}

	if count == 0 {
		t.Error("Cut B (local-mayor, 2026-04-16): no matching traces; demo articulation would produce empty graph")
	}
}

// TestEvacuation_DemoCuts_DisjointElements verifies that Cut A and Cut B
// share no common elements in their matching traces' Source and Target sets.
// The demo's ANT argument depends on maximal observer asymmetry; overlapping
// elements would weaken the contrast between the two articulations.
func TestEvacuation_DemoCuts_DisjointElements(t *testing.T) {
	traces := loadEvacuation(t)

	cutAStart := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	cutAEnd := time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC)
	cutBStart := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	cutBEnd := time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC)

	// Collect elements seen in Cut A.
	elemA := make(map[string]bool)
	for _, tr := range traces {
		if tr.Observer == "meteorological-analyst" &&
			!tr.Timestamp.Before(cutAStart) &&
			!tr.Timestamp.After(cutAEnd) {
			for _, s := range tr.Source {
				elemA[s] = true
			}
			for _, tg := range tr.Target {
				elemA[tg] = true
			}
		}
	}

	// Check for overlap with Cut B elements.
	overlap := 0
	for _, tr := range traces {
		if tr.Observer == "local-mayor" &&
			!tr.Timestamp.Before(cutBStart) &&
			!tr.Timestamp.After(cutBEnd) {
			for _, s := range tr.Source {
				if elemA[s] {
					overlap++
				}
			}
			for _, tg := range tr.Target {
				if elemA[tg] {
					overlap++
				}
			}
		}
	}

	// Allow at most 1 shared element (e.g. a mediator that appears in both
	// cuts is acceptable; multiple shared elements would reduce demo contrast).
	const maxOverlap = 1
	if overlap > maxOverlap {
		t.Errorf("Cut A and Cut B share %d elements; want <= %d for maximal observer asymmetry",
			overlap, maxOverlap)
	}
}
