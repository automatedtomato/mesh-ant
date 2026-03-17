# MeshAnt — M9 Plan: CLI + Docs + Release v1.0.0

This milestone brings MeshAnt to v1.0.0 by adding a CLI binary, a trace authoring guide,
and updating the README. No new library code is needed — the CLI wraps existing packages.

**Full plan:** `tasks/plan_m9.md` (this file)

---

## Goal

v1.0.0 is the Library + CLI form: the framework can be used without writing Go.
- A `meshant` binary with four subcommands: `summarize`, `articulate`, `diff`, `validate`
- A trace authoring guide for users writing their own datasets
- A README that explains who MeshAnt is for and how to use the CLI

---

## Sub-milestones

### M9.1 — CLI core: `summarize` and `validate`

**Branch:** `feat/m9-cli-core`

Establishes the CLI architecture. All subsequent subcommands follow the same pattern.

**Files:**

- `meshant/cmd/meshant/main.go`
  - `main()` — reads `os.Args[1]`, dispatches to handler; prints usage on unknown/missing command
  - `run(w io.Writer, args []string) error` — testable entry point that mirrors `main()`
  - `cmdSummarize(w io.Writer, args []string) error` — `args[0]` = path; calls `loader.Load → Summarise → PrintSummary`
  - `cmdValidate(w io.Writer, args []string) error` — `args[0]` = path; calls `loader.Load`; on success prints "N traces: all valid"; on error prints each failing trace ID and error
  - Usage text: `meshant <command> [flags] <traces.json>`

- `meshant/cmd/meshant/main_test.go` (package `main`)
  - Tests for `cmdSummarize`: happy path against evacuation dataset; invalid path error
  - Tests for `cmdValidate`: happy path; writer error
  - Tests for `run()`: unknown subcommand; missing subcommand

**Design notes:**
- Each subcommand is a `func cmdXxx(w io.Writer, args []string) error` — testable without process execution
- Stdlib only: no cobra, no viper
- Pattern mirrors `cmd/demo/main_test.go`

**Dependencies:** None
**Risk:** Low

---

### M9.2 — CLI `articulate` subcommand

**Branch:** `feat/m9-cli-articulate`

Most flag-rich subcommand. Introduces the `stringSliceFlag` helper for repeatable flags.

**Files:**

- `meshant/cmd/meshant/main.go` (extend)
  - `cmdArticulate(w io.Writer, args []string) error`
  - `stringSliceFlag` — custom `flag.Value` type; appends on each `Set()` call; enables `--observer a --observer b`
  - Flag parsing via `flag.NewFlagSet("articulate", flag.ContinueOnError)`:
    - `--observer` (repeatable, required)
    - `--from` (string, optional, RFC3339)
    - `--to` (string, optional, RFC3339)
    - `--format` (string, default `text`, one of `text|json|dot|mermaid`)
  - Builds `graph.ArticulationOptions`; calls `graph.Articulate`; dispatches to `PrintArticulation`, `PrintGraphJSON`, `PrintGraphDOT`, or `PrintGraphMermaid`
  - Error if no `--observer` provided
  - Error if `--from`/`--to` cannot be parsed; message includes expected format: `expected RFC3339 (e.g. 2026-04-14T00:00:00Z)`
  - Calls `TimeWindow.Validate()` before articulating

- `meshant/cmd/meshant/main_test.go` (extend)
  - Happy path: articulate evacuation dataset with `--observer meteorological-analyst`
  - Happy path with time window
  - Format variants: `--format json`, `--format dot`, `--format mermaid`
  - Error: missing `--observer`
  - Error: invalid `--from` value
  - Error: inverted time window (`--from` > `--to`)

**Dependencies:** M9.1 (CLI skeleton)
**Risk:** Medium — flag parsing has edge cases (empty observer, bad times, unknown format)

---

### M9.3 — CLI `diff` subcommand

**Branch:** `feat/m9-cli-diff`

Diffs two articulations from the same trace file.

**Files:**

