# Decision Record: Non-Text Source Adapters (v1)

**Issue:** #140
**Branch:** `140-non-text-source-adapters`
**Merged:** PR #156 → `develop` (2026-03-23)

---

## Problem

`meshant extract` and the broader ingestion pipeline accept plain-text source files only.
Real-world ingestion targets — incident post-mortems (PDF), web pages (HTML), structured
log exports (JSONL) — require a conversion step before LLM extraction. Without it,
analysts must convert manually, an invisible operation that breaks provenance.

---

## Decision: Named mediator pattern

A new `adapter` package exposes:

```go
type Adapter interface {
    Convert(path string) (ConvertResult, error)
}

type ConvertResult struct {
    Text        string
    AdapterName string
    Metadata    map[string]string
}

func ForName(name string) (Adapter, error)
```

Three adapters at v1: `pdf` → `PDFAdapter` ("pdf-extractor"), `html` → `HTMLAdapter`
("html-extractor"), `jsonlog` → `JSONLogAdapter` ("jsonlog-parser").

`ConvertResult.AdapterName` is set by the adapter itself — the mediator names itself.
This name travels through the pipeline into `ExtractionConditions.AdapterName`
(json:"adapter_name,omitempty") in the session record, making the transformation visible
in every provenance file.

---

## Alternatives considered

**Auto-detection by file extension**: Rejected. Extension-based routing hides the analyst's
choice behind a convention. The analyst must name the mediator explicitly (`--adapter pdf`)
so the act of format translation is an observable, recorded decision, not invisible magic.

**Single `meshant extract --adapter` workflow only**: Rejected. Analysts need to inspect
converted text before committing to an LLM call. The separate `meshant convert` subcommand
serves this: convert → inspect → extract. The `--adapter` flag on `extract` is a
convenience path for when the analyst is confident in the adapter output.

**CGo-based PDF parsers** (poppler, pdfium): Rejected. They require build toolchain
dependencies and complicate cross-compilation. `github.com/ledongthuc/pdf` is pure Go with
no CGo. Its known limitation (complex multi-column layouts may lose structure) is documented
in `adapter/pdf.go` and acceptable for a v1 boundary tool.

**DOM-based HTML parsing**: Rejected in favour of `golang.org/x/net/html` tokenizer
(iterative, not DOM). Lower memory overhead; sufficient for text extraction purposes.

---

## Implementation

### Raw file size cap

`maxRawBytes = 10 MiB` is enforced before any adapter parsing (via `os.Stat`). This is
distinct from the existing per-document text cap in `llm/shared.go` (which caps the
converted plain-text content). Source files are larger than their text representations,
so the raw cap is set higher.

### HTML adapter

Uses `golang.org/x/net/html` tokenizer. Skips `<script>`, `<style>`, `<noscript>`.
Block elements (`p`, `div`, `br`, `h1`–`h6`, `li`, `tr`, `td`, etc.) produce newlines.
Normalises whitespace: splits on newlines, trims each line, joins non-empty lines.
`Metadata` is empty at v1 (no element count tracking).

### JSONLog adapter

`bufio.Scanner` line-by-line (default 64 KB buffer per line). Each line: if valid JSON
object, extracts `message` field and renders remaining fields as sorted `key=value` pairs;
if not valid JSON, passes verbatim. `Metadata["line_count"]` records total lines processed.

### PDF adapter

`github.com/ledongthuc/pdf` page-by-page `GetPlainText`. `Metadata["page_count"]` records
total pages. Known limitation: complex multi-column layouts or scanned PDFs without OCR
may produce degraded text. This is a v1 limitation; analysts should use `meshant convert`
to inspect output before extraction.

### Two-workflow design

| Workflow | Command | Use case |
|----------|---------|----------|
| Inspect first | `meshant convert --adapter pdf --source-doc f.pdf --output f.txt` | Review converted text; decide whether to proceed |
| Single step | `meshant extract --adapter pdf --source-doc f.pdf` | Trusted adapter; direct extraction |

Both use the same `adapter.ForName` + `Adapter.Convert` chain. `meshant extract --adapter`
converts to OS temp files (`meshant-convert-*.txt`), deferred for removal, then hands the
temp path to the existing extraction pipeline unchanged.

### Provenance chain

```
--adapter pdf
  → adapter.ForName("pdf")
  → PDFAdapter.Convert(srcPath)
  → ConvertResult{AdapterName: "pdf-extractor", ...}
  → convertedAdapterName = result.AdapterName
  → ExtractionOptions{AdapterName: convertedAdapterName}
  → RunExtraction → ExtractionConditions{AdapterName: "pdf-extractor"}
  → session record JSON: "adapter_name": "pdf-extractor"
```

Early validation: `adapter.ForName` is called before any LLM call or temp file creation.
Unknown adapter names fail fast without wasting API quota.

---

## Standing tensions

### Shadow of the adapter's cut

The adapter transforms the source: HTML strips scripts and styles, PDF may lose layout,
JSONL restructures log lines. What was excluded is not currently recorded in `Metadata`.
Future enhancement: add skipped-element counts or a `CutSummary` field to `ConvertResult`
to make the adapter's excisions visible in the session record.

### Single adapter per session

`meshant extract --adapter` accepts one adapter name. In a multi-document session (#139),
all documents share the same adapter assumption. Mixed-format sessions (some PDF, some HTML)
require separate extraction sessions at v1. Per-document adapter specification would
require richer `ExtractionOptions` typing.

### Duplicate `stringSlice` type

`cmd_extract.go` defines `stringSlice` (from #139); `main.go` defines `stringSliceFlag`
(earlier). Both implement `flag.Value` identically. Deferred for consolidation in the
Phase 1 per-thread refactor-cleaner pass.

---

## External dependencies added

| Dependency | Version | Purpose |
|------------|---------|---------|
| `github.com/ledongthuc/pdf` | `v0.0.0-20250511090121-5959a4027728` | Pure-Go PDF text extraction |
| `golang.org/x/net` | `v0.52.0` | HTML tokenizer (`golang.org/x/net/html`) |
