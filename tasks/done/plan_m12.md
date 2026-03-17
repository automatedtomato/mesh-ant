# Plan: M12 — Anti-Ontology Critique Pass (Re-articulation as a Second Cut)

**Date**: 2026-03-16
**Status**: Confirmed — follows from M11 (TraceDraft + provenance-first ingestion) and brainstorm session 2026-03-16
**Parent issue**: #50
**Source**: `docs/tmp/brainstorm_next_2026-03-16.md`, `docs/decisions/tracedraft-v2.md`

---

## Problem

M11 opens the ingestion mouth: `meshant draft` reads LLM-produced extraction JSON and
produces TraceDraft records. But the path from raw material into the mesh is a one-way
funnel: once a span is extracted, it enters the dataset as-is. The LLM's vocabulary —
stable actors, intentions, root causes, causal chains — enters unchallenged.

The CVE dataset already contains two seeded over-actorized records (E3: "attacker", E14:
"cve-2026-44228"). These are structurally indistinguishable from well-drafted records by
design (Decision 9 in `tracedraft-v2.md`). What is missing is a mechanism to *critically
re-examine* them — not to correct them, but to produce an alternative reading of the same
source span that is equally provenance-bearing and equally provisional.

Widening the ingestion mouth (more source types, more templates) before this mechanism
exists would make LLM vocabulary repatriation harder to resist at scale.

---

## Goal

Introduce re-articulation as a first-class operation: given a TraceDraft, produce an
alternative TraceDraft of the same SourceSpan, linked by DerivedFrom.

The re-articulation is a second cut of the same material. It is:
- Not a correction (the original is not modified)
- Not a quality check (there is no "better" result)
- Not automated (the critiquing agent is always external)

It is another node in the DerivedFrom chain — inspectable, challengeable, itself
provenance-bearing.

---

## Non-goals

- Automated critique judgment or scoring
- Live LLM calls (the boundary remains a file on disk, per tracedraft-v2.md Decision 4)
- New TraceDraft fields (the existing schema fully supports this)
- Structural distinction between "good" and "over-actorized" drafts (Decision 9)
- Finalising which observer positions should produce critique drafts (future: M12.5+)

---

## Design principles

### P1: Re-articulation is a cut, not a correction

The subcommand must not frame itself as "fixing" or "improving" drafts. A critique draft
is a parallel reading, not a verdict on the original. Both the original and the critique
are analytical objects with equal standing in the DerivedFrom chain.

### P2: SourceSpan is the invariant

A re-articulation must preserve the SourceSpan verbatim. This is the ground truth anchor
(tracedraft-v2.md Decision 2). Everything else — WhatChanged, Source, Target, Mediation,
Observer — is the critiquing agent's interpretation and may differ freely.

### P3: The scaffold's job is to set DerivedFrom, not to pre-fill content

`meshant rearticulate` outputs a skeleton: SourceSpan copied, ID blank (to be assigned
by `meshant draft`), DerivedFrom set to the original's ID, content fields blank. The
critiquing agent fills in the interpretation. Blank content fields are correct output for
the scaffold — they are honest abstentions, not missing data (tracedraft-v2.md Decision 3).

### P4: The critique prompt template is the methodological constraint

Without a documented extraction contract for the critique step, a re-articulation pass
just produces a different draft, not necessarily an ANT-faithful one. The template
specifies: what to preserve (SourceSpan verbatim), what to question (stable actor
attributions, imputed intentions), what an honest abstention looks like.

### P5: The lineage reader makes the chain followable

Re-articulation without a lineage reader produces linked records that can only be read
by inspecting raw JSON. A `meshant lineage` subcommand that walks DerivedFrom links
makes the critique chain visible as a first-class CLI output.

---

## Phases

### Phase 1 — Re-articulation scaffold (M12.1, issue #51)

**File**: `meshant/cmd/meshant/main.go` (new `cmdRearticulate`)

`meshant rearticulate <drafts.json>` reads a drafts file and outputs a skeleton JSON
array: for each draft, one skeleton record with:
- `source_span`: copied verbatim from the original
- `derived_from`: set to the original's ID
- `source_doc_ref`: copied if present (ground truth provenance, not interpretation)
- All content fields (`what_changed`, `source`, `target`, `mediation`, `observer`,
  `tags`, `uncertainty_note`): blank (P3)
