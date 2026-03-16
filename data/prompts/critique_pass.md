# Critique Pass — Extraction Contract

**Purpose**: This document is the methodological constraint for the re-articulation step.
It specifies what the critiquing agent should preserve, what to question, and what
an honest abstention looks like. Without this contract, re-articulation produces another
draft — not necessarily an ANT-faithful alternative reading.

A critique draft is a second cut of the same source span, not a correction of the
original. Both the original and the critique are analytical objects with equal standing
in the DerivedFrom chain.

---

## 1. What to preserve: SourceSpan verbatim

Copy `source_span` exactly as it appears in the original draft. Do not paraphrase, summarize,
or clean up the text. The span is the ground truth anchor — your critique is a reading of
the span, not a replacement for it.

If you find yourself editing the span, stop. The span is not yours to edit. Your job is
to produce an alternative interpretation of an unchanged span.

---

## 2. What to question: stable actor attributions and imputed intentions

Look for any of the following in the original draft's content fields and ask whether the
source span actually supports them:

- **Named entities treated as agents with intentions**. Does the span name a specific
  actor performing an action, or does it describe a condition, mechanism, or document?
  Example: "attacker targeted the API" implies a motivated agent. The span may only
  describe a class of requests.

- **Causal chains implying a single responsible originator**. Does the span name who or
  what caused the condition? Or does it describe a state without naming an originator?

- **Source/target assignments where the span shows a condition, not an act**. A span
  describing "a vulnerability exists in the middleware" does not name a source that acted.
  It names a condition in a component.

- **CVE identifiers or document references treated as agents**. A CVE number is an
  identifier — a record in a database. It does not "exploit" or "threaten". If the
  original treats it as an actor, question whether the span supports that framing.

The goal is not to produce a "better" draft. It is to produce an alternative reading that
makes different analytical choices about what the span does and does not support.

---

## 3. What honest abstention looks like

If you cannot confidently attribute a source, target, or mediation from the span alone,
leave the field blank.

An empty field is correct — it records that the span does not support the attribution,
not that the attribution is missing or unknown. Blank is the honest answer, not a gap.

Use `uncertainty_note` to explain your abstention. A note like "the span describes a
condition, not an act — no attributable source" is more analytically useful than a
fabricated value.

Do not fill a field just because the original did. If the original named "attacker" as
source and the span does not support naming an actor, leave source blank and explain why
in `uncertainty_note`.

---

## 4. What DerivedFrom means

Your critique is linked to the original by `derived_from`. This link records that your
reading is a second cut of the same span. It does not record that the original was wrong
or that your reading is better.

Both readings are analytical objects. The differentiation between them — what each chose
to name, what each left blank, what each noted as uncertain — is visible in the chain.
The chain is the record of the analytical process.

`extraction_stage: "reviewed"` records your position in the pipeline. It does not mean
the record is final, authoritative, or quality-assured. It names where it was produced.

---

## 5. Worked example: E3 original vs E3 critique

**Source span (unchanged in both)**:

```
An unauthenticated attacker can craft requests that bypass all route-level authentication checks.
```

**E3 original** (from `cve_response_drafts.json`):

```json
{
  "what_changed": "attacker exploits authentication bypass to access protected routes",
  "source": ["attacker"],
  "target": ["storefront-api-routes"],
  "mediation": "",
  "observer": "security-lead",
  "uncertainty_note": ""
}
```

The original names "attacker" as source. The span contains the word "attacker" — but
"an unauthenticated attacker can" describes a capability class, not a specific actor
performing a specific action. The span does not name a person, identity, or entity.

**E3 critique** (from `cve_critique_drafts.json`):

```json
{
  "what_changed": "a condition was described in the advisory in which requests bypass route-level authentication checks",
  "source": [],
  "target": ["storefront-api-routes"],
  "mediation": "fastmiddleware-token-validation",
  "observer": "security-lead",
  "uncertainty_note": "The span names a vulnerability class, not an attributable actor. 'Attacker' is an inference from the CVE framing, not from the span itself. The span describes a condition (requests that bypass checks) without naming a source agent. Source left blank: the span does not support confident actor attribution."
}
```

The critique leaves source blank, names the mediation (the mechanism through which bypass
occurs), and explains why "attacker" is an inference rather than a reading of the span.

The original is not wrong. The critique is not better. Both are now part of the chain.

---

## Field assignment guide

| Field | What to do |
|-------|-----------|
| `source_span` | Copy verbatim from original. Never edit. |
| `source_doc_ref` | Copy from original if present. It is provenance, not interpretation. |
| `derived_from` | Set to the original draft's `id`. |
| `extraction_stage` | Set to `"reviewed"`. |
| `extracted_by` | Set to your identifier (e.g. `"human-reviewer"`, `"llm-critique-pass1"`). |
| `id` | Leave blank (assigned by `meshant draft`). |
| `timestamp` | Leave blank (assigned by `meshant draft`). |
| `what_changed` | Write a description of what the span records — not what the original said it records. |
| `source` | Name only what the span directly supports. Leave blank if the span describes a condition, not an act. |
| `target` | Name what the span indicates is affected. May match original if supported by span. |
| `mediation` | Name the mechanism if the span identifies one. |
| `observer` | Name the position from which this reading is made. |
| `tags` | Use only tags supported by the span. |
| `uncertainty_note` | Explain every blank content field. Prefer a note to a fabricated value. |
