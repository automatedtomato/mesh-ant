# Decision Record: Web UI + Provenance Panel (v1)

**Issue:** #146 — Phase 4, Layer 3: Interactive graph output
**Date:** 2026-03-24
**Status:** Accepted

---

## Context

Phase 4 adds an interactive browser-based interface to `meshant serve`. The
server already exposes four analytical endpoints (`/articulate`, `/diff`,
`/shadow`, `/traces`); this record covers adding `/element/{name}` and the
embedded static web assets.

The goal is a single-page application that enforces the core ANT constraint at
the UI layer: no graph is displayed without naming its observer position. The
observer gate is structural HTML, not a JavaScript guard.

---

## Decisions

### D1: Cytoscape.js over D3.js

Cytoscape.js is purpose-built for graph/network visualisation with a high-level
API. D3.js is a general-purpose data visualisation library that requires
significant custom code to produce the same force-directed graph layout.
Cytoscape.js provides `cose` layout, tap events, and zoom/pan out of the box.
The minified library (~374KB) is vendored in `web/lib/` and served via
`go:embed`; no CDN dependency at runtime.

### D2: `go:embed` for static assets

The web assets (`index.html`, `style.css`, `app.js`, `render.js`, `export.js`,
`lib/cytoscape.min.js`) are embedded into the binary via `//go:embed web`.
`fs.Sub` strips the `web/` prefix so that `web/index.html` serves at `/`.
The result is a self-contained binary: `meshant serve` requires no accompanying
file tree.

### D3: No build step — vanilla JS + vendored Cytoscape

No npm, webpack, TypeScript, or bundler. Vanilla ES5/ES6 is sufficient for the
current SPA scope and keeps the project buildable with only a Go toolchain. The
JS files (`app.js`, `render.js`, `export.js`) are loaded as separate `<script>`
tags in `index.html`.

If the UI grows to require a module bundler (e.g. for TypeScript types or
tree-shaking), that decision should be made then with a separate record.

### D4: Observer gate is structural HTML

`<header id="observer-gate">` is an HTML form that is visible on page load.
`#cut-header` and `#main` carry the HTML `hidden` attribute and are only revealed
by `app.js` after a successful articulate response. The enforcement is structural:
the browser does not render the graph area before the observer is named.

This is stronger than a JavaScript validation check, which can be bypassed or
skipped if JS fails to load.

### D5: `/element/{name}` — in-memory filtering

`handleElement` follows the same full-substrate design as the other endpoints
(D3 in `serve-v1.md`): all traces are loaded from the store and filtered
in-memory. No pre-filtering at the store layer; `filterByElement` runs over the
observer-filtered result set. This preserves the ANT guarantee that the
store holds the full substrate and the analytical engine makes all cuts.

Element matching uses exact string equality on individual `Source` and `Target`
slice entries (not substring or case-insensitive matching). This is consistent
with how `graph.Articulate` counts node appearances.

### D6: Provenance from trace fields only

The detail panel shows a "Promoted from LLM session" notice when a trace carries
the `session` tag (`schema.TagValueSession`). This is derived from
`trace.tags` — no additional HTTP fetch to retrieve the originating
`SessionRecord`. Fetching session metadata would require a `/session/{id}`
endpoint that does not yet exist. The full `SessionRecord` provenance panel is
deferred to Phase 5.

### D7: Client-side DOT export

`export.js:buildDOT` replicates the DOT format from `graph/export.go:PrintGraphDOT`
client-side. The alternative — a server-side `/export?format=dot` endpoint — was
deferred to keep the API surface minimal for this phase. If formats diverge, the
Go `PrintGraphDOT` output is authoritative.

---

## ANT Tensions

**T1: Web UI is a mediator not recorded as a Trace**

The browser interface shapes what the user sees (node click → detail fetch,
shadow panel → shadow items) and is therefore a mediator, not a neutral
intermediary. This mediation is not recorded as a Trace. The UI apparatus enters
the mesh without being observed by it. Noted; closing this gap (reflexive UI
tracing) is deferred.

**T2: Client-side DOT duplicates Go logic**

