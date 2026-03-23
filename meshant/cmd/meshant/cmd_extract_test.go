package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// extractMockClient is a minimal llm.LLMClient test double for cmdExtract
// tests. It returns a canned response or error on each call.
// When responses is non-empty, calls are served in order (indexed); once
// exhausted, the fallback response/err fields are used.
type extractMockClient struct {
	response  string
	err       error
	responses []string // indexed: each call advances the counter
	calls     int
}

func (m *extractMockClient) Complete(_ context.Context, _, _ string) (string, error) {
	if len(m.responses) > 0 {
		i := m.calls
		m.calls++
		if i < len(m.responses) {
			return m.responses[i], nil
		}
		// Fall through to fallback after exhausting indexed responses.
	}
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

// writeExtractSourceDoc writes content to a temp file and returns its path.
func writeExtractSourceDoc(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "source.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeExtractSourceDoc: %v", err)
	}
	return path
}

// writeExtractPromptTemplate writes a minimal prompt template and returns its path.
func writeExtractPromptTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(path, []byte("Extract trace drafts."), 0o644); err != nil {
		t.Fatalf("writeExtractPromptTemplate: %v", err)
	}
	return path
}

// twoValidDrafts is a JSON response returning two well-formed TraceDraft records.
const twoValidDrafts = `[
  {"source_span": "The service went down at 09:00.", "what_changed": "service failure"},
  {"source_span": "Service restored at 09:45.", "what_changed": "service restored"}
]`

// --- Group: cmdExtract ---

// TestCmdExtract_MissingSourceDoc verifies that cmdExtract returns an error
// containing "required" when --source-doc is not provided.
func TestCmdExtract_MissingSourceDoc(t *testing.T) {
	var buf bytes.Buffer
	client := &extractMockClient{response: twoValidDrafts}
	err := cmdExtract(&buf, client, []string{})
	if err == nil {
		t.Fatal("cmdExtract() with no --source-doc: want error, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention required: %v", err)
	}
}

// TestCmdExtract_HappyPath verifies that cmdExtract writes a TraceDraft JSON
// array and a SessionRecord JSON file when the LLM returns valid output.
func TestCmdExtract_HappyPath(t *testing.T) {
	src := writeExtractSourceDoc(t, "The service went down at 09:00. Service restored at 09:45.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &extractMockClient{response: twoValidDrafts}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() returned unexpected error: %v", err)
	}

	// Drafts file must exist and parse to 2 records.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
	if len(drafts) != 2 {
		t.Errorf("want 2 drafts, got %d", len(drafts))
	}

	// Session file must exist and have non-empty ID.
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
	if rec.DraftCount != 2 {
		t.Errorf("SessionRecord.DraftCount: want 2, got %d", rec.DraftCount)
	}
	if rec.ErrorNote != "" {
		t.Errorf("SessionRecord.ErrorNote should be empty on success, got %q", rec.ErrorNote)
	}

	// w output should confirm the file was written.
	out := buf.String()
	if !strings.Contains(out, outputPath) {
		t.Errorf("stdout should mention output path; got: %q", out)
	}
}

// TestCmdExtract_EmptyResponse verifies that a "[]" LLM response produces an
// empty drafts file and a valid session record without error.
func TestCmdExtract_EmptyResponse(t *testing.T) {
	src := writeExtractSourceDoc(t, "Some text.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &extractMockClient{response: "[]"}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() with empty response: want no error, got: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("output JSON invalid: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts, got %d", len(drafts))
	}

	// Session file must still be written.
	if _, err := os.Stat(sessionPath); err != nil {
		t.Errorf("session file must be written even for empty response: %v", err)
	}
}

// TestCmdExtract_ClientError verifies that when the LLM client returns an
// error, cmdExtract returns an error and still writes the session record with
// a populated ErrorNote.
func TestCmdExtract_ClientError(t *testing.T) {
	src := writeExtractSourceDoc(t, "Some text.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &extractMockClient{err: errors.New("network timeout")}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err == nil {
		t.Fatal("cmdExtract() with client error: want error, got nil")
	}

	// Session record must be written even on error.
	sdata, readErr := os.ReadFile(sessionPath)
	if readErr != nil {
		t.Fatalf("session file must exist even on error: %v", readErr)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session file is not valid JSON: %v", err)
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on client error")
	}
}

