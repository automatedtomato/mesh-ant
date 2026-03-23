// jsonlog.go implements JSONLogAdapter — JSON lines / plain log to text conversion.
//
// Reads the source file line by line with bufio.Scanner. Each line is tried as JSON
// (json.Unmarshal into map[string]any). Valid JSON lines are rendered as a
// human-readable "message (key=value, ...)" string so the LLM sees structured
// context. Non-JSON lines are passed through verbatim — plain log lines are
// not discarded.
//
// The adapter is a named mediator: its name ("jsonlog-parser") travels with the
// ConvertResult so downstream provenance records which transformation was applied.
// Metadata carries line_count so the analyst knows the scope of what was parsed.
//
// Note: this adapter does not attempt to understand log schemas. It surfaces
// all JSON fields as key=value pairs. Analysts wanting schema-specific extraction
// (e.g., only "message" and "error" fields) can filter the LLM prompt instead.
package adapter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// JSONLogAdapter converts JSON lines or plain log files to plain text.
// It is a mediator: the conversion is named ("jsonlog-parser") and is
// visible in session provenance.
type JSONLogAdapter struct{}

// Convert reads the file at path line by line and returns a human-readable
// text representation. JSON lines are expanded to key=value format; non-JSON
// lines are passed through verbatim. Empty files produce empty text with no error.
// Returns an error if the file is missing or exceeds the raw size cap.
func (a *JSONLogAdapter) Convert(path string) (ConvertResult, error) {
	// Enforce raw size cap before reading.
	info, err := os.Stat(path)
	if err != nil {
		return ConvertResult{}, fmt.Errorf("jsonlog-parser: stat %q: %w", path, err)
	}
	if info.Size() > maxRawBytes {
		return ConvertResult{}, fmt.Errorf("jsonlog-parser: %q exceeds %d bytes raw size limit", path, maxRawBytes)
	}

	f, err := os.Open(path)
	if err != nil {
		return ConvertResult{}, fmt.Errorf("jsonlog-parser: open %q: %w", path, err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, formatLogLine(line))
	}
	if err := scanner.Err(); err != nil {
		return ConvertResult{}, fmt.Errorf("jsonlog-parser: scan %q: %w", path, err)
	}

	return ConvertResult{
		Text:        strings.Join(lines, "\n"),
		AdapterName: "jsonlog-parser",
		Metadata: map[string]string{
			"line_count": strconv.Itoa(len(lines)),
		},
	}, nil
}

// formatLogLine renders one source line as human-readable text.
// If line is valid JSON (an object), renders as "message (key=value, ...)".
// Otherwise the line is returned verbatim.
func formatLogLine(line string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		// Not JSON — pass through as-is.
		return line
	}

	// Extract "message" field first for prominence, then all remaining fields sorted.
	msg, _ := obj["message"].(string)

	// Collect non-message fields as key=value pairs in sorted order for stability.
	var kvs []string
	for k, v := range obj {
		if k == "message" {
			continue
		}
		kvs = append(kvs, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(kvs)

	if msg == "" && len(kvs) == 0 {
		// Empty or unrecognisable JSON object — return the raw line.
		return line
	}

	var sb strings.Builder
	if msg != "" {
		sb.WriteString(msg)
	} else {
		// No message field — lead with the first sorted key=value pair.
		sb.WriteString(kvs[0])
		kvs = kvs[1:]
	}
	if len(kvs) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(kvs, ", "))
		sb.WriteString(")")
	}
	return sb.String()
}
