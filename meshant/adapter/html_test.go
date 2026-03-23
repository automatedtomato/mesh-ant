// html_test.go tests HTMLAdapter in black-box style.
package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/adapter"
)

// sampleHTML returns the path to the committed test HTML file.
func sampleHTML(t *testing.T) string {
	t.Helper()
	p := filepath.Join("testdata", "sample.html")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("testdata/sample.html not found: %v", err)
	}
	return p
}

// mustAdapter is a test helper that calls ForName and fatals on error.
func mustAdapter(t *testing.T, name string) adapter.Adapter {
	t.Helper()
	a, err := adapter.ForName(name)
	if err != nil {
		t.Fatalf("ForName(%q): %v", name, err)
	}
	return a
}

// --- Group: HTMLAdapter ---

// TestHTMLAdapter_HappyPath verifies that a valid HTML file produces non-empty text.
func TestHTMLAdapter_HappyPath(t *testing.T) {
	a := mustAdapter(t, "html")
	result, err := a.Convert(sampleHTML(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if strings.TrimSpace(result.Text) == "" {
		t.Error("Convert() result.Text: want non-empty, got empty")
	}
}

// TestHTMLAdapter_TextContent verifies that visible text from the HTML body is present.
func TestHTMLAdapter_TextContent(t *testing.T) {
	a := mustAdapter(t, "html")
	result, err := a.Convert(sampleHTML(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	wantPhrases := []string{"Incident Report", "09:00", "database connection"}
	for _, phrase := range wantPhrases {
		if !strings.Contains(result.Text, phrase) {
			t.Errorf("Text: want %q present, not found in:\n%s", phrase, result.Text)
		}
	}
}

// TestHTMLAdapter_ScriptExcluded verifies that <script> content does not appear in output.
func TestHTMLAdapter_ScriptExcluded(t *testing.T) {
	a := mustAdapter(t, "html")
	result, err := a.Convert(sampleHTML(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if strings.Contains(result.Text, "alert(") {
		t.Error("Text: script content must be excluded, found 'alert('")
	}
}

// TestHTMLAdapter_StyleExcluded verifies that <style> content does not appear in output.
func TestHTMLAdapter_StyleExcluded(t *testing.T) {
	a := mustAdapter(t, "html")
	result, err := a.Convert(sampleHTML(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if strings.Contains(result.Text, "color: red") {
		t.Error("Text: style content must be excluded, found 'color: red'")
	}
}

// TestHTMLAdapter_AdapterName verifies that ConvertResult.AdapterName is "html-extractor".
func TestHTMLAdapter_AdapterName(t *testing.T) {
	a := mustAdapter(t, "html")
	result, err := a.Convert(sampleHTML(t))
	if err != nil {
		t.Fatalf("Convert() want no error, got: %v", err)
	}
	if result.AdapterName != "html-extractor" {
		t.Errorf("AdapterName: want %q, got %q", "html-extractor", result.AdapterName)
	}
}

// TestHTMLAdapter_FileNotFound verifies that a missing file returns an error.
func TestHTMLAdapter_FileNotFound(t *testing.T) {
	a := mustAdapter(t, "html")
	_, err := a.Convert("/no/such/file.html")
	if err == nil {
		t.Fatal("Convert() with missing file: want error, got nil")
	}
}

// TestHTMLAdapter_OversizedFile verifies that a file exceeding the raw size cap returns an error.
func TestHTMLAdapter_OversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.html")
	big := make([]byte, 11*1024*1024)
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "html")
	_, err := a.Convert(path)
	if err == nil {
		t.Fatal("Convert() with oversized file: want error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q: want 'exceeds' in message", err.Error())
	}
}

// TestHTMLAdapter_NoTagsInOutput verifies that HTML tags are stripped from output.
func TestHTMLAdapter_NoTagsInOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tagged.html")
	content := `<html><body><p>Hello <b>world</b></p></body></html>`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "html")
	result, err := a.Convert(path)
	if err != nil {
		t.Fatalf("Convert(): %v", err)
	}
	if strings.Contains(result.Text, "<") || strings.Contains(result.Text, ">") {
		t.Errorf("Text: HTML tags must be stripped, got: %q", result.Text)
	}
	if !strings.Contains(result.Text, "Hello") || !strings.Contains(result.Text, "world") {
		t.Errorf("Text: visible content must be preserved, got: %q", result.Text)
	}
}

// TestHTMLAdapter_EmptyFile verifies that an empty HTML file produces empty text with no error.
// This mirrors TestJSONLogAdapter_EmptyFile for consistency across adapters.
func TestHTMLAdapter_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.html")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	a := mustAdapter(t, "html")
	result, err := a.Convert(path)
	if err != nil {
		t.Fatalf("Convert() on empty file: want no error, got: %v", err)
	}
	if strings.TrimSpace(result.Text) != "" {
		t.Errorf("Text: want empty for empty file, got: %q", result.Text)
	}
}

// TestForName_HTML verifies that ForName("html") returns a non-nil adapter.
func TestForName_HTML(t *testing.T) {
	a, err := adapter.ForName("html")
	if err != nil {
		t.Fatalf("ForName(%q) want no error, got: %v", "html", err)
	}
	if a == nil {
		t.Fatal("ForName(html) returned nil adapter")
	}
}