// TestCmdExtract_Refusal verifies that an LLM refusal response produces an
// ErrLLMRefusal-typed error and still writes the session record with ErrorNote.
func TestCmdExtract_Refusal(t *testing.T) {
	src := writeExtractSourceDoc(t, "Some text.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &extractMockClient{response: "I cannot assist with this request."}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err == nil {
		t.Fatal("cmdExtract() with refusal: want error, got nil")
	}

	// The wrapped error chain must contain ErrLLMRefusal.
	var refusal *llm.ErrLLMRefusal
	if !errors.As(err, &refusal) {
		t.Errorf("want ErrLLMRefusal in error chain, got %T: %v", err, err)
	}

	// Session record must exist with ErrorNote.
	sdata, readErr := os.ReadFile(sessionPath)
	if readErr != nil {
		t.Fatalf("session file must exist on refusal: %v", readErr)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session JSON invalid: %v", err)
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on refusal")
	}
}

// TestCmdExtract_SessionOutputDefaulting_WithOutputFile verifies that when
// --output is provided and --session-output is omitted, the session file is
// written to <output>.session.json and contains valid JSON.
func TestCmdExtract_SessionOutputDefaulting_WithOutputFile(t *testing.T) {
	src := writeExtractSourceDoc(t, "Service failure at noon.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out.json")
	expectedSession := outputPath + ".session.json"

	var buf bytes.Buffer
	client := &extractMockClient{response: `[{"source_span":"Service failure at noon."}]`}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--output", outputPath,
		// --session-output intentionally omitted
	})
	if err != nil {
		t.Fatalf("cmdExtract() returned error: %v", err)
	}

	// Session file must exist and contain a valid SessionRecord.
	sdata, readErr := os.ReadFile(expectedSession)
	if readErr != nil {
		t.Fatalf("expected session file %q was not created: %v", expectedSession, readErr)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session file is not valid JSON: %v", err)
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty in defaulted session file")
	}
}

// TestCmdExtract_SessionOutputDefaulting_Stdout verifies that when neither
// --output nor --session-output is provided, a session_*.json file is written
// to the current working directory.
//
// NOTE: This test uses os.Chdir and is NOT parallel-safe. Do not add
// t.Parallel() to this test without changing the session-output defaulting
// implementation to avoid cwd-relative paths.
func TestCmdExtract_SessionOutputDefaulting_Stdout(t *testing.T) {
	src := writeExtractSourceDoc(t, "Cache expired.")
	prompt := writeExtractPromptTemplate(t)

	// Change to a temp dir so the cwd-relative session file lands there.
	// Restore the original cwd via t.Cleanup regardless of test outcome.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	var buf bytes.Buffer
	client := &extractMockClient{response: `[{"source_span":"Cache expired."}]`}
	err = cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		// --output and --session-output both omitted
	})
	if err != nil {
		t.Fatalf("cmdExtract() stdout mode returned error: %v", err)
	}

	// A session_*.json file must have been created in tmpDir.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var sessionFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "session_") && strings.HasSuffix(e.Name(), ".json") {
			sessionFile = filepath.Join(tmpDir, e.Name())
			break
		}
	}
	if sessionFile == "" {
		t.Fatal("no session_*.json file created in cwd for stdout mode")
	}
	sdata, _ := os.ReadFile(sessionFile)
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session JSON invalid: %v", err)
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must be non-empty")
	}
}

