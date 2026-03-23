package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConvertHTML writes a minimal HTML file and returns its path.
func writeConvertHTML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "source.html")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeConvertHTML: %v", err)
	}
	return path
}

// writeConvertJSONL writes a JSONL file and returns its path.
func writeConvertJSONL(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "source.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeConvertJSONL: %v", err)
	}
	return path
}

// --- Group: cmdConvert ---

// TestCmdConvert_HTML_Stdout verifies that convert --adapter html writes text to stdout.
func TestCmdConvert_HTML_Stdout(t *testing.T) {
	src := writeConvertHTML(t, `<html><body><h1>Incident</h1><p>Service failed.</p></body></html>`)

	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--adapter", "html",
		"--source-doc", src,
	})
	if err != nil {
		t.Fatalf("cmdConvert() want no error, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Incident") {
		t.Errorf("stdout: want 'Incident' in output, got: %q", out)
	}
	if !strings.Contains(out, "Service failed") {
		t.Errorf("stdout: want 'Service failed' in output, got: %q", out)
	}
}

// TestCmdConvert_HTML_ToFile verifies that --output writes text to a file.
func TestCmdConvert_HTML_ToFile(t *testing.T) {
	src := writeConvertHTML(t, `<html><body><p>Root cause: timeout.</p></body></html>`)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out.txt")

	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--adapter", "html",
		"--source-doc", src,
		"--output", outputPath,
	})
	if err != nil {
		t.Fatalf("cmdConvert() with --output: want no error, got: %v", err)
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("output file not written: %v", readErr)
	}
	if !strings.Contains(string(data), "Root cause") {
		t.Errorf("output file: want 'Root cause' in content, got: %q", string(data))
	}
}

// TestCmdConvert_JSONLog_Stdout verifies that convert --adapter jsonlog works on stdout.
func TestCmdConvert_JSONLog_Stdout(t *testing.T) {
	src := writeConvertJSONL(t, `{"message":"service started","level":"INFO"}`)

	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--adapter", "jsonlog",
		"--source-doc", src,
	})
	if err != nil {
		t.Fatalf("cmdConvert() jsonlog want no error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "service started") {
		t.Errorf("stdout: want 'service started', got: %q", buf.String())
	}
}

// TestCmdConvert_UnknownAdapter verifies that an unknown adapter name returns an error.
func TestCmdConvert_UnknownAdapter(t *testing.T) {
	src := writeConvertHTML(t, `<html></html>`)
	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--adapter", "nosuchformat",
		"--source-doc", src,
	})
	if err == nil {
		t.Fatal("cmdConvert() with unknown adapter: want error, got nil")
	}
	if !strings.Contains(err.Error(), "nosuchformat") {
		t.Errorf("error %q: want adapter name in message", err.Error())
	}
}

// TestCmdConvert_MissingSourceDoc verifies that --source-doc is required.
func TestCmdConvert_MissingSourceDoc(t *testing.T) {
	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--adapter", "html",
	})
	if err == nil {
		t.Fatal("cmdConvert() without --source-doc: want error, got nil")
	}
}

// TestCmdConvert_MissingAdapter verifies that --adapter is required.
func TestCmdConvert_MissingAdapter(t *testing.T) {
	src := writeConvertHTML(t, `<html></html>`)
	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--source-doc", src,
	})
	if err == nil {
		t.Fatal("cmdConvert() without --adapter: want error, got nil")
	}
}

// TestCmdConvert_SourceDocNotFound verifies that convert returns an error when
// the --source-doc path does not exist (flag provided, file missing).
// This is distinct from missing-flag tests and covers the Convert() error branch.
func TestCmdConvert_SourceDocNotFound(t *testing.T) {
	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--adapter", "html",
		"--source-doc", "/no/such/file.html",
	})
	if err == nil {
		t.Fatal("cmdConvert() with missing source file: want error, got nil")
	}
}

// TestCmdConvert_PDF_Stdout verifies that the pdf adapter path works through the
// CLI layer (adapter lookup → Convert → write to stdout). Conversion correctness
// is covered by the adapter unit tests; this test confirms CLI wiring.
func TestCmdConvert_PDF_Stdout(t *testing.T) {
	// Use the committed sample.pdf fixture from the adapter testdata directory.
	src := filepath.Join("..", "..", "adapter", "testdata", "sample.pdf")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("adapter testdata/sample.pdf not found: %v", err)
	}

	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--adapter", "pdf",
		"--source-doc", src,
	})
	if err != nil {
		t.Fatalf("cmdConvert() with pdf adapter: want no error, got: %v", err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("stdout: want non-empty output from PDF conversion")
	}
}

// TestCmdConvert_AdapterNameInOutput verifies that the output includes a
// confirmation line naming the adapter that was used.
func TestCmdConvert_AdapterNameInOutput(t *testing.T) {
	src := writeConvertHTML(t, `<html><body><p>Test.</p></body></html>`)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out.txt")

	var buf bytes.Buffer
	err := cmdConvert(&buf, []string{
		"--adapter", "html",
		"--source-doc", src,
		"--output", outputPath,
	})
	if err != nil {
		t.Fatalf("cmdConvert() want no error, got: %v", err)
	}
	// When writing to file, confirmation line should mention the adapter and output path.
	out := buf.String()
	if !strings.Contains(out, "html-extractor") {
		t.Errorf("stdout: want adapter name 'html-extractor' in confirmation, got: %q", out)
	}
}