- `meshant/cmd/meshant/main.go` (extend)
  - `cmdDiff(w io.Writer, args []string) error`
  - Flags (via `flag.NewFlagSet`):
    - `--observer-a` (repeatable, `stringSliceFlag`)
    - `--observer-b` (repeatable, `stringSliceFlag`)
    - `--from-a`, `--to-a` (string, optional, RFC3339)
    - `--from-b`, `--to-b` (string, optional, RFC3339)
    - `--format` (string, default `text`, one of `text|json`)
  - Loads traces once; articulates two graphs (opts A, opts B); calls `graph.Diff`; prints
  - Error if neither `--observer-a` nor `--observer-b` provided
  - Error if `--format dot` or `--format mermaid`: `"diff --format dot/mermaid not supported: no PrintDiffDOT exists"`

- `meshant/cmd/meshant/main_test.go` (extend)
  - Happy path: diff `meteorological-analyst` vs `local-mayor` on evacuation dataset
  - Happy path: `--format json`
  - Error: no observers provided
  - Error: unsupported format (`dot`, `mermaid`)
  - Error: invalid time format

**Dependencies:** M9.2 (`stringSliceFlag` and time-parsing helpers)
**Risk:** Medium — many flags; underlying `Diff` + `PrintDiff` are well-tested

---

### M9.4 — Trace authoring guide

**Branch:** `feat/m9-authoring-guide`

Practical guide for users writing their own `traces.json` files. Independent of CLI work.

**File:** `docs/authoring-traces.md`

**Sections:**

1. **What a trace captures** — the 5 key fields (`source`, `target`, `observer`, `mediation`, `tags`) with one-sentence explanations; schema reference (`meshant/schema/trace.go`)
2. **Required vs optional fields** — `id`, `timestamp`, `what_changed`, `observer` are required; `source`, `target`, `mediation`, `tags` are optional and their absence is meaningful
3. **The 6 tag types** — `delay`, `threshold`, `blockage`, `amplification`, `redirection`, `translation`; when to use each; one-line example per tag
4. **Handling absent sources** — when no identifiable source exists (automated triggers, webhooks), leave `source` empty; explain why this is valid
5. **Observer position choices** — what makes a good observer string; consistency advice; warn against collapsing multiple positions into one
6. **Graph references in traces** — `meshgraph:<uuid>` and `meshdiff:<uuid>` in `source`/`target`; when to use (reflexive tracing)
7. **Worked example** — a short 3–5 trace JSON snippet with inline commentary
8. **Validating your traces** — `meshant validate traces.json`

**Constraints:** under 200 lines; practical first, not theoretical; link to `docs/principles.md` for theory.

**Dependencies:** None (can be developed in parallel with M9.1–M9.3)
**Risk:** Low

---

### M9.5 — README + decision record

**Branch:** `feat/m9-readme`

Documentation pass. Release tag comes after the quality gates (M9.6–M9.8).

**Files:**

- `README.md` (modify)
  - Add "Who is this for?" section after the opening paragraph:
    > MeshAnt is a trace-first framework for making sense of messy distributed or
    > socio-technical systems — especially when behavior emerges from interactions between
    > services, policies, interfaces, delays, and human actions rather than from a single
    > explicit actor.
  - Add "CLI" section with usage examples for all four subcommands
  - Update "Run from source" section to mention the CLI binary alongside the demo
  - Remove the "Known gap — Principle 8 partially open" note (closed in M7-B)

- `docs/decisions/cli-v2.md` (new) — 6 decisions:
  1. Stdlib `flag` only — no cobra/viper; zero new dependencies; consistent with project policy
  2. Subcommand dispatch via `os.Args[1]` string match — simplest approach for 4 commands
  3. `--observer` as repeatable flag (custom `stringSliceFlag`) — more explicit than comma-separated; handles observer names containing commas
  4. `validate` included in v1.0.0 — validates the schema contract externally; natural verification step for trace authors using the authoring guide
  5. All output to stdout; no `--output` file flag — Unix convention; redirect with `>`
  6. `diff --format` restricted to `text|json` — `PrintDiffDOT`/`PrintDiffMermaid` do not exist; deferred to future milestone

