// pdf.go implements PDFAdapter — PDF to plain text conversion.
//
// Uses github.com/ledongthuc/pdf (pure Go, no CGo) to extract text page by page.
// The adapter is a named mediator: its name ("pdf-extractor") travels with the
// ConvertResult so downstream provenance records which transformation was applied.
//
// Known limitation: ledongthuc/pdf produces reasonable output for simple, single-column
// PDFs. Complex layouts (multi-column, rotated text, embedded images as content) may
// yield garbled or incomplete text. Analysts should use meshant convert to inspect the
// extracted text before running extraction. This limitation is inherent to pure-Go PDF
// parsing without a rendering engine.
package adapter

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFAdapter converts a PDF file to plain text.
// It is a mediator: the conversion is named ("pdf-extractor") and carries
// page count metadata so the transformation is visible in provenance.
type PDFAdapter struct{}

// Convert reads the PDF at path and returns extracted plain text.
// Text is extracted page by page; pages are separated by a blank line.
// Returns an error if the file is missing, oversized, or not a valid PDF.
func (a *PDFAdapter) Convert(path string) (ConvertResult, error) {
	// Enforce raw size cap before attempting to parse.
	info, err := os.Stat(path)
	if err != nil {
		return ConvertResult{}, fmt.Errorf("pdf-extractor: stat %q: %w", path, err)
	}
	if info.Size() > maxRawBytes {
		return ConvertResult{}, fmt.Errorf("pdf-extractor: %q exceeds %d bytes raw size limit", path, maxRawBytes)
	}

	f, r, err := pdf.Open(path)
	if err != nil {
		return ConvertResult{}, fmt.Errorf("pdf-extractor: open %q: %w", path, err)
	}
	defer f.Close()

	totalPages := r.NumPage()
	var sb strings.Builder
	for i := 1; i <= totalPages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			return ConvertResult{}, fmt.Errorf("pdf-extractor: page %d of %q: %w", i, path, err)
		}
		if i > 1 {
			sb.WriteString("\n")
		}
		sb.WriteString(text)
	}

	return ConvertResult{
		Text:        sb.String(),
		AdapterName: "pdf-extractor",
		Metadata: map[string]string{
			"page_count": strconv.Itoa(totalPages),
		},
	}, nil
}
