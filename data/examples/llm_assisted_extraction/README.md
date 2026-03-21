# LLM-Assisted Extraction Example

This directory documents a complete v2.0.0 pipeline run using the open source governance
domain: adoption of a mandatory code-signing policy by the libvalidate project following
a supply chain compromise.

**Pipeline**:
```
source_document.md
  → meshant extract   → raw_drafts.json + extraction_session.json
  → meshant assist    → reviewed_drafts.json
  → meshant promote   → promoted_traces.json
  → meshant articulate → articulation_output.json
```

---

## Source document

`source_document.md` is a composite governance record (7 spans) covering the period
2026-02-01 to 2026-03-15. It is new territory for the existing example datasets: the
domain is **open source governance** — how a policy travels through deliberative bodies
before becoming a structural enforcement mechanism.

The document was chosen because it contains:
- Clear mediating instruments (committee charters, bylaws articles, governance rules)
- Spans where source/target attribution is unambiguous, and spans where it is not
- A dissent event that tests whether the framework reads absence-of-compliance or
  re-articulation-from-a-different-position

---

## Extraction session

`extraction_session.json` is the `SessionRecord` produced by `meshant extract`. It
records the extraction conditions: model ID, prompt template path, source document
reference, and the 7 draft IDs produced. The session ID
(`f5000000-0000-4000-8000-000000000001`) is stamped on every raw draft's `session_ref`
field, linking each draft to the conditions under which it was made.

**Commands used**:
```
meshant extract \
  --source-doc data/examples/llm_assisted_extraction/source_document.md \
  --source-doc-ref data/examples/llm_assisted_extraction/source_document.md \
  --model claude-sonnet-4-6 \
  --output data/examples/llm_assisted_extraction/raw_drafts.json \
  --session-output data/examples/llm_assisted_extraction/extraction_session.json
```

---

## Raw drafts (LLM extraction pass)

`raw_drafts.json` contains 7 `weak-draft` records. All carry:
- `extracted_by: "claude-sonnet-4-6"` — the model is named, not abstracted
- `session_ref: "f5000000-0000-4000-8000-000000000001"` — links to extraction_session.json
- `uncertainty_note: "LLM-produced candidate; unverified by human review"` — framework-appended

The LLM produced analytically defensible readings on most spans but introduced two
divergences that surfaced during human review (see below).

---

## Human review (meshant assist)

`reviewed_drafts.json` contains 9 drafts: the 7 from the extraction pass (carried
forward, some accepted, some retained as progenitors) plus 2 human-derived corrections.

The `meshant assist` session walked each span interactively. Five spans were accepted
without change. Two were edited.

---

## Divergences

### Divergence A — LLM was more faithful to the source (Span 2: working group formation)

The LLM extracted `source: ["board-security-committee"]` and
`mediation: "foundation-committee-charter"`, directly naming the actors and instrument
present in the span.

The reviewer's first instinct was to generalize to `source: ["foundation-board"]` and
`mediation: "security-charter"` — a coarser reading that elides the specific committee
and the specific charter article. On reflection, the LLM's reading was more faithful to
the span. The reviewer accepted the LLM draft without editing.

This is **Divergence A**: the LLM's candidate was adopted over the reviewer's initial
impulse. This is not unusual — LLM extraction can be faithful to specific named actors
where a human reviewer reaches for familiar categories.

**Note**: This cut names the LLM as a more precise reader in this instance. A different
observer position would read the same span differently. The cut itself is one reading, not
a verdict about model capability.

### Divergence B — Reviewer was more faithful to the source (Span 5: ratification)

The LLM extracted `mediation: "majority-vote-protocol"` — a reasonable generalization,
but the span explicitly names `foundation-bylaws-article-7` as the governing instrument.
The LLM compressed the specific governing document into a generic process name.

The reviewer edited the draft:
- `mediation` changed from `"majority-vote-protocol"` to `"foundation-bylaws-article-7"`
- `what_changed` updated to reference article 7 directly

Both versions are preserved in `reviewed_drafts.json`. The LLM draft
(`f5000001-0000-4000-8000-000000000005`) and the human-derived revision
(`f5000002-0000-4000-8000-000000000005`) form a derivation chain.
`DerivedFrom: "f5000001-0000-4000-8000-000000000005"` links them.

### Divergence C — Tag classification (Span 7: formal dissent)

The LLM tagged the dissent record as `["blockage"]`. The reviewer re-tagged it as
`["translation"]`.

The analytical difference: the dissent does not block the policy — the policy was already
ratified and deployed. The dissenting maintainers are re-articulating the policy from a
different observer position: translating enforcement into an accessibility exclusion. This
is a translation event (the objection transforms what the policy *means* from a
contributor-facing position), not a blockage.

The reviewer also changed `target: ["PROPOSAL-CSRP-001"]` to
`target: ["low-resource-contributors"]` and `observer: "registry-security-team"` to
`observer: "dissenting-maintainers"` — the dissent is visible from the position of the
dissenters, not the security team.

Both versions appear in `reviewed_drafts.json`. The LLM draft
(`f5000001-0000-4000-8000-000000000007`) and the human-derived revision
(`f5000002-0000-4000-8000-000000000007`) form a derivation chain.

---

## Promotion

`promoted_traces.json` was produced by running `meshant promote` on `reviewed_drafts.json`.
All 9 drafts were promotable (valid UUID, non-empty `what_changed` and `observer`), so
all 9 were promoted.

```
meshant promote \
  --output data/examples/llm_assisted_extraction/promoted_traces.json \
  data/examples/llm_assisted_extraction/reviewed_drafts.json
```

In a more selective pipeline, a user might extract only the `reviewed`-stage drafts
before promoting — retaining only the human-revised records rather than both versions.
This example promotes all to show the full provenance chain in the trace layer.

---

## Articulation

`articulation_output.json` was produced from the registry-security-team observer position.
Of the 9 promoted traces, 6 are visible from this position; 3 are shadowed (from
`governance-working-group` and `dissenting-maintainers` observer positions).

```
meshant articulate \
  --observer registry-security-team \
  --format json \
  --output data/examples/llm_assisted_extraction/articulation_output.json \
  data/examples/llm_assisted_extraction/promoted_traces.json
```

The articulation names its own cut: this is one observer position. A `dissenting-maintainers`
articulation would render a different graph — including the accessibility-exclusion trace
(`f5000002-0000-4000-8000-000000000007`) that is shadowed here.

---

## Sessions and provenance

Two session records are associated with this example:

- **`extraction_session.json`** — `f5000000-0000-4000-8000-000000000001` — the
  `meshant extract` session. Referenced by `session_ref` on all 7 raw drafts.
- **Assist session** — `f5000000-0000-4000-8000-000000000002` — the `meshant assist`
  session. Referenced by `session_ref` on the 2 human-derived reviewed drafts.
  The assist session record is not included in this directory; it is recorded by
  `meshant assist` as a sibling file alongside the output in practice.

Both sessions are structurally distinct: the extract session records the LLM's extraction
conditions; the assist session records the human reviewer's curation decisions
(accept/edit dispositions per span).