- `Dockerfile` (modify)
  - Add `go build -o /meshant ./meshant/cmd/meshant` to build stage
  - Copy CLI binary to `/usr/local/bin/meshant` in runtime image
  - Keep demo as default `CMD` (backwards compatibility)
  - CLI available inside container as `meshant`

**Dependencies:** M9.1–M9.4 all merged to develop
**Risk:** Low

---

### M9.6 — Refactor and clean pass (whole codebase)

**Branch:** `feat/m9-refactor`

This is a major release. Code delivered to users should be clean, consistent, and free of
dead weight across the entire module — not just the new M9 additions.

**Scope:** all packages under `meshant/`: `schema`, `loader`, `graph`, `persist`, `cmd/demo`, `cmd/meshant`

**What to check and fix:**

- Dead code — unexported functions, types, or constants that are defined but never used
- Naming consistency — exported names should read naturally; internal names should be concise
- File size — no file over 800 lines; extract where appropriate
- Function length — flag any function over 50 lines
- Error handling — no silently swallowed errors; all error paths explicit
- Immutability — no unexpected in-place mutation of slice/map arguments
- Test coverage — confirm all packages remain at their established coverage targets after refactoring
- Comments — remove stale or misleading comments introduced during earlier milestones

**Agent:** `refactor-cleaner` — runs analysis tools (`knip` equivalent for Go: `staticcheck`, `go vet`, `deadcode`), identifies candidates, removes safely.

**Note:** This pass should not change external API signatures. Any API-level change would
require a decision record and an update to `docs/CODEMAPS/meshant.md`.

**Dependencies:** M9.5 merged to develop (all code present)
**Risk:** Medium — broad scope; must not break tests

---

### M9.7 — Philosophical review

**Branch:** `feat/m9-philosophical-review`

Before shipping v1.0.0, confirm the implementation is consistent with the methodological
commitments in `docs/principles.md` and `docs/manifesto.md`. This is not a code review —
it is a conceptual alignment check.

**What to review:**

- Does the CLI surface the shadow as a first-class output, or does it make it easy to skip?
- Does `meshant validate` reinforce the trace-first approach, or does it impose a schema
  that feels like role-definition in disguise?
- Does the authoring guide (`docs/authoring-traces.md`) lead users toward genuine
  mediation tracking, or toward conventional log-style recording?
- Are there any naming choices (subcommand names, flag names, output labels) that subtly
  import god's-eye framing (e.g. "full graph", "all actors", "complete network")?
- Does the README "Who is this for?" section accurately frame the tool without overclaiming?
- Is there any new code that introduces a premature actor taxonomy or a fixed ontology?

**Output:** a brief review note (`docs/reviews/review_philosophical_m9.md`) recording what
was checked, what was found, and any corrections made or deferred.

**Agent:** `.claude/skills/philosophical-review/` skill.

**Dependencies:** M9.6 (clean codebase to review)
**Risk:** Low — findings are likely small naming or framing corrections

---

### M9.8 — Codemap update + release v1.0.0

**Branch:** `feat/m9-release`

Final assembly: update the codemap, record M9 complete in `tasks/todo.md`, and tag the release.

**Files:**

- `docs/CODEMAPS/meshant.md` (modify)
  - Add `cmd/meshant` package section with subcommand list and flag summary
  - Reflect any renames or removals from M9.6 refactor pass
  - Update cross-package relationships

- `tasks/todo.md` (modify) — add M9 section with all sub-milestones marked complete

- Tag `v1.0.0` on main after merge

**Release notes should cover:**
- CLI: four subcommands, stdlib only, stdin/stdout, all export formats
- Authoring guide in `docs/`
- Refactor pass: module-wide clean before first public release
- Philosophical review: alignment with principles confirmed
- What is not in v1.0.0 (interactive CLI, DOT/Mermaid for diffs, tag-filter axis) — see `docs/directions.md`

**Dependencies:** M9.7 merged to develop
**Risk:** Low

---

## Parallelization

