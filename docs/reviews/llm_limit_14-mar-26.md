# LLM Limits and Countermeasures in MeshAnt

## Core problem

MeshAnt may eventually need an entrypoint that turns unstructured material into trace-like structures.

Examples:
- error logs
- administrative documents
- reports
- transcripts
- notes
- mixed human / machine operational records

The problem is that an LLM, by its nature, tends to pull such material back into familiar vocabularies:

- subject / object
- actor / agent
- intention
- role
- root cause
- stable entity

This is a serious risk for MeshAnt.

MeshAnt does **not** want to begin by treating actors as given.  
It wants to follow traces, passages, ruptures, transformations, and unresolved continuities before stabilizing actors.

So the design challenge is:

> How can an LLM help produce MeshAnt-compatible trace material without prematurely imposing actors, subjects, intentions, or fixed ontologies?

---

## Main design stance

MeshAnt should **not** ask the LLM to tell us what the actors are.

Instead, it should ask the LLM to help surface:

- where something changes
- where something passes through
- where continuity is uncertain
- where thresholds, delays, or reformattings appear
- where one operational form may become another
- where there are candidate mediations or translations
- where observer cues appear
- where the text does **not** support stable actorization

This means the LLM should be used as a helper for **difference extraction**, not as an authority for **entity stabilization**.

---

## Critical principle

### The LLM must be treated as a mediator, not an intermediary.

The LLM does not simply pass the source material through unchanged.

It:
- reformulates
- compresses
- selects
- stabilizes
- fills gaps
- imposes vocabulary
- often over-articulates

Therefore the LLM should not be treated as a transparent extractor.

It should be explicitly acknowledged as a **mediator** in the MeshAnt pipeline.

This has a major consequence:

> The LLM’s own transformations should be allowed into the graph.

That means MeshAnt should feel free to represent the LLM itself, or its outputs, as part of the articulated chain:
- raw source span
- LLM-produced draft trace
- anti-ontology critique
- human revision
- final accepted trace

In this sense, the ingestion pipeline itself becomes part of the mesh.

This is good.

It makes the mediation visible instead of pretending the extraction is neutral.

---

## What the LLM should not do

MeshAnt should avoid pipelines where the LLM is asked to directly produce final, canonical traces from raw material in one pass.

That encourages:
- hidden assumptions
- actor-first extraction
- ontology hardening
- false certainty
- hallucinated continuity
- disguised god’s-eye summaries

The LLM should not be asked questions like:
- “Who are the actors here?”
- “What is the root cause?”
- “What is the correct network?”
- “Which entity caused this?”

Those prompts push the system in the wrong direction.

---

## What the LLM should do instead

A better role for the LLM is to assist with:

### 1. Span harvesting
Identify textual spans where something appears to:
- change
- pass through a threshold
- become delayed
- be blocked
- be reformatted
- become relayed or transformed
- become more or less consequential

At this stage, do not require stable actors.

### 2. Weak trace drafting
For each span, draft only weak, provisional information such as:
- candidate `what_changed`
- candidate continuity
- candidate rupture
- possible observer cue
- possible mediation cue
- unresolved references
- uncertainty note

Do not force the system to fill `source`, `target`, or actor labels if the source does not support them.

### 3. Anti-ontology critique
Use a second pass to critique the first pass:
- where did the LLM smuggle in an actor?
- where did it infer intention too strongly?
- where did it collapse ambiguity too early?
- where did it normalize language into standard role/entity categories?
- where should the draft abstain?

### 4. Human / interactive review
Only after weak drafting and anti-ontology critique should a human or an interactive reviewer decide whether to:
- stabilize fields
- add source/target
- add mediation
- add criterion
- accept unresolved ambiguity
- preserve abstention

---

## Preferred pipeline shape

A good MeshAnt-style ingestion pipeline may look like this:

### Pass 1: raw span extraction
Input:
- raw text / logs / documents / transcripts

Output:
- candidate source spans
- no stable actorization yet

### Pass 2: weak trace draft
Output:
- draft trace candidates
- weak descriptions of change/passage/rupture
- unresolved references preserved

### Pass 3: anti-ontology critique
Output:
- critique of premature actorization
- critique of overconfident subject/object assignments
- suggestions for abstention

### Pass 4: review / refinement
Output:
- reviewed trace drafts
- accepted uncertainties
- candidate canonical traces

### Pass 5: articulation
Only then:
- produce cuts
- render bundle-like actor candidates
- compare readings
- inspect shadow

This sequence keeps the LLM from silently deciding ontology too early.

---

## Data model implication

MeshAnt probably needs something like a `TraceDraft`, distinct from a final `Trace`.

A `TraceDraft` could hold things such as:
- source span
- source document reference
- extracted candidate description
- unresolved source/target
- uncertainty note
- anti-ontology warning
- candidate observer cue
- candidate mediation cue
- provenance
- extraction stage
- extracted_by (human / LLM / reviewer pass)

This is important because it prevents the framework from pretending that the first LLM output is already a clean trace.

---

## Provenance must be preserved

Every LLM-assisted extraction should preserve provenance.

That means keeping:
- source spans
- source document identifiers
- paragraph / line references when possible
- extraction rationale
- uncertainty notes
- review history

Without provenance, the LLM’s articulation becomes too easy to mistake for an immediate reading of the source.

MeshAnt should resist that.

---

## Treating the LLM as part of the mesh

One of the most promising ideas is this:

> The LLM is not outside the mesh. It is one of the mediators in the mesh.

That means a future MeshAnt system may explicitly model:
- raw span
- LLM extraction
- critique pass
- human revision
- accepted trace

as linked stages in a graph.

This is philosophically strong and technically useful.

It means:
- the LLM’s intervention is visible
- transformation is inspectable
- disagreement can be located
- the ingestion chain itself becomes analyzable

So MeshAnt should not be shy about placing the LLM into the graph.

That is not a corruption of the framework.  
It is one of the most faithful ways to handle the fact that extraction is itself a mediated process.

---

## Prompting guidance for LLM-assisted ingestion

Prompting should actively resist premature stabilization.

Prefer prompts like:
- “Identify passages where something changes, shifts, is delayed, or is reformatted.”
- “Do not infer stable actors unless the text explicitly supports them.”
- “Preserve unresolved references rather than resolving them confidently.”
- “Mark points where continuity is unclear.”
- “Surface possible mediations without assuming final classification.”
- “State where the source does not justify stable subject/object assignments.”

Avoid prompts like:
- “Summarize the actors and their roles”
- “Identify the responsible entity”
- “Extract the agent that caused the event”
- “Convert this directly into a final causal graph”

---

## Design principle to preserve

MeshAnt should prefer:

- **difference extraction** over entity extraction
- **trace drafts** over immediate canonical traces
- **abstention** over forced completion
- **provenance** over polished summaries
- **visible mediation** over hidden extraction
- **LLM as mediator** over LLM as neutral intermediary

---

## Practical future milestone direction

A good future milestone would not be:

> “Use an LLM to automatically generate final MeshAnt traces from documents.”

A better milestone would be:

> “Build an LLM-assisted ingestion pipeline that produces trace drafts, preserves provenance, critiques premature actorization, and allows reviewed stabilization.”

That would be much more compatible with MeshAnt’s design.

---

## Final principle

MeshAnt should not ask the LLM:

> “What are the actors?”

It should ask the LLM:

> “Where do differences, passages, ruptures, transformations, and unresolved continuities appear — and where should actorization be deferred?”

That is the right orientation.
