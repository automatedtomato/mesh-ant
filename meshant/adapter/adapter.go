// Package adapter provides format-conversion adapters for the MeshAnt ingestion pipeline.
//
// Each adapter is a mediator in the ANT sense: it transforms what passes through it and
// names that transformation. The ConvertResult carries AdapterName so the mediating act
// is visible downstream — in session records, in provenance, in the analyst's workflow.
//
// Adapters convert non-text sources (PDF, HTML, structured logs) to plain text so the
// existing LLM extraction pipeline can ingest them without modification. The text → trace
// transformation remains the LLM's responsibility; the format → text transformation is the
// adapter's.
//
// Usage:
//
//	a, err := adapter.ForName("pdf")
//	if err != nil { ... }
//	result, err := a.Convert("/path/to/report.pdf")
//	// result.Text is ready for meshant extract
//	// result.AdapterName is "pdf-extractor" (recorded in session provenance)
package adapter

import "fmt"

// maxRawBytes caps the raw source file size before conversion. 10 MiB is generous
// for pre-conversion files (a 10 MiB PDF typically yields far less than 1 MiB of text).
// The downstream readSourceDoc in llm/shared.go enforces a separate 1 MiB cap on
// the extracted text.
const maxRawBytes = 10 * 1024 * 1024

// ConvertResult carries the output of an adapter conversion.
// Text is the extracted plain-text content; AdapterName identifies the mediator
// that performed the transformation.
type ConvertResult struct {
	// Text is the extracted plain-text content ready for the LLM extraction pipeline.
	Text string
	// AdapterName is the canonical name of the adapter that produced this result.
	// It is recorded in the session provenance so the mediating act is visible.
	AdapterName string
	// Metadata holds adapter-specific details (e.g. page_count for PDF, line_count for logs).
	// Keys and values are strings; the adapter is responsible for populating these.
	Metadata map[string]string
}

// Adapter converts a non-text source file to plain text for downstream extraction.
// Each Adapter is a mediator: it transforms what passes through it, and must name
// that transformation via ConvertResult.AdapterName.
type Adapter interface {
	// Convert reads the source file at path and returns extracted text.
	// The returned ConvertResult always carries AdapterName even on partial results.
	// Returns an error if the file cannot be read, exceeds size limits, or the
	// format is unrecognisable by this adapter.
	Convert(path string) (ConvertResult, error)
}

// ForName returns the Adapter registered under name.
// Known names: "pdf", "html", "jsonlog".
// Returns an error for unknown names so the caller sees a clear message rather than a nil panic.
func ForName(name string) (Adapter, error) {
	switch name {
	case "pdf":
		return &PDFAdapter{}, nil
	case "html":
		return &HTMLAdapter{}, nil
	case "jsonlog":
		return &JSONLogAdapter{}, nil
	default:
		return nil, fmt.Errorf("adapter: unknown adapter %q — known adapters: pdf, html, jsonlog", name)
	}
}