```
M9.1 (CLI core: summarize + validate)
  └─> M9.2 (articulate)
        └─> M9.3 (diff)
              └─> M9.5 (README + decision record)
                    └─> M9.6 (refactor + clean, whole codebase)
                          └─> M9.7 (philosophical review)
                                └─> M9.8 (codemap + release v1.0.0)

M9.4 (authoring guide)  ← independent; merge to develop at any point before M9.7
```

M9.4 has no code dependencies and can be written and merged in parallel with any of M9.1–M9.3.
M9.6, M9.7, M9.8 are sequential quality gates before the release tag.

---

## Open questions

**1. Dockerfile: CLI as primary entrypoint or secondary binary?**

Recommendation: keep demo as default `CMD`; add CLI binary at `/usr/local/bin/meshant`
so `docker run --rm mesh-ant-demo meshant summarize /data/dataset.json` works.
Avoids a breaking change to the Docker interface introduced in v0.2.0.

Decide before M9.5.

**2. `validate` in v1.0.0?**

Recommendation: yes. It costs ~10 lines and completes the authoring guide's workflow.
Without it, the guide would have to say "write a Go program to check your traces."

---

## Testing strategy

- All CLI tests in `meshant/cmd/meshant/main_test.go`, package `main`
- Each `cmdXxx` accepts `io.Writer` + `[]string` — testable without process execution
- `run(w, args)` as the testable entry point for dispatch tests
- Happy paths against real datasets (`evacuation_order.json`, `incident_response.json`)
- Error paths: missing file, missing required flags, bad flag values, writer errors
- Coverage target: ≥ 80% on `cmd/meshant` (Go cannot cover `main()` itself)

---

## Merge chain

```
feat/m9-cli-core              → develop
feat/m9-cli-articulate        → develop
feat/m9-cli-diff              → develop
feat/m9-authoring-guide       → develop  (any order relative to CLI branches)
feat/m9-readme                → develop  (after M9.1–M9.4)
feat/m9-refactor              → develop  (after M9.5)
feat/m9-philosophical-review  → develop  (after M9.6)
feat/m9-release               → develop  (after M9.7)
develop                       → main
git tag v1.0.0
```

---

## Success criteria

- [ ] `meshant summarize data/examples/evacuation_order.json` prints mesh summary
- [ ] `meshant articulate data/examples/evacuation_order.json --observer meteorological-analyst --from 2026-04-14T00:00:00Z --to 2026-04-14T23:59:59Z` prints articulation
- [ ] `meshant articulate ... --format json` produces valid JSON
- [ ] `meshant articulate ... --format dot` produces valid Graphviz DOT
- [ ] `meshant articulate ... --format mermaid` produces valid Mermaid syntax
- [ ] `meshant diff data/examples/evacuation_order.json --observer-a meteorological-analyst --observer-b local-mayor` prints diff
- [ ] `meshant validate data/examples/evacuation_order.json` reports "N traces: all valid"
- [ ] `meshant` (no args) prints usage
- [ ] `meshant badcommand` prints usage + error
- [ ] `go test ./...` passes from `meshant/`
- [ ] `cmd/meshant` coverage ≥ 80%
- [ ] README contains "Who is this for?" section
- [ ] README contains CLI usage examples for all 4 subcommands
- [ ] README does not contain "Known gap — Principle 8" note
- [ ] `docs/authoring-traces.md` exists and is ≤ 200 lines
- [ ] `docs/decisions/cli-v2.md` exists
- [ ] `docs/CODEMAPS/meshant.md` updated with `cmd/meshant`
- [ ] Docker image builds; `meshant` binary available inside container
- [ ] `go vet ./...` and `staticcheck ./...` pass with no findings
- [ ] No unexported dead code remaining in any package
- [ ] Philosophical review note exists at `docs/reviews/review_philosophical_m9.md`
- [ ] No god's-eye framing in CLI output labels, flag names, or README
- [ ] `git tag v1.0.0` on main

---

*Written: 2026-03-12. Updated: 2026-03-12 (added M9.6 refactor, M9.7 philosophical review, M9.8 release). Confirmed by user — implementation in progress.*
