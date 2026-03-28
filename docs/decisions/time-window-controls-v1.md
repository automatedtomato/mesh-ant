# Decision Record: Web UI Time Window Controls (v1)

**Issue:** #180 — Web UI time series controls
**Date:** 2026-03-28
**Status:** Accepted
**Files:** `meshant/serve/web/app.js`, `index.html`, `style.css`
**Depends on:** `web-ui-v1.md`, `time-window-v1.md`

---

## Context

The backend already supports `?from=RFC3339&to=RFC3339` query parameters on
`/articulate`, `/shadow`, `/element/{name}`, and `/traces`. The web UI always
sent unbounded requests, so the time window axis was inaccessible from the browser.

This record documents the UI design decisions and named tensions.

---

## Decisions

### D1: datetime-local inputs, not a range slider

Two `<input type="datetime-local">` fields (From / To) were chosen over a
range slider or date-picker library. Rationale:

- No dependency on an external library (consistent with `web-ui-v1.md D3`: no
  build step, no npm)
- Works without a timeline view or trace density preview, which would require
  a separate API endpoint or client-side pre-fetch
- An analyst who knows the approximate temporal range of their dataset can
  type or select the bounds directly
- A slider would imply continuous density; the trace substrate is sparse and
  discontinuous (see T4 below)

### D2: Observer-gate pattern extended to the time picker

`#time-window-picker` is hidden until a successful `loadGraph()` call.
It appears and disappears in lockstep with `#cut-header` and `#main`.

No time window can be set from a positionless vantage. This enforces the same
structural constraint as the observer gate: a cut requires a position before
it can be further constrained.

### D3: Apply is explicit; time inputs do not auto-reload

The From/To inputs do not trigger a reload on `change` or `blur`. An explicit
**Apply** button is required to commit the window. Rationale:

- Each `loadGraph()` call is a server round-trip; auto-firing on each keystroke
  would be noisy and expensive
- The analyst should be able to adjust both bounds before committing the cut

### D4: "Reset to unbounded" naming

The reset button is labelled "Reset to unbounded", not "Clear". An unbounded
time window is not the absence of a temporal constraint — it is the constraint
"include all traces regardless of timestamp" (`time-window-v1.md Decision 2`).
"Clear" would imply the constraint is removed; "Reset to unbounded" names the
state the analyst is moving to.

### D5: Observer change resets the time window

When the observer form is submitted with a new observer, `currentFrom` and
`currentTo` are reset to `''` (unbounded). The time inputs are also cleared.
The new observer's view is loaded with no time constraint.

Rationale: a time window chosen while reading from position A may be
meaningless, misleading, or silently narrow for position B. Starting unbounded
on each observer change is the safer default — the analyst can re-apply the
window explicitly if the same bounds are relevant.

### D6: `toRFC3339` assumes UTC

`datetime-local` inputs provide no timezone information. The conversion function
appends `:00Z` (or `Z` if seconds are present) to produce a UTC RFC3339 string.
This is the simplest correct implementation: all trace timestamps in MeshAnt
are stored as RFC3339 UTC, so UTC is the appropriate reference frame for
temporal queries.

---

## Tensions

| ID | Description | Status |
|----|-------------|--------|
| T1 | UTC assumption in `toRFC3339` — traces from other timezones may be unexpectedly included/excluded | Named, deferred |
| T2 | Observer change resets window — silently discards a user constraint that might have been intentional | Named, defensible default |
| T3 | "Reset to unbounded" names a state, but users may expect "Clear" — the label is ANT-correct but unconventional | Named, accepted |
| T4 | datetime-local presents a continuous time axis; trace reality is discontinuous — analyst may set a window over a temporal void and receive an empty graph without knowing why | Named, deferred |

---

## Deferred items

- **D-1:** Temporal density indicator (T4) — surface trace min/max timestamps
  or a density preview near the time picker so analysts can calibrate their
  window against actual data distribution.

- **D-2:** Timezone selection (T1) — if the trace store accumulates data from
  multiple timezones, a timezone selector adjacent to the From/To inputs would
  make the UTC assumption explicit and user-adjustable.

- **D-3:** Tag filter controls — the backend already supports `?tags=` on all
  endpoints. The time-window picker bar establishes the visual pattern (dark bar
  between cut header and main) that a tag filter bar could follow. Candidate for
  a follow-up issue after `#181`+.

---

## Relation to prior records

- **`web-ui-v1.md`**: this record extends the UI with a new cut axis. Observer
  gate pattern (`web-ui-v1.md D4`) is preserved and extended (D2 above).
- **`time-window-v1.md`**: the backend `TimeWindow` type (decisions 1–7 there)
  is what the UI now exposes. Decision 2 there ("zero TimeWindow = full temporal
  cut") is what "Reset to unbounded" communicates in the UI.
