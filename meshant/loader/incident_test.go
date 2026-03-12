// incident_test.go specifies the structure and content requirements for the
// software incident response dataset (data/examples/incident_response.json).
//
// The scenario: a high-traffic e-commerce API service suffers a database
// connection pool exhaustion during a flash sale. The outage spans two days:
// Day 1 (2026-05-10) is the incident itself — detection, escalation, and
// mitigation. Day 2 (2026-05-11) is the postmortem and process changes.
//
// Non-human actants are central participants, not peripheral mentions:
// alerting-pipeline, auto-scaler, circuit-breaker, sla-timer,
// runbook-engine, dashboard-service, connection-pool-monitor, and
// pagerduty-webhook all appear in Source and/or Target fields. Their
// presence demonstrates that agency in this network is distributed across
// heterogeneous technical and human entities.
//
// The demo cuts use:
//   - Cut A: observer "monitoring-service", day 2026-05-10
//     Sees the automated detection chain (metrics → alerts → pipeline).
//     Human coordination (incident commander, product manager) is invisible.
//   - Cut B: observer "incident-commander", day 2026-05-11
//     Sees the postmortem and process correction.
//     The automated metric chain that triggered the original incident is invisible.
//
// These two positions see near-disjoint sub-networks. Diffing them exposes
// the structural blindness of each observer position.
package loader_test

