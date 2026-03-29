// cmd_split_test.go tests the cmdSplit subcommand (white-box, package main).
//
// Tests inject a mock LLMClient to avoid real network calls. Each test verifies
// one observable behaviour: flag validation, output JSON, session record
// persistence, error propagation, and help flag handling.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
)

// splitMockClient is a minimal llm.LLMClient test double for cmdSplit tests.
// It returns a canned response or error on each call to Complete.
type splitMockClient struct {
	response string
	err      error
}

func (m *splitMockClient) Complete(_ context.Context, _, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

// writeSplitSourceDoc writes content to a temp file and returns its path.
func writeSplitSourceDoc(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "source.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeSplitSourceDoc: %v", err)
	}
	return path
}

// writeSplitPromptTemplate writes a minimal prompt template and returns its path.
func writeSplitPromptTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "split_prompt.md")
	if err := os.WriteFile(path, []byte("Split into spans."), 0o644); err != nil {
		t.Fatalf("writeSplitPromptTemplate: %v", err)
	}
	return path
}

// threeSpansResponse is a canned LLM response yielding 3 observation spans.
const threeSpansResponse = `["First span: system was healthy.", "Second span: alert fired.", "Third span: incident resolved."]`

// --- Group: cmdSplit ---

// TestCmdSplit_missingSourceDoc verifies that cmdSplit returns an error
// containing "required" when --source-doc is not provided.
func TestCmdSplit_missingSourceDoc(t *testing.T) {
	var buf bytes.Buffer
	client := &splitMockClient{response: threeSpansResponse}
	err := cmdSplit(&buf, client, []string{})
	if err == nil {
		t.Fatal("cmdSplit() with no --source-doc: want error, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention 'required': %v", err)
	}
}

// TestCmdSplit_success verifies that cmdSplit writes a JSON array of spans to
// the output file, prints a summary line containing span count, and writes a
// valid SessionRecord to the session file.
func TestCmdSplit_success(t *testing.T) {
	src := writeSplitSourceDoc(t, "A multi-paragraph document about an incident.")
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "spans.json")
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &splitMockClient{response: threeSpansResponse}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdSplit() returned unexpected error: %v", err)
	}

	// Output file must contain a JSON array of 3 spans.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	var spans []string
	if err := json.Unmarshal(data, &spans); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
	if len(spans) != 3 {
		t.Errorf("want 3 spans in output, got %d", len(spans))
	}

	// Session file must exist and have Command = "split".
	sdata, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("session file not created: %v", err)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session file is not valid JSON: %v", err)
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty")
	}
	if rec.Command != "split" {
		t.Errorf("SessionRecord.Command: want %q, got %q", "split", rec.Command)
	}
	if rec.DraftCount != 3 {
		t.Errorf("SessionRecord.DraftCount: want 3, got %d", rec.DraftCount)
	}
	if rec.ErrorNote != "" {
		t.Errorf("SessionRecord.ErrorNote should be empty on success, got %q", rec.ErrorNote)
	}

	// Summary line on stdout must mention span count.
	out := buf.String()
	if !strings.Contains(out, "3 candidate observation spans") {
		t.Errorf("stdout summary should mention '3 candidate observation spans'; got: %q", out)
	}
}

// TestCmdSplit_llmError verifies that when the LLM client returns an error,
// cmdSplit returns an error and still writes the session record with ErrorNote.
func TestCmdSplit_llmError(t *testing.T) {
	src := writeSplitSourceDoc(t, "Document content.")
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &splitMockClient{err: errors.New("network timeout")}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err == nil {
		t.Fatal("cmdSplit() with LLM error: want error, got nil")
	}

	// Session record must be written even on LLM error.
	sdata, readErr := os.ReadFile(sessionPath)
	if readErr != nil {
		t.Fatalf("session file must exist even on LLM error: %v", readErr)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session file is not valid JSON: %v", err)
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on LLM error")
	}
}

// TestCmdSplit_refusal verifies that an LLM refusal response causes cmdSplit
// to return an ErrLLMRefusal-wrapped error.
func TestCmdSplit_refusal(t *testing.T) {
	src := writeSplitSourceDoc(t, "Document content.")
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &splitMockClient{response: "I cannot assist with this request."}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err == nil {
		t.Fatal("cmdSplit() with refusal: want ErrLLMRefusal, got nil")
	}
	var refusal *llm.ErrLLMRefusal
	if !errors.As(err, &refusal) {
		t.Errorf("want ErrLLMRefusal in error chain, got %T: %v", err, err)
	}
}

// TestCmdSplit_outputFile verifies that when --output is provided, spans are
// written to the named file (not to w/stdout).
func TestCmdSplit_outputFile(t *testing.T) {
	src := writeSplitSourceDoc(t, "Document content.")
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out_spans.json")
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &splitMockClient{response: `["spanA","spanB"]`}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdSplit() returned error: %v", err)
	}

	// File must exist with 2 spans.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	var spans []string
	if err := json.Unmarshal(data, &spans); err != nil {
		t.Fatalf("output file not valid JSON: %v", err)
	}
	if len(spans) != 2 {
		t.Errorf("want 2 spans, got %d", len(spans))
	}

	// w should mention the output path (confirmOutput).
	if !strings.Contains(buf.String(), outputPath) {
		t.Errorf("stdout should mention output path; got: %q", buf.String())
	}
}