- `id`, `timestamp`: blank (to be assigned by `meshant draft`)
- `extraction_stage`: `"reviewed"` (position in the pipeline, not a quality claim)
- `extracted_by`: blank (to be filled by the critiquing agent)

**Flags**:
- `--id <id>` — produce skeleton for a single draft by ID (default: all drafts)
- `--output <path>` — write skeleton JSON to file (default: stdout)

**Workflow**:
```
meshant rearticulate cve_response_drafts.json --id <E3-id> > critique_skeleton.json
# analyst (or LLM) fills in interpretation fields
meshant draft critique_skeleton.json --extracted-by "human-reviewer" --stage "reviewed"
```

**Re-articulation fixture**:
- `data/examples/cve_critique_skeleton.json` — skeleton output for E3 and E14
- `data/examples/cve_critique_drafts.json` — filled critique drafts for E3 and E14
  (E3: resists naming "attacker" as stable actor; E14: names "cve-2026-44228" as a
  document, not an agent)

**Tests**: `meshant/cmd/meshant/main_test.go` (Group 14)
- Valid drafts file → skeleton output with SourceSpan + DerivedFrom set
- `--id` flag → single skeleton only
- `--output` flag → file written
- Missing drafts file → error
- Malformed JSON → error
- Draft with no ID (edge case) → error with helpful message
- Skeleton round-trips through `meshant draft` (loaded by LoadDrafts, passes Validate)

### Phase 2 — DerivedFrom lineage reader (M12.2, issue TBD)

**File**: `meshant/cmd/meshant/main.go` (new `cmdLineage`)

`meshant lineage <drafts.json>` reads a drafts file and prints the DerivedFrom chains
present in the dataset. For each chain root (a draft with no DerivedFrom), it prints
the lineage tree: root → critique → revision → ...

**Output format**:
```
=== DerivedFrom Chains ===

[E1-id] span-harvest / llm-pass1
  "The dependency fastmiddleware v1.3.1 is affected..."
  └── [E3-critique-id] reviewed / human-reviewer  ← DerivedFrom E1-id
        "fastmiddleware v1.3.1 was flagged by dependabot-bot..."

Standalone drafts (no DerivedFrom, no children): 12
```

**Flags**:
- `--id <id>` — show lineage for a single draft only
- `--format text|json` — text (default) or JSON (for downstream processing)

**Tests**: `meshant/cmd/meshant/main_test.go` (Group 15)
- Dataset with DerivedFrom chains → chains rendered
- Dataset with no chains → "no chains" message
- `--id` → single chain only
- `--format json` → JSON envelope with chain structure
- Circular DerivedFrom reference → error (cycle detection)
- Missing file → error

### Phase 3 — Anti-ontology critique prompt template (M12.3, issue TBD)

**File**: `data/prompts/critique_pass.md`

Documented extraction contract for the critique step. This is the methodological
constraint that makes re-articulation ANT-faithful rather than just "another draft."

Contents:
1. **What to preserve**: SourceSpan verbatim. Do not paraphrase. The span is the ground
   truth anchor. Your critique is a reading of the span, not a replacement for it.

