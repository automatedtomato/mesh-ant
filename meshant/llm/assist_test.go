// assist_test.go tests ParseSpans and RunAssistSession.
//
// All tests follow black-box style (package llm_test). LLM calls are
// intercepted by assistMockClient; no real API calls are made.
package llm_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
)

// assistMockClient implements llm.LLMClient and returns preset responses
// for each successive Complete call. When calls exceed len(responses), the
// last entry is repeated so short response lists still work for multi-span
// sessions.
//
// errs, when non-nil, is consulted per-call before responses. If errs[i] is
// non-nil for call index i, that error is returned. If len(errs) is less than
// the call index, it defaults to nil (no error). This allows tests to model
// sessions where some calls succeed and others fail.
type assistMockClient struct {
	responses []string
	errs      []error // per-call errors; nil slice = no errors
	calls     int
	err       error // if non-nil, returned on every Complete call
}

func (m *assistMockClient) Complete(_ context.Context, _, _ string) (string, error) {
	idx := m.calls
	m.calls++
	// Per-call error takes precedence over global err.
	if idx < len(m.errs) && m.errs[idx] != nil {
		return "", m.errs[idx]
	}
	if m.err != nil {
		return "", m.err
	}
	if len(m.responses) == 0 {
		return `[{"source_span":"fallback"}]`, nil
	}
	ri := idx
	if ri >= len(m.responses) {
		ri = len(m.responses) - 1
	}
	return m.responses[ri], nil
}

// minimalDraft is a JSON object that parseSingleDraft accepts: only source_span
// is required for TraceDraft.Validate().
const minimalDraftJSON = `[{"source_span":"test-span"}]`

// --- ParseSpans ---

// TestParseSpans_JSONArray verifies that a JSON string array is parsed into
// a slice of span strings.
func TestParseSpans_JSONArray(t *testing.T) {
	input := []byte(`["span A","span B","span C"]`)
	got, err := llm.ParseSpans(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"span A", "span B", "span C"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]: want %q, got %q", i, want[i], got[i])
		}
	}
}

// TestParseSpans_NewlineSeparated verifies that newline-separated text is
// split into individual span strings with blank lines dropped.
func TestParseSpans_NewlineSeparated(t *testing.T) {
	input := []byte("span one\nspan two\n\nspan three\n")
	got, err := llm.ParseSpans(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"span one", "span two", "span three"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d, got %d; got %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]: want %q, got %q", i, want[i], got[i])
		}
	}
}

// TestParseSpans_PlainText verifies that plain text with no newlines is
// returned as a single-element slice.
func TestParseSpans_PlainText(t *testing.T) {
	input := []byte("the system was redeployed")
	got, err := llm.ParseSpans(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "the system was redeployed" {
		t.Errorf("want [%q], got %v", "the system was redeployed", got)
	}
}

// TestParseSpans_Empty verifies that empty input returns an error.
func TestParseSpans_Empty(t *testing.T) {
	_, err := llm.ParseSpans([]byte{})
	if err == nil {
		t.Fatal("want error for empty input, got nil")
	}
}

// TestParseSpans_JSONArrayDropsBlanks verifies that blank strings inside a
// JSON array are silently dropped from the result.
func TestParseSpans_JSONArrayDropsBlanks(t *testing.T) {
	input := []byte(`["span1","","span2","   "]`)
	got, err := llm.ParseSpans(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"span1", "span2"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d, got %d; got %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]: want %q, got %q", i, want[i], got[i])
		}
	}
}

// --- RunAssistSession ---

// TestRunAssistSession_InvalidPromptTemplate verifies that a non-existent
// prompt template path causes RunAssistSession to return a non-nil error
// and a well-formed SessionRecord with a non-empty ErrorNote.
func TestRunAssistSession_InvalidPromptTemplate(t *testing.T) {
	client := &assistMockClient{}
	var out bytes.Buffer
	_, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"some span"},
		llm.AssistOptions{
			ModelID:            "test-model",
			PromptTemplatePath: "/nonexistent/prompt.md",
		},
		strings.NewReader(""), &out,
	)
	if err == nil {
		t.Fatal("want error for non-existent prompt template, got nil")
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must not be empty even on early error")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must describe the failure")
	}
}

