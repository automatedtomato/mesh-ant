// incident_e2e_test.go exercises the full Load → Articulate → Diff pipeline
// against the software incident response dataset (incident_response.json).
//
// Scenario: a database connection pool exhaustion during a flash sale brings
// down a high-traffic e-commerce API. Day 1 (2026-05-10) covers detection,
// escalation, and mitigation. Day 2 (2026-05-11) covers the postmortem and
// process changes.
//
// Two demo cuts expose structural blindness between observer positions:
//   - Cut A: observer "monitoring-service", day 2026-05-10
//     The automated detection chain is visible; human coordination is invisible.
//   - Cut B: observer "incident-commander", day 2026-05-11
//     Postmortem deliberations are visible; the automated metric chain is invisible.
//
// Diffing Cut A against Cut B makes the near-disjoint structure of these
// two epistemic positions visible — neither observer can see the full mesh.
package graph_test

import (
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// incidentDatasetPath is the path from the graph package directory to the
// incident response example dataset. Go test sets cwd to the package directory.
const incidentDatasetPath = "../../data/examples/incident_response.json"

// articulateIncidentCutA returns a graph articulated for monitoring-service
// on Day 1 (2026-05-10). This cut sees the automated detection chain:
// connection-pool-monitor, alerting-pipeline, pagerduty-webhook.
// Human coordination (incident-commander, product-manager) is invisible.
func articulateIncidentCutA(t *testing.T) graph.MeshGraph {
	t.Helper()
	traces, err := loader.Load(incidentDatasetPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"monitoring-service"},
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-05-10T00:00:00Z"),
			End:   mustParseTime(t, "2026-05-10T23:59:59Z"),
		},
	}
	return graph.Articulate(traces, opts)
}

// articulateIncidentCutB returns a graph articulated for incident-commander
// on Day 2 (2026-05-11). This cut sees postmortem deliberations and process
// corrections. The automated metric chain from Day 1 is invisible.
func articulateIncidentCutB(t *testing.T) graph.MeshGraph {
	t.Helper()
	traces, err := loader.Load(incidentDatasetPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	opts := graph.ArticulationOptions{
		ObserverPositions: []string{"incident-commander"},
		TimeWindow: graph.TimeWindow{
			Start: mustParseTime(t, "2026-05-11T00:00:00Z"),
			End:   mustParseTime(t, "2026-05-11T23:59:59Z"),
		},
	}
	return graph.Articulate(traces, opts)
}

// TestIncident_CutA_MonitoringService_Day1 articulates with observer
// "monitoring-service" on 2026-05-10 and verifies the graph contains the
// automated detection actors. Human coordination should be in the shadow
// because it is invisible to the monitoring system.
func TestIncident_CutA_MonitoringService_Day1(t *testing.T) {
	g := articulateIncidentCutA(t)

	// The monitoring-service cut must include at least 1 trace.
	if g.Cut.TracesIncluded == 0 {
		t.Fatal("Cut A: TracesIncluded == 0; monitoring-service must have traces on day 1")
	}

	// alerting-pipeline must appear as a node: it is the central automated
	// actor in the detection chain visible to the monitoring-service.
	if _, ok := g.Nodes["alerting-pipeline"]; !ok {
		t.Error("Nodes: alerting-pipeline not found in monitoring-service cut; " +
			"expected in source/target of detection traces")
	}

	// connection-pool-monitor is the non-human actant that first detects the
	// exhaustion — it must be visible in the monitoring-service cut.
	if _, ok := g.Nodes["connection-pool-monitor"]; !ok {
		t.Error("Nodes: connection-pool-monitor not found in monitoring-service cut; " +
			"expected as detection chain origin")
	}

	// The shadow must be non-empty: human coordination (incident-commander,
	// product-manager) happens outside the monitoring-service's view.
	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty for monitoring-service cut; " +
			"human coordination actors should be invisible to the monitoring system")
	}
}

// TestIncident_CutB_IncidentCommander_Day2 articulates with observer
// "incident-commander" on 2026-05-11 (postmortem day) and verifies the
// graph contains postmortem-related elements. The automated detection
// chain from Day 1 should be in the shadow.
func TestIncident_CutB_IncidentCommander_Day2(t *testing.T) {
	g := articulateIncidentCutB(t)

	// The incident-commander cut on Day 2 must include at least 1 trace.
	if g.Cut.TracesIncluded == 0 {
		t.Fatal("Cut B: TracesIncluded == 0; incident-commander must have traces on day 2")
	}

	// Postmortem traces must be present as edges; at least 1 edge confirms
	// the articulation is non-trivial.
	if len(g.Edges) == 0 {
		t.Error("Edges: want at least 1 edge in incident-commander day-2 cut")
	}

	// The shadow must be non-empty: the automated metric detection chain
	// (Day 1 monitoring traces) is invisible to the incident-commander on Day 2.
	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty for incident-commander day-2 cut; " +
			"automated monitoring chain should be invisible to the incident commander postmortem view")
	}
}

// TestIncident_Diff_CutA_CutB diffs the two demo cuts and verifies the
// structural contrast between them. The two observer positions are designed
// to see near-disjoint sub-networks, so NodesAdded, NodesRemoved, and
// ShadowShifts should all be non-empty.
func TestIncident_Diff_CutA_CutB(t *testing.T) {
	gA := articulateIncidentCutA(t)
	gB := articulateIncidentCutB(t)
	d := graph.Diff(gA, gB)

	// NodesAdded: elements visible to incident-commander on Day 2 that were
	// not visible to monitoring-service on Day 1.
	if len(d.NodesAdded) == 0 {
		t.Error("NodesAdded: want > 0 (postmortem elements not visible to monitoring-service), got 0")
	}

	// NodesRemoved: elements visible to monitoring-service on Day 1 that are
	// not visible to incident-commander on Day 2.
	if len(d.NodesRemoved) == 0 {
		t.Error("NodesRemoved: want > 0 (detection chain elements not visible to incident-commander), got 0")
	}

	// ShadowShifts: elements crossing the visibility boundary between cuts.
	// This is the core ANT insight — the two positions compose different worlds.
	if len(d.ShadowShifts) == 0 {
		t.Error("ShadowShifts: want > 0 (elements moving between shadow and visibility across cuts), got 0")
	}
}

// TestIncident_CutA_HasShadow verifies that the monitoring-service cut
// has shadow elements. Human coordination (incident-commander, product-manager,
// customer-support) is invisible to the monitoring system — they never appear
// in the automated detection chain traces.
func TestIncident_CutA_HasShadow(t *testing.T) {
	g := articulateIncidentCutA(t)

	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty for monitoring-service cut; " +
			"human coordination actors are structurally invisible to automated monitoring")
	}
}

// TestIncident_CutB_HasShadow verifies that the incident-commander cut
// has shadow elements. The automated metric chain (connection-pool-monitor,
// alerting-pipeline, pagerduty-webhook) that triggered the original incident
// is not part of the incident-commander's Day 2 postmortem view.
func TestIncident_CutB_HasShadow(t *testing.T) {
	g := articulateIncidentCutB(t)

	if len(g.Cut.ShadowElements) == 0 {
		t.Error("ShadowElements: want non-empty for incident-commander day-2 cut; " +
			"automated metric detection chain should be invisible from postmortem vantage point")
	}
}
