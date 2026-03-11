// longitudinal_test.go specifies the structure and content requirements for
// the deforestation longitudinal dataset (data/examples/deforestation_longitudinal.json).
//
// These tests are written in the RED phase — the dataset does not yet exist.
// All tests here should FAIL until the dataset is created. The tests define
// what the dataset must contain; the dataset must be written to satisfy them.
//
// The longitudinal dataset extends the 20-trace deforestation scenario across
// three calendar days (2026-03-11, 2026-03-14, 2026-03-18), producing 40
// traces total. Day 1 copies all 20 traces from deforestation.json verbatim.
// Days 2 and 3 add 10 new traces each, advancing the policy, satellite,
// community-legal, and carbon-market threads.
//
// Key properties this dataset exercises beyond the base dataset:
//   - temporal spread across multiple calendar days
//   - per-day tag coverage (each day must independently cover all 6 tag types)
//   - cross-day element persistence (elements from day 1 reappear in day 3)
//   - all 8 original observer positions active in day 3
package loader_test

import (
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// longitudinalPath is the relative path from the loader package directory
// to the longitudinal dataset. Go test sets the working directory to the
// package directory, so two levels up reaches the module root.
const longitudinalPath = "../../data/examples/deforestation_longitudinal.json"

// loadLongitudinal is a shared helper that loads the dataset and fails the
// calling test immediately if the file is missing or invalid. Using t.Fatal
// here (via the helper) stops the calling test without cascading: each test
// that calls this helper independently decides whether to continue.
//
// Downstream assertion tests use t.Errorf (not t.Fatalf) so that all failures
// in a single test are collected before the test ends.
func loadLongitudinal(t *testing.T) []schema.Trace {
	t.Helper()
	traces, err := loader.Load(longitudinalPath)
	if err != nil {
		t.Fatalf("Load(%q): %v", longitudinalPath, err)
	}
	return traces
}

// day1Date, day2Date, day3Date are the three calendar days expected in the
// dataset. Each is expressed as a UTC date string for comparison after
// stripping the time component from each trace's Timestamp.
const (
	day1Date = "2026-03-11"
	day2Date = "2026-03-14"
	day3Date = "2026-03-18"
)

// dateOf returns the calendar date of a time.Time as "YYYY-MM-DD" in UTC.
// Used throughout these tests to group traces by day without time-of-day noise.
func dateOf(ts time.Time) string {
	return ts.UTC().Format("2006-01-02")
}

// tracesForDay returns the subset of traces whose Timestamp falls on the
// given calendar date (in UTC). Reused across multiple tests to avoid
// duplicating the filtering logic.
func tracesForDay(traces []schema.Trace, date string) []schema.Trace {
	var result []schema.Trace
	for _, tr := range traces {
		if dateOf(tr.Timestamp) == date {
			result = append(result, tr)
		}
	}
	return result
}

// --- Group 1: Load ---

// TestLongitudinal_Load_Count verifies the dataset contains exactly 40 traces:
// 20 from day 1 (copied verbatim from deforestation.json), 10 new for day 2,
// and 10 new for day 3. The exact count is required so that per-day and
// cross-day assertions in later tests are meaningful and not trivially met.
func TestLongitudinal_Load_Count(t *testing.T) {
	traces := loadLongitudinal(t)
	if len(traces) != 40 {
		t.Errorf("Load: want 40 traces, got %d", len(traces))
	}
}

// TestLongitudinal_Load_AllValid confirms that every trace passes
// schema.Validate(). Load() already calls Validate() internally and returns
// an error on the first failure, so a successful Load() implies all traces
// are valid. This test records that expectation explicitly and double-checks
// the count so the name is not misleading.
func TestLongitudinal_Load_AllValid(t *testing.T) {
	traces := loadLongitudinal(t)
	if len(traces) != 40 {
		t.Errorf("Load: want 40 valid traces, got %d", len(traces))
	}
}

// TestLongitudinal_Load_AllHaveObserver ensures every trace records the
// observer position. Required by Principle 8: the designer is inside the
// mesh. An empty Observer would hide the cut that made the trace.
func TestLongitudinal_Load_AllHaveObserver(t *testing.T) {
	traces := loadLongitudinal(t)
	for i, tr := range traces {
		if tr.Observer == "" {
			t.Errorf("trace[%d] (id=%q): Observer is empty", i, tr.ID)
		}
	}
}

// TestLongitudinal_Load_AllHaveTimestamp ensures every trace carries a
// non-zero timestamp. A zero time.Time would indicate the field was missing
// or failed to parse, which breaks all temporal grouping in later tests.
func TestLongitudinal_Load_AllHaveTimestamp(t *testing.T) {
	traces := loadLongitudinal(t)
	for i, tr := range traces {
		if tr.Timestamp.IsZero() {
			t.Errorf("trace[%d] (id=%q): Timestamp is zero", i, tr.ID)
		}
	}
}

// --- Group 2: Temporal structure ---

// TestLongitudinal_Timestamps_ThreeDays verifies that the dataset spans
// exactly 3 distinct calendar dates. A longitudinal dataset with fewer
// distinct dates would not demonstrate temporal spread; more would indicate
// unexpected traces outside the designed time windows.
func TestLongitudinal_Timestamps_ThreeDays(t *testing.T) {
	traces := loadLongitudinal(t)

	dates := make(map[string]bool)
	for _, tr := range traces {
		dates[dateOf(tr.Timestamp)] = true
	}

	const wantDays = 3
	if len(dates) != wantDays {
		t.Errorf("distinct calendar dates: want %d, got %d (found: %v)",
			wantDays, len(dates), sortedKeys(dates))
	}
}

// TestLongitudinal_Timestamps_Day1Count verifies that exactly 20 traces fall
// on 2026-03-11. Day 1 is the verbatim copy of the base deforestation dataset;
// any deviation indicates a missing or malformed day-1 trace.
func TestLongitudinal_Timestamps_Day1Count(t *testing.T) {
	traces := loadLongitudinal(t)
	day1 := tracesForDay(traces, day1Date)

	const want = 20
	if len(day1) != want {
		t.Errorf("traces on %s: want %d, got %d", day1Date, want, len(day1))
	}
}

// TestLongitudinal_Timestamps_Day2Count verifies that exactly 10 traces fall
// on 2026-03-14. Day 2 introduces policy escalation, satellite retasking,
// community legal, and carbon-market threads.
func TestLongitudinal_Timestamps_Day2Count(t *testing.T) {
	traces := loadLongitudinal(t)
	day2 := tracesForDay(traces, day2Date)

	const want = 10
	if len(day2) != want {
		t.Errorf("traces on %s: want %d, got %d", day2Date, want, len(day2))
	}
}

// TestLongitudinal_Timestamps_Day3Count verifies that exactly 10 traces fall
// on 2026-03-18. Day 3 introduces the formal conservation order, UNFF case
// acknowledgement, carbon recalibration, and cross-thread closure.
func TestLongitudinal_Timestamps_Day3Count(t *testing.T) {
	traces := loadLongitudinal(t)
	day3 := tracesForDay(traces, day3Date)

	const want = 10
	if len(day3) != want {
		t.Errorf("traces on %s: want %d, got %d", day3Date, want, len(day3))
	}
}

// --- Group 3: Tag coverage ---

// TestLongitudinal_Tags_AllTypesPresent verifies that all 6 ANT tag types
// appear at least once across the full 40-trace dataset. A longitudinal
// dataset that omits any tag type provides incomplete coverage of the
// framework's conceptual vocabulary.
func TestLongitudinal_Tags_AllTypesPresent(t *testing.T) {
	traces := loadLongitudinal(t)
	assertAllTagsPresent(t, traces, "full dataset")
}

// TestLongitudinal_Tags_AllTypesInDay2 verifies that all 6 ANT tag types
// appear in day-2 traces alone. This ensures each temporal layer is
// independently complete — downstream per-day analysis must be able to
// see the full vocabulary without merging days.
func TestLongitudinal_Tags_AllTypesInDay2(t *testing.T) {
	traces := loadLongitudinal(t)
	day2 := tracesForDay(traces, day2Date)
	assertAllTagsPresent(t, day2, "day 2 ("+day2Date+")")
}

// TestLongitudinal_Tags_AllTypesInDay3 verifies that all 6 ANT tag types
// appear in day-3 traces alone, for the same reason as day 2.
func TestLongitudinal_Tags_AllTypesInDay3(t *testing.T) {
	traces := loadLongitudinal(t)
	day3 := tracesForDay(traces, day3Date)
	assertAllTagsPresent(t, day3, "day 3 ("+day3Date+")")
}

// assertAllTagsPresent is a shared helper that checks for all 6 ANT tag types
// in a slice of traces. The label parameter is included in error messages so
// the caller can distinguish which group of traces failed.
func assertAllTagsPresent(t *testing.T, traces []schema.Trace, label string) {
	t.Helper()

	// required lists all six standard tag values that must appear at least once.
	required := []string{
		"delay",
		"threshold",
		"blockage",
		"amplification",
		"redirection",
		"translation",
	}

	// Build a set of all tags present across the given traces for O(1) lookup.
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

// --- Group 4: Observer positions ---

// TestLongitudinal_Observers_DistinctCount verifies that the full 40-trace
// dataset captures at least 8 distinct observer positions. Day 1 alone has 8
// (inherited from deforestation.json); day 2 must maintain that breadth and
// day 3 is required to include all 8 originals.
func TestLongitudinal_Observers_DistinctCount(t *testing.T) {
	traces := loadLongitudinal(t)

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

// TestLongitudinal_Observers_AllPresentOnDay3 verifies that all 8 observer
// positions from the original deforestation dataset appear in day-3 traces.
// Day 3 represents the cross-thread closure: every major observer must still
// be active at this point, signalling that no thread was abandoned.
func TestLongitudinal_Observers_AllPresentOnDay3(t *testing.T) {
	traces := loadLongitudinal(t)
	day3 := tracesForDay(traces, day3Date)

	// These are the 8 observer positions established in deforestation.json.
	required := []string{
		"satellite-operator",
		"deforestation-detection-algorithm",
		"national-forest-agency",
		"ngo-field-coordinator",
		"carbon-registry-auditor",
		"carbon-credit-broker",
		"policy-enforcement-officer",
		"international-treaty-body",
	}

	present := make(map[string]bool)
	for _, tr := range day3 {
		present[tr.Observer] = true
	}

	for _, obs := range required {
		if !present[obs] {
			t.Errorf("day 3: observer %q not found; required on day 3", obs)
		}
	}
}

// --- Group 5: Structural properties ---

// TestLongitudinal_AbsentSource_Count verifies that at least 5 traces across
// the full dataset have a nil or empty Source slice. Day 1 contributes 3
// (inherited from deforestation.json); days 2 and 3 should each add at least
// one more to signal that absent attribution is a persistent methodological
// feature, not a day-1 accident.
func TestLongitudinal_AbsentSource_Count(t *testing.T) {
	traces := loadLongitudinal(t)

	count := 0
	for _, tr := range traces {
		if len(tr.Source) == 0 {
			count++
		}
	}

	const minAbsentSource = 5
	if count < minAbsentSource {
		t.Errorf("traces with nil/empty Source: want >= %d, got %d", minAbsentSource, count)
	}
}

// TestLongitudinal_MultiSource_Count verifies that at least 4 traces across
// the dataset have two or more entries in their Source slice. Multi-source
// traces signal distributed agency — a core ANT position. Day 1 contributes
// 3; at least one day-2 or day-3 trace must add another.
func TestLongitudinal_MultiSource_Count(t *testing.T) {
	traces := loadLongitudinal(t)

	count := 0
	for _, tr := range traces {
		if len(tr.Source) >= 2 {
			count++
		}
	}

	const minMultiSource = 4
	if count < minMultiSource {
		t.Errorf("traces with len(Source) >= 2: want >= %d, got %d", minMultiSource, count)
	}
}

// TestLongitudinal_CrossDay_ElementPersistence verifies that at least one
// element appearing in a day-1 trace source or target also appears in a
// day-3 trace source or target. Cross-day persistence is methodologically
// significant: it shows that actors and inscriptions remain active across
// time, accumulating relational weight across the mesh.
func TestLongitudinal_CrossDay_ElementPersistence(t *testing.T) {
	traces := loadLongitudinal(t)

	// Collect all elements from day-1 traces.
	day1Elements := make(map[string]bool)
	for _, tr := range tracesForDay(traces, day1Date) {
		for _, s := range tr.Source {
			day1Elements[s] = true
		}
		for _, tg := range tr.Target {
			day1Elements[tg] = true
		}
	}

	// Check whether any day-3 trace references a day-1 element.
	day3 := tracesForDay(traces, day3Date)
	found := false
	for _, tr := range day3 {
		for _, s := range tr.Source {
			if day1Elements[s] {
				found = true
				break
			}
		}
		if found {
			break
		}
		for _, tg := range tr.Target {
			if day1Elements[tg] {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		t.Error("cross-day element persistence: no element from day 1 appears in any day-3 trace source or target")
	}
}

// --- Group 6: ID uniqueness ---

// TestLongitudinal_IDs_AllUnique verifies that all 40 trace IDs are
// distinct. Duplicate IDs would create ambiguity in any downstream analysis
// that uses ID as a key (lookups, deduplication, references). Day-1 traces
// are copied verbatim from deforestation.json, so their IDs must not collide
// with the new day-2 and day-3 UUIDs.
func TestLongitudinal_IDs_AllUnique(t *testing.T) {
	traces := loadLongitudinal(t)

	seen := make(map[string]int) // id → first index where it appeared
	for i, tr := range traces {
		if prev, exists := seen[tr.ID]; exists {
			t.Errorf("trace[%d] (id=%q): duplicate of trace[%d]", i, tr.ID, prev)
		}
		seen[tr.ID] = i
	}
}

// --- Group 7: Summarise ---

// TestLongitudinal_Summarise_ElementCount verifies that Summarise finds at
// least 40 distinct elements across all Source and Target slices. With 40
// traces across three deforestation-monitoring threads, fewer than 40
// distinct elements would imply an unrealistically narrow set of named
// participants or heavy element reuse that collapses the network.
func TestLongitudinal_Summarise_ElementCount(t *testing.T) {
	traces := loadLongitudinal(t)
	s := loader.Summarise(traces)

	const minElements = 40
	if len(s.Elements) < minElements {
		t.Errorf("Summarise: distinct elements: want >= %d, got %d", minElements, len(s.Elements))
	}
}

// TestLongitudinal_Summarise_MediatedTraceCount verifies that Summarise
// counts at least 25 mediated traces across the 40-trace dataset. Day 1
// contributes 18 mediated traces; days 2 and 3 should each add several more.
// Mediation is the mechanism by which action is relayed and transformed
// between source and target — a dataset where most traces lack mediation
// would be underdetermined from an ANT perspective.
func TestLongitudinal_Summarise_MediatedTraceCount(t *testing.T) {
	traces := loadLongitudinal(t)
	s := loader.Summarise(traces)

	const minMediatedTraces = 25
	if s.MediatedTraceCount < minMediatedTraces {
		t.Errorf("Summarise: MediatedTraceCount: want >= %d, got %d",
			minMediatedTraces, s.MediatedTraceCount)
	}
}

