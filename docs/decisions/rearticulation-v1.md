# Decision Record: Re-articulation v1

**Date:** 2026-03-16
**Status:** Active
**Milestone:** M12 — Anti-ontology critique pass
**Packages:** `meshant/cmd/meshant` (cmdRearticulate, cmdLineage)
**Related:** `docs/decisions/tracedraft-v1.md`, `tasks/plan_m12.md`

---

## What was decided

1. **Re-articulation is a cut, not a correction**
2. **SourceSpan is the invariant — the only field always preserved**
3. **The scaffold produces blank content fields — blank is correct, not incomplete**
4. **The critique prompt template is the methodological constraint, not the CLI**
5. **DerivedFrom is positional — original and critique have equal analytical standing**
6. **`extraction_stage: "reviewed"` names position in the pipeline, not quality**
7. **cmdLineage makes the critique chain a first-class CLI output**
8. **E3 and E14 are methodological demonstration material, not test data**

---

## Context

M11 opens the ingestion mouth: `meshant draft` reads LLM-produced extraction JSON and
produces TraceDraft records. But once a span is extracted, it enters the dataset as-is.
The LLM's vocabulary — stable actors, intentions, root causes, causal chains — enters
unchallenged.

The CVE dataset contains two seeded over-actorized records:
- E3 (`d0cve001-0000-4000-8000-000000000003`): treats "attacker" as a stable actor
- E14 (`d0cve001-0000-4000-8000-00000000000e`): treats "cve-2026-44228" as an agent

These are structurally indistinguishable from well-drafted records (Decision 9 in
`tracedraft-v1.md`). M12 introduces the mechanism for producing an alternative reading
of the same source span, linked by DerivedFrom.

---

## Decision 1: Re-articulation is a cut, not a correction

A critique draft is a parallel reading of the same SourceSpan, not a verdict on the
original. The original is not modified. The critique is a sibling, linked by DerivedFrom.

This language is enforced throughout M12:
- `cmdRearticulate` output strings use "alternative reading" language
- Plan and documentation never use "fix", "improve", "better", or "correct" in relation
  to the critique draft
- `data/prompts/critique_pass.md` section 5 explicitly states: "The original is not wrong.
  The critique is not better."

The equal standing of original and critique is a design commitment, not a rhetorical
choice. Both are inspectable analytical objects in the DerivedFrom chain.

---

## Decision 2: SourceSpan is the invariant

`cmdRearticulate` copies `source_span` verbatim and nothing else from the content fields.
`source_doc_ref` is also copied because it records ground truth provenance (where the span
came from), not interpretation.

All other fields — `what_changed`, `source`, `target`, `mediation`, `observer`, `tags`,
`uncertainty_note` — are left blank. These are interpretation fields. They belong to the
critiquing agent, not to the scaffold.

The critique prompt template (`data/prompts/critique_pass.md`) reinforces this: "Do not
paraphrase, summarize, or clean up the text." SourceSpan is the ground truth anchor.
Everything else is layered on top.

---

## Decision 3: The scaffold produces blank content fields — blank is correct

`cmdRearticulate` intentionally does not call `Validate()` on its output. Validate() would
pass (SourceSpan is present), but the intent is that ID and Timestamp are left blank —
these are assigned by a subsequent `meshant draft` call when the critiquing agent submits
the filled skeleton.

Blank content fields are correct output. They are honest abstentions: the scaffold records
that the critiquing agent has not yet provided an interpretation, not that interpretation
is missing or unknown. This mirrors Decision 3 in `tracedraft-v1.md`: "Empty is
structurally enforced."

The test `TestCmdRearticulate_SkeletonRoundTrip` verifies that skeleton output can be
decoded by `LoadDrafts` (which calls `Validate` per record). All records pass because
`source_span` is present — the only required field.

---

## Decision 4: The critique prompt template is the methodological constraint

The CLI (`cmdRearticulate`) is not the methodological constraint — it is a scaffold
generator. Without `data/prompts/critique_pass.md`, re-articulation is just another draft
with DerivedFrom set. With the template, it becomes an ANT-faithful second reading.

