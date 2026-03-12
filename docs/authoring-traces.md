# Authoring Traces

A practical guide for writing your own `traces.json` files.

For the design principles behind these choices, see [docs/principles.md](./principles.md).

---

## 1. What a trace captures

A trace records a moment where something made a difference — a change, a redirect,
a transformation — observed from a specific position.

| Field | Required | Description |
|-------|----------|-------------|
| `id` | yes | Unique identifier for this trace record (lowercase UUID). |
| `timestamp` | yes | When the mediation occurred (RFC3339, e.g. `2026-04-14T09:15:00Z`). |
| `what_changed` | yes | Plain-language description of what happened. |
| `observer` | yes | The position from which this trace was recorded; shapes what is visible. |
| `source` | no | Who or what initiated the action (absent = no identifiable origin). |
| `target` | no | Who or what was acted upon. |
| `mediation` | no | What translated, transformed, or relayed the action. |
| `tags` | no | How to classify the mediation (open vocabulary). |

---

## 2. Required vs optional fields

**Required:** `id`, `timestamp`, `what_changed`, `observer`

**Optional:** `source`, `target`, `mediation`, `tags`

Absent optional fields are not missing data — they are meaningful states.
A trace without `mediation` means no intermediary was observed, not that
mediation was impossible. A trace without `source` means the origin was
genuinely unattributable, which is a finding in itself.

Do not fabricate values for optional fields to fill the form.

---

## 3. The tag types

Tags classify what kind of difference a trace records. The vocabulary is open —
you may use your own descriptors alongside these. Multiple tags are allowed.

| Tag | When to use | Example |
|-----|-------------|---------|
| `delay` | Action was slowed, queued, or deferred | Approval request sat in a queue for 18 hours before processing. |
| `threshold` | A rule, limit, or criterion was applied | Legal threshold of 70% probability required before mandatory order was valid. |
| `blockage` | Action was stopped entirely | Permit was rejected; the project could not proceed. |
| `amplification` | Effect was strengthened or multiplied | Advisory broadcast on all regional channels tripled its reach. |
| `redirection` | Action was routed to a different target | Request sent to department A was silently forwarded to department B. |
| `translation` | Content, meaning, or form was transformed | Raw sensor reading reclassified as a life-threatening scenario by a model. |

---

## 4. Handling absent sources

Leave `source` empty when:

- The action was triggered by an automated system with no single attributable actor.
- The origin is an environmental signal (a pressure drop, a sensor threshold crossing).
- The initiating process is a background routine where attributing agency to one
  element would misrepresent how the difference was produced.

Omitting `source` is methodologically honest. It records that the origin could
not be located from this observer position — not that attribution was overlooked.

```json
{
  "id": "...",
  "timestamp": "2026-04-14T08:30:00Z",
  "what_changed": "Overnight temperature drop triggered heating system activation in all monitored rooms.",
  "observer": "building-management-system",
  "target": ["heating-system-zone-a", "heating-system-zone-b"],
  "tags": ["threshold"]
}
```

---

## 5. Choosing observer positions

The `observer` field records the position from which this trace was captured.

**Good practice:**

- Use the actual vantage point: `meteorological-analyst`, `local-mayor`, `hospital-administrator`.
- Keep the string consistent across all traces captured from the same position.
- Be specific enough that two distinct positions remain distinguishable.

**Avoid:**

- Generic strings like `"system"` or `"admin"` that collapse multiple distinct
  positions into one label — this hides the cut you made.
- Invented god's-eye positions like `"ground-truth"` or `"objective-observer"` —
  all traces are situated.

Different observers will legitimately see different, partially overlapping worlds.
That is expected and analytically useful, not a problem to resolve by merging them.

---

## 6. Graph references in traces

`source` and `target` may contain graph reference strings:

- `meshgraph:<uuid>` — refers to a `MeshGraph` articulation
- `meshdiff:<uuid>` — refers to a `GraphDiff` between two articulations

Use these when the act of articulation or comparison itself became an actor in
the network — for example, when a graph produced from one set of traces was
then used as input to a decision in another trace. This is reflexive tracing:
the observation apparatus enters the mesh it observes. See
[docs/decisions/graph-as-actor-v1.md](./decisions/graph-as-actor-v1.md) for
the design rationale.

---

## 7. Worked example

A three-trace excerpt from a document approval flow. Each trace is annotated
with the reasoning behind the field choices.

```json
[
  {
    "id": "a1b2c3d4-0000-4000-8000-000000000001",
    "timestamp": "2026-03-10T09:00:00Z",
    "what_changed": "Draft contract submitted to legal team for review.",
    "source": ["procurement-officer"],
    "target": ["legal-review-queue"],
    "observer": "procurement-officer"
  },
  {
    "id": "a1b2c3d4-0000-4000-8000-000000000002",
    "timestamp": "2026-03-12T14:30:00Z",
    "what_changed": "Legal review queue held submission for 53 hours; no reviewer assigned during that window.",
    "target": ["draft-contract-v1"],
    "mediation": "legal-review-queue",
    "tags": ["delay"],
    "observer": "procurement-officer"
  },
  {
    "id": "a1b2c3d4-0000-4000-8000-000000000003",
    "timestamp": "2026-03-12T15:00:00Z",
    "what_changed": "Legal counsel revised indemnity clause; meaning of liability cap shifted from individual incidents to aggregate annual cap.",
    "source": ["legal-counsel"],
    "target": ["draft-contract-v2"],
    "mediation": "draft-contract-v1",
    "tags": ["translation"],
    "observer": "legal-counsel"
  }
]
```

**Trace 1** — required fields only; `source` and `target` are present but
`mediation` and `tags` are absent because the submission was direct with no
observable intermediary and no evident transformation.

**Trace 2** — `source` is absent: the delay was produced by the queue itself
(a structural condition), not by any single person choosing to wait.
`mediation` names the queue as the relay.

**Trace 3** — `observer` switches to `legal-counsel` because this trace was
recorded from that position; `translation` reflects that the document's meaning,
not just its text, changed.

---

## 8. Validating your traces

Command-line:

```sh
meshant validate traces.json
```

Go library:

```go
err := trace.Validate()
```

`Validate` checks that all required fields are present and well-formed. It
collects all violations in a single pass and returns them together, so you
see the full list of problems rather than stopping at the first error.