// TestRunAssistSession_NoSpans verifies that an empty spans slice returns
// 0 drafts, a well-formed SessionRecord, and no error.
func TestRunAssistSession_NoSpans(t *testing.T) {
	client := &assistMockClient{}
	var out bytes.Buffer
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, nil,
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader(""), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts, got %d", len(drafts))
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must not be empty")
	}
	if rec.DraftCount != 0 {
		t.Errorf("DraftCount: want 0, got %d", rec.DraftCount)
	}
}

// TestRunAssistSession_AcceptOneDraft verifies that accepting a single span's
// draft produces 1 result with correct provenance.
func TestRunAssistSession_AcceptOneDraft(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"the span text"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("a\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	d := drafts[0]
	if d.SourceSpan == "" {
		t.Error("SourceSpan must not be empty")
	}
	if d.ID == "" {
		t.Error("ID must not be empty")
	}
	if d.ExtractedBy != "test-model" {
		t.Errorf("ExtractedBy: want %q, got %q", "test-model", d.ExtractedBy)
	}
	if d.ExtractionStage != "weak-draft" {
		t.Errorf("ExtractionStage: want %q, got %q", "weak-draft", d.ExtractionStage)
	}
	if rec.DraftCount != 1 {
		t.Errorf("DraftCount: want 1, got %d", rec.DraftCount)
	}
	if rec.Command != "assist" {
		t.Errorf("Command: want %q, got %q", "assist", rec.Command)
	}
}

// TestRunAssistSession_SkipPreservesDraft verifies that skipping a draft still
// appends the LLM draft to the results (shadow is not absence), with
// disposition "skipped" in the SessionRecord.
func TestRunAssistSession_SkipPreservesDraft(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"skip span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("s\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Skipped draft must still be in output.
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft (skipped preserved), got %d", len(drafts))
	}
	if len(rec.Dispositions) != 1 {
		t.Fatalf("want 1 disposition, got %d", len(rec.Dispositions))
	}
	if rec.Dispositions[0].Action != "skipped" {
		t.Errorf("disposition action: want %q, got %q", "skipped", rec.Dispositions[0].Action)
	}
}

// TestRunAssistSession_QuitReturnsPartial verifies that quitting mid-session
// returns the drafts collected so far without error. The second span is
// not processed.
func TestRunAssistSession_QuitReturnsPartial(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON, minimalDraftJSON}}
	var out bytes.Buffer
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"span-1", "span-2"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("a\nq\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First span accepted; second span not reached after quit.
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft after accept+quit, got %d", len(drafts))
	}
	if rec.DraftCount != 1 {
		t.Errorf("DraftCount: want 1, got %d", rec.DraftCount)
	}
}

// TestRunAssistSession_EOFTreatedAsQuit verifies that an empty reader (immediate
// EOF) causes the session to return 0 drafts and nil error.
func TestRunAssistSession_EOFTreatedAsQuit(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer
	drafts, _, err := llm.RunAssistSession(
		context.Background(), client, []string{"span-eof"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader(""), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error on EOF: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts on EOF, got %d", len(drafts))
	}
}

// TestRunAssistSession_SessionRecord verifies the SessionRecord fields after
// accepting one draft: Command, ID non-empty, DraftCount, DraftIDs, Dispositions.
func TestRunAssistSession_SessionRecord(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"span-rec"},
		llm.AssistOptions{ModelID: "m-001", CriterionRef: "c-001"},
		strings.NewReader("a\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must not be empty")
	}
	if rec.Command != "assist" {
		t.Errorf("Command: want %q, got %q", "assist", rec.Command)
	}
	if rec.DraftCount != 1 {
		t.Errorf("DraftCount: want 1, got %d", rec.DraftCount)
	}
	if len(rec.DraftIDs) != 1 || rec.DraftIDs[0] != drafts[0].ID {
		t.Errorf("DraftIDs: want [%q], got %v", drafts[0].ID, rec.DraftIDs)
	}
	if rec.Conditions.ModelID != "m-001" {
		t.Errorf("Conditions.ModelID: want %q, got %q", "m-001", rec.Conditions.ModelID)
	}
	if rec.Conditions.CriterionRef != "c-001" {
		t.Errorf("Conditions.CriterionRef: want %q, got %q", "c-001", rec.Conditions.CriterionRef)
	}
}

