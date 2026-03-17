# MeshAnt Glossary

This glossary explains vocabulary that MeshAnt inherits from Actor-Network Theory (ANT) or
coins for its own purposes. These terms are not accidental branding. They carry conceptual
differences that would be flattened if rewritten in standard software-engineering language.

The purpose of this glossary is not to make everything familiar. It is to explain why these
terms are being used, what they are trying to preserve, and how they differ from nearby but
misleading conventional terms.

---

## Trace

A record of something that made a difference in a network.

**Not simply:** event, log entry, span

**In MeshAnt:**
A `Trace` is the fundamental unit of record. It captures a moment where something changed,
redirected, mediated, or transformed a relation. It has required fields (`ID`, `Timestamp`,
`WhatChanged`, `Observer`) and optional fields (`Source`, `Target`, `Mediation`, `Tags`).

A trace does not presuppose who or what acted. The `Source` and `Target` fields are `[]string`
— open slices that can name humans, rules, sensors, queues, forms, or anything else that
participated. A trace does not assign blame or credit to a single agent. It records that
something made a difference, as seen from somewhere.

**Why this word matters:**
An "event" suggests something that simply happened. A "log entry" suggests a record made for
debugging. A trace, in Latour's sense, is what something leaves behind when it acts — the mark
that lets you follow it. MeshAnt begins from traces because it refuses to begin from actors.
You cannot follow what you have already decided in advance.

See: [`meshant/schema/trace.go`](../meshant/schema/trace.go) — `Trace` struct and `Validate()`