2. **What to question**: Any attribution to a stable, pre-formed actor. Specifically:
   - Named entities treated as agents with intentions ("attacker targeted...", "CVE
     exploited...")
   - Causal chains that imply a single responsible originator
   - Source/target assignments where the span only shows a condition, not an act

3. **What an honest abstention looks like**: If you cannot confidently name a source,
   target, or mediation from the span alone, leave the field blank. An empty field is
   correct — it records that the span does not support the attribution, not that the
   attribution is missing.

4. **What DerivedFrom means**: Your critique is linked to the original by DerivedFrom.
   The original is not wrong. Your critique is a second reading. Both are analytical
   objects. The differentiation between them is visible in the chain.

5. **Worked example**: E3 original vs. E3 critique
   - Original: `source: ["attacker"]`, `target: ["storefront-api"]`, no uncertainty_note
   - Critique: `source: []`, `target: ["storefront-api"]`, `uncertainty_note: "The span
     names a vulnerability class, not an attributable actor. 'Attacker' is an inference
     from the CVE framing, not from the span itself."`

**Tests**: No code tests. Fixture test: `data/examples/cve_critique_drafts.json` is
the living proof-of-concept for this template.

### Phase 4 — Decision record + codemap (M12.4, issue TBD)

**File**: `docs/decisions/rearticulation-v2.md`

Decisions to record:
1. Re-articulation is a cut, not a correction (P1)
2. SourceSpan is the invariant — the only field always preserved (P2)
3. The scaffold produces blank content fields — blank is correct, not incomplete (P3)
4. The critique prompt template is the methodological constraint, not the CLI (P4)
5. DerivedFrom is positional, not genealogical — original and critique have equal standing
6. `extraction_stage: "reviewed"` names position in the pipeline, not quality
7. `cmdLineage` makes the critique chain a first-class CLI output — chains are not just
   data relationships, they are the visible record of the analytical process
8. Over-actorized records are seeded (E3, E14) and intended for critique — not as test
   data but as methodological demonstration material

Update `docs/CODEMAPS/meshant.md` and `tasks/todo.md`.

---

## Key design rules

1. **The critique does not modify the original.** The original draft remains in the dataset
   unchanged. The critique is a sibling, linked by DerivedFrom.

2. **Blank content fields are correct scaffold output.** `meshant rearticulate` must not
   pre-fill content fields from the original, even partially. Pre-filling would bias the
   critique toward the original's reading.

3. **The lineage reader is not a diff tool.** It shows chains, not differences between
   chain members. Comparing original and critique is the analyst's job.

4. **The critique prompt template must not frame re-articulation as improvement.** The
   template should say "alternative reading," not "better reading" or "corrected reading."

---

## What M12 does NOT do

- Automated critique (the critiquing agent is always external — human or LLM-via-file)
- Live LLM calls
- Criterion-aware critique (EquivalenceCriterion from M10.5+ is not wired into the
  critique pass yet — this is a future milestone)
- Multi-step chain management (the scaffold handles one re-articulation step; iterative
  critique chains are composed manually for now)

---

## What this enables (future milestones, not planned here)

- **Criterion-aware critique**: a re-articulation that uses an EquivalenceCriterion to
  declare what it is preserving or questioning
- **Critique as a trace**: the act of re-articulation recorded as a canonical Trace in
  the mesh (following the ArticulationTrace pattern from M7)
- **Automated critique pipeline**: `meshant critique` with live LLM — deferred to v2.0.0
- **Inter-chain comparison**: comparing two lineage trees over the same source material

---

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Critique framed as "fixing" in UX or docs | Medium | Explicit P1 in all output strings and docs; "alternative reading" not "correction" |
| SourceSpan pre-interpreted in template | Low | P2: SourceSpan is copied verbatim, never paraphrased by the scaffold |
| Lineage reader becomes a diff tool | Low | Named explicitly as a chain reader, not a comparison tool |
| E3/E14 fixtures treated as ground truth | Medium | Decision record explicitly states these are methodological demonstration material, not authoritative |

---

## Estimated scope

- Phase 1 (M12.1): medium — new subcommand, 2 fixtures, ~8 tests
- Phase 2 (M12.2): medium — new subcommand, chain-walk logic, ~8 tests
- Phase 3 (M12.3): small — documentation + fixture, no code
- Phase 4 (M12.4): small — docs only

Total: moderate scope, high methodological weight.

---

## Related

- `docs/decisions/tracedraft-v2.md` — Decision 2 (SourceSpan as ground truth), Decision 5
  (DerivedFrom chain), Decision 9 (over-actorized by design)
- `docs/directions.md` — Layer 1 work, ingestion critique before widening the mouth
- `data/examples/cve_response_extraction.json` — E3, E14 (seeded re-articulation targets)
- `tasks/plan_m11.md` — TraceDraft schema, ingestion contract
- `tasks/plan_m10_5_plus.md` — EquivalenceCriterion (future: criterion-aware critique)
