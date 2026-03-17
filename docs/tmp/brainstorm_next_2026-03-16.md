# Brainstorm: What's Next — 2026-03-16

Prompted by a contributor inquiry and a post-v1.1.0 orientation session.

---

## Where we actually are

The kernel (L2) is solid: load → articulate → diff → follow chain → classify → equivalence criterion. The ingestion entry (L1) exists but is shallow: `meshant draft` reads a file, `meshant promote` batch-promotes. The CVE dataset has two seeded over-actorized records (E3, E14) waiting for a critique pass that doesn't exist yet. Nothing in L3 (interpretation outputs) exists.

---

## Three threads worth pulling

### Thread A — The critique pass (M12)

The DerivedFrom chain is structurally ready. E3 ("attacker") and E14 ("cve-2026-44228") are in the dataset. But the interesting question here isn't "build `meshant critique`" — it's: *what is a critique in ANT terms?*

A critique draft linked by DerivedFrom isn't a quality flag. It's a re-articulation of the same source span that asks: does the trace set actually support this as a recurring translation point, or is this a one-time attribution that was solidified into a stable actor? That's a chain query, not a lint check. A `meshant critique` subcommand could feed a draft's SourceSpan + the existing mesh into an LLM with that specific question, and the output is a new draft — linked, critiquable, not authoritative.

The philosophical alignment is strong here. The question is whether the engineering is ready.

### Thread B — Source adapters, but minimal

The contributor named codebases, CI logs, PRs, issues. The minimum useful version isn't a generic parser — it's an *extraction template* per source type. A GitHub PR has a known structure: author, reviewers, comments, diff, CI status, merge decision. The template tells the extraction step what to look for ("what changed, who was required to make something happen, from which position"). Template + existing `meshant draft` pipeline = a path from GitHub PR to TraceDraft, with no new MeshAnt code.

The question is whether this belongs in MeshAnt itself, or whether it's a separate tool/repo that feeds MeshAnt. Given the LLM-as-file boundary commitment, I lean toward: extraction templates live *outside* the CLI, as companion artifacts. MeshAnt stays the ingestion endpoint, not the extractor.

### Thread C — Shadow analysis operations

The shadow is arguably the most ANT-native concept in the framework, and it's currently underused — it exists but isn't deeply interrogable. Some operations that could come from it:

- *What elements are in everyone's shadow?* — surfaces genuine blind spots in the dataset
- *What moved from mesh to shadow across a diff?* — what disappeared, and from whose view?
- *Which observer position would un-shadow the most?* — a reading-gap analysis

This deepens L2 without requiring new input pathways. The contributor's question about the kernel being the core — this is kernel work.

---

## The tension worth holding

The contributor's SDK/compiler idea maps to Form 4 (v3.0.0+) in `docs/directions.md`. But there's a nearer version worth discussing: if you have a well-annotated trace set, can you *derive* the extraction instructions for a new domain? The mesh tells you what observer positions exist, what mediators recur, what the "what changed" vocabulary looks like. Those become the grounding for new extraction prompts. Not an agent compiler — a **trace-to-prompt** path. Closer than Form 4 and doesn't require abandoning the inversion.

---

## Open questions for colleague discussion

- Is M12 (critique pass) the priority, or is it more interesting to widen the ingestion mouth first (templates, source adapters)?
- Does the extraction template work belong in MeshAnt itself, or as a companion artifact repo?
- Is the contributor proposing to contribute directly, or feeling out the project's direction first?
- What does "a critique in ANT terms" actually mean operationally — chain query vs. LLM pass vs. human annotation?