`export.js:buildDOT` replicates `graph/export.go:PrintGraphDOT`. If either
implementation is updated without updating the other, the DOT outputs will
diverge. Accepted: the duplication is small, well-commented, and the canonical
format is the Go output. Decision D7 above.

**T3: Provenance shows trace fields, not full SessionRecord**

The `session` tag signals a promoted SessionRecord but the UI cannot show the
full `SessionRecord` (model ID, session timestamp, raw LLM output). Only the
`what_changed` field and tag presence are visible. Deferred to Phase 5
(`/session/{id}` endpoint + session provenance panel).

**T4: Observer as free-text input — no `/observers` picker**

The observer gate accepts any string. There is no endpoint listing known observer
positions (names of analysts who have recorded traces). A positioned reading
requires knowing the positions; an empty free-text input admits typos and
unknown positions silently (they return an empty graph, not an error). This is
ANT-defensible: every reading is genuinely free to choose its position, including
new ones not yet present in the substrate. A `/observers` enumeration endpoint
is a Phase 5 candidate.

**T5: `shadow_count` semantics differ between endpoint types**

The `/articulate` and `/shadow` endpoints compute `shadow_count` as the number
of shadow *elements* (from `len(g.Cut.ShadowElements)`). The `/traces` and
`/element/{name}` endpoints compute `shadow_count` as the number of excluded
*traces* (`len(allTraces) - len(observerFiltered)`). The `CutMeta.shadow_count`
field therefore means different things depending on which endpoint produced the
envelope. The UI displays the value uniformly as "Shadow: N". Users comparing
the shadow count from `/articulate` with that from `/traces` will see different
numbers for the same observer and may not know why. This continues the tension
documented in `serve-v1.md` T2 (traces, not elements, for `/traces`). The
discrepancy is named here and deferred; resolving it would require either a
per-endpoint label change in the UI or separate `shadow_element_count` and
`shadow_trace_count` fields in `CutMeta`. Phase 5 candidate.

---

## Files

| File | Type | Purpose |
|------|------|---------|
| `meshant/serve/handlers.go` | Modified | Added `filterByElement`, `handleElement` |
| `meshant/serve/server.go` | Modified | Added `//go:embed web`, `fs.Sub`, static file handler, `/element/{name}` route |
| `meshant/serve/web/index.html` | New | SPA shell: observer gate, cut header, main layout |
| `meshant/serve/web/style.css` | New | Layout and component styles |
| `meshant/serve/web/app.js` | New | Observer gate, API fetch, cut meta, node click wiring, export wiring, init |
| `meshant/serve/web/render.js` | New | Cytoscape initialisation, graph element mapping, shadow panel, detail panel |
| `meshant/serve/web/export.js` | New | JSON export, DOT export, download trigger |
| `meshant/serve/web/lib/cytoscape.min.js` | New | Vendored Cytoscape.js 3.30.4 (~374KB) |
| `docs/decisions/web-ui-v1.md` | New | This document |

---

## Definition of Done

- [x] `/element/{name}?observer=` returns 200 with trace array; 400 when observer absent; 500 on store error; empty array for unknown element; observer filters applied before element filter; URL-encoding handled
- [x] `go:embed web` in `server.go`; `GET /` serves `index.html`; `GET /style.css` and `GET /app.js` return 200 with correct Content-Type
- [x] API routes take precedence over static file handler
- [x] All new tests pass (7 handleElement tests + 4 static-serving tests)
- [x] `go test ./serve/...` green; coverage 93.4% (> 80%)
- [x] `go vet ./...` clean
- [x] `index.html`: observer gate (form), cut header (permanent), graph + sidebar layout
- [x] `style.css`: observer gate, cut header, main flex layout, shadow panel (amber), detail panel (grey)
- [x] `app.js`: observer gate, API fetch wrapper, cut meta render, node click, export wiring, DOMContentLoaded init
- [x] `render.js`: Cytoscape init, graph element mapping, shadow panel, detail panel with provenance
- [x] `export.js`: JSON download, DOT generation (mirrors PrintGraphDOT), file trigger
- [x] Cytoscape.js 3.30.4 vendored in `web/lib/`
- [x] Decision record written
- [x] `tasks/todo.md` updated
- [x] `docs/CODEMAPS/meshant.md` updated
