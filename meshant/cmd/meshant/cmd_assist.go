package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// maxSpansBytes caps the spans file size at 4 MiB. Spans files are plain text
// or JSON string arrays — 4 MiB is generous for any realistic analysis session.
// Mirroring the size-cap pattern from LoadPromptTemplate and loadCriterionFile.
const maxSpansBytes = 4 * 1024 * 1024

// defaultAssistPrompt is the path to the bundled assist prompt template,
// relative to the repository root. Used when --prompt-template is not supplied.
const defaultAssistPrompt = "data/prompts/assist_pass.md"

// defaultAssistModel is the model ID used when --model is not supplied.
const defaultAssistModel = "claude-sonnet-4-6"

// cmdAssist implements the "assist" subcommand.
//
// It reads a spans file, calls the LLM once per span to produce a candidate
// TraceDraft, presents each draft to the user for accept/edit/skip/quit
// decisions, and writes the resulting drafts plus a SessionRecord to disk.
//
// The LLM client is injected via the client parameter. When client is nil,
// cmdAssist constructs a real AnthropicClient from the environment (reading
// MESHANT_LLM_API_KEY or ANTHROPIC_API_KEY). Tests inject a mock client.
//
// Session output defaulting (mirrors cmdExtract):
//   - if --session-output is provided: write to that path
//   - if --output is a file: write to <output>.session.json
//   - if output is stdout: write to session_<compact-ISO8601-timestamp>.json in cwd
//
// The interactive prompts are written to out (w). The user's responses are
// read from in (os.Stdin in production; a strings.Reader in tests).
//
// Flags:
//   - --spans-file <path>          path to spans file (required); newline-separated
//     text, JSON string array, or single line
//   - --prompt-template <path>     path to assist prompt template (default: data/prompts/assist_pass.md)
//   - --model <id>                 LLM model ID (default: claude-sonnet-4-6)
//   - --source-doc-ref <ref>       document reference string for provenance (optional)
//   - --criterion-file <path>      optional criterion JSON file (CriterionRef provenance)
//   - --output <file>              write TraceDraft JSON to file (default: stdout)
//   - --session-output <file>      write SessionRecord JSON to file (see defaulting above)
func cmdAssist(w io.Writer, client llm.LLMClient, in io.Reader, args []string) error {
	fs := flag.NewFlagSet("assist", flag.ContinueOnError)

	var spansFile string
	fs.StringVar(&spansFile, "spans-file", "", "path to spans file (required)")

	var promptTemplate string
	fs.StringVar(&promptTemplate, "prompt-template", defaultAssistPrompt, "path to assist prompt template")

	var modelID string
	fs.StringVar(&modelID, "model", defaultAssistModel, "LLM model ID")

	var sourceDocRef string
	fs.StringVar(&sourceDocRef, "source-doc-ref", "", "document reference string for provenance (optional)")

	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to criterion JSON file (optional)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write TraceDraft JSON to file (default: stdout)")

	var sessionOutputPath string
	fs.StringVar(&sessionOutputPath, "session-output", "", "write SessionRecord JSON to file (see default rules)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if spansFile == "" {
		return fmt.Errorf("assist: --spans-file is required\n\nUsage: meshant assist --spans-file <path> [--prompt-template <path>] [--model <id>] [--source-doc-ref <ref>] [--criterion-file <path>] [--output <file>] [--session-output <file>]")
	}

	// Derive sessionOutputPath from outputPath when not explicitly provided.
	if sessionOutputPath == "" {
		if outputPath != "" {
			sessionOutputPath = outputPath + ".session.json"
		} else {
			sessionOutputPath = "session_" + time.Now().UTC().Format("20060102T150405Z") + ".json"
		}
	}

	// Resolve criterion reference string from optional --criterion-file.
	var criterionRef string
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("assist: %w", err)
		}
		criterionRef = c.Name
	}

	// Read spans file with a size cap — mirrors the pattern used by
	// LoadPromptTemplate and loadCriterionFile to prevent unbounded reads.
	spansData, err := readSpansFile(spansFile)
	if err != nil {
		return fmt.Errorf("assist: %w", err)
	}
	spans, err := llm.ParseSpans(spansData)
	if err != nil {
		return fmt.Errorf("assist: %w", err)
	}

	// Construct real LLM client from environment when none is injected.
	// Tests inject a mock client so no env var is needed.
	if client == nil {
		c, err := llm.NewAnthropicClient(modelID)
		if err != nil {
			return fmt.Errorf("assist: %w", err)
		}
		client = c
	}

	opts := llm.AssistOptions{
		ModelID:            modelID,
		InputPath:          spansFile,
		PromptTemplatePath: promptTemplate,
		CriterionRef:       criterionRef,
		SourceDocRef:       sourceDocRef,
		OutputPath:         outputPath,
		SessionOutputPath:  sessionOutputPath,
	}

	drafts, rec, err := llm.RunAssistSession(context.Background(), client, spans, opts, in, w)

	// Always write the SessionRecord before returning — even on error.
	// When the session succeeded but the session write fails, that is a hard
	// error: the provenance record is lost.
	sessionWriteErr := writeSessionRecord(sessionOutputPath, rec)
	if err != nil {
		// Primary session error takes precedence; demote session-write failure
		// to a warning so the session error is not masked.
		if sessionWriteErr != nil {
			fmt.Fprintf(w, "assist: warning: could not write session record to %q: %v\n", sessionOutputPath, sessionWriteErr)
		}
		return fmt.Errorf("assist: %w", err)
	}
	if sessionWriteErr != nil {
		// Session succeeded but record lost — provenance is broken.
		return fmt.Errorf("assist: write session record: %w", sessionWriteErr)
	}

	// Write TraceDraft JSON array to output destination.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("assist: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(drafts); err != nil {
		return fmt.Errorf("assist: encode output: %w", err)
	}

	// Print a provenance summary to w (always stdout).
	summary := loader.SummariseDrafts(drafts)
	if err := loader.PrintDraftSummary(w, summary); err != nil {
		return fmt.Errorf("assist: %w", err)
	}

	// Confirm file output paths.
	if err := confirmOutput(w, outputPath); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "wrote session record to %s\n", sessionOutputPath)
	return err
}

// readSpansFile opens the spans file at path and reads up to maxSpansBytes.
// Returns a clear error if the file is missing or exceeds the cap. Mirrors
// the size-cap pattern used by LoadPromptTemplate and loadCriterionFile.
func readSpansFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read spans file %q: %w", path, err)
	}
	defer f.Close()

	// Read one byte beyond the cap to detect oversized files.
	limited := io.LimitReader(f, int64(maxSpansBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read spans file %q: %w", path, err)
	}
	if len(data) > maxSpansBytes {
		return nil, fmt.Errorf("spans file %q exceeds %d bytes", path, maxSpansBytes)
	}
	return data, nil
}
