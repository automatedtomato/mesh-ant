// html.go implements HTMLAdapter — HTML to plain text conversion.
//
// Uses golang.org/x/net/html tokenizer (iterative, not DOM-recursive) to walk
// the token stream and collect visible text nodes, skipping <script>, <style>,
// and <noscript> content. Block-level elements (p, div, h1-h6, li, tr, br)
// emit a newline to preserve readable paragraph structure.
//
// The adapter is a named mediator: its name ("html-extractor") travels with the
// ConvertResult so downstream provenance records which transformation was applied.
package adapter

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/net/html"
)

// blockElements is the set of HTML element names that should produce a newline
// in the extracted text. Used to preserve paragraph structure without rendering.
var blockElements = map[string]bool{
	"p": true, "div": true, "br": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"li": true, "tr": true, "td": true, "th": true,
	"article": true, "section": true, "header": true, "footer": true,
	"blockquote": true, "pre": true,
}

// skipElements is the set of element names whose entire subtree should be skipped.
// Content inside these elements is not user-visible in a browser.
var skipElements = map[string]bool{
	"script":   true,
	"style":    true,
	"noscript": true,
}

// HTMLAdapter converts an HTML file to plain text.
// It is a mediator: the conversion is named ("html-extractor") and the
// transformation (tag stripping, script/style exclusion) is visible in provenance.
type HTMLAdapter struct{}

// Convert reads the HTML at path and returns extracted plain text.
// Script, style, and noscript subtrees are excluded. Block-level elements
// produce newlines to preserve readable structure. HTML tags are stripped.
// Returns an error if the file is missing or exceeds the size cap.
func (a *HTMLAdapter) Convert(path string) (ConvertResult, error) {
	// Enforce raw size cap before parsing.
	info, err := os.Stat(path)
	if err != nil {
		return ConvertResult{}, fmt.Errorf("html-extractor: stat %q: %w", path, err)
	}
	if info.Size() > maxRawBytes {
		return ConvertResult{}, fmt.Errorf("html-extractor: %q exceeds %d bytes raw size limit", path, maxRawBytes)
	}

	f, err := os.Open(path)
	if err != nil {
		return ConvertResult{}, fmt.Errorf("html-extractor: open %q: %w", path, err)
	}
	defer f.Close()

	text, err := extractHTMLText(f)
	if err != nil {
		return ConvertResult{}, fmt.Errorf("html-extractor: parse %q: %w", path, err)
	}

	return ConvertResult{
		Text:        text,
		AdapterName: "html-extractor",
		Metadata:    map[string]string{},
	}, nil
}

// extractHTMLText walks the HTML token stream and collects visible text.
// Script, style, and noscript subtrees are skipped. Block elements emit newlines.
// Uses the tokenizer directly (not a DOM tree) to keep memory bounded.
func extractHTMLText(r io.Reader) (string, error) {
	var sb strings.Builder
	z := html.NewTokenizer(r)
	skipDepth := 0 // nesting depth inside a skip element

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return normaliseWhitespace(sb.String()), nil
			}
			return "", z.Err()

		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if skipDepth > 0 {
				if tt == html.StartTagToken {
					skipDepth++
				}
				continue
			}
			if skipElements[tag] {
				if tt == html.StartTagToken {
					skipDepth = 1
				}
				continue
			}
			if blockElements[tag] {
				sb.WriteString("\n")
			}

		case html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if skipDepth > 0 {
				skipDepth--
				continue
			}
			if blockElements[tag] {
				sb.WriteString("\n")
			}

		case html.TextToken:
			if skipDepth > 0 {
				continue
			}
			sb.Write(z.Text())
		}
	}
}

// normaliseWhitespace collapses runs of whitespace (preserving single newlines
// as paragraph separators) and trims leading/trailing space.
func normaliseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, "\n")
}