import (
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// incidentPath is the relative path from the loader package directory
// to the incident response dataset. Go test sets the working directory
// to the package directory, so two levels up reaches the module root.
const incidentPath = "../../data/examples/incident_response.json"

// loadIncident is a shared helper that loads the dataset and fails the
// calling test immediately if the file is missing or invalid.
func loadIncident(t *testing.T) []schema.Trace {
	t.Helper()
	traces, err := loader.Load(incidentPath)
	if err != nil {
		t.Fatalf("Load(%q): %v", incidentPath, err)
	}
	return traces
}

// incidentDay1 and incidentDay2 are the two calendar days in the dataset.
const (
	incidentDay1 = "2026-05-10"
	incidentDay2 = "2026-05-11"
)

// --- Group 1: Load ---

// TestIncident_Load_Count verifies the dataset has between 20 and 25 traces
// (inclusive). The scenario design calls for ~15 on Day 1 and ~7 on Day 2.
func TestIncident_Load_Count(t *testing.T) {
	traces := loadIncident(t)
	count := len(traces)
	if count < 20 || count > 25 {
		t.Errorf("Load: want 20-25 traces, got %d", count)
	}
}

// TestIncident_Load_AllValid confirms that every trace passes
// schema.Validate(). Load() returns an error on the first failure, so a
// successful Load() implies all traces are valid. This test states the
// expectation explicitly for documentation purposes.
func TestIncident_Load_AllValid(t *testing.T) {
	traces := loadIncident(t)
	for i, tr := range traces {
		if err := tr.Validate(); err != nil {
			t.Errorf("trace[%d] (id=%q): Validate() error: %v", i, tr.ID, err)
		}
	}
}

// TestIncident_Load_AllHaveObserver ensures every trace records an
// observer position, enforcing Principle 8: the designer is inside the mesh;
// a trace without an observer hides the cut that made it.
func TestIncident_Load_AllHaveObserver(t *testing.T) {
	traces := loadIncident(t)
	for i, tr := range traces {
		if tr.Observer == "" {
			t.Errorf("trace[%d] (id=%q): Observer is empty", i, tr.ID)
		}
	}
}

// TestIncident_Load_AllHaveTimestamp ensures every trace carries a non-zero
// Timestamp. Zero timestamps break temporal grouping and the time-window cut axis.
func TestIncident_Load_AllHaveTimestamp(t *testing.T) {
	traces := loadIncident(t)
	for i, tr := range traces {
		if tr.Timestamp.IsZero() {
			t.Errorf("trace[%d] (id=%q): Timestamp is zero", i, tr.ID)
		}
	}
}

// --- Group 2: Temporal structure ---

// TestIncident_Timestamps_TwoDays verifies the dataset spans exactly
// 2 distinct calendar dates: 2026-05-10 (incident day) and 2026-05-11
// (postmortem day).
func TestIncident_Timestamps_TwoDays(t *testing.T) {
	traces := loadIncident(t)

	dates := make(map[string]bool)
	for _, tr := range traces {
		dates[tr.Timestamp.UTC().Format("2006-01-02")] = true
	}

	const wantDays = 2
	if len(dates) != wantDays {
		t.Errorf("distinct calendar dates: want %d, got %d (found: %v)",
			wantDays, len(dates), sortedKeys(dates))
	}
}

// TestIncident_Timestamps_Day1Present verifies at least 12 traces on
// 2026-05-10 (incident day). The scenario places ~15 traces on Day 1.
func TestIncident_Timestamps_Day1Present(t *testing.T) {
	traces := loadIncident(t)
	day1 := tracesForDate(traces, incidentDay1)
	const minDay1 = 12
	if len(day1) < minDay1 {
		t.Errorf("traces on %s: want >= %d, got %d", incidentDay1, minDay1, len(day1))
	}
}

// TestIncident_Timestamps_Day2Present verifies at least 5 traces on
// 2026-05-11 (postmortem day). The scenario places ~7 traces on Day 2.
func TestIncident_Timestamps_Day2Present(t *testing.T) {
	traces := loadIncident(t)
	day2 := tracesForDate(traces, incidentDay2)
	const minDay2 = 5
	if len(day2) < minDay2 {
		t.Errorf("traces on %s: want >= %d, got %d", incidentDay2, minDay2, len(day2))
	}
}

// --- Group 3: Observer positions ---

// TestIncident_Observers_FiveDistinct verifies at least 5 distinct observer
// positions across the full dataset, matching the scenario design:
// on-call-engineer, incident-commander, product-manager, monitoring-service,
// customer-support.
func TestIncident_Observers_FiveDistinct(t *testing.T) {
	traces := loadIncident(t)

	observers := make(map[string]bool)
	for _, tr := range traces {
		if tr.Observer != "" {
			observers[tr.Observer] = true
		}
	}

	const minObservers = 5
	if len(observers) < minObservers {
		t.Errorf("distinct observers: want >= %d, got %d (found: %v)",
			minObservers, len(observers), sortedKeys(observers))
	}
}

// TestIncident_Observers_RequiredPresent verifies that all five named observer
// positions appear in the dataset. Without traces from each of these positions,
// the demo cuts would produce empty or degenerate graphs.
func TestIncident_Observers_RequiredPresent(t *testing.T) {
	traces := loadIncident(t)

	required := []string{
		"on-call-engineer",
		"incident-commander",
		"product-manager",
		"monitoring-service",
		"customer-support",
	}

	for _, obs := range required {
		if found := tracesForObserver(traces, obs); len(found) == 0 {
			t.Errorf("observer %q: no traces found; required for dataset completeness", obs)
		}
	}
}

// TestIncident_Observers_MonitoringOnDay1 verifies that monitoring-service
// has at least 2 traces on 2026-05-10. Cut A uses this observer on Day 1;
// too few traces would produce a trivially sparse articulation.
func TestIncident_Observers_MonitoringOnDay1(t *testing.T) {
	traces := loadIncident(t)
	day1 := tracesForDate(traces, incidentDay1)
	monitoring := tracesForObserver(day1, "monitoring-service")
	const min = 2
	if len(monitoring) < min {
		t.Errorf("monitoring-service traces on %s: want >= %d, got %d",
			incidentDay1, min, len(monitoring))
	}
}

// TestIncident_Observers_CommanderOnDay2 verifies that incident-commander
// has at least 2 traces on 2026-05-11. Cut B uses this observer on Day 2.
func TestIncident_Observers_CommanderOnDay2(t *testing.T) {
	traces := loadIncident(t)
	day2 := tracesForDate(traces, incidentDay2)
	commander := tracesForObserver(day2, "incident-commander")
	const min = 2
	if len(commander) < min {
		t.Errorf("incident-commander traces on %s: want >= %d, got %d",
			incidentDay2, min, len(commander))
	}
}

// --- Group 4: Tag coverage ---

// TestIncident_Tags_AllSixPresent verifies all 6 required ANT tag types
// appear at least once across the full dataset. The incident response scenario
// exercises the full vocabulary:
//   - threshold: connection pool limit hit, SLA timer breach
//   - delay: alert delivery lag, auto-scaler warm-up
//   - blockage: circuit breaker tripping, runbook approval blockage
//   - amplification: customer support ticket volume spike
//   - redirection: traffic rerouted to fallback region
//   - translation: raw metrics converted to incident severity
func TestIncident_Tags_AllSixPresent(t *testing.T) {
	traces := loadIncident(t)
	assertIncidentTagsPresent(t, traces, "full incident dataset")
}

// TestIncident_Tags_Day1Coverage verifies all 6 tag types appear on
// 2026-05-10 (incident day). The incident day spans the entire operational
// vocabulary: detection, escalation, blockage, amplification, redirection.
func TestIncident_Tags_Day1Coverage(t *testing.T) {
	traces := loadIncident(t)
	day1 := tracesForDate(traces, incidentDay1)
	assertIncidentTagsPresent(t, day1, "day 1 ("+incidentDay1+")")
}

// assertIncidentTagsPresent checks for all 6 required ANT tag types in a
// slice of traces and reports missing tags. Used by multiple tag coverage tests.
func assertIncidentTagsPresent(t *testing.T, traces []schema.Trace, label string) {
	t.Helper()

	required := []string{
		"threshold",
		"delay",
		"blockage",
		"amplification",
		"redirection",
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

// --- Group 5: Special trace requirements ---

// TestIncident_AbsentSource_Present verifies at least one trace has an
// empty source slice (absent-source trace). This represents automated
// detection with no prior actor identity — the alerting system fires
// without a traceable human initiator.
func TestIncident_AbsentSource_Present(t *testing.T) {
	traces := loadIncident(t)

	found := false
	for _, tr := range traces {
		if len(tr.Source) == 0 {
			found = true
			break
		}
	}

	if !found {
		t.Error("no absent-source trace found; at least 1 trace with empty Source required")
	}
}

// TestIncident_MultiSource_Present verifies at least one trace has 2 or more
// source entries. This represents distributed agency — e.g., the postmortem
// influenced by multiple prior actors working in concert.
func TestIncident_MultiSource_Present(t *testing.T) {
	traces := loadIncident(t)

	found := false
	for _, tr := range traces {
		if len(tr.Source) >= 2 {
			found = true
			break
		}
	}

	if !found {
		t.Error("no multi-source trace found; at least 1 trace with 2+ Source entries required")
	}
}

// TestIncident_GraphRef_Present verifies at least one trace has a graph-ref
// string ("meshgraph:" or "meshdiff:" prefix) in its Source or Target.
// This exercises M5 schema functionality and confirms the incident dataset
// participates in the framework's own referential universe.
func TestIncident_GraphRef_Present(t *testing.T) {
	traces := loadIncident(t)

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

// --- Group 6: Mediation coverage ---

// TestIncident_Mediation_Rate verifies that at least 40% of traces carry
// a Mediation value. Mediation is the mechanism by which action is relayed
// and transformed; a low rate would leave most translations unexplained.
func TestIncident_Mediation_Rate(t *testing.T) {
	traces := loadIncident(t)

	mediated := 0
	for _, tr := range traces {
		if tr.Mediation != "" {
			mediated++
		}
	}

	// 40% of 20 = 8 minimum. Use 8 as the floor even for larger datasets.
	const minMediated = 8
	total := len(traces)
	rate := float64(mediated) / float64(total)
	if rate < 0.40 {
		t.Errorf("mediation rate: want >= 40%%, got %.1f%% (%d/%d traces)",
			rate*100, mediated, total)
	}
	// Also check an absolute floor to catch corner cases.
	if mediated < minMediated {
		t.Errorf("traces with Mediation: want >= %d, got %d (out of %d)",
			minMediated, mediated, total)
	}
}

// --- Group 7: Non-human actants ---

// TestIncident_NonHumanActants_Present verifies that at least 4 named
// non-human actants appear in Source or Target fields across the dataset.
// Non-human actors are central to this scenario: the outage propagates
// through technical infrastructure before humans engage.
func TestIncident_NonHumanActants_Present(t *testing.T) {
	traces := loadIncident(t)

	// These are the named non-human actants in the scenario design.
	// At least 4 must appear in source or target to confirm they are
	// active network participants.
	nonHuman := []string{
		"alerting-pipeline",
		"auto-scaler",
		"circuit-breaker",
		"sla-timer",
		"runbook-engine",
		"dashboard-service",
		"connection-pool-monitor",
		"pagerduty-webhook",
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

	const minNonHuman = 4
	if len(found) < minNonHuman {
		t.Errorf("non-human actants in source/target: want >= %d, got %d (found: %v)",
			minNonHuman, len(found), sortedKeys(found))
	}
}

// --- Group 8: ID uniqueness ---

// TestIncident_IDs_AllUnique verifies all trace IDs are distinct.
// Duplicate IDs create ambiguity in any downstream analysis that uses ID as key.
func TestIncident_IDs_AllUnique(t *testing.T) {
	traces := loadIncident(t)

	seen := make(map[string]int)
	for i, tr := range traces {
		if prev, exists := seen[tr.ID]; exists {
			t.Errorf("trace[%d] (id=%q): duplicate of trace[%d]", i, tr.ID, prev)
		}
		seen[tr.ID] = i
	}
}

// --- Group 9: Summarise ---

// TestIncident_Summarise_ElementCount verifies Summarise finds at least
// 15 distinct elements across all Source and Target slices. With 20+ traces
// and many named actants, fewer than 15 elements would indicate unexpected
// element collapse.
func TestIncident_Summarise_ElementCount(t *testing.T) {
	traces := loadIncident(t)
	s := loader.Summarise(traces)

	const minElements = 15
	if len(s.Elements) < minElements {
		t.Errorf("Summarise: distinct elements: want >= %d, got %d",
			minElements, len(s.Elements))
	}
}

// TestIncident_Summarise_GraphRefsPopulated verifies that Summarise
// populates GraphRefs with the graph-ref strings present in the dataset.
func TestIncident_Summarise_GraphRefsPopulated(t *testing.T) {
	traces := loadIncident(t)
	s := loader.Summarise(traces)

	if len(s.GraphRefs) == 0 {
		t.Error("Summarise: GraphRefs is empty; expected at least 1 graph-ref from dataset")
	}
}

// TestIncident_Summarise_MediatedTraceCount verifies Summarise counts
// at least 8 mediated traces, consistent with the >=40% mediation rate
// tested in TestIncident_Mediation_Rate.
func TestIncident_Summarise_MediatedTraceCount(t *testing.T) {
	traces := loadIncident(t)
	s := loader.Summarise(traces)

	const minMediated = 8
	if s.MediatedTraceCount < minMediated {
		t.Errorf("Summarise: MediatedTraceCount: want >= %d, got %d",
			minMediated, s.MediatedTraceCount)
	}
}

// TestIncident_Summarise_MediationsNamed verifies at least 5 distinct
// mediator names appear in the Mediations slice. The incident scenario
// has multiple named mediating protocols and systems; 5 is a conservative floor.
func TestIncident_Summarise_MediationsNamed(t *testing.T) {
	traces := loadIncident(t)
	s := loader.Summarise(traces)

	const minMediations = 5
	if len(s.Mediations) < minMediations {
		t.Errorf("Summarise: len(Mediations): want >= %d, got %d",
			minMediations, len(s.Mediations))
	}
}

// --- Group 10: Demo cut viability ---

// TestIncident_DemoCutA_NonEmpty verifies that articulating Cut A
// (monitoring-service, 2026-05-10) would include at least 1 trace.
// This is a loader-level proxy check confirming the graph package will
// not receive empty input when the demo runs.
func TestIncident_DemoCutA_NonEmpty(t *testing.T) {
	traces := loadIncident(t)

	cutAStart := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	cutAEnd := time.Date(2026, 5, 10, 23, 59, 59, 0, time.UTC)

	count := 0
	for _, tr := range traces {
		if tr.Observer == "monitoring-service" &&
			!tr.Timestamp.Before(cutAStart) &&
			!tr.Timestamp.After(cutAEnd) {
			count++
		}
	}

	if count == 0 {
		t.Error("Cut A (monitoring-service, 2026-05-10): no matching traces; demo articulation would produce empty graph")
	}
}

// TestIncident_DemoCutB_NonEmpty verifies that articulating Cut B
// (incident-commander, 2026-05-11) would include at least 1 trace.
// Same rationale as TestIncident_DemoCutA_NonEmpty.
func TestIncident_DemoCutB_NonEmpty(t *testing.T) {
	traces := loadIncident(t)

	cutBStart := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	cutBEnd := time.Date(2026, 5, 11, 23, 59, 59, 0, time.UTC)

	count := 0
	for _, tr := range traces {
		if tr.Observer == "incident-commander" &&
			!tr.Timestamp.Before(cutBStart) &&
			!tr.Timestamp.After(cutBEnd) {
			count++
		}
	}

	if count == 0 {
		t.Error("Cut B (incident-commander, 2026-05-11): no matching traces; demo articulation would produce empty graph")
	}
}