// TestRunAssistSession_LLMError verifies that when the LLM fails on a span,
// the session displays the error and prompts [s]kip / [q]uit. On user skip,
// the session continues and returns nil error. The ErrorNote in the
// SessionRecord records the failure.
func TestRunAssistSession_LLMError(t *testing.T) {
	// First call errors; second call succeeds. User skips the failed span.
	okJSON := `[{"source_span":"ok-span","what_changed":"something"}]`
	client := &assistMockClient{
		errs:      []error{errors.New("network failure")},
		responses: []string{okJSON},
	}
	var out bytes.Buffer
	// "s\n" skips the failed span; "a\n" accepts the second span.
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"err-span", "ok-span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("s\na\n"), &out,
	)
	if err != nil {
		t.Fatalf("want nil error (session continues past LLM failure), got: %v", err)
	}
	// The successful span should produce one draft.
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft (ok-span accepted), got %d", len(drafts))
	}
	if rec.ID == "" {
		t.Error("SessionRecord.ID must not be empty")
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must record the LLM failure")
	}
}

// TestRunAssistSession_LLMRefusal verifies that an LLM refusal on one span
// does not terminate the session — the user is prompted to skip or quit, and
// the session continues to the next span on skip.
func TestRunAssistSession_LLMRefusal(t *testing.T) {
	// First call returns a refusal response; second call returns a valid draft.
	refusalResponse := "I'm sorry, I can't help with that."
	okJSON := `[{"source_span":"next-span","what_changed":"a condition"}]`
	client := &assistMockClient{
		responses: []string{refusalResponse, okJSON},
	}
	var out bytes.Buffer
	// "s\n" skips the refused span; "a\n" accepts the next span.
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"refused-span", "next-span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("s\na\n"), &out,
	)
	if err != nil {
		t.Fatalf("want nil error (session continues past refusal), got: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft (next-span accepted), got %d", len(drafts))
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must record the refusal/parse failure")
	}
}

// TestRunAssistSession_EditDraft verifies that editing a draft produces both
// the original LLM draft (weak-draft) and a derived edited draft (reviewed),
// and that the disposition is "edited".
func TestRunAssistSession_EditDraft(t *testing.T) {
	draftJSON := `[{"source_span":"edit-span","what_changed":"original"}]`
	client := &assistMockClient{responses: []string{draftJSON}}
	var out bytes.Buffer

	// "e\n" = edit; "new description\n" replaces what_changed; 7x "\n" = defaults.
	input := "e\nnew description\n\n\n\n\n\n\n\n"
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"edit-span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader(input), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect 2 drafts: original LLM draft + derived edited draft.
	if len(drafts) != 2 {
		t.Fatalf("want 2 drafts (llm + derived), got %d", len(drafts))
	}

	llmDraft := drafts[0]
	editedDraft := drafts[1]

	// LLM draft must stay as weak-draft.
	if llmDraft.ExtractionStage != "weak-draft" {
		t.Errorf("llmDraft.ExtractionStage: want %q, got %q", "weak-draft", llmDraft.ExtractionStage)
	}
	// Edited draft must be derived from LLM draft.
	if editedDraft.DerivedFrom != llmDraft.ID {
		t.Errorf("editedDraft.DerivedFrom: want %q, got %q", llmDraft.ID, editedDraft.DerivedFrom)
	}
	if editedDraft.ExtractionStage != "reviewed" {
		t.Errorf("editedDraft.ExtractionStage: want %q, got %q", "reviewed", editedDraft.ExtractionStage)
	}
	if editedDraft.ExtractedBy != "meshant-assist" {
		t.Errorf("editedDraft.ExtractedBy: want %q, got %q", "meshant-assist", editedDraft.ExtractedBy)
	}
	if editedDraft.WhatChanged != "new description" {
		t.Errorf("editedDraft.WhatChanged: want %q, got %q", "new description", editedDraft.WhatChanged)
	}

	// Disposition must be "edited".
	if len(rec.Dispositions) != 1 || rec.Dispositions[0].Action != "edited" {
		t.Errorf("disposition: want [{edited}], got %v", rec.Dispositions)
	}
}

