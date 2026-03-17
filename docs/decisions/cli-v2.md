# M9 Decision Record — CLI Design (v1.0.0)

## Context

M9 adds a `meshant` CLI binary that wraps the existing library packages. Six design
decisions shaped its form.

---

## Decision 1 — Stdlib `flag` only; no third-party CLI framework

The CLI uses Go's stdlib `flag` package exclusively. No cobra, viper, or similar frameworks.

Rationale: MeshAnt has no external dependencies beyond the Go standard library. Adding a
CLI framework to one binary would be the first such dependency in the project. The four
subcommands and their flags are simple enough that `flag.FlagSet` per subcommand handles
them without boilerplate. Adding a framework for four commands would introduce complexity
and an ongoing maintenance obligation that the project does not need at v1.0.0.

---

## Decision 2 — Subcommand dispatch via `os.Args[1]` string match

The top-level `run()` function dispatches to `cmdSummarize`, `cmdValidate`, `cmdArticulate`,
or `cmdDiff` by switching on `args[0]`.

Rationale: the simplest correct approach for four fixed subcommands. A plugin-style or
reflection-based approach would add complexity without benefit at this scale. If the command
set grows significantly, this decision can be revisited.

---

## Decision 3 — `--observer` as a repeatable flag (custom `stringSliceFlag`)

`--observer` and the A/B observer flags accept multiple values by repeating the flag:
`--observer pos-one --observer pos-two`.

Rationale: more explicit than comma-separated values (which are ambiguous if observer names
contain commas, unlikely but possible) and more natural in shell scripting than a quoted
comma-delimited string. The implementation is a 3-line custom `flag.Value` type.

---

## Decision 4 — `validate` included in v1.0.0

The `meshant validate` subcommand validates all traces in a file and reports errors.

Rationale: `validate` is the natural verification step for users following the trace
authoring guide (`docs/authoring-traces.md`). Without it, the guide would have to say
"write a Go program to check your traces," which contradicts the purpose of shipping a CLI.
The implementation cost is minimal (wraps `loader.Load` which already validates internally).

---

## Decision 5 — All output to stdout; no `--output` file flag *(reversed in M10)*

Every subcommand writes to stdout. File redirection is left to the shell: `meshant articulate ... > graph.dot`.

Rationale: Unix convention. Output-to-file is trivially composable via shell redirection;
adding `--output` would duplicate that capability and require handling file creation, mode,
and error reporting in every subcommand. Keeping all output on stdout also makes the
functions easy to test without touching the filesystem.

**Reversed in M10**: `--output <file>` flag added to `articulate`, `diff`, and `follow`.
Rationale for reversal: users working with VS Code Graphviz/Mermaid extensions need files
they can open directly; shell redirection loses the confirmation message. The implementation
uses `outputWriter(w, outputPath)` + `defer f.Close()` + `confirmOutput(w, outputPath)`,
keeping the testable structure unchanged (tests still pass a `bytes.Buffer` as `w`).

---

## Decision 6 — `diff --format` restricted to `text|json` *(reversed in M10)*

`meshant articulate` supports four output formats: text, json, dot, mermaid.
`meshant diff` supports only text and json.

Rationale: `PrintDiffDOT` and `PrintDiffMermaid` do not exist. Diff visualization requires
rendering two cuts side by side with delta annotations — a non-trivial layout problem
deferred since M8 (see `docs/decisions/structured-export-v1.md`, "What M8 does not close").
Requesting `--format dot` or `--format mermaid` on `diff` returns a clear error:
`"diff: --format %q not supported (text|json only)"`.

**Reversed in M10**: `PrintDiffDOT` and `PrintDiffMermaid` implemented in M10 (`export.go`).
`meshant diff` now supports all four formats. Visual conventions: added=green/bold,
removed=red/dashed, shadow shifts in dedicated subgraph. See `docs/decisions/m10-tag-filter-diff-export-cli-v1.md` Decision 3.

---

## M9 scope note

The four subcommands at v1.0.0: `summarize`, `validate`, `articulate`, `diff`.

Subsequent milestones added:
- **M10**: `--tag`, `--output`, `--format dot|mermaid` on `diff` (no new subcommands)
- **M10.5**: `follow` subcommand (`--element`, `--direction`, `--depth`, `--criterion-file`)
- **M11**: `draft`, `promote` subcommands (TraceDraft ingestion pipeline)
- **M12**: `rearticulate`, `lineage` subcommands (re-articulation as second cut)

Total at M12: 9 subcommands. The `run()` dispatcher switch remains the routing mechanism; no CLI framework has been introduced.