// TestCmdExtract_OutputToFile verifies that the TraceDraft JSON written to the
// output file has all framework provenance fields stamped by RunExtraction.
func TestCmdExtract_OutputToFile(t *testing.T) {
	src := writeExtractSourceDoc(t, "The deployment failed.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &extractMockClient{response: `[{"source_span":"The deployment failed.","observer":"ops-team"}]`}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--model", "claude-haiku-4-5",
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() returned error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("output JSON invalid: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	d := drafts[0]
	if d.ExtractedBy != "claude-haiku-4-5" {
		t.Errorf("ExtractedBy: want %q, got %q", "claude-haiku-4-5", d.ExtractedBy)
	}
	if d.ExtractionStage != "weak-draft" {
		t.Errorf("ExtractionStage: want %q, got %q", "weak-draft", d.ExtractionStage)
	}
	if d.SessionRef == "" {
		t.Error("SessionRef must be non-empty")
	}
	if !strings.Contains(d.UncertaintyNote, "LLM-produced candidate; unverified by human review") {
		t.Errorf("UncertaintyNote missing framework suffix: %q", d.UncertaintyNote)
	}

	// Session record must reference the same ID.
	sdata, _ := os.ReadFile(sessionPath)
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session JSON invalid: %v", err)
	}
	if d.SessionRef != rec.ID {
		t.Errorf("draft.SessionRef %q != SessionRecord.ID %q", d.SessionRef, rec.ID)
	}
}

// TestCmdExtract_SourceDocRef_DefaultsToPath verifies that when --source-doc-ref
// is omitted, the SourceDocRef on each draft defaults to the source doc path.
func TestCmdExtract_SourceDocRef_DefaultsToPath(t *testing.T) {
	src := writeExtractSourceDoc(t, "Network partition observed.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &extractMockClient{response: `[{"source_span":"Network partition observed."}]`}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() returned error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("output JSON invalid: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	if drafts[0].SourceDocRef != src {
		t.Errorf("SourceDocRef: want %q (source path), got %q", src, drafts[0].SourceDocRef)
	}
}

// writeMinimalCriterionFile writes a minimal valid criterion JSON and returns its path.
// A name-only criterion is the simplest valid form (no Declaration required for
// a transport handle, per graph.EquivalenceCriterion design notes).
func writeMinimalCriterionFile(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "criterion.json")
	content := `{"name":"` + name + `"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeMinimalCriterionFile: %v", err)
	}
	return path
}

// TestCmdExtract_CriterionFile_HappyPath verifies that passing --criterion-file
// propagates the criterion's Name into the session record's CriterionRef.
func TestCmdExtract_CriterionFile_HappyPath(t *testing.T) {
	src := writeExtractSourceDoc(t, "Load balancer failed.")
	prompt := writeExtractPromptTemplate(t)
	criterion := writeMinimalCriterionFile(t, "ops-incident-criterion")
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	var buf bytes.Buffer
	client := &extractMockClient{response: `[{"source_span":"Load balancer failed."}]`}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--criterion-file", criterion,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() with --criterion-file returned error: %v", err)
	}

	// CriterionRef is recorded in the session record conditions, not on drafts.
	sdata, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("session file not created: %v", err)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session JSON invalid: %v", err)
	}
	if rec.Conditions.CriterionRef != "ops-incident-criterion" {
		t.Errorf("Conditions.CriterionRef: want %q, got %q", "ops-incident-criterion", rec.Conditions.CriterionRef)
	}
}

// TestCmdExtract_CriterionFile_Missing verifies that passing a nonexistent path
// to --criterion-file returns an error containing "criterion-file".
func TestCmdExtract_CriterionFile_Missing(t *testing.T) {
	src := writeExtractSourceDoc(t, "Some text.")
	prompt := writeExtractPromptTemplate(t)

	var buf bytes.Buffer
	client := &extractMockClient{response: twoValidDrafts}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src,
		"--prompt-template", prompt,
		"--criterion-file", "/nonexistent/criterion.json",
	})
	if err == nil {
		t.Fatal("cmdExtract() with missing --criterion-file: want error, got nil")
	}
	if !strings.Contains(err.Error(), "criterion-file") {
		t.Errorf("error should mention criterion-file: %v", err)
	}
}

// --- Multi-document CLI tests ---

// TestCmdExtract_MultiDoc_HappyPath verifies that passing --source-doc twice
// produces a combined drafts file and a single session record.
func TestCmdExtract_MultiDoc_HappyPath(t *testing.T) {
	src0 := writeExtractSourceDoc(t, "First document content.")
	src1 := writeExtractSourceDoc(t, "Second document content.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	const doc0Draft = `[{"source_span": "First document content.", "what_changed": "c0"}]`
	const doc1Draft = `[{"source_span": "Second document content.", "what_changed": "c1"}]`

	var buf bytes.Buffer
	client := &extractMockClient{responses: []string{doc0Draft, doc1Draft}}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src0,
		"--source-doc", src1,
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() multi-doc returned unexpected error: %v", err)
	}

	// Drafts file must parse to 2 records.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
	if len(drafts) != 2 {
		t.Errorf("want 2 drafts, got %d", len(drafts))
	}

	// Session file must exist and reflect 2 drafts.
	sdata, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("session file not created: %v", err)
	}
	var rec llm.SessionRecord
	if err := json.Unmarshal(sdata, &rec); err != nil {
		t.Fatalf("session file is not valid JSON: %v", err)
	}
	if rec.DraftCount != 2 {
		t.Errorf("rec.DraftCount: want 2, got %d", rec.DraftCount)
	}
	if len(rec.InputPaths) != 2 {
		t.Errorf("rec.InputPaths: want 2 entries, got %d", len(rec.InputPaths))
	}
}

// TestCmdExtract_MultiDoc_WithRefs verifies that --source-doc-ref flags
// are matched to the corresponding --source-doc flags in order.
func TestCmdExtract_MultiDoc_WithRefs(t *testing.T) {
	src0 := writeExtractSourceDoc(t, "Doc A content.")
	src1 := writeExtractSourceDoc(t, "Doc B content.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	const docADraft = `[{"source_span": "Doc A content.", "what_changed": "cA"}]`
	const docBDraft = `[{"source_span": "Doc B content.", "what_changed": "cB"}]`

	var buf bytes.Buffer
	client := &extractMockClient{responses: []string{docADraft, docBDraft}}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src0,
		"--source-doc", src1,
		"--source-doc-ref", "RefA",
		"--source-doc-ref", "RefB",
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() multi-doc with refs returned error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("want 2 drafts, got %d", len(drafts))
	}
	if drafts[0].SourceDocRef != "RefA" {
		t.Errorf("draft[0].SourceDocRef: want %q, got %q", "RefA", drafts[0].SourceDocRef)
	}
	if drafts[1].SourceDocRef != "RefB" {
		t.Errorf("draft[1].SourceDocRef: want %q, got %q", "RefB", drafts[1].SourceDocRef)
	}
}

// TestCmdExtract_MultiDoc_MismatchedRefCount verifies that providing two
// --source-doc flags but only one --source-doc-ref flag returns an error.
func TestCmdExtract_MultiDoc_MismatchedRefCount(t *testing.T) {
	src0 := writeExtractSourceDoc(t, "Doc A content.")
	src1 := writeExtractSourceDoc(t, "Doc B content.")
	prompt := writeExtractPromptTemplate(t)

	var buf bytes.Buffer
	client := &extractMockClient{responses: []string{"[]", "[]"}}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src0,
		"--source-doc", src1,
		"--source-doc-ref", "only-one-ref",
		"--prompt-template", prompt,
	})
	if err == nil {
		t.Fatal("cmdExtract() with mismatched --source-doc-ref count: want error, got nil")
	}
	// Validation must fail before any LLM call is made.
	if client.calls != 0 {
		t.Errorf("LLM was called %d times; should be 0 for flag-count validation error", client.calls)
	}
}

// TestCmdExtract_MultiDoc_PartialFailure verifies that when the second document
// fails the LLM call, cmdExtract returns an error AND the session file is still
// written with ErrorNote and partial DraftCount populated.
func TestCmdExtract_MultiDoc_PartialFailure(t *testing.T) {
	src0 := writeExtractSourceDoc(t, "First document.")
	src1 := writeExtractSourceDoc(t, "Second document.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")

	// First call succeeds; second exhausts responses and falls through to err.
	client := &extractMockClient{
		responses: []string{`[{"source_span": "First document.", "what_changed": "c0"}]`},
		err:       errors.New("network timeout on second doc"),
	}

	var buf bytes.Buffer
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src0,
		"--source-doc", src1,
		"--prompt-template", prompt,
		"--session-output", sessionPath,
	})
	if err == nil {
		t.Fatal("cmdExtract() partial failure: want error, got nil")
	}

	// Session file must exist and carry ErrorNote + partial DraftCount.
	data, readErr := os.ReadFile(sessionPath)
	if readErr != nil {
		t.Fatalf("session file not written on partial failure: %v", readErr)
	}
	var rec llm.SessionRecord
	if jsonErr := json.Unmarshal(data, &rec); jsonErr != nil {
		t.Fatalf("session file is not valid JSON: %v", jsonErr)
	}
	if rec.ErrorNote == "" {
		t.Error("session file: ErrorNote must be set on partial failure")
	}
	if rec.DraftCount != 1 {
		t.Errorf("session file: DraftCount want 1 (first doc succeeded), got %d", rec.DraftCount)
	}
}

// TestCmdExtract_MultiDoc_SourceDocRef_DefaultsToPath verifies that when
// --source-doc-ref is omitted entirely with multiple --source-doc flags, each
// draft's SourceDocRef defaults to its corresponding source doc path.
func TestCmdExtract_MultiDoc_SourceDocRef_DefaultsToPath(t *testing.T) {
	src0 := writeExtractSourceDoc(t, "Doc 0 content.")
	src1 := writeExtractSourceDoc(t, "Doc 1 content.")
	prompt := writeExtractPromptTemplate(t)
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	const doc0Draft = `[{"source_span": "Doc 0 content.", "what_changed": "c0"}]`
	const doc1Draft = `[{"source_span": "Doc 1 content.", "what_changed": "c1"}]`

	var buf bytes.Buffer
	client := &extractMockClient{responses: []string{doc0Draft, doc1Draft}}
	err := cmdExtract(&buf, client, []string{
		"--source-doc", src0,
		"--source-doc", src1,
		// --source-doc-ref intentionally omitted
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() multi-doc no refs returned error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var drafts []schema.TraceDraft
	if err := json.Unmarshal(data, &drafts); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("want 2 drafts, got %d", len(drafts))
	}
	if drafts[0].SourceDocRef != src0 {
		t.Errorf("draft[0].SourceDocRef: want path %q, got %q", src0, drafts[0].SourceDocRef)
	}
	if drafts[1].SourceDocRef != src1 {
		t.Errorf("draft[1].SourceDocRef: want path %q, got %q", src1, drafts[1].SourceDocRef)
	}
}

// TestCmdExtract_Adapter_HTML verifies that --adapter html converts the source
// HTML file to text before extraction, and that the session record carries
// adapter_name so the mediating act is visible in provenance.
func TestCmdExtract_Adapter_HTML(t *testing.T) {
	// Write an HTML source file (not a plain .md file).
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "source.html")
	htmlContent := `<html><body><h1>Incident</h1><p>The service failed at 09:00.</p></body></html>`
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	prompt := writeExtractPromptTemplate(t)
	outputPath := filepath.Join(dir, "drafts.json")
	sessionPath := filepath.Join(dir, "session.json")

	// Mock client returns one draft.
	const draft = `[{"source_span": "service failed at 09:00", "what_changed": "service failure"}]`
	client := &extractMockClient{response: draft}

	var buf bytes.Buffer
	err := cmdExtract(&buf, client, []string{
		"--adapter", "html",
		"--source-doc", htmlPath,
		"--source-doc-ref", "incident-report",
		"--prompt-template", prompt,
		"--output", outputPath,
		"--session-output", sessionPath,
	})
	if err != nil {
		t.Fatalf("cmdExtract() with --adapter html: want no error, got: %v", err)
	}

	// Session file must carry adapter_name so the conversion act is recorded.
	data, readErr := os.ReadFile(sessionPath)
	if readErr != nil {
		t.Fatalf("session file not written: %v", readErr)
	}
	var rec llm.SessionRecord
	if jsonErr := json.Unmarshal(data, &rec); jsonErr != nil {
		t.Fatalf("session file not valid JSON: %v", jsonErr)
	}
	if rec.Conditions.AdapterName != "html-extractor" {
		t.Errorf("session.Conditions.AdapterName: want %q, got %q", "html-extractor", rec.Conditions.AdapterName)
	}
}

// TestCmdExtract_UnknownAdapter verifies that an unrecognised --adapter value
// returns an error before any LLM call.
func TestCmdExtract_UnknownAdapter(t *testing.T) {
	src := writeExtractSourceDoc(t, "some content")
	prompt := writeExtractPromptTemplate(t)
	client := &extractMockClient{response: "[]"}

	var buf bytes.Buffer
	err := cmdExtract(&buf, client, []string{
		"--adapter", "nosuchformat",
		"--source-doc", src,
		"--prompt-template", prompt,
	})
	if err == nil {
		t.Fatal("cmdExtract() with unknown adapter: want error, got nil")
	}
	if client.calls != 0 {
		t.Errorf("LLM was called %d times; should be 0 for unknown adapter", client.calls)
	}
}
