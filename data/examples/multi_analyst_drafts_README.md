# multi_analyst_drafts.json — Companion Note

This dataset contains TraceDraft records produced by two analyst positions
(`analyst-a` and `analyst-b`) reading the same incident report: a database
connection pool exhaustion event triggered by a flash sale traffic spike.

The file is designed to exercise `meshant extraction-gap` (C.2) and
`meshant chain-diff` (C.3). The divergences described below are deliberate.
Neither position is adjudicated here — each reflects where it stands.

---

## Source scenario

An e-commerce platform's primary database cluster reached 95% connection
pool saturation (190 of 200 connections) during a flash sale. An alert fired,
the on-call engineer was paged, and the pool limit was raised from 200 to 400
as an immediate mitigation.

The incident report from which both analysts extracted drafts is referenced as
`incident_report_2026_db_pool.md` (hypothetical; not included in this repo).

---

## Analyst positions

| Label | Description |
|---|---|
| `analyst-a` | Reads the incident from a systems-monitoring perspective; treats the connection-pool-monitor as the primary detection actor |
| `analyst-b` | Reads the incident from a platform-engineering perspective; treats Prometheus alerting as the detection layer and emphasises the causal role of the flash sale load |

Both positions used `extraction_stage: "weak-draft"` as their initial stage.
Analyst-b produced a reviewed revision of one span (see classification divergence below).

---

## Span inventory

### Shared spans (both analysts extracted)

**`span-connection-pool-saturation`**
The saturation event itself.

- *Mediation disagreement*: analyst-a names `connection-pool-saturation-threshold-policy`
  as the mediator; analyst-b names `prometheus-alertmanager-rules`. The source material
  does not identify the detection layer explicitly — both readings are consistent with
  the text.
- *Source/target disagreement*: follows from the mediation disagreement. Analyst-a
  identifies `connection-pool-monitor` as source; analyst-b identifies
  `prometheus-alertmanager`.
- *`(multiple-drafts)` flag*: both analysts produced a two-draft derivation chain for
  this span, so field-by-field comparison is surfaced as `(multiple-drafts)` in the
  extraction gap report.

**`span-alert-classification`**
The act of classifying and routing the alert.

- *`what_changed` disagreement*: analyst-a frames this as the alerting pipeline's
  classification act (pipeline → pagerduty-webhook); analyst-b frames it as the
  on-call engineer's receipt of the page (pagerduty-webhook → on-call-engineer).
  The source span is compatible with both readings — it describes both the
  classification and the delivery in a single sentence.
- *Mediation, source, and target all differ* as a consequence of the framing difference.
- *`uncertainty_note` differs*: analyst-a has no note; analyst-b records their interpretive
  reasoning in the note field. This surfaces as a fifth field-level disagreement in the
  extraction-gap report alongside the four content-field differences.

**`span-remediation-decision`**
The engineer's decision to increase the connection pool limit.

- *No disagreement*: both positions extracted this span identically. Convergence
  alongside divergence is intentional — not every span will be contested.

---

### Spans extracted by one analyst only

**`span-pagerduty-retry-queue`** — analyst-a only

Analyst-a extracted the 75-second alert delivery lag as a distinct event.
Analyst-b did not extract it separately; this detail may have been absorbed
into analyst-b's `span-alert-classification` reading or treated as noise.

**`span-flash-sale-load-pattern`** — analyst-b only

Analyst-b extracted the flash sale traffic spike as an explicit causal precondition.
Analyst-a did not extract it as a separate span; from analyst-a's perspective, the
flash sale is context, not a traceable event in itself.

---

## Classification divergence (chain-diff)

For `span-connection-pool-saturation`, both analysts produced a two-draft chain:

| Analyst | Step | Classification | Reason |
|---|---|---|---|
| analyst-a | 1 | `mediator` | Content fields revised; extraction_stage unchanged (both `weak-draft`) |
| analyst-b | 1 | `translation` | Content fields revised AND extraction_stage advanced (`weak-draft` → `reviewed`) |

Note: the `mediator` classification also applies when the extraction_stage advances but
no content fields change (endorsement step). The dataset does not exercise that path —
analyst-a's step 1 is a content-change mediator, not an endorsement mediator.

Run to reproduce:

```
meshant chain-diff \
  --analyst-a analyst-a \
  --analyst-b analyst-b \
  --span span-connection-pool-saturation \
  data/examples/multi_analyst_drafts.json
```

---

## Extraction gap summary

Run to reproduce:

```
meshant extraction-gap \
  --analyst-a analyst-a \
  --analyst-b analyst-b \
  data/examples/multi_analyst_drafts.json
```

Expected structure:
- Only in A: 1 (`span-pagerduty-retry-queue`)
- Only in B: 1 (`span-flash-sale-load-pattern`)
- In both: 3
- Disagreements: 6 (5 field-level on `span-alert-classification` + 1 `(multiple-drafts)` on `span-connection-pool-saturation`)

---

## What this dataset does not show

Spans neither analyst extracted are not visible in this report.
The dataset cannot tell you what the incident report contains that both
analysts missed — only what each position chose to make traceable.

Neither position is authoritative. Each reflects where it stands.
