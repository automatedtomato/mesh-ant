// pdf_test.go tests PDFAdapter in black-box style.
package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/adapter"
)

// samplePDF returns the path to the committed minimal test PDF.
func samplePDF(t *testing.T) string {
	t.Helper()
	p := filepath.Join("testdata", "sample.pdf")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("testdata/sample.pdf not found: %v", err)
	}
	return p
}

// --- Group: PDFAdapter ---

// TestPDFAdapter_HappyPath verifies that a valid PDF produces non-empty text.
func TestPDFAdapter_HappyPath(t *testing.T) {
	a := mustAdapter(t, "pdf")
	result, err := a.Convert(samplePDF(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if strings.TrimSpace(result.Text) == "" {
		t.Error("Convert() result.Text: want non-empty, got empty")
	}
}

// TestPDFAdapter_AdapterName verifies that ConvertResult.AdapterName is "pdf-extractor".
func TestPDFAdapter_AdapterName(t *testing.T) {
	a := mustAdapter(t, "pdf")
	result, err := a.Convert(samplePDF(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if result.AdapterName != "pdf-extractor" {
		t.Errorf("AdapterName: want %q, got %q", "pdf-extractor", result.AdapterName)
	}
}

// TestPDFAdapter_MetadataPageCount verifies that the page_count metadata key is set.
func TestPDFAdapter_MetadataPageCount(t *testing.T) {
	a := mustAdapter(t, "pdf")
	result, err := a.Convert(samplePDF(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if result.Metadata["page_count"] == "" {
		t.Error("Metadata[page_count]: want non-empty, got empty")
	}
}

// TestPDFAdapter_FileNotFound verifies that a missing file returns an error.
func TestPDFAdapter_FileNotFound(t *testing.T) {
	a := mustAdapter(t, "pdf")
	_, err := a.Convert("/no/such/file.pdf")
	if err == nil {
		t.Fatal("Convert() with missing file: want error, got nil")
	}
}

// TestPDFAdapter_NonPDFFile verifies that a non-PDF file returns an error.
func TestPDFAdapter_NonPDFFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-a-pdf.pdf")
	if err := os.WriteFile(path, []byte("this is plain text, not PDF"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "pdf")
	_, err := a.Convert(path)
	if err == nil {
		t.Fatal("Convert() with non-PDF content: want error, got nil")
	}
}

// TestPDFAdapter_OversizedFile verifies that a file exceeding the raw size cap returns an error.
func TestPDFAdapter_OversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.pdf")
	// Write a file larger than maxRawBytes (10 MiB) — content is irrelevant.
	big := make([]byte, 11*1024*1024)
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "pdf")
	_, err := a.Convert(path)
	if err == nil {
		t.Fatal("Convert() with oversized file: want error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q: want 'exceeds' in message", err.Error())
	}
}

// TestForName_PDF verifies that ForName("pdf") returns a non-nil adapter.
func TestForName_PDF(t *testing.T) {
	a, err := adapter.ForName("pdf")
	if err != nil {
		t.Fatalf("ForName(%q) want no error, got: %v", "pdf", err)
	}
	if a == nil {
		t.Fatal("ForName(pdf) returned nil adapter")
	}
}