Related: [Mediation](#mediation), [Observer position](#observer-position)

---

## Articulation

A provisional operation that renders something distinct, linked, and consequential within a
network.

**Not simply:** grouping, modeling, graph construction, aggregation

**In MeshAnt:**
`Articulate()` takes a set of traces and produces a `MeshGraph` — a graph of nodes and edges
— from a particular observer position, time window, and tag filter. The result is not a
neutral model of "what is really there." It is a situated rendering: some elements become
visible and connected; others are placed in [shadow](#shadow).

An articulation is always partial. It is made from somewhere, by someone, at some time.
Different articulation options produce different graphs from the same traces — not because one
is right and the others wrong, but because each performs a different [cut](#cut).

**Why this word matters:**
"Modeling" implies there is a pre-existing reality that the model approximates. "Aggregation"
implies summing up. Articulation, in Latour's sense, is how something becomes sayable,
visible, and consequential. It is a performative operation — the act of articulating changes
what can be seen and what cannot. MeshAnt uses this word to preserve the idea that every
analysis is also an intervention.

See: [`meshant/graph/graph.go`](../meshant/graph/graph.go) — `Articulate()`, `ArticulationOptions`

Related: [Cut](#cut), [Shadow](#shadow), [Re-articulation](#re-articulation)

---

## Cut

The boundary that every articulation draws between what is made visible and what is left in
shadow.

**Not simply:** filter, query, selection, scope

**In MeshAnt:**
Every call to `Articulate()` performs a cut. The cut is defined by which observer positions are
included, which time window is applied, and which tags are required. The `Cut` struct records
these parameters alongside statistics about what was included and what was excluded — so the
cut is self-describing. You can always ask: how was this graph made? What did it leave out?

A cut is not a neutral filter applied to pre-existing data. It is an intervention that shapes
what becomes visible. Two cuts on the same trace dataset can produce graphs with almost no
overlap — not because the data is inconsistent, but because different positions compose
different worlds.

**Why this word matters:**
A "filter" suggests that you are removing noise to reveal signal. A cut acknowledges that what
you exclude is not noise — it is the part of the network that this position cannot see. The
word "cut" preserves the sense that analysis is a separation, and that every separation has
consequences. There is no cut that leaves everything visible.

See: [`meshant/graph/graph.go`](../meshant/graph/graph.go) — `Cut` struct

Related: [Shadow](#shadow), [Articulation](#articulation), [Observer position](#observer-position)

---

## Shadow

What a cut excludes — the elements that exist in the trace dataset but are not visible from
the current articulation's position.

**Not simply:** filtered-out data, excluded results, null set, hidden nodes

**In MeshAnt:**
When an articulation is performed, every element mentioned only in excluded traces enters the
shadow. The `ShadowElement` struct records the element's name, which observer positions could
see it, and *why* it was excluded (`ShadowReason`: `"observer"`, `"time-window"`, or
`"tag-filter"`).

The shadow is not discarded data. It is a named, structured part of the articulation's output.
A `MeshGraph` without its shadow would be incomplete — it would claim to show the whole network
when it can only show a position within it.

Shadow shifts ([emerged](#shadow-shift), [submerged](#shadow-shift), reason-changed) are
tracked when two articulations are [diffed](#diff). An element that was in shadow and is now
visible has *emerged*; one that was visible and is now hidden has *submerged*.

**Why this word matters:**
"Filtered-out data" implies that excluded items are irrelevant. The shadow is the opposite
claim: these elements are consequential, but invisible from here. The word forces the analyst
to acknowledge that their view is partial. Every articulation casts a shadow, and the shadow
is part of the result.

See: [`meshant/graph/graph.go`](../meshant/graph/graph.go) — `ShadowElement`, `ShadowReason`

Related: [Cut](#cut), [Shadow shift](#shadow-shift), [Diff](#diff)

---

## Shadow shift

The movement of an element across or within the shadow boundary between two articulations.

**Not simply:** diff delta, state change

**In MeshAnt:**
When two `MeshGraph` articulations are compared via `Diff()`, elements can move across the
shadow boundary:
- **Emerged**: was in shadow (invisible), now a visible node
- **Submerged**: was a visible node, now in shadow (invisible)
- **Reason-changed**: remained in shadow but the *reason* for exclusion changed (e.g., from
  `"observer"` to `"time-window"`)

`ShadowShift` records the element name, the kind of movement, and the reasons on both sides.

**Why this word matters:**
Calling these "additions" and "deletions" would imply that elements come into existence or
cease to exist. They don't — they become visible or invisible from a particular position. The
directional language (emerged/submerged) preserves the idea that the element was always there;
only the cut changed.

See: [`meshant/graph/diff.go`](../meshant/graph/diff.go) — `ShadowShift`, `ShadowShiftKind`

Related: [Shadow](#shadow), [Diff](#diff)

---

## Mediation

What transformed, redirected, amplified, blocked, or deferred action — not by faithfully
relaying it, but by changing it.

**Not simply:** middleware, handler, intermediary, proxy, relay

**In MeshAnt:**
`Trace.Mediation` is an optional string field naming what mediated the action. Its absence is
meaningful: "no mediator was observed — not that mediation is impossible." The field is
carried through to `Edge.Mediation` on the articulated graph.

The loader surfaces mediation statistics: `MeshSummary.MediatedTraceCount` (how many traces
involved mediation) and `MeshSummary.Mediations` (which mediators appeared). These help the
analyst see what is doing transformative work in the network.

Mediation is a broader category than [translation](#translation). A delay mediates (it
changes timing). A threshold mediates (it gates passage). A form mediates (it reformats what
can be said). Not all of these are translations — but all of them transform action in
passage.

**Why this word matters:**
The critical distinction is between a **mediator** and an **intermediary**. An intermediary
transports without transforming — the output is predictable from the input. It is a faithful
conduit, a bridge. A mediator transforms what passes through it — the output is *not*
predictable from the input. The action that arrives is not the action that departed.

The prefix "inter-" in intermediary implies being sandwiched between fixed endpoints, a
passive role. Mediation is the opposite claim: what stands between source and target is not
passive. It adds, subtracts, delays, redirects, or deforms. MeshAnt names this field
`Mediation` — not `Intermediary` — because it insists that what carries action also changes
it.

See: [`meshant/schema/trace.go`](../meshant/schema/trace.go) — `Trace.Mediation` field;
[`meshant/loader/loader.go`](../meshant/loader/loader.go) — `MeshSummary.Mediations`

Related: [Intermediary](#intermediary), [Translation](#translation), [Trace](#trace)

---

## Intermediary

What transports action without transforming it — a faithful conduit.

**Not simply:** mediator, middleware, proxy

**In MeshAnt:**
MeshAnt does not currently have a dedicated field or type for intermediaries. The
`Trace.Mediation` field names mediators — things that transform action in passage. If
something merely relays without transforming, the `Mediation` field would typically be empty
(no transformative mediator was observed).

This is a deliberate analytical choice. MeshAnt's interest is in what *changes* action, not
in what faithfully transmits it. A pure intermediary — if one exists — is analytically
invisible in the current schema, because it leaves no trace of transformation.

**Why this word matters:**
The mediator/intermediary distinction is one of Latour's sharpest: a mediator transforms; an
intermediary transports. The two are easily confused because both "stand between" source and
target. But the difference is consequential. If you treat a mediator as an intermediary, you
miss the transformation. If you treat an intermediary as a mediator, you see transformation
where there is none.

The prefix "inter-" suggests a fixed, sandwiched position — being placed *between* two
things that are already defined. A mediator, by contrast, may redefine what the source and
target are. It does not sit passively between pre-given endpoints; it participates in
composing them.

Related: [Mediation](#mediation)

---

## Translation

The conversion of something into another operational form — often changing what counts, who
acts, or how alignment is maintained.

**Not simply:** transformation, conversion, mapping, serialization

**In MeshAnt:**
Translation is not yet a structural type in MeshAnt (as of March 2026). It exists as a tag
value (`TagTranslation = "translation"`) and is discussed extensively in the project's
theoretical documents. This gap is acknowledged — see
[`docs/tmp/dialogue_2026-03-13T000000.md`](tmp/dialogue_2026-03-13T000000.md) for the ongoing
design discussion.

The conceptual distinction from [mediation](#mediation) is important:

- **Mediation** asks: *what transformed, redirected, or displaced the action?*
  (focus on the mediator)
- **Translation** asks: *what did the thing become as it moved?*
  (focus on the displacement)

In Callon/Latour, translation is the process by which networks are actually composed:
interests are displaced, meanings are reformatted, roles are reassigned, one regime is
converted into another. A chain of translations can turn raw sensor data into a legal
obligation — each step changes what the thing *is* in operational terms.

Translation can fail. Interessement doesn't hold, enrollment is contested, the chain breaks.
The failure modes of translation are as analytically significant as the successes.

**Why this word matters:**
"Transformation" implies a neutral change of form. "Conversion" implies a mechanical
operation. Translation, in the ANT sense, carries the overtone of *traduttore, traditore*
(translator, traitor) — something is always gained and something is always lost. The thing
that arrives is not the thing that departed. MeshAnt keeps this word because it names the
productive infidelity at the heart of how networks hold together.

Related: [Mediation](#mediation), [Trace](#trace)

---

## Mesh

The uneven field of traces, mediations, and relations through which action is composed.

**Not simply:** graph, network, system, topology

**In MeshAnt:**
The "mesh" in MeshAnt is not a clean, fully-connected graph. It is an irregular,
heterogeneous terrain of linked traces. Some regions are dense with observations; others are
sparse. Some connections are well-attested; others are seen from only one position.

A `MeshGraph` is an articulation *of* the mesh — a provisional rendering from a specific
position. The mesh itself is never fully visible. It is what every articulation cuts into, and
what every shadow points back toward.

**Why this word matters:**
"Graph" implies a mathematical object with well-defined nodes and edges. "Network" (in common
usage) implies infrastructure or connectivity. "Mesh" preserves the sense of unevenness — an
interwoven, irregular, partially-opaque fabric. It resists the implication that the structure
is clean, complete, or knowable from a single position.

Related: [Articulation](#articulation), [Cut](#cut)

---

## Observer position

The situated standpoint from which a trace is recorded or an articulation is made.

**Not simply:** user, role, perspective, tenant, scope

**In MeshAnt:**
Every `Trace` has a required `Observer` field — a string naming who or what captured this
trace, and from what position. Different observers do not merely see the same world from
different angles; they can compose different worlds by rendering different traces visible.

`ArticulationOptions.ObserverPositions` filters traces to those recorded by specific
observers. The resulting graph shows what those positions could see. The `Cut` struct records
which observers were included and which were excluded. Shadow elements name which excluded
observers could see them.

**Why this word matters:**
A "user" is an identity. A "role" is a permission boundary. An observer position, in MeshAnt,
is an epistemic location — it determines what can be witnessed, not just what is permitted.
Two observers at the same event may record different traces because they are situated
differently. The word "position" insists that observation is always from somewhere, and that
the somewhere matters.

See: [`meshant/schema/trace.go`](../meshant/schema/trace.go) — `Trace.Observer`;
[`meshant/graph/graph.go`](../meshant/graph/graph.go) — `ArticulationOptions.ObserverPositions`

Related: [Cut](#cut), [Shadow](#shadow)

---

## Actant

Anything — human or non-human — that makes a difference in the network.

**Not simply:** agent, actor, entity, service, component

**In MeshAnt:**
MeshAnt does not reserve agency for humans or human-like software agents. A sensor, a
threshold, a legal document, a queue, a price display, or an evacuation shelter can all be
actants — if they redirect, amplify, block, or transform action.

Actants appear as strings in `Trace.Source` and `Trace.Target`. The schema does not
distinguish human from non-human participants. This is the principle of
[generalised symmetry](#generalised-symmetry): the analytical vocabulary treats all
difference-makers with the same descriptive apparatus.

**Why this word matters:**
"Agent" in software engineering implies autonomy, goals, and usually human-like reasoning.
"Entity" implies a stable thing in a database. "Actant" (from Latour, borrowed from
semiotics) names something that acts without implying anything about what kind of thing it
is. A speed bump is an actant. A form is an actant. A regulatory deadline is an actant.
MeshAnt uses this word to hold the space open for non-human difference-makers.

Related: [Generalised symmetry](#generalised-symmetry), [Trace](#trace)

---

## Generalised symmetry

The methodological commitment to describe human and non-human participants with the same
analytical vocabulary.

**Not simply:** polymorphism, duck typing, interface abstraction

**In MeshAnt:**
`Source` and `Target` are `[]string` — they can hold the name of a person, a sensor, a legal
document, or a graph produced by MeshAnt itself. There is no `HumanActor` vs. `SystemActor`
distinction. Graph-reference strings (`meshgraph:<uuid>`, `meshdiff:<uuid>`) appear in the
same `Source`/`Target` fields as any other actant — no privileged field, no separate type.

**Why this word matters:**
Polymorphism abstracts over implementation differences while preserving type hierarchies.
Generalised symmetry, in Callon and Latour's sense, is a stronger commitment: it refuses
to explain humans and non-humans with different vocabularies *at the descriptive level*. It
is not a claim that humans and sensors are the same. It is a methodological rule: do not build
the human/non-human distinction into your analytical apparatus before you have followed the
traces.

See: [`docs/decisions/graph-as-actor-v2.md`](decisions/graph-as-actor-v2.md) — Decision 2

Related: [Actant](#actant), [Graph-as-actor](#graph-as-actor)

---

## Re-articulation

The possibility that an earlier articulation — an earlier cut — was useful, partial, and
contestable, but never final.

**Not simply:** re-query, re-run, re-analysis, versioning

**In MeshAnt:**
Re-articulation is not yet a dedicated operation (as of March 2026), but it is a named design
principle (Principle 6: "re-articulation before essence"). Every articulation is provisional.
Actors may split, merge, dissolve, or be redescribed as the mesh changes or as new traces
are followed.

`Diff()` is a partial form of re-articulation: it compares two articulations and surfaces what
changed — what emerged from shadow, what submerged, what shifted. But full re-articulation
would also allow revising the *boundaries* of what counts as an element.

**Why this word matters:**
"Re-running a query" implies the same operation on updated data. Re-articulation implies that
the operation itself may need to change — that the categories, boundaries, and observer
positions of the first articulation were contingent and revisable. It preserves the
philosophical commitment that no cut is final.

Related: [Articulation](#articulation), [Diff](#diff), [Cut](#cut)

---

## Friction

A first-class condition of action — not noise, not error, not inefficiency.

**Not simply:** latency, error, bug, bottleneck, technical debt

**In MeshAnt:**
Friction appears in trace datasets as delays, blockages, thresholds, incompatible formats,
unequal access, and missing signals. The tag vocabulary includes `"delay"`, `"threshold"`,
`"blockage"` — all forms of friction.

MeshAnt treats friction as constitutive, not accidental. A delay is not a failure of the
system to be fast; it is part of how the system works. A threshold is not an obstacle in the
way of action; it is a mediator that shapes what passes and what does not.

**Why this word matters:**
In most engineering contexts, friction is something to be eliminated — latency reduced,
errors handled, bottlenecks removed. MeshAnt preserves the word because, in ANT, friction
is what makes the network real. Without friction, there is no difference between a connection
that works and one that doesn't. Friction is where the interesting things happen — it is often
where mediation and translation are most visible.

See: [`docs/principles.md`](principles.md) — Principle 7

Related: [Mediation](#mediation), [Trace](#trace)

---

## Diff

A comparison of two articulations that names what became visible, invisible, or shifted
between two situated cuts.

**Not simply:** changelog, delta, diff (in the `git diff` sense)

**In MeshAnt:**
`Diff()` compares two `MeshGraph` articulations and produces a `GraphDiff`: nodes added,
nodes removed, nodes persisted (with changed appearance counts), edges added, edges removed,
and [shadow shifts](#shadow-shift). It carries the `Cut` from both input graphs so the
comparison is self-situated.

A `GraphDiff` is not a neutral changelog. It does not say "these things changed." It says
"from this pair of positions, these things became visible or invisible." The difference
between these framings is the difference between a god's-eye view and a situated observation.

**Why this word matters:**
A `git diff` assumes a shared, linear timeline and a single authoritative state. MeshAnt's
diff compares two *positions*, not two *versions*. Two graphs diffed may be from the same
moment but different observers, or the same observer at different times, or different
observers at different times. The word is familiar but the operation is not: it compares
cuts, not commits.

See: [`meshant/graph/diff.go`](../meshant/graph/diff.go) — `Diff()`, `GraphDiff`

Related: [Shadow shift](#shadow-shift), [Cut](#cut), [Articulation](#articulation)

---

## Graph-as-actor

The principle that a graph produced by MeshAnt can itself enter the mesh as an actant.

**Not simply:** self-reference, metadata, provenance tracking

**In MeshAnt:**
`IdentifyGraph()` assigns a UUID to a `MeshGraph`, making it referable. `GraphRef()` produces
a reference string (`meshgraph:<uuid>`) that can appear in the `Source` or `Target` of
subsequent traces. This means the observation apparatus enters the mesh it observes.

This is explicit opt-in — `Articulate()` does not automatically identify its output. The
analyst calls `IdentifyGraph()` when they decide the graph should become an actant. Recording
is a curatorial act, not an automatic side effect.

**Why this word matters:**
"Provenance tracking" records where data came from. Graph-as-actor goes further: the graph
is not just metadata about the analysis — it is a participant in the network that subsequent
traces can reference, respond to, and be shaped by. This operationalises Principle 8 (the
designer is inside the mesh) at the data level: the tools of observation become observable.

See: [`meshant/graph/actor.go`](../meshant/graph/actor.go) — `IdentifyGraph()`, `GraphRef()`;
[`meshant/schema/graphref.go`](../meshant/schema/graphref.go) — `IsGraphRef()`

Related: [Generalised symmetry](#generalised-symmetry), [Actant](#actant)

---

## Reflexive tracing

Recording the act of observation as a trace within the mesh being observed.

**Not simply:** audit logging, telemetry, self-monitoring

**In MeshAnt:**
`ArticulationTrace()` and `DiffTrace()` produce `Trace` values that record the act of
articulating or diffing. The resulting trace names the observer (`"meshant-graph"`), the
mediation (`"graph.Articulate"` or `"graph.Diff"`), and references the produced graph or diff
via graph-ref strings in `Target`.

These traces are tagged with `"articulation"` and can be loaded back into the dataset — so the
next articulation can include the act of the previous one.

**Why this word matters:**
Audit logging records what the system did for compliance or debugging. Reflexive tracing makes
the analytical apparatus visible as a participant in the network it analyzes. The point is not
accountability (though it enables that) — the point is that observation is never outside the
mesh. Recording it honestly is better than pretending it doesn't happen.

See: [`meshant/graph/reflexive.go`](../meshant/graph/reflexive.go) — `ArticulationTrace()`,
`DiffTrace()`

Related: [Graph-as-actor](#graph-as-actor), [Observer position](#observer-position)

---

## Assemblage

A heterogeneous collection of elements acting together — not a uniform group.

**Not simply:** array, list, group, set, collection

**In MeshAnt:**
`Source` and `Target` are `[]string` because the producer or recipient of a difference is
often not a single entity but a heterogeneous assemblage: a sensor *and* a threshold *and* a
network connection acting together. Forcing a single name would perform what the schema calls
"a premature singularization of attribution."

**Why this word matters:**
A "list" or "group" implies items of the same kind collected for convenience. An assemblage
(in Deleuze/Latour usage) is a gathering of unlike things that act together without being
reducible to a single agent. MeshAnt uses `[]string` for source and target to hold this
heterogeneity open — a trace can name a human, a rule, and a sensor in the same slice without
implying they are the same kind of thing.

See: [`meshant/schema/trace.go`](../meshant/schema/trace.go) — `Trace.Source`, `Trace.Target`

Related: [Actant](#actant), [Generalised symmetry](#generalised-symmetry)