// TestCmdSplit_sessionOutputDefault verifies that when --session-output is
// omitted and --output is provided, the session file defaults to
// <output>.session.json.
func TestCmdSplit_sessionOutputDefault(t *testing.T) {
	src := writeSplitSourceDoc(t, "Document content.")
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "spans.json")

	var buf bytes.Buffer
	client := &splitMockClient{response: threeSpansResponse}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--output", outputPath,
	})
	if err != nil {
		t.Fatalf("cmdSplit() returned error: %v", err)
	}

	expectedSessionPath := outputPath + ".session.json"
	if _, err := os.Stat(expectedSessionPath); err != nil {
		t.Errorf("default session file %q should exist, got: %v", expectedSessionPath, err)
	}
}

// TestCmdSplit_sessionOutputExplicit verifies that when --session-output is
// provided, the session record is written to exactly that path.
func TestCmdSplit_sessionOutputExplicit(t *testing.T) {
	src := writeSplitSourceDoc(t, "Document content.")
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "my_session.json")

	var buf bytes.Buffer
	client := &splitMockClient{response: threeSpansResponse}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdSplit() returned error: %v", err)
	}

	if _, err := os.Stat(sessionPath); err != nil {
		t.Errorf("explicit session file %q should exist, got: %v", sessionPath, err)
	}

	// Stdout must mention the session-output path.
	if !strings.Contains(buf.String(), sessionPath) {
		t.Errorf("stdout should mention session path; got: %q", buf.String())
	}
}

// countingClient wraps splitMockClient and counts calls to Complete.
type countingClient struct {
	inner *splitMockClient
	calls int
}

func (c *countingClient) Complete(ctx context.Context, sys, user string) (string, error) {
	c.calls++
	return c.inner.Complete(ctx, sys, user)
}

// TestCmdSplit_helpFlag verifies that --help returns flag.ErrHelp and does not
// invoke the LLM client.
func TestCmdSplit_helpFlag(t *testing.T) {
	inner := &splitMockClient{response: threeSpansResponse}
	client := &countingClient{inner: inner}

	var buf bytes.Buffer
	err := cmdSplit(&buf, client, []string{"--help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("--help should return flag.ErrHelp, got %v", err)
	}
	if client.calls != 0 {
		t.Errorf("LLM client must not be called for --help, got %d calls", client.calls)
	}
}

// TestRun_Split_dispatch verifies that run() correctly dispatches "split" to
// cmdSplit. We pass --help so no LLM or file I/O is required.
func TestRun_Split_dispatch(t *testing.T) {
	var buf bytes.Buffer
	// run() with "split" and "--help" should not return an "unknown command" error.
	err := run(strings.NewReader(""), &buf, []string{"split", "--help"})
	// flag.ErrHelp is acceptable; "unknown command" is not.
	if err != nil && strings.Contains(err.Error(), "unknown command") {
		t.Errorf("run([\"split\", ...]) should dispatch to cmdSplit, not return unknown command: %v", err)
	}
}

// TestCmdSplit_fileNotFound verifies that cmdSplit returns an error when
// --source-doc points to a nonexistent file.
func TestCmdSplit_fileNotFound(t *testing.T) {
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &splitMockClient{response: threeSpansResponse}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", "/nonexistent/path/source.md",
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err == nil {
		t.Fatal("cmdSplit() with nonexistent --source-doc: want error, got nil")
	}
}

// TestCmdSplit_malformedOutput verifies that when the LLM returns unparseable
// JSON, cmdSplit returns an error and still writes a session record with ErrorNote.
func TestCmdSplit_malformedOutput(t *testing.T) {
	src := writeSplitSourceDoc(t, "Document content.")
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &splitMockClient{response: "not a json array at all"}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err == nil {
		t.Fatal("cmdSplit() with malformed LLM output: want error, got nil")
	}

	// Session record must be written even when LLM output is malformed.
	sdata, readErr := os.ReadFile(sessionPath)
	if readErr != nil {
		t.Fatalf("session file must exist even on malformed output: %v", readErr)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session file is not valid JSON: %v", err)
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on malformed LLM output")
	}
}

// TestCmdSplit_stdoutOutput verifies that when --output is omitted, the JSON
// array of spans is written to w (stdout) as valid parseable JSON.
func TestCmdSplit_stdoutOutput(t *testing.T) {
	src := writeSplitSourceDoc(t, "A source document.")
	prompt := writeSplitPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &splitMockClient{response: threeSpansResponse}
	err := cmdSplit(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdSplit() returned error: %v", err)
	}

	// buf contains both the JSON output and the summary line; extract the JSON array.
	out := buf.String()
	jsonStart := strings.Index(out, "[")
	if jsonStart < 0 {
		t.Fatalf("stdout should contain JSON array, got: %q", out)
	}
	// The JSON array ends at the matching ']'.
	jsonEnd := strings.LastIndex(out, "]")
	if jsonEnd < jsonStart {
		t.Fatalf("no closing ']' found in stdout: %q", out)
	}
	jsonPart := out[jsonStart : jsonEnd+1]
	var spans []string
	if err := json.Unmarshal([]byte(jsonPart), &spans); err != nil {
		t.Fatalf("stdout JSON is not a valid string array: %v\noutput: %q", err, out)
	}
	if len(spans) != 3 {
		t.Errorf("want 3 spans in stdout JSON, got %d", len(spans))
	}
}