// TestRunAssistSession_UncertaintyNoteAppended verifies that the framework
// uncertainty note is appended to every LLM draft produced by the session,
// consistent with F.1 convention D3.
func TestRunAssistSession_UncertaintyNoteAppended(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer
	drafts, _, err := llm.RunAssistSession(
		context.Background(), client, []string{"uncertainty span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("a\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	if !strings.Contains(drafts[0].UncertaintyNote, "unverified") {
		t.Errorf("UncertaintyNote: expected framework note, got %q", drafts[0].UncertaintyNote)
	}
}

// TestRunAssistSession_PromptContainsActions verifies that the session prompt
// includes all four action options so the user knows what to type.
func TestRunAssistSession_PromptContainsActions(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer
	_, _, err := llm.RunAssistSession(
		context.Background(), client, []string{"prompt span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("q\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := out.String()
	for _, action := range []string{"[a]ccept", "[e]dit", "[s]kip", "[q]uit"} {
		if !strings.Contains(prompt, action) {
			t.Errorf("prompt should contain %q; got:\n%s", action, prompt)
		}
	}
}

// TestRunAssistSession_SessionRefStampedOnDraft verifies that every LLM draft
// produced by RunAssistSession has SessionRef == SessionRecord.ID (FM4).
func TestRunAssistSession_SessionRefStampedOnDraft(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"sess-ref span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("a\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) == 0 {
		t.Fatal("expected at least 1 draft")
	}
	if drafts[0].SessionRef != rec.ID {
		t.Errorf("SessionRef: want %q (SessionRecord.ID), got %q", rec.ID, drafts[0].SessionRef)
	}
}

// TestRunAssistSession_EditThenEOFDispositionAbandoned verifies that an EOF
// mid-edit records disposition "abandoned" (not "skipped"), so provenance
// auditors can distinguish an edit-interrupted-by-EOF from a deliberate skip.
func TestRunAssistSession_EditThenEOFDispositionAbandoned(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer

	// "e\n" enters edit mode; no further input → EOF before what_changed.
	_, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"span-abandoned"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("e\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.Dispositions) != 1 {
		t.Fatalf("want 1 disposition, got %d", len(rec.Dispositions))
	}
	if rec.Dispositions[0].Action != "abandoned" {
		t.Errorf("disposition action: want %q, got %q", "abandoned", rec.Dispositions[0].Action)
	}
}

// TestParseSpans_WhitespaceOnlyLines verifies that lines containing only
// whitespace are treated as blank and dropped from the result.
func TestParseSpans_WhitespaceOnlyLines(t *testing.T) {
	input := []byte("span A\n   \nspan B\n\t\nspan C\n")
	got, err := llm.ParseSpans(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"span A", "span B", "span C"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d, got %d; got %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]: want %q, got %q", i, want[i], got[i])
		}
	}
}

// TestRunAssistSession_JSONObjectResponse verifies that parseSingleDraft
// handles an LLM response that is a JSON object (not a JSON array). This
// exercises the object-fallback parse path in parseSingleDraft.
func TestRunAssistSession_JSONObjectResponse(t *testing.T) {
	// LLM returns a bare JSON object instead of the more common array form.
	objectJSON := `{"source_span":"object-form span","what_changed":"from object"}`
	client := &assistMockClient{responses: []string{objectJSON}}
	var out bytes.Buffer
	drafts, _, err := llm.RunAssistSession(
		context.Background(), client, []string{"object-form span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("a\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	if drafts[0].SourceSpan != "object-form span" {
		t.Errorf("SourceSpan: want %q, got %q", "object-form span", drafts[0].SourceSpan)
	}
}

// TestRunAssistSession_MalformedLLMResponse verifies that a response with no
// JSON at all does not terminate the session — the parse failure is accumulated
// in SessionRecord.ErrorNote and the user is offered [s]kip / [q]uit.
// EOF on the prompt is treated as quit: nil error is returned.
func TestRunAssistSession_MalformedLLMResponse(t *testing.T) {
	// Plain text with no JSON structure — no '[' or '{'.
	plainText := "I cannot help with that request."
	client := &assistMockClient{responses: []string{plainText}}
	var out bytes.Buffer
	// Empty reader → EOF on the s/q prompt → session quits with nil error.
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"some span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader(""), &out,
	)
	if err != nil {
		t.Fatalf("want nil error (parse failures accumulate in ErrorNote), got: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts (no successful parse), got %d", len(drafts))
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on parse failure")
	}
}

// TestRunAssistSession_MissingSourceSpan verifies that a draft missing
// source_span fails validation without terminating the session. The failure
// is accumulated in SessionRecord.ErrorNote; nil error is returned on EOF.
func TestRunAssistSession_MissingSourceSpan(t *testing.T) {
	// LLM returns a draft with no source_span — schema validation should fail.
	noSpanJSON := `[{"what_changed":"something happened"}]`
	client := &assistMockClient{responses: []string{noSpanJSON}}
	var out bytes.Buffer
	// Empty reader → EOF on the s/q prompt → session quits with nil error.
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"some span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader(""), &out,
	)
	if err != nil {
		t.Fatalf("want nil error (validation failures accumulate in ErrorNote), got: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts (no successful parse), got %d", len(drafts))
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must be set on validation failure")
	}
}

// TestRunAssistSession_UncertaintyNoteConcatenation verifies that the framework
// uncertainty note is APPENDED to (not replaced by) an LLM-supplied note,
// preserving both signals per F.1 decision D3.
func TestRunAssistSession_UncertaintyNoteConcatenation(t *testing.T) {
	// LLM response includes its own uncertainty_note.
	withNoteJSON := `[{"source_span":"note-span","uncertainty_note":"LLM is unsure"}]`
	client := &assistMockClient{responses: []string{withNoteJSON}}
	var out bytes.Buffer
	drafts, _, err := llm.RunAssistSession(
		context.Background(), client, []string{"note-span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("a\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	note := drafts[0].UncertaintyNote
	if !strings.Contains(note, "LLM is unsure") {
		t.Errorf("UncertaintyNote: LLM note must be preserved; got %q", note)
	}
	if !strings.Contains(note, "unverified") {
		t.Errorf("UncertaintyNote: framework note must be appended; got %q", note)
	}
}

// TestRunAssistSession_UnknownActionReprompts verifies that an unrecognised
// input is rejected with a re-prompt message and the session resumes normally.
func TestRunAssistSession_UnknownActionReprompts(t *testing.T) {
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	var out bytes.Buffer
	// "x\n" = unknown; "a\n" = accept after re-prompt.
	drafts, _, err := llm.RunAssistSession(
		context.Background(), client, []string{"reprompt span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("x\na\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft after reprompt+accept, got %d", len(drafts))
	}
	if !strings.Contains(out.String(), "unknown") {
		t.Errorf("output should contain re-prompt message; got:\n%s", out.String())
	}
}

// TestRunAssistSession_MultipleErrorsAccumulate verifies that consecutive
// LLM failures on different spans each append an entry to ErrorNote, producing
// a semicolon-joined string with two distinct error records.
// This exercises the splitErrNotes / joinErrNotes accumulation path directly.
func TestRunAssistSession_MultipleErrorsAccumulate(t *testing.T) {
	// Both calls fail — two ErrorNote entries should be accumulated.
	client := &assistMockClient{
		errs: []error{
			errors.New("failure on span 1"),
			errors.New("failure on span 2"),
		},
	}
	var out bytes.Buffer
	// Skip both failed spans, then EOF ends the session.
	_, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"span-1", "span-2"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("s\ns\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ErrorNote should contain both failure messages.
	if !strings.Contains(rec.ErrorNote, "failure on span 1") {
		t.Errorf("ErrorNote: expected first failure, got %q", rec.ErrorNote)
	}
	if !strings.Contains(rec.ErrorNote, "failure on span 2") {
		t.Errorf("ErrorNote: expected second failure, got %q", rec.ErrorNote)
	}
	// The two entries should be separated by "; ".
	if !strings.Contains(rec.ErrorNote, "; ") {
		t.Errorf("ErrorNote: expected semicolon separator between entries, got %q", rec.ErrorNote)
	}
}

// TestRunAssistSession_QuitOnErrorPrompt verifies that typing "q" on the
// [s]kip / [q]uit error prompt terminates the session without processing
// further spans.
func TestRunAssistSession_QuitOnErrorPrompt(t *testing.T) {
	okJSON := `[{"source_span":"second-span","what_changed":"something"}]`
	client := &assistMockClient{
		errs:      []error{errors.New("network failure")},
		responses: []string{okJSON},
	}
	var out bytes.Buffer
	// "q\n" on the error prompt: session terminates before processing second span.
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"err-span", "second-span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader("q\n"), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No drafts — session quit before any success.
	if len(drafts) != 0 {
		t.Errorf("want 0 drafts after quit on error prompt, got %d", len(drafts))
	}
	// The LLM was only called once (the second span was never reached).
	if client.calls != 1 {
		t.Errorf("want 1 LLM call (second span not reached), got %d", client.calls)
	}
	if rec.ErrorNote == "" {
		t.Error("SessionRecord.ErrorNote must record the failure from span 1")
	}
}

// TestRunAssistSession_EditDraftSessionRef verifies that the derived (edited)
// draft carries SessionRef == SessionRecord.ID — session provenance must
// propagate through the DeriveEdited path (FM4).
func TestRunAssistSession_EditDraftSessionRef(t *testing.T) {
	draftJSON := `[{"source_span":"sessref-edit-span"}]`
	client := &assistMockClient{responses: []string{draftJSON}}
	var out bytes.Buffer

	// "e\n" + 8x "\n" accepts all defaults through RunEditFlow's 8 fields.
	input := "e\n\n\n\n\n\n\n\n\n"
	drafts, rec, err := llm.RunAssistSession(
		context.Background(), client, []string{"sessref-edit-span"},
		llm.AssistOptions{ModelID: "test-model"},
		strings.NewReader(input), &out,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("want 2 drafts (llm + derived), got %d", len(drafts))
	}
	// Both the LLM draft and the derived (edited) draft must carry the session ref.
	for i, d := range drafts {
		if d.SessionRef != rec.ID {
			t.Errorf("drafts[%d].SessionRef: want %q, got %q", i, rec.ID, d.SessionRef)
		}
	}
}

// --- PromptHash tests ---

// TestRunAssistSession_PromptHash_Set verifies that RunAssistSession populates
// Conditions.PromptHash with a 16-character hex string when a prompt template
// path is provided.
func TestRunAssistSession_PromptHash_Set(t *testing.T) {
	prompt := writePromptTemplate(t)
	opts := llm.AssistOptions{
		ModelID:            "claude-sonnet-4-6",
		PromptTemplatePath: prompt,
		SourceDocRef:       "test-doc",
	}
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	// Single span; immediately accept.
	input := strings.NewReader("a\n")
	var out strings.Builder

	_, rec, err := llm.RunAssistSession(context.Background(), client, []string{"test span"}, opts, input, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := rec.Conditions.PromptHash
	if len(h) != 16 {
		t.Errorf("Conditions.PromptHash: want 16-char hex, got %q (len=%d)", h, len(h))
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character %q in PromptHash %q", c, h)
		}
	}
}

// TestRunAssistSession_PromptHash_EmptyPath verifies that when no prompt template
// path is provided, Conditions.PromptHash is empty (not an error).
func TestRunAssistSession_PromptHash_EmptyPath(t *testing.T) {
	opts := llm.AssistOptions{
		ModelID:      "claude-sonnet-4-6",
		SourceDocRef: "test-doc",
		// PromptTemplatePath intentionally empty.
	}
	client := &assistMockClient{responses: []string{minimalDraftJSON}}
	input := strings.NewReader("a\n")
	var out strings.Builder

	_, rec, err := llm.RunAssistSession(context.Background(), client, []string{"test span"}, opts, input, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Conditions.PromptHash != "" {
		t.Errorf("Conditions.PromptHash: want empty for no-template path, got %q", rec.Conditions.PromptHash)
	}
}
