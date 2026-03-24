// jsonlog_test.go tests JSONLogAdapter in black-box style.
package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/adapter"
)

// sampleJSONL returns the path to the committed test JSONL file.
func sampleJSONL(t *testing.T) string {
	t.Helper()
	p := filepath.Join("testdata", "sample.jsonl")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("testdata/sample.jsonl not found: %v", err)
	}
	return p
}

// sampleLog returns the path to the committed plain-text log file.
func sampleLog(t *testing.T) string {
	t.Helper()
	p := filepath.Join("testdata", "sample.log")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("testdata/sample.log not found: %v", err)
	}
	return p
}

// --- Group: JSONLogAdapter ---

// TestJSONLogAdapter_HappyPath verifies that a JSONL file produces non-empty text.
func TestJSONLogAdapter_HappyPath(t *testing.T) {
	a := mustAdapter(t, "jsonlog")
	result, err := a.Convert(sampleJSONL(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if strings.TrimSpace(result.Text) == "" {
		t.Error("Convert() result.Text: want non-empty, got empty")
	}
}

// TestJSONLogAdapter_JSONLContent verifies that key content from JSON lines appears in output.
func TestJSONLogAdapter_JSONLContent(t *testing.T) {
	a := mustAdapter(t, "jsonlog")
	result, err := a.Convert(sampleJSONL(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	wantPhrases := []string{"database connection exhausted", "restarting connection pool", "service recovered"}
	for _, phrase := range wantPhrases {
		if !strings.Contains(result.Text, phrase) {
			t.Errorf("Text: want %q present, not found in:\n%s", phrase, result.Text)
		}
	}
}

// TestJSONLogAdapter_PlainLogLines verifies that non-JSON lines are passed through verbatim.
func TestJSONLogAdapter_PlainLogLines(t *testing.T) {
	a := mustAdapter(t, "jsonlog")
	result, err := a.Convert(sampleLog(t))
	if err != nil {
		t.Fatalf("Convert() on plain log: want no error, got: %v", err)
	}
	if !strings.Contains(result.Text, "database connection exhausted") {
		t.Errorf("Text: plain log lines must appear verbatim, got:\n%s", result.Text)
	}
}

// TestJSONLogAdapter_AdapterName verifies that ConvertResult.AdapterName is "jsonlog-parser".
func TestJSONLogAdapter_AdapterName(t *testing.T) {
	a := mustAdapter(t, "jsonlog")
	result, err := a.Convert(sampleJSONL(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if result.AdapterName != "jsonlog-parser" {
		t.Errorf("AdapterName: want %q, got %q", "jsonlog-parser", result.AdapterName)
	}
}

// TestJSONLogAdapter_MetadataLineCount verifies that line_count metadata key is set.
func TestJSONLogAdapter_MetadataLineCount(t *testing.T) {
	a := mustAdapter(t, "jsonlog")
	result, err := a.Convert(sampleJSONL(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if result.Metadata["line_count"] == "" {
		t.Error("Metadata[line_count]: want non-empty, got empty")
	}
}

// TestJSONLogAdapter_EmptyFile verifies that an empty file produces empty text with no error.
func TestJSONLogAdapter_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "jsonlog")
	result, err := a.Convert(path)
	if err != nil {
		t.Fatalf("Convert() on empty file: want no error, got: %v", err)
	}
	if strings.TrimSpace(result.Text) != "" {
		t.Errorf("Text: want empty for empty file, got: %q", result.Text)
	}
}

// TestJSONLogAdapter_MixedContent verifies that mixed JSON and plain lines are both included.
func TestJSONLogAdapter_MixedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.jsonl")
	content := `{"message":"json line one"}
plain text line two
{"message":"json line three"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "jsonlog")
	result, err := a.Convert(path)
	if err != nil {
		t.Fatalf("Convert() on mixed content: want no error, got: %v", err)
	}
	if !strings.Contains(result.Text, "json line one") {
		t.Error("Text: JSON message must appear")
	}
	if !strings.Contains(result.Text, "plain text line two") {
		t.Error("Text: plain line must appear verbatim")
	}
	if !strings.Contains(result.Text, "json line three") {
		t.Error("Text: second JSON message must appear")
	}
}

// TestJSONLogAdapter_NoMessageField verifies that a JSON object without a "message" key
// is rendered using the first sorted key=value pair as the lead. This covers the branch
// in formatLogLine where no "message" field is present.
func TestJSONLogAdapter_NoMessageField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no_msg.jsonl")
	content := `{"level":"ERROR","code":"500"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "jsonlog")
	result, err := a.Convert(path)
	if err != nil {
		t.Fatalf("Convert() on no-message JSON: want no error, got: %v", err)
	}
	// The formatted output should contain the key=value pairs.
	// With no "message" field, all fields are rendered as key=value pairs.
	if !strings.Contains(result.Text, "code=500") && !strings.Contains(result.Text, "level=ERROR") {
		t.Errorf("Text: want key=value pairs in output, got: %q", result.Text)
	}
}

// TestJSONLogAdapter_FileNotFound verifies that a missing file returns an error.
func TestJSONLogAdapter_FileNotFound(t *testing.T) {
	a := mustAdapter(t, "jsonlog")
	_, err := a.Convert("/no/such/file.jsonl")
	if err == nil {
		t.Fatal("Convert() with missing file: want error, got nil")
	}
}

// TestJSONLogAdapter_OversizedFile verifies that a file exceeding the raw size cap returns an error.
func TestJSONLogAdapter_OversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.jsonl")
	big := make([]byte, 11*1024*1024)
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "jsonlog")
	_, err := a.Convert(path)
	if err == nil {
		t.Fatal("Convert() with oversized file: want error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q: want 'exceeds' in message", err.Error())
	}
}

// TestForName_JSONLog verifies that ForName("jsonlog") returns a non-nil adapter.
func TestForName_JSONLog(t *testing.T) {
	a, err := adapter.ForName("jsonlog")
	if err != nil {
		t.Fatalf("ForName(%q) want no error, got: %v", "jsonlog", err)
	}
	if a == nil {
		t.Fatal("ForName returned nil adapter")
	}
}

// TestForName_Unknown verifies that ForName with an unknown name returns an error.
func TestForName_Unknown(t *testing.T) {
	_, err := adapter.ForName("nosuchformat")
	if err == nil {
		t.Fatal("ForName(nosuchformat) with unknown name: want error, got nil")
	}
	if !strings.Contains(err.Error(), "nosuchformat") {
		t.Errorf("error %q: want adapter name in message", err.Error())
	}
}
