# MeshAnt — Potential Output Forms

This document records the plausible shapes MeshAnt could take as a finished project.
The form factor is deliberately left open in the project's design principles — this is
not a roadmap commitment, but a set of candidate attractors to inform direction as the
work matures.

---

## 1. A Go library / SDK for trace analysis

The most conservative and immediately available form. MeshAnt's packages —
`meshant/schema`, `meshant/loader`, `meshant/graph` — are published as a Go module.
Other tools import them to articulate observer-situated cuts from their own trace data,
compare articulations across time or position, and name what each cut excludes.

This form requires no additional layer. The API already exists. The value is that
downstream tools inherit MeshAnt's methodological commitments without having to
implement them: every articulation names its shadow; every diff is situated; the
designer is inside the mesh.

**Natural consumers:** research tools, audit pipelines, custom analysis scripts.

---

## 2. A CLI for trace dataset inspection

A command-line tool that takes JSON trace files as input and produces observer-situated
output: articulations, diffs, summaries, shadow analyses.

```
meshant articulate --observer satellite-operator --from 2026-03-11 --to 2026-03-11 traces.json
meshant diff --observer satellite-operator --day1 2026-03-11 --day2 2026-03-18 traces.json
meshant summarise traces.json
```

The `PrintArticulation`, `PrintDiff`, and `PrintSummary` functions are already the core
of this form. Adding a thin CLI layer (flag parsing, stdin/stdout routing) is the only
remaining work.

**Natural consumers:** analysts and researchers working with existing trace datasets;
anyone who needs a quick situated read of a JSON trace file without writing Go.

---

## 3. An agent observability layer

AI agent systems produce traces naturally: tool calls, handoffs, decisions, delegations,
failures, corrections. MeshAnt's schema fits this domain directly — `source`, `target`,
`mediation`, `observer`, `tags` map cleanly onto agent interactions.

In this form, MeshAnt is a library that sits alongside an agent framework (Claude Agent
SDK, LangGraph, or similar) and articulates situated graphs of what actually happened in
a run — not an idealized flow diagram, but a trace-first record of who acted on whom,
what was delegated, what was translated, what hit a threshold. The shadow names the parts
of the run that no single observer could see.

Key capability: multiple observer positions in the same run (the orchestrator sees
something the subagent does not; the user sees something the orchestrator does not). The
graph makes those asymmetries explicit rather than flattening them into a single timeline.

**Natural consumers:** agent framework developers; teams running multi-agent pipelines
who need to understand what actually happened, not just what was logged.

---

## 4. A multi-agent mesh monitor

A longer step beyond form 3. Once graph-as-actor lands (M5), a produced graph can be
re-injected as a source or target in new traces. That opens a feedback loop: agents
observe, traces are recorded, articulations are produced, articulations become inputs to
new observations — including observations about the observation apparatus itself.

In this form, MeshAnt is a running monitor that tracks the evolving state of a
multi-agent system from multiple observer positions simultaneously. Diffs between
articulations across time become signals: an element that submerges into the shadow
between two cuts signals a loss of visibility; an element that emerges signals a new
actor entering the mesh.

The monitor is not a dashboard in the conventional sense. It does not present a single,
authoritative view of the system. It presents a set of situated cuts with their shadows
named — a methodologically honest account of a system that cannot be fully seen from any
one position.

**Natural consumers:** teams running persistent multi-agent systems where understanding
asymmetries, blind spots, and actor drift over time is operationally important.

---

## 5. A research and teaching tool for ANT practitioners

Actor-Network Theory is mostly applied through text, ethnography, and hand-drawn diagrams.
MeshAnt could be the first computational implementation of ANT's core analytical moves:
following traces rather than declaring actors in advance; articulating observer-situated
cuts rather than producing god's-eye network graphs; naming shadows as mandatory output
rather than treating invisibility as absence.

In this form, MeshAnt is a tool for sociologists, STS (Science and Technology Studies)
researchers, designers, and others working in the ANT tradition. They load trace datasets
— field notes encoded as JSON, interview fragments, document trails — and explore them
through articulations and diffs rather than static node-link diagrams.

The diff layer (M4) makes temporal analysis tractable: a researcher can compare what was
visible to one actor at the beginning of a controversy with what was visible at the end,
and name what submerged or emerged across that interval.

**Natural consumers:** academic researchers; design practitioners using ANT-inflected
methods; educators teaching network analysis, STS, or relational sociology.

---

## Which forms are most likely

Forms 3 and 4 (agent observability and mesh monitor) are the natural attractor given the
project's current foundations. The trace schema, observer-situated articulation, and shadow
semantics address a real gap in AI agent tooling: current observability tools (logs,
traces in the OpenTelemetry sense) record what happened, but do not articulate from whose
position or name what each position cannot see. MeshAnt's vocabulary — mediation,
translation, threshold, delegation — maps onto the domain more precisely than conventional
observability primitives.

Forms 1 and 2 (library and CLI) are stepping stones already half-built. They are the
intermediate output that makes forms 3 and 4 possible.

Form 5 (research tool) is a genuine use case with a small but distinct audience. It is
the most direct expression of the project's ANT inspiration, and the one most likely to
produce unexpected applications.

---

*This document is not a roadmap. It records candidate attractors to inform direction as
the work matures. The form factor remains deliberately open until the traces say otherwise.*