The template specifies:
1. What to preserve: SourceSpan verbatim
2. What to question: stable actor attributions, imputed intentions, documents-as-agents
3. What honest abstention looks like: blank fields with `uncertainty_note` explaining why
4. What DerivedFrom means: positional link, not genealogy or quality ranking
5. Worked example: E3 original vs E3 critique (side-by-side field comparison)

The CLI enforces the structural constraint (SourceSpan copied, DerivedFrom set). The
template enforces the interpretive constraint (what the critiquing agent should and should
not do).

---

## Decision 5: DerivedFrom is positional — original and critique have equal standing

DerivedFrom names a position in a sequence: "this reading came after that one." It does
not name a genealogy (the root is not more authentic), a quality ranking (the leaf is not
more refined), or a correction chain (the child is not a fix of the parent).

Both the original extraction and the critique share the same `source_span`. The DerivedFrom
link makes their relationship followable without implying a direction of improvement.

This follows Decision 5 in `tracedraft-v1.md`: "DerivedFrom is named as a positional link,
not a genealogical one."

---

## Decision 6: `extraction_stage: "reviewed"` names pipeline position, not quality

All skeletons produced by `cmdRearticulate` have `extraction_stage: "reviewed"`. This
records where in the pipeline the critique record was produced — the review step — not
a quality claim about the critique.

A "reviewed" draft may be wrong, partial, or itself over-actorized. It is "reviewed" in
the sense of "produced at the review step," not "authoritative" or "approved."

This follows Decision 7 in `tracedraft-v1.md`: "ExtractionStage names positions, not
progress." The framework does not enforce movement between stages, and "reviewed" does not
imply a higher quality than "span-harvest" or "weak-draft."

---

## Decision 7: cmdLineage makes the critique chain a first-class CLI output

Re-articulation without a lineage reader produces linked records that can only be read
by inspecting raw JSON. `cmdLineage` walks DerivedFrom links and renders chains as
indented trees, making the critique-as-chain visible as a CLI output.

cmdLineage is a chain reader, not a diff tool. It shows structure (which reading followed
which, at what stage, by whom), not differences between chain members. Comparing any two
readings in a chain is the analyst's job.

Cycle detection is mandatory: a circular DerivedFrom reference (A→B→A) returns an error
naming the cycle rather than silently looping or producing incorrect output.

---

## Decision 8: E3 and E14 are methodological demonstration material, not test data

`data/examples/cve_critique_drafts.json` contains filled critique drafts for E3 and E14.
These are not authoritative — they are worked examples for the methodological demonstration
described in `data/prompts/critique_pass.md` section 5.

A human reviewer or a different LLM pass might produce different critique drafts for the
same spans. That is expected. E3 and E14 are not ground truth for what critique should
look like; they are one instance of what critique can look like.

Their seeded over-actorization (Decision 9 in `tracedraft-v1.md`) was intentional: they
were placed in the dataset to be the targets of critique demonstration. They are not
evidence that the framework failed to catch them — they are evidence that the framework
preserves them for deliberate analytical engagement.

---

## What M12 does NOT do

- **Automated critique**: the critiquing agent is always external (human or LLM via file)
- **Live LLM calls**: the file-as-boundary from M11 is preserved (Decision 4 in
  `tracedraft-v1.md`)
- **Criterion-aware critique**: EquivalenceCriterion from M10.5+ is not wired into the
  critique pass — this is a future milestone
- **Structural distinction between original and critique**: both are `TraceDraft` records;
  the DerivedFrom link is the only structural difference

---

## Related

- `docs/decisions/tracedraft-v1.md` — Decision 2 (SourceSpan), Decision 5 (DerivedFrom),
  Decision 7 (ExtractionStage), Decision 9 (over-actorized by design)
- `data/prompts/critique_pass.md` — extraction contract for the critique step
- `data/examples/cve_critique_skeleton.json` — skeleton output for all CVE drafts
- `data/examples/cve_critique_drafts.json` — filled critique drafts for E3 and E14
- `tasks/plan_m12.md` — full M12 plan and design rules
