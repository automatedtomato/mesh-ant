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

## Decision 5 — All output to stdout; no `--output` file flag

Every subcommand writes to stdout. File redirection is left to the shell: `meshant articulate ... > graph.dot`.

Rationale: Unix convention. Output-to-file is trivially composable via shell redirection;
adding `--output` would duplicate that capability and require handling file creation, mode,
and error reporting in every subcommand. Keeping all output on stdout also makes the
functions easy to test without touching the filesystem.

---

## Decision 6 — `diff --format` restricted to `text|json`

`meshant articulate` supports four output formats: text, json, dot, mermaid.
`meshant diff` supports only text and json.

Rationale: `PrintDiffDOT` and `PrintDiffMermaid` do not exist. Diff visualization requires
rendering two cuts side by side with delta annotations — a non-trivial layout problem
deferred since M8 (see `docs/decisions/structured-export-v1.md`, "What M8 does not close").
Requesting `--format dot` or `--format mermaid` on `diff` returns a clear error:
`"diff: --format %q not supported (text|json only)"`.
