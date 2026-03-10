# Implementation Plan: M1.1 — Trace Schema

**Branch:** `feat/m1-trace-schema`
**Status:** Confirmed — in progress

## Requirements Restatement

Define `meshant/schema/trace.go`: a Go struct representing a single MeshAnt trace — a record of something that made a difference in a network. No actor types, no external deps, stdlib only. TDD approach.

## Files to Create

| File | Purpose |
|---|---|
| `meshant/go.mod` | Go module (`github.com/automatedtomato/mesh-ant/meshant`, go 1.22) |
| `meshant/schema/trace.go` | `Trace` struct + `TagValue` constants + `Validate()` method |
| `meshant/schema/trace_test.go` | Table-driven tests (JSON round-trip, validation, zero-value safety) |
| `docs/decisions/trace-schema-v1.md` | Design decision record (what was cut and why) |

## Phase 1 — Scaffold

- Create `meshant/go.mod`
- Define `Trace` struct with JSON tags, `TagValue` type, tag constants
- Stub `Validate() error` returning `nil` (allows compilation)

## Phase 2 — RED (tests first)

Write `trace_test.go` with test groups:
1. **JSON round-trip** — full record, minimal record, omitempty behavior, key names
2. **Validation** — missing `id`, zero `timestamp`, empty `what_changed`, empty `observer` all fail; empty `source`/`target`/`mediation` are permitted
3. **Zero-value safety** — `Trace{}` marshals without panic, `Validate()` returns error
4. **Tag constants** — each constant serializes to expected string
5. **Interoperability** — unknown JSON fields ignored, unknown tag values accepted

## Phase 3 — GREEN

Implement `Validate()`: check non-empty `ID` + UUID format (stdlib `regexp`), non-zero `Timestamp`,
non-empty `WhatChanged`, non-empty `Observer`.

## Phase 4 — Refactor + decision doc

`gofmt`, review comments, write `docs/decisions/trace-schema-v1.md`.

## Risks

- LOW: UUID format check via stdlib regexp — no external dep needed
- LOW: `Observer` required field may be verbose in tests — intentional by design

## Estimated Complexity: LOW

## Key design decisions (from planner)

- `source`/`target` stay as plain `string` — no `Actor` type yet
- `tags` is `[]string` not `[]TagValue` — keeps vocabulary open
- `observer` is required by `Validate()` — enforces Principle 8 (designer inside the mesh)
- `mediation` is optional — its absence means no intermediary was observed, not that none exists
- UUID validation via stdlib `regexp` — no external deps
